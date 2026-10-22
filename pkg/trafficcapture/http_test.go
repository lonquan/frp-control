package trafficcapture

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestHTTPMiddlewareCapturesWebSocketFrames(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(Settings{
		Enabled:       true,
		Proxies:       []string{"ws"},
		TTLSeconds:    3600,
		MaxEntryBytes: 1 << 20,
		MaxTotalBytes: 1 << 20,
	}))

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	handler := manager.WrapHTTPHandler("ws", "http", "http", "127.0.0.1:1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.WriteMessage(messageType, payload)
	}))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("hello")))
	_, payload, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "hello", string(payload))
	require.NoError(t, conn.Close())

	var capture *Capture
	require.Eventually(t, func() bool {
		items, err := manager.List("ws", "", "", "", 10)
		if err != nil || len(items) == 0 {
			return false
		}
		capture, err = manager.Get(items[0].ID)
		return err == nil && capture.WebSocket && len(capture.WebSocketFrames) >= 2
	}, 2*time.Second, 20*time.Millisecond)
	require.Equal(t, "hello", string(capture.WebSocketFrames[0].Payload))
}

func TestHTTPMiddlewareCapturesSourceAddressAndPort(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(Settings{
		Enabled:       true,
		Proxies:       []string{"web"},
		TTLSeconds:    3600,
		MaxEntryBytes: 1 << 20,
		MaxTotalBytes: 1 << 20,
	}))

	handler := manager.WrapHTTPHandler("web", "http", "http", "127.0.0.1:8080",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "203.0.113.10", r.Header.Get("X-Forwarded-For"))
			w.WriteHeader(http.StatusNoContent)
		}))
	request := httptest.NewRequest(http.MethodGet, "http://example.com/items", nil)
	request.RemoteAddr = "203.0.113.10:54321"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	items, err := manager.List("web", "", "", "", 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "203.0.113.10", items[0].SourceAddr)
	require.Equal(t, uint16(54321), items[0].SourcePort)

	stored, err := manager.Get(items[0].ID)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.10", stored.SourceAddr)
	require.Equal(t, uint16(54321), stored.SourcePort)
	require.NotContains(t, stored.RequestHeaders, "X-Forwarded-For")
}

func TestHTTPMiddlewareExcludesMatchingRequestsButForwardsThem(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(Settings{
		Enabled:         true,
		Proxies:         []string{"web"},
		ExcludePatterns: []string{"favicon.ico", "/static/*"},
		TTLSeconds:      3600,
		MaxEntryBytes:   1 << 20,
		MaxTotalBytes:   1 << 20,
	}))

	called := false
	handler := manager.WrapHTTPHandler("web", "http", "http", "127.0.0.1:8080",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			require.Equal(t, "2001:db8::1", r.Header.Get("X-Forwarded-For"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Equal(t, "payload", string(body))
			w.WriteHeader(http.StatusTeapot)
		}))
	request := httptest.NewRequest(
		http.MethodPost,
		"http://example.com/favicon.ico?size=32",
		strings.NewReader("payload"),
	)
	request.RemoteAddr = "[2001:db8::1]:4567"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	require.True(t, called)
	require.Equal(t, http.StatusTeapot, recorder.Code)
	items, err := manager.List("", "", "", "", 10)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestHTTPMiddlewareForwardsSourceWhenCaptureIsDisabled(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	require.NoError(t, err)

	handler := manager.WrapHTTPHandler("web", "http", "http", "127.0.0.1:8080",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "198.51.100.20", r.Header.Get("X-Forwarded-For"))
			w.WriteHeader(http.StatusNoContent)
		}))
	request := httptest.NewRequest(http.MethodGet, "http://example.com/healthz", nil)
	request.RemoteAddr = "198.51.100.20:32100"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	items, err := manager.List("", "", "", "", 10)
	require.NoError(t, err)
	require.Empty(t, items)
}
