//go:build !frps

package client

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPBridgePluginPreservesSourceAddressAndPort(t *testing.T) {
	listener := NewProxyListener()
	bridge := &httpBridgePlugin{
		l:                   listener,
		useSourceRemoteAddr: true,
	}
	conn, underlying := net.Pipe()
	t.Cleanup(func() {
		_ = conn.Close()
		_ = underlying.Close()
		_ = listener.Close()
	})

	source := &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 54321}
	bridge.Handle(context.Background(), &ConnectionInfo{
		Conn:           conn,
		UnderlyingConn: underlying,
		SrcAddr:        source,
	})

	accepted, err := listener.Accept()
	require.NoError(t, err)
	require.Equal(t, source.String(), accepted.RemoteAddr().String())
}
