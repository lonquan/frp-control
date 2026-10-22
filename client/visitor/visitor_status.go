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

package visitor

import v1 "github.com/fatedier/frp/pkg/config/v1"

const (
	PhaseStarting   = "starting"
	PhaseRunning    = "running"
	PhaseStartError = "start error"
	PhaseClosed     = "closed"
)

// WorkingStatus contains safe, API-facing runtime information for a visitor.
// SecretKey and other authentication material are intentionally omitted.
type WorkingStatus struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Err        string `json:"err"`
	BindAddr   string `json:"bind_addr"`
	BindPort   int    `json:"bind_port"`
	ServerUser string `json:"server_user"`
	ServerName string `json:"server_name"`
	Plugin     string `json:"plugin"`
}

func newWorkingStatus(cfg v1.VisitorConfigurer) *WorkingStatus {
	base := cfg.GetBaseConfig()
	bindAddr := base.BindAddr
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}
	return &WorkingStatus{
		Name:       base.Name,
		Type:       base.Type,
		BindAddr:   bindAddr,
		BindPort:   base.BindPort,
		ServerUser: base.ServerUser,
		ServerName: base.ServerName,
		Plugin:     base.Plugin.Type,
	}
}

func cloneWorkingStatus(status *WorkingStatus) *WorkingStatus {
	if status == nil {
		return nil
	}
	out := *status
	return &out
}
