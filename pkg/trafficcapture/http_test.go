package trafficcapture

import (
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
