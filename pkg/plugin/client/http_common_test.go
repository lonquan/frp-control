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

//go:build !frps

package client

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendForwardedHeadersAddsSourceAddress(t *testing.T) {
	in := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	in.RemoteAddr = "127.0.0.1:43210"
	in.Header.Set("X-Forwarded-For", "2.2.2.2")
	in.Header.Set("X-Forwarded-Host", "original.example.com")
	in.Header.Set("X-Forwarded-Proto", "https")
	out := in.Clone(in.Context())
	req := &httputil.ProxyRequest{In: in, Out: out}

	appendForwardedHeaders(req)

	require.Equal(t, "2.2.2.2, 127.0.0.1", out.Header.Get("X-Forwarded-For"))
	require.Equal(t, "original.example.com", out.Header.Get("X-Forwarded-Host"))
	require.Equal(t, "https", out.Header.Get("X-Forwarded-Proto"))
}

func TestCopyForwardedHeadersDoesNotDuplicateServerSource(t *testing.T) {
	in := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	in.RemoteAddr = "127.0.0.1:43210"
	in.Header.Set("X-Forwarded-For", "2.2.2.2, 3.3.3.3, 127.0.0.1")
	in.Header.Set("X-Forwarded-Host", "original.example.com")
	in.Header.Set("X-Forwarded-Proto", "https")
	out := in.Clone(in.Context())
	req := &httputil.ProxyRequest{In: in, Out: out}

	copyForwardedHeaders(req)

	require.Equal(t, "2.2.2.2, 3.3.3.3, 127.0.0.1", out.Header.Get("X-Forwarded-For"))
	require.Equal(t, "original.example.com", out.Header.Get("X-Forwarded-Host"))
	require.Equal(t, "https", out.Header.Get("X-Forwarded-Proto"))
}
