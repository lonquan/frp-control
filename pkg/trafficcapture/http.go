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
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

func (m *Manager) WrapHTTPHandler(proxy, protocol, scheme, target string, next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.Enabled(proxy) {
			next.ServeHTTP(w, r)
			return
		}
		maxEntryBytes, _ := m.Limits()
		requestBody, requestTruncated, err := captureRequestBody(r, maxEntryBytes)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		capture := m.Begin(
			proxy,
			protocol,
			scheme,
			target,
			r.Method,
			r.Host,
			r.URL.Path,
			r.URL.RawQuery,
			r.Header,
			requestBody,
			requestTruncated,
		)
		writer := &captureResponseWriter{
			ResponseWriter: w,
			capture:        capture,
			maxBodyBytes:   maxEntryBytes,
			manager:        m,
		}
		defer func() {
			capture.ResponseStatus = writer.status
			capture.ResponseHeaders = cloneHeaders(writer.Header())
			capture.ResponseBody = append([]byte(nil), writer.body.Bytes()...)
			capture.Truncated = capture.Truncated || writer.truncated
			_ = m.Complete(capture)
		}()
		next.ServeHTTP(writer, r)
	})
}

type captureResponseWriter struct {
	http.ResponseWriter
	capture      *Capture
	manager      *Manager
	maxBodyBytes int64
	status       int
	body         bytes.Buffer
	truncated    bool
	hijackMu     sync.Mutex
}

func (w *captureResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if int64(w.body.Len()) < w.maxBodyBytes {
		remaining := w.maxBodyBytes - int64(w.body.Len())
		if remaining > int64(len(data)) {
			remaining = int64(len(data))
		}
		_, _ = w.body.Write(data[:remaining])
		if remaining < int64(len(data)) {
			w.truncated = true
		}
	} else if len(data) > 0 {
		w.truncated = true
	}
	return w.ResponseWriter.Write(data)
}

func (w *captureResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *captureResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijackMu.Lock()
	defer w.hijackMu.Unlock()
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	w.capture.WebSocket = true
	w.capture.ResponseStatus = http.StatusSwitchingProtocols
	wrapped := newWebSocketCaptureConn(conn, w.capture, w.maxBodyBytes)
	return wrapped, bufio.NewReadWriter(bufio.NewReader(wrapped), bufio.NewWriter(wrapped)), nil
}

func (w *captureResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func readBodyLimit(body io.ReadCloser, limit int64) ([]byte, bool, error) {
	if body == nil {
		return nil, false, nil
	}
	defer body.Close()
	var buffer bytes.Buffer
	truncated := false
	if limit <= 0 {
		limit = defaultMaxEntryBytes
	}
	_, err := io.CopyN(&buffer, body, limit+1)
	if errors.Is(err, io.EOF) {
		return buffer.Bytes(), truncated, nil
	}
	if err != nil {
		return nil, false, err
	}
	truncated = true
	data := append([]byte(nil), buffer.Bytes()[:limit]...)
	return data, truncated, nil
}

func captureRequestBody(r *http.Request, limit int64) ([]byte, bool, error) {
	if r.Body == nil {
		return nil, false, nil
	}
	if limit <= 0 {
		limit = defaultMaxEntryBytes
	}
	var prefix bytes.Buffer
	_, err := io.CopyN(&prefix, r.Body, limit+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	fullPrefix := prefix.Bytes()
	truncated := int64(len(fullPrefix)) > limit
	if truncated {
		fullPrefix = fullPrefix[:limit]
	}
	captured := append([]byte(nil), fullPrefix...)
	r.Body = &multiReadCloser{
		Reader: io.MultiReader(bytes.NewReader(prefix.Bytes()), r.Body),
		Closer: r.Body,
	}
	return captured, truncated, nil
}

type multiReadCloser struct {
	io.Reader
	io.Closer
}

func (m *Manager) ServeDirectHTTP(conn net.Conn, target, proxy string) error {
	if m == nil || !m.Enabled(proxy) {
		return errors.New("traffic capture is not enabled")
	}
	done := make(chan struct{})
	listener := &oneShotListener{
		conn: &signalConn{Conn: conn, done: done},
		done: done,
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: target})
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
	handler := m.WrapHTTPHandler(proxy, "http", "http", target, reverseProxy)
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 60 * time.Second,
	}
	err := server.Serve(listener)
	server.Close()
	listener.close()
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

type oneShotListener struct {
	conn     net.Conn
	accepted bool
	mu       sync.Mutex
	done     chan struct{}
}

func (l *oneShotListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if !l.accepted {
		l.accepted = true
		conn := l.conn
		l.mu.Unlock()
		return conn, nil
	}
	l.mu.Unlock()
	<-l.done
	return nil, io.EOF
}

func (l *oneShotListener) Close() error {
	l.close()
	return nil
}

func (l *oneShotListener) close() {
	select {
	case <-l.done:
	default:
		close(l.done)
	}
}

func (l *oneShotListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

type signalConn struct {
	net.Conn
	done chan struct{}
	once sync.Once
}

func (c *signalConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { close(c.done) })
	return err
}
