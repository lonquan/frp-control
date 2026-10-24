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

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	runtimepprof "runtime/pprof"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/fatedier/frp/pkg/trafficcapture"
)

type SupervisorStatus struct {
	State       string    `json:"state"`
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"startedAt"`
	RestartedAt time.Time `json:"restartedAt,omitempty"`
	LastError   string    `json:"lastError,omitempty"`
	Rollback    bool      `json:"rollback,omitempty"`
}

type RuntimeOptions struct {
	Role               string
	DataDir            string
	ConfigRoot         string
	ConfigPath         string
	SessionTTL         time.Duration
	SecureCookie       bool
	Restart            func(context.Context) error
	SupervisorStatus   func() SupervisorStatus
	Metrics            func() any
	ReloadCertificates func()
	CertificateRefs    func(id, certPath, keyPath string) []string
}

type Runtime struct {
	role       string
	dataDir    string
	auth       *Authenticator
	config     *ConfigManager
	restart    func(context.Context) error
	supervisor func() SupervisorStatus
	metrics    func() any
	certs      *CertificateManager
	captures   *trafficcapture.Manager

	settingsMu sync.Mutex
	settings   adminSettings
	profileMu  sync.Mutex
}

type adminSettings struct {
	PprofEnabled bool `json:"pprofEnabled"`
}

func NewRuntime(opts RuntimeOptions) (*Runtime, error) {
	if opts.DataDir == "" {
		return nil, errors.New("FRP_DATA_DIR is required")
	}
	auth, err := NewAuthenticator(AuthOptions{
		DataDir:      opts.DataDir,
		Role:         opts.Role,
		SessionTTL:   opts.SessionTTL,
		SecureCookie: opts.SecureCookie,
	})
	if err != nil {
		return nil, err
	}
	configManager, err := NewConfigManager(opts.ConfigRoot, opts.ConfigPath, opts.Role)
	if err != nil {
		return nil, err
	}
	runtime := &Runtime{
		role:       opts.Role,
		dataDir:    opts.DataDir,
		auth:       auth,
		config:     configManager,
		restart:    opts.Restart,
		supervisor: opts.SupervisorStatus,
		metrics:    opts.Metrics,
		settings:   loadSettings(opts.DataDir),
	}
	if opts.Role == "client" {
		certs, err := NewCertificateManager(opts.DataDir, opts.ReloadCertificates, opts.CertificateRefs)
		if err != nil {
			return nil, err
		}
		runtime.certs = certs
		certs.StartRenewal(context.Background())
		captures, err := trafficcapture.NewManager(opts.DataDir)
		if err != nil {
			return nil, err
		}
		runtime.captures = captures
		trafficcapture.SetDefault(captures)
	}
	return runtime, nil
}

func (r *Runtime) Certificates() *CertificateManager {
	return r.certs
}

func (r *Runtime) Auth() *Authenticator {
	return r.auth
}

func (r *Runtime) Config() *ConfigManager {
	return r.config
}

func (r *Runtime) Captures() *trafficcapture.Manager {
	return r.captures
}

func (r *Runtime) RegisterPublic(router *mux.Router) {
	router.HandleFunc("/api/v2/auth/login", r.auth.Login).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/auth/session", r.auth.Session).Methods(http.MethodGet)
}

func (r *Runtime) RegisterPrivate(router *mux.Router) {
	router.HandleFunc("/api/v2/auth/logout", r.auth.Logout).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/auth/password", r.auth.ChangePassword).Methods(http.MethodPost)

	router.HandleFunc("/api/v2/config/tree", r.config.HandleTree).Methods(http.MethodGet)
	router.HandleFunc("/api/v2/config/file", r.config.HandleFile).Methods(http.MethodGet, http.MethodPut, http.MethodDelete)
	router.HandleFunc("/api/v2/config/file/create", r.config.HandleCreate).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/config/file/rename", r.config.HandleRename).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/config/validate", r.config.HandleValidate).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/config/history", r.config.HandleHistory).Methods(http.MethodGet, http.MethodPost)
	if r.certs != nil {
		r.certs.RegisterRoutes(router)
	}
	if r.captures != nil {
		r.captures.RegisterRoutes(router)
	}

	router.HandleFunc("/api/v2/system/restart", r.handleRestart).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/system/supervisor", r.handleSupervisor).Methods(http.MethodGet)
	router.HandleFunc("/api/v2/observability/summary", r.handleMetrics).Methods(http.MethodGet)
	router.HandleFunc("/api/v2/observability/runtime", r.handleRuntimeMetrics).Methods(http.MethodGet)

	router.HandleFunc("/api/v2/diagnostics/settings", r.handleDiagnosticSettings).Methods(http.MethodGet, http.MethodPut)
	router.HandleFunc("/api/v2/diagnostics/pprof/goroutine", r.handleGoroutine).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/diagnostics/pprof/heap", r.handleHeap).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/diagnostics/pprof/cpu", r.handleCPU).Methods(http.MethodPost)
	router.HandleFunc("/api/v2/captures/{id}/reveal", r.handleCaptureReveal).Methods(http.MethodPost)
}

func (r *Runtime) Wrap(next http.Handler) http.Handler {
	return r.auth.Middleware(next)
}

func (r *Runtime) handleRestart(w http.ResponseWriter, req *http.Request) {
	if r.restart == nil {
		writeError(w, http.StatusConflict, "当前运行环境未启用 Supervisor，无法重启服务")
		return
	}
	go func() {
		time.Sleep(250 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = r.restart(ctx)
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted": true,
		"message":  "服务即将重启，请稍候刷新页面",
	})
}

func (r *Runtime) handleSupervisor(w http.ResponseWriter, _ *http.Request) {
	if r.supervisor == nil {
		writeJSON(w, http.StatusOK, SupervisorStatus{State: "unmanaged"})
		return
	}
	writeJSON(w, http.StatusOK, r.supervisor())
}

func (r *Runtime) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	if r.metrics == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, r.metrics())
}

func (r *Runtime) handleRuntimeMetrics(w http.ResponseWriter, _ *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	writeJSON(w, http.StatusOK, map[string]any{
		"goVersion":       runtime.Version(),
		"goroutines":      runtime.NumGoroutine(),
		"heapAlloc":       mem.HeapAlloc,
		"heapInuse":       mem.HeapInuse,
		"sys":             mem.Sys,
		"gcCount":         mem.NumGC,
		"lastGCUnixNano":  mem.LastGC,
		"processUptime":   time.Since(processStart).Seconds(),
		"numCPU":          runtime.NumCPU(),
		"allocationBytes": mem.TotalAlloc,
	})
}

func (r *Runtime) handleCaptureReveal(w http.ResponseWriter, req *http.Request) {
	id := mux.Vars(req)["id"]
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少捕获记录 ID")
		return
	}
	entry := map[string]any{
		"at":       time.Now().UTC(),
		"action":   "capture.reveal_sensitive",
		"capture":  id,
		"username": r.auth.UsernameFromRequest(req),
		"clientIp": clientIP(req),
	}
	b, err := json.Marshal(entry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	auditDir := filepath.Join(r.dataDir, "admin")
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	file, err := os.OpenFile(filepath.Join(auditDir, "audit.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()
	if _, err := file.Write(append(b, '\n')); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"recorded": true})
}

func (r *Runtime) handleDiagnosticSettings(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPut {
		var body adminSettings
		if err := decodeJSON(w, req, &body); err != nil {
			return
		}
		r.settingsMu.Lock()
		r.settings = body
		err := saveSettings(r.dataDir, body)
		r.settingsMu.Unlock()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	r.settingsMu.Lock()
	settings := r.settings
	r.settingsMu.Unlock()
	writeJSON(w, http.StatusOK, settings)
}

func (r *Runtime) pprofEnabled() bool {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	return r.settings.PprofEnabled
}

func (r *Runtime) handleGoroutine(w http.ResponseWriter, req *http.Request) {
	if !r.requirePprof(w) {
		return
	}
	debug, _ := strconv.Atoi(req.URL.Query().Get("debug"))
	if debug != 2 {
		debug = 1
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=goroutine.txt")
	if err := runtimepprof.Lookup("goroutine").WriteTo(w, debug); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (r *Runtime) handleHeap(w http.ResponseWriter, _ *http.Request) {
	if !r.requirePprof(w) {
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=heap.pprof")
	if err := runtimepprof.WriteHeapProfile(w); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (r *Runtime) handleCPU(w http.ResponseWriter, req *http.Request) {
	if !r.requirePprof(w) {
		return
	}
	seconds, _ := strconv.Atoi(req.URL.Query().Get("seconds"))
	if seconds < 1 || seconds > 60 {
		seconds = 15
	}
	if !r.profileMu.TryLock() {
		writeError(w, http.StatusConflict, "已有 CPU profile 正在执行")
		return
	}
	defer r.profileMu.Unlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=cpu.pprof")
	if err := runtimepprof.StartCPUProfile(w); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-req.Context().Done():
	}
	runtimepprof.StopCPUProfile()
}

func (r *Runtime) requirePprof(w http.ResponseWriter) bool {
	if !r.pprofEnabled() {
		writeError(w, http.StatusForbidden, "诊断功能未启用")
		return false
	}
	return true
}

func loadSettings(dataDir string) adminSettings {
	settings := adminSettings{}
	b, err := os.ReadFile(filepath.Join(dataDir, "admin", "settings.json"))
	if err == nil {
		_ = json.Unmarshal(b, &settings)
	}
	return settings
}

func saveSettings(dataDir string, settings adminSettings) error {
	path := filepath.Join(dataDir, "admin", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, b)
}

var processStart = time.Now()

func (s SupervisorStatus) Message() string {
	if s.LastError != "" {
		return fmt.Sprintf("%s: %s", s.State, s.LastError)
	}
	return s.State
}
