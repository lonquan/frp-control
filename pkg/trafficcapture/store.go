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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultTTL           = 24 * time.Hour
	defaultMaxEntryBytes = 16 << 20
	defaultMaxTotalBytes = 500 << 20
)

type Settings struct {
	Enabled         bool     `json:"enabled"`
	Proxies         []string `json:"proxies"`
	ExcludePatterns []string `json:"excludePatterns"`
	TTLSeconds      int64    `json:"ttlSeconds"`
	MaxEntryBytes   int64    `json:"maxEntryBytes"`
	MaxTotalBytes   int64    `json:"maxTotalBytes"`
}

type Capture struct {
	ID              string              `json:"id"`
	Proxy           string              `json:"proxy"`
	Protocol        string              `json:"protocol"`
	Method          string              `json:"method"`
	Scheme          string              `json:"scheme"`
	Host            string              `json:"host"`
	Path            string              `json:"path"`
	Query           string              `json:"query"`
	SourceAddr      string              `json:"sourceAddr,omitempty"`
	SourcePort      uint16              `json:"sourcePort,omitempty"`
	RequestHeaders  map[string][]string `json:"requestHeaders,omitempty"`
	RequestBody     []byte              `json:"requestBody,omitempty"`
	ResponseStatus  int                 `json:"responseStatus"`
	ResponseHeaders map[string][]string `json:"responseHeaders,omitempty"`
	ResponseBody    []byte              `json:"responseBody,omitempty"`
	WebSocketFrames []WebSocketFrame    `json:"webSocketFrames,omitempty"`
	StartedAt       time.Time           `json:"startedAt"`
	DurationMS      int64               `json:"durationMs"`
	Target          string              `json:"target"`
	Truncated       bool                `json:"truncated"`
	WebSocket       bool                `json:"webSocket"`
	Error           string              `json:"error,omitempty"`
	ReplayResults   []ReplayResult      `json:"replayResults,omitempty"`
}

type WebSocketFrame struct {
	Direction string    `json:"direction"`
	Opcode    int       `json:"opcode"`
	Payload   []byte    `json:"payload,omitempty"`
	At        time.Time `json:"at"`
}

type ReplayResult struct {
	StartedAt      time.Time `json:"startedAt"`
	Status         int       `json:"status"`
	DurationMS     int64     `json:"durationMs"`
	ResponseBodies []byte    `json:"responseBody,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type ListItem struct {
	ID             string    `json:"id"`
	Proxy          string    `json:"proxy"`
	Protocol       string    `json:"protocol"`
	Method         string    `json:"method"`
	Host           string    `json:"host"`
	Path           string    `json:"path"`
	SourceAddr     string    `json:"sourceAddr,omitempty"`
	SourcePort     uint16    `json:"sourcePort,omitempty"`
	ResponseStatus int       `json:"responseStatus"`
	StartedAt      time.Time `json:"startedAt"`
	DurationMS     int64     `json:"durationMs"`
	Truncated      bool      `json:"truncated"`
	WebSocket      bool      `json:"webSocket"`
	Size           int64     `json:"size"`
}

type Manager struct {
	mu             sync.Mutex
	dir            string
	settingsPath   string
	settings       Settings
	exclude        excludeMatcher
	active         map[string]*Capture
	targetResolver func(proxy string) (string, string, bool)
}

func NewManager(dataDir string) (*Manager, error) {
	dir := filepath.Join(dataDir, "captures")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	m := &Manager{
		dir:          dir,
		settingsPath: filepath.Join(dir, "settings.json"),
		active:       make(map[string]*Capture),
		settings: Settings{
			Proxies:       []string{},
			TTLSeconds:    int64(defaultTTL.Seconds()),
			MaxEntryBytes: defaultMaxEntryBytes,
			MaxTotalBytes: defaultMaxTotalBytes,
		},
	}
	if err := m.loadSettings(); err != nil {
		return nil, err
	}
	return m, nil
}

var (
	defaultMu  sync.RWMutex
	defaultMgr *Manager
)

func SetDefault(manager *Manager) {
	defaultMu.Lock()
	defaultMgr = manager
	defaultMu.Unlock()
}

func GetDefault() *Manager {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultMgr
}

func (m *Manager) SetTargetResolver(resolver func(proxy string) (scheme, target string, ok bool)) {
	m.mu.Lock()
	m.targetResolver = resolver
	m.mu.Unlock()
}

func (m *Manager) loadSettings() error {
	b, err := os.ReadFile(m.settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &m.settings); err != nil {
		return err
	}
	if m.settings.Proxies == nil {
		m.settings.Proxies = []string{}
	}
	excludePatterns, err := normalizeExcludePatterns(m.settings.ExcludePatterns)
	if err != nil {
		return err
	}
	m.settings.ExcludePatterns = excludePatterns
	m.exclude = compileExcludePatterns(excludePatterns)
	m.applyDefaultLimits()
	return nil
}

func (m *Manager) applyDefaultLimits() {
	if m.settings.TTLSeconds <= 0 {
		m.settings.TTLSeconds = int64(defaultTTL.Seconds())
	}
	if m.settings.MaxEntryBytes <= 0 {
		m.settings.MaxEntryBytes = defaultMaxEntryBytes
	}
	if m.settings.MaxTotalBytes <= 0 {
		m.settings.MaxTotalBytes = defaultMaxTotalBytes
	}
}

func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := m.settings
	copy.Proxies = append([]string(nil), m.settings.Proxies...)
	copy.ExcludePatterns = append([]string(nil), m.settings.ExcludePatterns...)
	return copy
}

func (m *Manager) SaveSettings(settings Settings) error {
	m.applyDefaultLimits()
	excludePatterns, err := normalizeExcludePatterns(settings.ExcludePatterns)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if settings.TTLSeconds <= 0 {
		settings.TTLSeconds = int64(defaultTTL.Seconds())
	}
	if settings.MaxEntryBytes <= 0 || settings.MaxEntryBytes > 512<<20 {
		settings.MaxEntryBytes = defaultMaxEntryBytes
	}
	if settings.MaxTotalBytes <= 0 {
		settings.MaxTotalBytes = defaultMaxTotalBytes
	}
	settings.Proxies = normalizeProxies(settings.Proxies)
	settings.ExcludePatterns = excludePatterns
	m.settings = settings
	m.exclude = compileExcludePatterns(excludePatterns)
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		m.mu.Unlock()
		return err
	}
	err = atomicWrite(m.settingsPath, b, 0o600)
	m.mu.Unlock()
	return err
}

func (m *Manager) Enabled(proxy string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.settings.Enabled {
		return false
	}
	for _, name := range m.settings.Proxies {
		if name == proxy {
			return true
		}
	}
	return false
}

func (m *Manager) Limits() (maxEntryBytes, ttlSeconds int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings.MaxEntryBytes, m.settings.TTLSeconds
}

func (m *Manager) Excluded(path, rawQuery string) bool {
	m.mu.Lock()
	matcher := m.exclude
	m.mu.Unlock()
	return matcher.matches(path, rawQuery)
}

func (m *Manager) Begin(proxy, protocol, scheme, target, method, host, path, query string, requestHeaders map[string][]string, requestBody []byte, truncated bool) *Capture {
	capture := &Capture{
		ID:             randomID(),
		Proxy:          proxy,
		Protocol:       protocol,
		Method:         method,
		Scheme:         scheme,
		Host:           host,
		Path:           path,
		Query:          query,
		RequestHeaders: cloneHeaders(requestHeaders),
		RequestBody:    append([]byte(nil), requestBody...),
		StartedAt:      time.Now(),
		Target:         target,
		Truncated:      truncated,
	}
	m.mu.Lock()
	m.active[capture.ID] = capture
	m.mu.Unlock()
	return capture
}

func (m *Manager) Complete(capture *Capture) error {
	if capture == nil {
		return nil
	}
	capture.DurationMS = time.Since(capture.StartedAt).Milliseconds()
	m.mu.Lock()
	delete(m.active, capture.ID)
	m.mu.Unlock()
	b, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(m.dir, capture.ID+".json"), b, 0o600); err != nil {
		return err
	}
	return m.cleanup()
}

func (m *Manager) List(proxy, method, status, query string, limit int) ([]ListItem, error) {
	// Listing is the most reliable periodic touchpoint even when no new
	// captures are being written, so enforce retention before reading entries.
	_ = m.cleanup()
	m.mu.Lock()
	ttl := time.Duration(m.settings.TTLSeconds) * time.Second
	m.mu.Unlock()
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}
	items := make([]ListItem, 0)
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "settings.json" {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) > ttl {
			continue
		}
		b, err := os.ReadFile(filepath.Join(m.dir, entry.Name()))
		if err != nil {
			continue
		}
		var capture Capture
		if err := json.Unmarshal(b, &capture); err != nil {
			continue
		}
		if proxy != "" && capture.Proxy != proxy {
			continue
		}
		if method != "" && !strings.EqualFold(capture.Method, method) {
			continue
		}
		if status != "" && fmt.Sprint(capture.ResponseStatus) != status {
			continue
		}
		q := strings.ToLower(strings.TrimSpace(query))
		if q != "" && !strings.Contains(strings.ToLower(capture.Host+capture.Path), q) {
			continue
		}
		items = append(items, ListItem{
			ID:             capture.ID,
			Proxy:          capture.Proxy,
			Protocol:       capture.Protocol,
			Method:         capture.Method,
			Host:           capture.Host,
			Path:           capture.Path,
			SourceAddr:     capture.SourceAddr,
			SourcePort:     capture.SourcePort,
			ResponseStatus: capture.ResponseStatus,
			StartedAt:      capture.StartedAt,
			DurationMS:     capture.DurationMS,
			Truncated:      capture.Truncated,
			WebSocket:      capture.WebSocket,
			Size:           int64(len(capture.RequestBody) + len(capture.ResponseBody)),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].StartedAt.After(items[j].StartedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (m *Manager) Get(id string) (*Capture, error) {
	if strings.ContainsAny(id, `/\`) || id == "" {
		return nil, errors.New("无效的捕获记录 ID")
	}
	b, err := os.ReadFile(filepath.Join(m.dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var capture Capture
	if err := json.Unmarshal(b, &capture); err != nil {
		return nil, err
	}
	return &capture, nil
}

func (m *Manager) Delete(id string) error {
	if strings.ContainsAny(id, `/\`) || id == "" {
		return errors.New("无效的捕获记录 ID")
	}
	return os.Remove(filepath.Join(m.dir, id+".json"))
}

func (m *Manager) cleanup() error {
	m.mu.Lock()
	settings := m.settings
	m.mu.Unlock()
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return err
	}
	type infoItem struct {
		path string
		size int64
		time time.Time
	}
	items := make([]infoItem, 0)
	var total int64
	now := time.Now()
	ttl := time.Duration(settings.TTLSeconds) * time.Second
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "settings.json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(m.dir, entry.Name())
		if now.Sub(info.ModTime()) > ttl {
			_ = os.Remove(path)
			continue
		}
		total += info.Size()
		items = append(items, infoItem{path: path, size: info.Size(), time: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].time.Before(items[j].time) })
	for _, item := range items {
		if total <= settings.MaxTotalBytes {
			break
		}
		if err := os.Remove(item.path); err == nil {
			total -= item.size
		}
	}
	return nil
}

func normalizeProxies(proxies []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		if _, ok := seen[proxy]; ok {
			continue
		}
		seen[proxy] = struct{}{}
		result = append(result, proxy)
	}
	return result
}

func cloneHeaders(headers map[string][]string) map[string][]string {
	result := make(map[string][]string, len(headers))
	for key, values := range headers {
		result[key] = append([]string(nil), values...)
	}
	return result
}

func randomID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
