// Copyright 2025 The frp Authors
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

package model

const (
	SourceConfig = "config"
	SourceStore  = "store"
)

// StatusResp is the response for GET /api/status
type StatusResp map[string][]ProxyStatusResp

// ProxyStatusResp contains proxy status information
type ProxyStatusResp struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	Err           string `json:"err"`
	LocalAddr     string `json:"local_addr"`
	Plugin        string `json:"plugin"`
	RemoteAddr    string `json:"remote_addr"`
	Source        string `json:"source,omitempty"` // "store" or "config"
	CertificateID string `json:"certificate_id,omitempty"`
}

// ProxyListResp is the response for GET /api/store/proxies
type ProxyListResp struct {
	Proxies []ProxyDefinition `json:"proxies"`
}

// ProxyConfigSummary is the list view for all proxy configuration sources.
// It intentionally omits full configuration and authentication material.
type ProxyConfigSummary struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Source        string `json:"source"`
	Enabled       bool   `json:"enabled"`
	LocalAddr     string `json:"local_addr,omitempty"`
	RemoteAddr    string `json:"remote_addr,omitempty"`
	Plugin        string `json:"plugin,omitempty"`
	CertificateID string `json:"certificate_id,omitempty"`
}

type ProxyConfigListResp struct {
	Proxies []ProxyConfigSummary `json:"proxies"`
}

// VisitorListResp is the response for GET /api/store/visitors
type VisitorListResp struct {
	Visitors []VisitorDefinition `json:"visitors"`
}

// VisitorConfigSummary exposes configuration metadata without SecretKey.
type VisitorConfigSummary struct {
	Name             string `json:"name"`
	Type             string `json:"type"`
	Source           string `json:"source"`
	Enabled          bool   `json:"enabled"`
	SecretConfigured bool   `json:"secret_configured"`
	ServerUser       string `json:"server_user,omitempty"`
	ServerName       string `json:"server_name,omitempty"`
	BindAddr         string `json:"bind_addr,omitempty"`
	BindPort         int    `json:"bind_port"`
	Plugin           string `json:"plugin,omitempty"`
	UseEncryption    bool   `json:"use_encryption"`
	UseCompression   bool   `json:"use_compression"`
	XTCPProtocol     string `json:"xtcp_protocol,omitempty"`
	KeepTunnelOpen   bool   `json:"keep_tunnel_open,omitempty"`
	FallbackTo       string `json:"fallback_to,omitempty"`
}

type VisitorConfigListResp struct {
	Visitors []VisitorConfigSummary `json:"visitors"`
}

type VisitorStatusResp struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Err        string `json:"err"`
	BindAddr   string `json:"bind_addr"`
	BindPort   int    `json:"bind_port"`
	ServerUser string `json:"server_user,omitempty"`
	ServerName string `json:"server_name,omitempty"`
	Plugin     string `json:"plugin,omitempty"`
}

type VisitorStatusListResp struct {
	Visitors []VisitorStatusResp `json:"visitors"`
}
