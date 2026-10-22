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

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/fatedier/frp/pkg/config/v1"
)

func TestManagerTracksVisitorStartSuccessAndFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	manager := NewManager(
		ctx,
		"run-id",
		&v1.ClientCommonConfig{},
		nil,
		nil,
		nil,
	)
	t.Cleanup(manager.Close)

	manager.UpdateAll([]v1.VisitorConfigurer{
		newSTCPVisitorConfig("working", "server-proxy", 0),
		newSTCPVisitorConfig("failed", "server-proxy", 70000),
	})

	statuses := statusByName(manager.GetAllVisitorStatus())
	require.Equal(t, PhaseRunning, statuses["working"].Status)
	require.Empty(t, statuses["working"].Err)
	require.Equal(t, PhaseStartError, statuses["failed"].Status)
	require.NotEmpty(t, statuses["failed"].Err)
	require.Equal(t, 70000, statuses["failed"].BindPort)

	manager.UpdateAll(nil)
	require.Empty(t, manager.GetAllVisitorStatus())
}

func TestManagerRefreshesVisitorStatusAfterRelaunchAndClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	manager := NewManager(
		ctx,
		"run-id",
		&v1.ClientCommonConfig{},
		nil,
		nil,
		nil,
	)

	manager.UpdateAll([]v1.VisitorConfigurer{
		newSTCPVisitorConfig("visitor", "server-one", 0),
	})
	require.Equal(t, PhaseRunning, statusByName(manager.GetAllVisitorStatus())["visitor"].Status)

	manager.UpdateAll([]v1.VisitorConfigurer{
		newSTCPVisitorConfig("visitor", "server-two", 0),
	})
	status := statusByName(manager.GetAllVisitorStatus())["visitor"]
	require.Equal(t, PhaseRunning, status.Status)
	require.Equal(t, "server-two", status.ServerName)

	manager.Close()
	status = statusByName(manager.GetAllVisitorStatus())["visitor"]
	require.Equal(t, PhaseClosed, status.Status)
}

func newSTCPVisitorConfig(name, serverName string, bindPort int) *v1.STCPVisitorConfig {
	return &v1.STCPVisitorConfig{
		VisitorBaseConfig: v1.VisitorBaseConfig{
			Name:       name,
			Type:       "stcp",
			SecretKey:  "must-not-leak",
			ServerName: serverName,
			BindAddr:   "127.0.0.1",
			BindPort:   bindPort,
		},
	}
}

func statusByName(statuses []*WorkingStatus) map[string]*WorkingStatus {
	result := make(map[string]*WorkingStatus, len(statuses))
	for _, status := range statuses {
		result[status.Name] = status
	}
	return result
}
