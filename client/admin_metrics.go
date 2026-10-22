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

package client

import (
	"time"

	"github.com/fatedier/frp/client/proxy"
	"github.com/fatedier/frp/pkg/util/version"
)

func (svr *Service) adminMetrics() any {
	statuses := svr.getAllProxyStatus()
	proxyTypeCounts := make(map[string]int)
	running := 0
	for _, status := range statuses {
		proxyTypeCounts[status.Type]++
		if status.Phase == proxy.ProxyPhaseRunning {
			running++
		}
	}
	svr.ctlMu.RLock()
	connected := svr.ctl != nil
	svr.ctlMu.RUnlock()
	return map[string]any{
		"role":                "client",
		"version":             version.Full(),
		"distributionVersion": version.Distribution(),
		"upstreamVersion":     version.Full(),
		"connected":           connected,
		"proxies":             len(statuses),
		"runningProxies":      running,
		"proxyTypeCounts":     proxyTypeCounts,
		"serverAddr":          svr.common.ServerAddr,
		"serverPort":          svr.common.ServerPort,
		"protocol":            svr.common.Transport.Protocol,
		"serverTime":          time.Now().Format(time.RFC3339),
	}
}
