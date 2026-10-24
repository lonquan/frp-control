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
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

//go:embed dns_providers_generated.json
var dnsProviderRegistryJSON []byte

type DNSProviderField struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

type DNSProviderDefinition struct {
	Code   string             `json:"code"`
	Name   string             `json:"name"`
	URL    string             `json:"url,omitempty"`
	Fields []DNSProviderField `json:"fields,omitempty"`
}

type DNSProviderCredentialView struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Configured bool   `json:"configured"`
}

type DNSProviderConfigView struct {
	Configured   bool                        `json:"configured"`
	Provider     string                      `json:"provider,omitempty"`
	ProviderName string                      `json:"providerName,omitempty"`
	Credentials  []DNSProviderCredentialView `json:"credentials"`
	UpdatedAt    time.Time                   `json:"updatedAt,omitempty"`
}

type saveDNSProviderRequest struct {
	Provider    string            `json:"provider"`
	Credentials map[string]string `json:"credentials"`
}

type dnsProviderRegistry struct {
	Version   int                     `json:"version"`
	LegoVer   string                  `json:"legoVersion"`
	Providers []dnsProviderDefinition `json:"providers"`
	byCode    map[string]*dnsProviderDefinition
	envNames  []string
}

type dnsProviderDefinition struct {
	Code        string             `json:"code"`
	Name        string             `json:"name"`
	URL         string             `json:"url,omitempty"`
	Fields      []DNSProviderField `json:"fields,omitempty"`
	Environment []string           `json:"environment,omitempty"`
}

type dnsProviderConfigState struct {
	Version     int               `json:"version"`
	Provider    string            `json:"provider"`
	Credentials map[string]string `json:"credentials"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type providerCheckRequest struct {
	Provider    string            `json:"provider"`
	ProviderEnv map[string]string `json:"providerEnv"`
}

func loadDNSProviderRegistry() (*dnsProviderRegistry, error) {
	var registry dnsProviderRegistry
	if err := json.Unmarshal(dnsProviderRegistryJSON, &registry); err != nil {
		return nil, fmt.Errorf("解析 DNS provider 注册表失败: %w", err)
	}
	if registry.Version != 1 || len(registry.Providers) == 0 {
		return nil, errors.New("DNS provider 注册表为空或版本不支持")
	}
	registry.byCode = make(map[string]*dnsProviderDefinition, len(registry.Providers))
	envSet := make(map[string]struct{})
	for i := range registry.Providers {
		provider := &registry.Providers[i]
		registry.byCode[provider.Code] = provider
		for _, name := range provider.Environment {
			envSet[name] = struct{}{}
		}
	}
	registry.envNames = make([]string, 0, len(envSet))
	for name := range envSet {
		registry.envNames = append(registry.envNames, name)
	}
	sort.Strings(registry.envNames)
	return &registry, nil
}

func (r *dnsProviderRegistry) list() []DNSProviderDefinition {
	result := make([]DNSProviderDefinition, 0, len(r.Providers))
	for _, provider := range r.Providers {
		result = append(result, DNSProviderDefinition{
			Code:   provider.Code,
			Name:   provider.Name,
			URL:    provider.URL,
			Fields: append([]DNSProviderField(nil), provider.Fields...),
		})
	}
	return result
}

func (r *dnsProviderRegistry) provider(code string) *dnsProviderDefinition {
	return r.byCode[strings.TrimSpace(code)]
}

func (m *CertificateManager) RegisterDNSProviderRoutes(router *mux.Router) {
	router.HandleFunc("/api/v2/certificates/dns-providers", m.handleDNSProviders).Methods(http.MethodGet)
	router.HandleFunc("/api/v2/certificates/dns-provider", m.handleDNSProvider).Methods(http.MethodGet, http.MethodPut, http.MethodDelete)
}

func (m *CertificateManager) handleDNSProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, m.providerRegistry.list())
}

func (m *CertificateManager) handleDNSProvider(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, m.dnsProviderConfigView())
	case http.MethodPut:
		var body saveDNSProviderRequest
		if err := decodeJSON(w, r, &body); err != nil {
			return
		}
		view, err := m.saveDNSProviderConfig(body)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, view)
	case http.MethodDelete:
		if err := m.clearDNSProviderConfig(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
	}
}

func (m *CertificateManager) dnsProviderConfigView() DNSProviderConfigView {
	m.providerMu.Lock()
	defer m.providerMu.Unlock()
	return m.dnsProviderConfigViewLocked()
}

func (m *CertificateManager) dnsProviderConfigViewLocked() DNSProviderConfigView {
	view := DNSProviderConfigView{
		Configured:  m.providerConfig.Provider != "",
		Provider:    m.providerConfig.Provider,
		UpdatedAt:   m.providerConfig.UpdatedAt,
		Credentials: []DNSProviderCredentialView{},
	}
	if !view.Configured {
		return view
	}
	provider := m.providerRegistry.provider(view.Provider)
	if provider == nil {
		return view
	}
	view.ProviderName = provider.Name
	for _, field := range provider.Fields {
		view.Credentials = append(view.Credentials, DNSProviderCredentialView{
			Name:       field.Name,
			Label:      field.Label,
			Configured: strings.TrimSpace(m.providerConfig.Credentials[field.Name]) != "",
		})
	}
	return view
}

func (m *CertificateManager) saveDNSProviderConfig(input saveDNSProviderRequest) (DNSProviderConfigView, error) {
	code := strings.TrimSpace(input.Provider)
	provider := m.providerRegistry.provider(code)
	if provider == nil {
		return DNSProviderConfigView{}, fmt.Errorf("不支持的 DNS provider: %s", code)
	}
	allowed := make(map[string]struct{}, len(provider.Fields))
	for _, field := range provider.Fields {
		allowed[field.Name] = struct{}{}
	}
	for name := range input.Credentials {
		if _, ok := allowed[name]; !ok {
			return DNSProviderConfigView{}, fmt.Errorf("DNS provider %s 不支持凭据字段 %s", code, name)
		}
	}

	m.providerMu.Lock()
	defer m.providerMu.Unlock()

	credentials := make(map[string]string)
	if m.providerConfig.Provider == code {
		maps.Copy(credentials, m.providerConfig.Credentials)
	}
	for name, value := range input.Credentials {
		if value != "" {
			credentials[name] = value
		}
	}
	if err := m.checkDNSProviderConfig(code, credentials); err != nil {
		return DNSProviderConfigView{}, err
	}

	state := dnsProviderConfigState{
		Version:     1,
		Provider:    code,
		Credentials: credentials,
		UpdatedAt:   time.Now(),
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return DNSProviderConfigView{}, err
	}
	b = append(b, '\n')
	if err := atomicWrite(m.providerConfigPath, b); err != nil {
		return DNSProviderConfigView{}, err
	}
	m.providerConfig = state
	return m.dnsProviderConfigViewLocked(), nil
}

func (m *CertificateManager) clearDNSProviderConfig() error {
	m.providerMu.Lock()
	defer m.providerMu.Unlock()
	if err := os.Remove(m.providerConfigPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.providerConfig = dnsProviderConfigState{}
	return nil
}

func (m *CertificateManager) loadDNSProviderConfig() error {
	b, err := os.ReadFile(m.providerConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state dnsProviderConfigState
	if err := json.Unmarshal(b, &state); err != nil {
		return fmt.Errorf("解析 DNS provider 配置失败: %w", err)
	}
	if state.Version != 1 || state.Provider == "" {
		return errors.New("DNS provider 配置版本无效")
	}
	provider := m.providerRegistry.provider(state.Provider)
	if provider == nil {
		return fmt.Errorf("DNS provider 配置引用了不支持的厂商: %s", state.Provider)
	}
	if state.Credentials == nil {
		state.Credentials = make(map[string]string)
	}
	allowed := make(map[string]struct{}, len(provider.Fields))
	for _, field := range provider.Fields {
		allowed[field.Name] = struct{}{}
	}
	for name := range state.Credentials {
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("DNS provider 配置包含未知凭据字段: %s", name)
		}
	}
	m.providerConfig = state
	return nil
}

func (m *CertificateManager) dnsProviderSnapshot() (dnsProviderConfigState, error) {
	m.providerMu.Lock()
	defer m.providerMu.Unlock()
	if m.providerConfig.Provider == "" {
		return dnsProviderConfigState{}, errDNSProviderNotConfigured
	}
	state := dnsProviderConfigState{
		Version:     m.providerConfig.Version,
		Provider:    m.providerConfig.Provider,
		Credentials: make(map[string]string, len(m.providerConfig.Credentials)),
		UpdatedAt:   m.providerConfig.UpdatedAt,
	}
	maps.Copy(state.Credentials, m.providerConfig.Credentials)
	return state, nil
}

func (m *CertificateManager) checkDNSProviderConfig(provider string, credentials map[string]string) error {
	if m.providerCheck != nil {
		return m.providerCheck(provider, credentials)
	}
	request := providerCheckRequest{
		Provider:    provider,
		ProviderEnv: copyStringMap(credentials),
	}
	b, err := json.Marshal(request)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(m.dataDir, ".provider-check-*.json")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(b); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.helperPath, "--validate-provider", path)
	cmd.Env = m.helperEnvironment(credentials)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("DNS provider 配置预检超时")
		}
		message := strings.TrimSpace(output.String())
		if message == "" {
			return fmt.Errorf("DNS provider 配置预检失败: %w", err)
		}
		return fmt.Errorf("DNS provider 配置预检失败: %s", message)
	}
	return nil
}

func (m *CertificateManager) helperEnvironment(credentials map[string]string) []string {
	blocked := make(map[string]struct{}, len(m.providerRegistry.envNames))
	for _, name := range m.providerRegistry.envNames {
		blocked[name] = struct{}{}
	}
	env := make([]string, 0, len(os.Environ())+len(credentials))
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, skip := blocked[name]; skip {
			continue
		}
		env = append(env, entry)
	}
	names := make([]string, 0, len(credentials))
	for name := range credentials {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		env = append(env, name+"="+credentials[name])
	}
	return env
}

func copyStringMap(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	return maps.Clone(input)
}
