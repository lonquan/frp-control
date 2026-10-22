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

package server

import (
	"time"

	"github.com/fatedier/frp/pkg/metrics/mem"
	"github.com/fatedier/frp/pkg/util/version"
)

func (svr *Service) adminMetrics() any {
	stats := mem.StatsCollector.GetServer()
	clients := svr.clientRegistry.List()
	onlineClients := 0
	for _, client := range clients {
		if client.Online {
			onlineClients++
		}
	}
	proxyCount := int64(0)
	for _, count := range stats.ProxyTypeCounts {
		proxyCount += count
	}
	return map[string]any{
		"role":                "server",
		"version":             version.Full(),
		"distributionVersion": version.Distribution(),
		"upstreamVersion":     version.Full(),
		"onlineClients":       onlineClients,
		"knownClients":        len(clients),
		"proxies":             proxyCount,
		"proxyTypeCounts":     stats.ProxyTypeCounts,
		"curConns":            stats.CurConns,
		"totalTrafficIn":      stats.TotalTrafficIn,
		"totalTrafficOut":     stats.TotalTrafficOut,
		"bindPort":            svr.cfg.BindPort,
		"vhostHTTPPort":       svr.cfg.VhostHTTPPort,
		"vhostHTTPSPort":      svr.cfg.VhostHTTPSPort,
		"kcpBindPort":         svr.cfg.KCPBindPort,
		"quicBindPort":        svr.cfg.QUICBindPort,
		"tcpmuxHTTPConnect":   svr.cfg.TCPMuxHTTPConnectPort,
		"serverTime":          time.Now().Format(time.RFC3339),
	}
}
