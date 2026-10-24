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

package trafficcapture

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"
)

const maxWebSocketFrames = 2000

var httpVersionPrefix = []byte("HTTP/")

type webSocketCaptureConn struct {
	net.Conn
	capture      *Capture
	maxBodyBytes int64
	mu           sync.Mutex
	clientParser *wsParser
	serverParser *wsParser
}

func newWebSocketCaptureConn(conn net.Conn, capture *Capture, maxBodyBytes int64) *webSocketCaptureConn {
	wrapped := &webSocketCaptureConn{
		Conn:         conn,
		capture:      capture,
		maxBodyBytes: maxBodyBytes,
	}
	capturedBytes := int64(0)
	wrapped.clientParser = &wsParser{
		direction: "client", capture: capture, maxBodyBytes: maxBodyBytes,
		capturedBytes: &capturedBytes, mu: &wrapped.mu,
	}
	wrapped.serverParser = &wsParser{
		direction: "server", capture: capture, maxBodyBytes: maxBodyBytes,
		capturedBytes: &capturedBytes, mu: &wrapped.mu,
	}
	return wrapped
}

func (c *webSocketCaptureConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.clientParser.Feed(p[:n])
	}
	return n, err
}

func (c *webSocketCaptureConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 {
		c.serverParser.Feed(p[:n])
	}
	return n, err
}

type wsParser struct {
	direction     string
	capture       *Capture
	buffer        []byte
	maxBodyBytes  int64
	capturedBytes *int64
	mu            *sync.Mutex
	httpUpgrade   bool
}

func (p *wsParser) Feed(data []byte) {
	p.buffer = append(p.buffer, data...)
	if p.direction == "server" && !p.httpUpgrade {
		if len(p.buffer) >= len(httpVersionPrefix) {
			p.httpUpgrade = true
		} else if !bytes.Equal(p.buffer, httpVersionPrefix[:len(p.buffer)]) {
			p.httpUpgrade = true
		}
	}
	if p.direction == "server" && p.httpUpgrade && bytes.HasPrefix(p.buffer, httpVersionPrefix) {
		if index := bytes.Index(p.buffer, []byte("\r\n\r\n")); index >= 0 {
			p.buffer = p.buffer[index+4:]
			p.httpUpgrade = false
		} else {
			return
		}
	}
	for {
		opcode, payload, consumed, ok := parseFrame(p.buffer)
		if !ok {
			break
		}
		p.buffer = append(p.buffer[:0], p.buffer[consumed:]...)
		if int64(len(payload)) > p.maxBodyBytes {
			payload = payload[:p.maxBodyBytes]
		}
		p.mu.Lock()
		switch {
		case len(p.capture.WebSocketFrames) >= maxWebSocketFrames:
			p.capture.Truncated = true
		case p.capturedBytes == nil:
			p.capture.Truncated = true
		case *p.capturedBytes >= p.maxBodyBytes:
			p.capture.Truncated = true
		default:
			remaining := p.maxBodyBytes - *p.capturedBytes
			if int64(len(payload)) > remaining {
				payload = payload[:remaining]
				p.capture.Truncated = true
			}
			p.capture.WebSocketFrames = append(p.capture.WebSocketFrames, WebSocketFrame{
				Direction: p.direction,
				Opcode:    opcode,
				Payload:   append([]byte(nil), payload...),
				At:        time.Now(),
			})
			*p.capturedBytes += int64(len(payload))
		}
		p.mu.Unlock()
	}
	if len(p.buffer) > int(p.maxBodyBytes)+14 {
		p.buffer = append(p.buffer[:0], p.buffer[len(p.buffer)-14:]...)
		p.mu.Lock()
		p.capture.Truncated = true
		p.mu.Unlock()
	}
}

func parseFrame(data []byte) (opcode int, payload []byte, consumed int, ok bool) {
	if len(data) < 2 {
		return 0, nil, 0, false
	}
	opcode = int(data[0] & 0x0f)
	masked := data[1]&0x80 != 0
	length := int64(data[1] & 0x7f)
	headerLength := 2
	switch length {
	case 126:
		if len(data) < 4 {
			return 0, nil, 0, false
		}
		length = int64(binary.BigEndian.Uint16(data[2:4]))
		headerLength = 4
	case 127:
		if len(data) < 10 {
			return 0, nil, 0, false
		}
		length = int64(binary.BigEndian.Uint64(data[2:10]))
		headerLength = 10
	}
	if length < 0 || length > 256<<20 {
		return 0, nil, 0, false
	}
	maskLength := 0
	if masked {
		maskLength = 4
	}
	total := headerLength + maskLength + int(length)
	if len(data) < total {
		return 0, nil, 0, false
	}
	payload = append([]byte(nil), data[headerLength+maskLength:total]...)
	if masked {
		mask := data[headerLength : headerLength+4]
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, total, true
}
