package trafficcapture

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFrameMaskedText(t *testing.T) {
	mask := []byte{0x12, 0x34, 0x56, 0x78}
	payload := []byte("hello")
	data := []byte{0x81, 0x80 | byte(len(payload))}
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

func textFrame(payload string) []byte {
	data := []byte{0x81, byte(len(payload))}
	return append(data, payload...)
}
