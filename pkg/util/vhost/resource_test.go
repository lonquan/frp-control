// Copyright 2026 fatedier, fatedier@gmail.com
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

package vhost

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotFoundResponseDefaultPage(t *testing.T) {
	setNotFoundPagePath(t, "")

	response := NotFoundResponse()
	defer response.Body.Close()

	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.Equal(t, "Not Found", response.Status)
	require.Equal(t, "text/html; charset=utf-8", response.Header.Get("Content-Type"))
	require.True(t, strings.HasPrefix(response.Header.Get("Server"), "frp/"))

	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, response.ContentLength, int64(len(content)))

	html := string(content)
	for _, expected := range []string{
		`<html lang="zh-CN">`,
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`<title>404 · 暂未找到可用的 frp 代理</title>`,
		`<p class="brand-name">FRP-Control</p>`,
		`<h1 id="page-title">暂未找到 <span class="title-line">可用的 frp 代理</span></h1>`,
		`当前域名未匹配到 HTTP/HTTPS 代理。`,
		`<code>customDomains</code>`,
		`<code>subdomain</code>`,
		`确认 DNS 指向当前 frps 后重载客户端。`,
		`href="https://github.com/lonquan/frp-control"`,
		`target="_blank"`,
		`rel="noopener noreferrer"`,
	} {
		require.Contains(t, html, expected)
	}
	require.NotContains(t, html, "<script")
	require.NotContains(t, html, "fonts.googleapis.com")
	require.NotContains(t, html, `<ol class="steps">`)
}

func TestHTTPReverseProxyNotFoundPage(t *testing.T) {
	setNotFoundPagePath(t, "")

	proxy := NewHTTPReverseProxy(HTTPReverseProxyOptions{}, NewRouters())
	request := httptest.NewRequest(http.MethodGet, "http://unbound.example.com/", nil)
	recorder := httptest.NewRecorder()

	proxy.ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", response.Header.Get("Content-Type"))
	require.True(t, strings.HasPrefix(response.Header.Get("Server"), "frp/"))

	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(
		t,
		string(content),
		`<h1 id="page-title">暂未找到 <span class="title-line">可用的 frp 代理</span></h1>`,
	)
}

func TestNotFoundResponseUsesCustomPage(t *testing.T) {
	customPage := `<!DOCTYPE html><html><body>custom 404</body></html>`
	pagePath := filepath.Join(t.TempDir(), "404.html")
	require.NoError(t, os.WriteFile(pagePath, []byte(customPage), 0o600))
	setNotFoundPagePath(t, pagePath)

	response := NotFoundResponse()
	defer response.Body.Close()

	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, customPage, string(content))
	require.Equal(t, int64(len(customPage)), response.ContentLength)
}

func TestNotFoundResponseFallsBackWhenCustomPageIsUnreadable(t *testing.T) {
	setNotFoundPagePath(t, filepath.Join(t.TempDir(), "missing.html"))

	response := NotFoundResponse()
	defer response.Body.Close()

	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, NotFound, string(content))
	require.Equal(t, int64(len(NotFound)), response.ContentLength)
}

func setNotFoundPagePath(t *testing.T, path string) {
	t.Helper()

	originalPath := NotFoundPagePath
	NotFoundPagePath = path
	t.Cleanup(func() {
		NotFoundPagePath = originalPath
	})
}
