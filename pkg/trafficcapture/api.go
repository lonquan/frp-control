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
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type replayRequest struct {
	IDs         []string `json:"ids"`
	Concurrency int      `json:"concurrency"`
	IntervalMS  int      `json:"intervalMs"`
}

type ReplayResponse struct {
	ID     string       `json:"id"`
	Result ReplayResult `json:"result"`
}

func (m *Manager) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v2/captures/settings", m.handleSettings).Methods(http.MethodGet, http.MethodPut)
	router.HandleFunc("/api/v2/captures", m.handleCollection).Methods(http.MethodGet, http.MethodDelete)
	router.HandleFunc("/api/v2/captures/{id}", m.handleDetailDelete).Methods(http.MethodGet, http.MethodDelete)
	router.HandleFunc("/api/v2/captures/replay", m.handleReplay).Methods(http.MethodPost)
}

func (m *Manager) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var settings Settings
		if err := decodeJSON(w, r, &settings); err != nil {
			return
		}
		if err := m.SaveSettings(settings); err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, m.Settings())
}

func (m *Manager) handleCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		if err := m.Clear(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	m.handleList(w, r)
}

func (m *Manager) handleList(w http.ResponseWriter, r *http.Request) {
	items, err := m.List(
		r.URL.Query().Get("proxy"),
		r.URL.Query().Get("method"),
		r.URL.Query().Get("status"),
		r.URL.Query().Get("q"),
		500,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (m *Manager) handleDetailDelete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if r.Method == http.MethodDelete {
		if err := m.Delete(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	capture, err := m.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, capture)
}

func (m *Manager) handleReplay(w http.ResponseWriter, r *http.Request) {
	var body replayRequest
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "请选择需要重放的捕获记录")
		return
	}
	if len(body.IDs) > 100 {
		writeError(w, http.StatusBadRequest, "单次最多重放 100 条记录")
		return
	}
	if body.Concurrency <= 0 {
		body.Concurrency = 1
	}
	if body.Concurrency > 10 {
		body.Concurrency = 10
	}
	results := m.replay(body)
	writeJSON(w, http.StatusOK, results)
}

func (m *Manager) replay(body replayRequest) []ReplayResponse {
	semaphore := make(chan struct{}, body.Concurrency)
	results := make([]ReplayResponse, len(body.IDs))
	var wg sync.WaitGroup
	for index, id := range body.IDs {
		index, id := index, id
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			if body.IntervalMS > 0 {
				time.Sleep(time.Duration(body.IntervalMS) * time.Millisecond)
			}
			capture, err := m.Get(id)
			if err != nil {
				results[index] = ReplayResponse{ID: id, Result: ReplayResult{Error: err.Error()}}
				return
			}
			result := m.replayOne(capture)
			capture.ReplayResults = append(capture.ReplayResults, result)
			if len(capture.ReplayResults) > 20 {
				capture.ReplayResults = capture.ReplayResults[len(capture.ReplayResults)-20:]
			}
			if b, err := json.Marshal(capture); err == nil {
				_ = atomicWrite(filepath.Join(m.dir, id+".json"), b, 0o600)
			}
			results[index] = ReplayResponse{ID: id, Result: result}
		}()
	}
	wg.Wait()
	return results
}

func (m *Manager) replayOne(capture *Capture) ReplayResult {
	result := ReplayResult{StartedAt: time.Now()}
	if capture.WebSocket {
		result.Error = "WebSocket 消息帧仅支持查看，不支持重放"
		return result
	}
	if capture.Truncated {
		result.Error = "捕获内容已截断，不能进行高保真重放"
		return result
	}
	m.mu.Lock()
	resolver := m.targetResolver
	m.mu.Unlock()
	if resolver == nil {
		result.Error = "代理目标解析器不可用"
		return result
	}
	scheme, target, ok := resolver(capture.Proxy)
	if !ok {
		result.Error = "原代理已不存在或后端不可解析"
		return result
	}
	if scheme != capture.Scheme || target != capture.Target {
		result.Error = "当前代理后端与捕获时不一致，已拒绝重放"
		return result
	}
	requestURL := &url.URL{
		Scheme:   capture.Scheme,
		Host:     capture.Target,
		Path:     capture.Path,
		RawQuery: capture.Query,
	}
	request, err := http.NewRequest(capture.Method, requestURL.String(), bytes.NewReader(capture.RequestBody))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	for key, values := range capture.RequestHeaders {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	request.Host = capture.Host
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		result.Error = err.Error()
		result.DurationMS = time.Since(result.StartedAt).Milliseconds()
		return result
	}
	defer response.Body.Close()
	maxEntryBytes, _ := m.Limits()
	responseBody, _, _ := readBodyLimit(response.Body, maxEntryBytes)
	result.Status = response.StatusCode
	result.ResponseBodies = responseBody
	result.DurationMS = time.Since(result.StartedAt).Milliseconds()
	return result
}

func isHopByHopHeader(header string) bool {
	switch strings.ToLower(header) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate",
		"proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade",
		"content-length", "host":
		return true
	default:
		return false
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取请求失败")
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		writeError(w, http.StatusBadRequest, "请求 JSON 格式错误")
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": status,
		"msg":  http.StatusText(status),
		"data": data,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": status,
		"msg":  message,
		"data": nil,
	})
}
