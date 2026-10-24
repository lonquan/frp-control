package trafficcapture

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFrameMaskedText(t *testing.T) {
	mask := []byte{0x12, 0x34, 0x56, 0x78}
	payload := []byte("hello")
	data := make([]byte, 2, 2+len(mask)+len(payload))
	data[0] = 0x81
	data[1] = 0x80 | byte(len(payload))
	data = append(data, mask...)
	for index, value := range payload {
		data = append(data, value^mask[index%4])
	}
	opcode, decoded, consumed, ok := parseFrame(data)
	require.True(t, ok)
	require.Equal(t, 1, opcode)
	require.Equal(t, payload, decoded)
	require.Equal(t, len(data), consumed)
}

func TestParseFrameWaitsForCompletePayload(t *testing.T) {
	data := []byte{0x81, 0x05, 'h', 'e'}
	_, _, _, ok := parseFrame(data)
	require.False(t, ok)
}

func TestWebSocketCaptureRespectsTotalByteLimit(t *testing.T) {
	capturedBytes := int64(0)
	capture := &Capture{}
	parser := &wsParser{
		direction:     "client",
		capture:       capture,
		maxBodyBytes:  7,
		capturedBytes: &capturedBytes,
		mu:            &sync.Mutex{},
	}

	parser.Feed(textFrame("hello"))
	parser.Feed(textFrame("world"))
	parser.Feed(textFrame("ignored"))

	require.Len(t, capture.WebSocketFrames, 2)
	require.Equal(t, "hello", string(capture.WebSocketFrames[0].Payload))
	require.Equal(t, "wo", string(capture.WebSocketFrames[1].Payload))
	require.Equal(t, int64(7), capturedBytes)
	require.True(t, capture.Truncated)
}

func TestWebSocketCaptureHandlesFragmentedHTTPUpgrade(t *testing.T) {
	capturedBytes := int64(0)
	capture := &Capture{}
	parser := &wsParser{
		direction:     "server",
		capture:       capture,
		maxBodyBytes:  1024,
		capturedBytes: &capturedBytes,
		mu:            &sync.Mutex{},
	}
	header := []byte("HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n\r\n")

	parser.Feed(header[:2])
	parser.Feed(append(header[2:], textFrame("hello")...))

	require.Len(t, capture.WebSocketFrames, 1)
	require.Equal(t, "hello", string(capture.WebSocketFrames[0].Payload))
	require.Empty(t, parser.buffer)
	require.False(t, parser.httpUpgrade)
}

func textFrame(payload string) []byte {
	data := make([]byte, 2, 2+len(payload))
	data[0] = 0x81
	data[1] = byte(len(payload))
	return append(data, payload...)
}
