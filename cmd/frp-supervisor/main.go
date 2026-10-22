// Copyright 2026 The frp Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type status struct {
	State       string    `json:"state"`
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"startedAt"`
	RestartedAt time.Time `json:"restartedAt,omitempty"`
	LastError   string    `json:"lastError,omitempty"`
	Rollback    bool      `json:"rollback,omitempty"`
}

type supervisor struct {
	mu        sync.Mutex
	child     *exec.Cmd
	socket    string
	readyFile string
	config    string
	command   []string
	status    status
	stopping  bool
	restart   chan string
	exited    chan struct{}
}

func main() {
	var (
		socket    string
		readyFile string
		config    string
		upstream  string
	)
	flag.StringVar(&socket, "socket", "/run/frp/supervisor.sock", "Unix socket path")
	flag.StringVar(&readyFile, "ready-file", "/run/frp/ready", "child readiness marker")
	flag.StringVar(&config, "config", "", "configuration file for startup rollback")
	flag.StringVar(&upstream, "upstream", "", "managed command, for example '/usr/bin/frps -c /etc/frp/frps.toml'")
	flag.Parse()
	if strings.TrimSpace(upstream) == "" {
		log.Fatal("--upstream is required")
	}
	parts := strings.Fields(upstream)
	if len(parts) == 0 {
		log.Fatal("invalid --upstream")
	}
	s := &supervisor{
		socket:    socket,
		readyFile: readyFile,
		config:    config,
		command:   parts,
		restart:   make(chan string, 4),
		exited:    make(chan struct{}, 4),
		status: status{
			State: "starting",
		},
	}
	if err := s.run(); err != nil {
		log.Fatal(err)
	}
}

func (s *supervisor) run() error {
	if err := os.MkdirAll(filepath.Dir(s.socket), 0o700); err != nil {
		return err
	}
	_ = os.Remove(s.socket)
	listener, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	_ = os.Chmod(s.socket, 0o600)
	go s.serveHTTP(listener)

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	if err := s.startChild(false); err != nil {
		log.Printf("initial startup failed: %v", err)
	}
	backoff := time.Second
	for {
		select {
		case <-signals:
			s.shutdown()
			return nil
		case reason := <-s.restart:
			s.status.RestartedAt = time.Now()
			s.status.State = "restarting"
			s.stopChild()
			if err := s.startChild(false); err != nil {
				log.Printf("restart failed: %v", err)
				s.status.LastError = err.Error()
			}
			s.drainExited()
			log.Printf("restart completed: %s", reason)
		case <-s.exited:
			if s.stopping {
				continue
			}
			s.status.State = "unexpected-exit"
			time.Sleep(backoff)
			if backoff < 20*time.Second {
				backoff *= 2
			}
			if err := s.startChild(false); err != nil {
				s.status.LastError = err.Error()
			}
		case <-time.After(24 * time.Hour):
		}
	}
}

func (s *supervisor) drainExited() {
	for {
		select {
		case <-s.exited:
		default:
			return
		}
	}
}

func (s *supervisor) startChild(rollback bool) error {
	_ = os.Remove(s.readyFile)
	cmd := exec.Command(s.command[0], s.command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"FRP_SUPERVISOR_SOCKET="+s.socket,
		"FRP_READY_FILE="+s.readyFile,
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	exitCh := make(chan struct{}, 1)
	s.mu.Lock()
	s.child = cmd
	s.status = status{
		State:       "starting",
		PID:         cmd.Process.Pid,
		StartedAt:   time.Now(),
		RestartedAt: s.status.RestartedAt,
		Rollback:    rollback,
	}
	s.mu.Unlock()
	go s.waitChild(cmd, exitCh)

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(s.readyFile); err == nil {
			s.mu.Lock()
			s.status.State = "running"
			s.status.LastError = ""
			s.mu.Unlock()
			s.saveLastGood()
			log.Printf("managed child ready, pid=%d", cmd.Process.Pid)
			return nil
		}
		select {
		case <-exitCh:
			err := errors.New("child exited before becoming ready")
			if s.config != "" && !rollback && s.restoreLastGood() {
				log.Printf("startup failed, restored last-known-good: %v", err)
				return s.startChild(true)
			}
			return err
		case <-time.After(250 * time.Millisecond):
		}
	}
	if s.config != "" && !rollback && s.restoreLastGood() {
		s.stopChild()
		log.Printf("startup timeout, restored last-known-good")
		return s.startChild(true)
	}
	return errors.New("child readiness timeout")
}

func (s *supervisor) waitChild(cmd *exec.Cmd, exitCh chan<- struct{}) {
	err := cmd.Wait()
	s.mu.Lock()
	if s.child == cmd {
		s.child = nil
		s.status.PID = 0
		if err != nil {
			s.status.LastError = err.Error()
		}
	}
	s.mu.Unlock()
	select {
	case exitCh <- struct{}{}:
	default:
	}
	select {
	case s.exited <- struct{}{}:
	default:
	}
}

func (s *supervisor) stopChild() {
	s.mu.Lock()
	cmd := s.child
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		current := s.child
		s.mu.Unlock()
		if current == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
}

func (s *supervisor) shutdown() {
	s.stopping = true
	s.mu.Lock()
	s.status.State = "stopping"
	s.mu.Unlock()
	s.stopChild()
	_ = os.Remove(s.socket)
}

func (s *supervisor) saveLastGood() {
	if s.config == "" {
		return
	}
	b, err := os.ReadFile(s.config)
	if err != nil {
		return
	}
	_ = os.WriteFile(s.config+".last-good", b, 0o600)
}

func (s *supervisor) restoreLastGood() bool {
	if s.config == "" {
		return false
	}
	b, err := os.ReadFile(s.config + ".last-good")
	if err != nil {
		return false
	}
	failedPath := s.config + ".failed-" + time.Now().Format("20060102T150405")
	if current, err := os.ReadFile(s.config); err == nil {
		_ = os.WriteFile(failedPath, current, 0o600)
	}
	return os.WriteFile(s.config, b, 0o600) == nil
}

func (s *supervisor) serveHTTP(listener net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		select {
		case s.restart <- "admin request":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"accepted":true}`))
		default:
			http.Error(w, "restart already queued", http.StatusConflict)
		}
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		current := s.status
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(current)
	})
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("supervisor socket server exited: %v", err)
	}
}
