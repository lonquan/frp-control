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
	"net"
	"net/http"
	"os"
	"time"
)

func SupervisorSocketPath() string {
	return os.Getenv("FRP_SUPERVISOR_SOCKET")
}

func RequestSupervisorRestart(ctx context.Context) error {
	path := SupervisorSocketPath()
	if path == "" {
		return errors.New("FRP_SUPERVISOR_SOCKET is not configured")
	}
	client := unixHTTPClient(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://supervisor/restart", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return errors.New("supervisor rejected restart request")
	}
	return nil
}

func ReadSupervisorStatus() SupervisorStatus {
	path := SupervisorSocketPath()
	if path == "" {
		return SupervisorStatus{State: "unmanaged"}
	}
	client := unixHTTPClient(path)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://supervisor/status", nil)
	if err != nil {
		return SupervisorStatus{State: "unavailable", LastError: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return SupervisorStatus{State: "unavailable", LastError: err.Error()}
	}
	defer resp.Body.Close()
	var status SupervisorStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return SupervisorStatus{State: "unavailable", LastError: err.Error()}
	}
	return status
}

func unixHTTPClient(socketPath string) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}
}
