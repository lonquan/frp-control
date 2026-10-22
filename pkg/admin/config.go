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

package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/fatedier/frp/pkg/config"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/policy/security"
)

var (
	errConfigNotFound = errors.New("配置文件不存在")
	errRevision       = errors.New("配置已被其他操作修改，请刷新后重试")
)

type ConfigManager struct {
	mu         sync.Mutex
	root       string
	mainPath   string
	role       string
	historyDir string
}

type ConfigFileInfo struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"modTime"`
	Main      bool      `json:"main"`
	Editable  bool      `json:"editable"`
	Directory bool      `json:"directory"`
	Children  []string  `json:"children,omitempty"`
}

type ConfigFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Revision string `json:"revision"`
	Main     bool   `json:"main"`
	Format   string `json:"format"`
}

type ConfigValidation struct {
	Valid    bool     `json:"valid"`
	Revision string   `json:"revision,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type ConfigHistoryEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Revision  string    `json:"revision"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewConfigManager(root, mainPath, role string) (*ConfigManager, error) {
	if root == "" {
		return nil, errors.New("config root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	mainPath, err = filepath.Abs(mainPath)
	if err != nil {
		return nil, err
	}
	if !pathWithin(root, mainPath) {
		return nil, fmt.Errorf("主配置 %s 不在配置根目录 %s 内", mainPath, root)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	dataDir := os.Getenv("FRP_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Dir(root)
	}
	historyDir := filepath.Join(dataDir, "config-history", role)
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		return nil, err
	}
	return &ConfigManager{
		root:       root,
		mainPath:   mainPath,
		role:       role,
		historyDir: historyDir,
	}, nil
}

func (m *ConfigManager) Root() string {
	return m.root
}

func (m *ConfigManager) MainPath() string {
	return m.mainPath
}

func (m *ConfigManager) ListFiles() ([]ConfigFileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	files := make([]ConfigFileInfo, 0)
	err := filepath.WalkDir(m.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".toml") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(m.root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, ConfigFileInfo{
			Path:     rel,
			Name:     entry.Name(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Main:     samePath(path, m.mainPath),
			Editable: true,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Main != files[j].Main {
			return files[i].Main
		}
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func (m *ConfigManager) ReadFile(rel string) (*ConfigFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path, err := m.resolve(rel)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errConfigNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ConfigFile{
		Path:     filepath.ToSlash(rel),
		Content:  string(b),
		Revision: revision(b),
		Main:     samePath(path, m.mainPath),
		Format:   "toml",
	}, nil
}

func (m *ConfigManager) Validate(rel string, content []byte) *ConfigValidation {
	res := &ConfigValidation{Valid: true}
	if len(content) == 0 {
		res.Valid = false
		res.Errors = []string{"配置内容不能为空"}
		return res
	}
	rendered, err := config.RenderWithTemplate(content, config.GetValues())
	if err != nil {
		res.Valid = false
		res.Errors = []string{fmt.Sprintf("配置模板渲染失败: %v", err)}
		return res
	}
	var raw map[string]any
	if err := toml.Unmarshal(rendered, &raw); err != nil {
		res.Valid = false
		res.Errors = []string{formatTOMLError(err)}
		return res
	}
	res.Revision = revision(content)

	if m.role == "server" {
		var cfg v1.ServerConfig
		if err := config.LoadConfigure(rendered, &cfg, true, "toml"); err != nil {
			res.Valid = false
			res.Errors = []string{err.Error()}
			return res
		}
		if err := cfg.Complete(); err != nil {
			res.Valid = false
			res.Errors = []string{err.Error()}
			return res
		}
		if rel == filepath.ToSlash(relativePath(m.root, m.mainPath)) &&
			os.Getenv("FRP_ALLOW_DASHBOARD_NETWORK_CHANGES") != "1" {
			if current, err := os.ReadFile(m.mainPath); err == nil {
				current, _ = config.RenderWithTemplate(current, config.GetValues())
				var currentCfg v1.ServerConfig
				if err := config.LoadConfigure(current, &currentCfg, true, "toml"); err == nil {
					_ = currentCfg.Complete()
					if dashboardNetworkChanged(&currentCfg, &cfg) {
						res.Valid = false
						res.Errors = []string{
							"WebServer 地址、端口、TLS 或 AssetsDir 已改变；如确认可恢复访问，请设置 FRP_ALLOW_DASHBOARD_NETWORK_CHANGES=1 后重试",
						}
						return res
					}
				}
			}
		}
		validator := validation.NewConfigValidator(unsafeFeatures())
		warning, err := validator.ValidateServerConfig(&cfg)
		if warning != nil {
			res.Warnings = append(res.Warnings, warning.Error())
		}
		if err != nil {
			res.Valid = false
			res.Errors = []string{err.Error()}
		}
		return res
	}

	var cfg v1.ClientConfig
	if err := config.LoadConfigure(rendered, &cfg, true, "toml"); err != nil {
		res.Valid = false
		res.Errors = []string{err.Error()}
		return res
	}
	if err := cfg.ClientCommonConfig.Complete(); err != nil {
		res.Valid = false
		res.Errors = []string{err.Error()}
		return res
	}
	proxies := make([]v1.ProxyConfigurer, 0, len(cfg.Proxies))
	for _, proxy := range cfg.Proxies {
		proxies = append(proxies, proxy.ProxyConfigurer)
	}
	visitors := make([]v1.VisitorConfigurer, 0, len(cfg.Visitors))
	for _, visitor := range cfg.Visitors {
		visitors = append(visitors, visitor.VisitorConfigurer)
	}
	proxies = config.CompleteProxyConfigurers(proxies)
	visitors = config.CompleteVisitorConfigurers(visitors)
	warning, err := validation.ValidateAllClientConfig(&cfg.ClientCommonConfig, proxies, visitors, unsafeFeatures())
	if warning != nil {
		res.Warnings = append(res.Warnings, warning.Error())
	}
	if err != nil {
		res.Valid = false
		res.Errors = []string{err.Error()}
	}
	return res
}

func dashboardNetworkChanged(a, b *v1.ServerConfig) bool {
	if a.WebServer.Addr != b.WebServer.Addr ||
		a.WebServer.Port != b.WebServer.Port ||
		a.WebServer.AssetsDir != b.WebServer.AssetsDir {
		return true
	}
	return !sameTLSConfig(a.WebServer.TLS, b.WebServer.TLS)
}

func sameTLSConfig(a, b *v1.TLSConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m *ConfigManager) WriteFile(rel string, content []byte, expectedRevision string) (*ConfigFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, err := m.resolve(rel)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".toml") {
		return nil, errors.New("只支持保存 TOML 配置文件")
	}
	validation := m.Validate(rel, content)
	if !validation.Valid {
		return nil, fmt.Errorf("配置校验失败: %s", strings.Join(validation.Errors, "; "))
	}
	current, err := os.ReadFile(path)
	if err == nil {
		if expectedRevision != "" && expectedRevision != revision(current) {
			return nil, errRevision
		}
		if err := m.archiveLocked(rel, current); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := atomicWrite(path, content, 0o600); err != nil {
		return nil, err
	}
	return &ConfigFile{
		Path:     filepath.ToSlash(rel),
		Content:  string(content),
		Revision: revision(content),
		Main:     samePath(path, m.mainPath),
		Format:   "toml",
	}, nil
}

func (m *ConfigManager) CreateFile(rel string, content []byte, include bool) (*ConfigFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path, err := m.resolve(rel)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		return nil, errors.New("配置文件已存在")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".toml") {
		return nil, errors.New("配置文件必须使用 .toml 扩展名")
	}
	validation := m.Validate(rel, content)
	if !validation.Valid {
		return nil, fmt.Errorf("配置校验失败: %s", strings.Join(validation.Errors, "; "))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	if err := atomicWrite(path, content, 0o600); err != nil {
		return nil, err
	}
	if include && m.role == "client" {
		if err := m.updateMainIncludesLocked(filepath.ToSlash(rel), true); err != nil {
			_ = os.Remove(path)
			return nil, err
		}
	}
	return &ConfigFile{
		Path:     filepath.ToSlash(rel),
		Content:  string(content),
		Revision: revision(content),
		Main:     false,
		Format:   "toml",
	}, nil
}

func (m *ConfigManager) DeleteFile(rel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	path, err := m.resolve(rel)
	if err != nil {
		return err
	}
	if samePath(path, m.mainPath) {
		return errors.New("主配置文件不能删除")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := m.archiveLocked(rel, current); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if m.role == "client" {
		_ = m.updateMainIncludesLocked(filepath.ToSlash(rel), false)
	}
	return nil
}

func (m *ConfigManager) RenameFile(oldRel, newRel string, updateIncludes bool) (*ConfigFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldPath, err := m.resolve(oldRel)
	if err != nil {
		return nil, err
	}
	newPath, err := m.resolve(newRel)
	if err != nil {
		return nil, err
	}
	if samePath(oldPath, m.mainPath) {
		return nil, errors.New("主配置文件不能重命名")
	}
	if !strings.EqualFold(filepath.Ext(newPath), ".toml") {
		return nil, errors.New("配置文件必须使用 .toml 扩展名")
	}
	if _, err := os.Stat(newPath); err == nil {
		return nil, errors.New("目标配置文件已存在")
	}
	content, err := os.ReadFile(oldPath)
	if err != nil {
		return nil, err
	}
	if err := m.archiveLocked(oldRel, content); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o750); err != nil {
		return nil, err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, err
	}
	if updateIncludes && m.role == "client" {
		if err := m.updateMainIncludesLocked(filepath.ToSlash(oldRel), false); err != nil {
			return nil, err
		}
		if err := m.updateMainIncludesLocked(filepath.ToSlash(newRel), true); err != nil {
			return nil, err
		}
	}
	return &ConfigFile{
		Path:     filepath.ToSlash(newRel),
		Content:  string(content),
		Revision: revision(content),
		Main:     false,
		Format:   "toml",
	}, nil
}

func (m *ConfigManager) History(rel string) ([]ConfigHistoryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entryDir := filepath.Join(m.historyDir, historyKey(rel))
	entries, err := os.ReadDir(entryDir)
	if errors.Is(err, os.ErrNotExist) {
		return []ConfigHistoryEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]ConfigHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(entryDir, entry.Name()))
		if err != nil {
			continue
		}
		result = append(result, ConfigHistoryEntry{
			Name:      entry.Name(),
			Path:      rel,
			Revision:  revision(b),
			CreatedAt: info.ModTime(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (m *ConfigManager) RestoreHistory(rel, name string) (*ConfigFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return nil, errors.New("历史版本名称无效")
	}
	path, err := m.resolve(rel)
	if err != nil {
		return nil, err
	}
	historyPath := filepath.Join(m.historyDir, historyKey(rel), name)
	content, err := os.ReadFile(historyPath)
	if err != nil {
		return nil, err
	}
	validation := m.Validate(rel, content)
	if !validation.Valid {
		return nil, fmt.Errorf("历史配置校验失败: %s", strings.Join(validation.Errors, "; "))
	}
	current, readErr := os.ReadFile(path)
	if readErr == nil {
		_ = m.archiveLocked(rel, current)
	}
	if err := atomicWrite(path, content, 0o600); err != nil {
		return nil, err
	}
	return &ConfigFile{
		Path:     filepath.ToSlash(rel),
		Content:  string(content),
		Revision: revision(content),
		Main:     samePath(path, m.mainPath),
		Format:   "toml",
	}, nil
}

func (m *ConfigManager) archiveLocked(rel string, content []byte) error {
	entryDir := filepath.Join(m.historyDir, historyKey(rel))
	if err := os.MkdirAll(entryDir, 0o700); err != nil {
		return err
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + ".toml"
	if err := atomicWrite(filepath.Join(entryDir, name), content, 0o600); err != nil {
		return err
	}
	entries, err := os.ReadDir(entryDir)
	if err != nil || len(entries) <= 20 {
		return err
	}
	for _, entry := range entries[:len(entries)-20] {
		_ = os.Remove(filepath.Join(entryDir, entry.Name()))
	}
	return nil
}

func (m *ConfigManager) updateMainIncludesLocked(path string, add bool) error {
	content, err := os.ReadFile(m.mainPath)
	if err != nil {
		return err
	}
	if strings.Contains(string(content), "{{") {
		return errors.New("主配置包含模板表达式，无法自动更新 includes；请在原始 TOML 中手动维护")
	}
	var root map[string]any
	if err := toml.Unmarshal(content, &root); err != nil {
		return fmt.Errorf("读取主配置 includes 失败: %w", err)
	}
	values := make([]string, 0)
	if current, ok := root["includes"]; ok {
		if typed, ok := current.([]any); ok {
			for _, item := range typed {
				if value, ok := item.(string); ok {
					values = append(values, value)
				}
			}
		}
	}
	filtered := make([]string, 0, len(values)+1)
	for _, value := range values {
		if add || filepath.ToSlash(value) != filepath.ToSlash(path) {
			filtered = append(filtered, value)
		}
	}
	if add {
		found := false
		for _, value := range filtered {
			if filepath.ToSlash(value) == filepath.ToSlash(path) {
				found = true
				break
			}
		}
		if !found {
			filtered = append(filtered, path)
		}
	}
	if len(filtered) == 0 {
		delete(root, "includes")
	} else {
		root["includes"] = filtered
	}
	updated, err := toml.Marshal(root)
	if err != nil {
		return err
	}
	res := m.Validate(filepath.ToSlash(relativePath(m.root, m.mainPath)), updated)
	if !res.Valid {
		return fmt.Errorf("更新 includes 后主配置校验失败: %s", strings.Join(res.Errors, "; "))
	}
	return atomicWrite(m.mainPath, updated, 0o600)
}

func (m *ConfigManager) resolve(rel string) (string, error) {
	rel = filepath.FromSlash(strings.TrimSpace(rel))
	if rel == "" {
		return "", errors.New("配置文件路径不能为空")
	}
	if filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("配置文件路径越界")
	}
	path := filepath.Clean(filepath.Join(m.root, rel))
	if !pathWithin(m.root, path) {
		return "", errors.New("配置文件路径越界")
	}
	if err := ensureNoSymlink(m.root, path); err != nil {
		return "", err
	}
	return path, nil
}

func (m *ConfigManager) HandleTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	files, err := m.ListFiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"root":  ".",
		"main":  filepath.ToSlash(relativePath(m.root, m.mainPath)),
		"files": files,
	})
}

func (m *ConfigManager) HandleFile(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	switch r.Method {
	case http.MethodGet:
		file, err := m.ReadFile(rel)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errConfigNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, file)
	case http.MethodPut:
		var body struct {
			Content  string `json:"content"`
			Revision string `json:"revision"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			return
		}
		file, err := m.WriteFile(rel, []byte(body.Content), body.Revision)
		if err != nil {
			status := http.StatusUnprocessableEntity
			if errors.Is(err, errRevision) {
				status = http.StatusConflict
			}
			writeError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, file)
	case http.MethodDelete:
		if err := m.DeleteFile(rel); err != nil {
			status := http.StatusUnprocessableEntity
			if errors.Is(err, errConfigNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
	}
}

func (m *ConfigManager) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Include *bool  `json:"include"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if body.Content == "" {
		if m.role == "server" {
			body.Content = `bindAddr = "0.0.0.0"` + "\n" + `bindPort = 7000` + "\n"
		} else {
			body.Content = `serverAddr = "127.0.0.1"` + "\n" + `serverPort = 7000` + "\n\n" +
				`webServer.addr = "0.0.0.0"` + "\n" + `webServer.port = 7400` + "\n" +
				`webServer.sessionAuth.enabled = true` + "\n"
		}
	}
	include := true
	if body.Include != nil {
		include = *body.Include
	}
	file, err := m.CreateFile(body.Path, []byte(body.Content), include)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, file)
}

func (m *ConfigManager) HandleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	var body struct {
		OldPath        string `json:"oldPath"`
		NewPath        string `json:"newPath"`
		UpdateIncludes *bool  `json:"updateIncludes"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	updateIncludes := true
	if body.UpdateIncludes != nil {
		updateIncludes = *body.UpdateIncludes
	}
	file, err := m.RenameFile(body.OldPath, body.NewPath, updateIncludes)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (m *ConfigManager) HandleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	writeJSON(w, http.StatusOK, m.Validate(body.Path, []byte(body.Content)))
}

func (m *ConfigManager) HandleHistory(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	switch r.Method {
	case http.MethodGet:
		entries, err := m.History(rel)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entries)
	case http.MethodPost:
		var body struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			return
		}
		file, err := m.RestoreHistory(body.Path, body.Name)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, file)
	default:
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
	}
}

func unsafeFeatures() *security.UnsafeFeatures {
	raw := strings.TrimSpace(os.Getenv("FRP_ALLOW_UNSAFE"))
	if raw == "" {
		return security.NewUnsafeFeatures(nil)
	}
	features := strings.Split(raw, ",")
	for i := range features {
		features[i] = strings.TrimSpace(features[i])
	}
	return security.NewUnsafeFeatures(features)
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
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
	return os.Rename(tmpName, path)
}

func revision(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func ensureNoSymlink(root, path string) error {
	current := root
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("配置目录不允许使用符号链接")
		}
	}
	return nil
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(aa) == filepath.Clean(bb)
}

func historyKey(rel string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(rel)))
	return hex.EncodeToString(sum[:8])
}

func formatTOMLError(err error) string {
	var decErr *toml.DecodeError
	if errors.As(err, &decErr) {
		row, col := decErr.Position()
		return fmt.Sprintf("TOML 第 %d 行、第 %d 列: %s", row, col, decErr.Error())
	}
	return err.Error()
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
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
