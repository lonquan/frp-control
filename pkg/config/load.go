// Copyright 2023 The frp Authors
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

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/yaml"

	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/msg"
	"github.com/fatedier/frp/pkg/util/jsonx"
	"github.com/fatedier/frp/pkg/util/util"
)

var glbEnvs map[string]string

func init() {
	glbEnvs = make(map[string]string)
	envs := os.Environ()
	for _, env := range envs {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}
		glbEnvs[pair[0]] = pair[1]
	}
}

type Values struct {
	Envs map[string]string // environment vars
}

func GetValues() *Values {
	return &Values{
		Envs: glbEnvs,
	}
}

func RenderWithTemplate(in []byte, values *Values) ([]byte, error) {
	tmpl, err := template.New("frp").Funcs(template.FuncMap{
		"parseNumberRange":     parseNumberRange,
		"parseNumberRangePair": parseNumberRangePair,
	}).Parse(string(in))
	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBufferString("")
	if err := tmpl.Execute(buffer, values); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func LoadFileContentWithTemplate(path string, values *Values) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return RenderWithTemplate(b, values)
}

func LoadConfigureFromFile(path string, c any, strict bool) error {
	if strings.EqualFold(filepath.Ext(path), ".ini") {
		return errors.New("INI configuration support has been removed; please migrate to TOML, YAML, or JSON")
	}
	content, err := LoadFileContentWithTemplate(path, GetValues())
	if err != nil {
		return err
	}
	return LoadConfigure(content, c, strict, detectFormatFromPath(path))
}

// detectFormatFromPath returns a format hint based on the file extension.
func detectFormatFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".toml":
		return "toml"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".ini":
		return "ini"
	default:
		return ""
	}
}

// parseYAMLWithDotFieldsHandling parses YAML with dot-prefixed fields handling
// This function handles both cases efficiently: with or without dot fields
func parseYAMLWithDotFieldsHandling(content []byte, target any) error {
	var temp any
	if err := yaml.Unmarshal(content, &temp); err != nil {
		return err
	}

	// Remove dot fields if it's a map
	if tempMap, ok := temp.(map[string]any); ok {
		for key := range tempMap {
			if strings.HasPrefix(key, ".") {
				delete(tempMap, key)
			}
		}
	}

	// Convert to JSON and decode with strict validation
	jsonBytes, err := jsonx.Marshal(temp)
	if err != nil {
		return err
	}
	return decodeJSONContent(jsonBytes, target, true)
}

func decodeJSONContent(content []byte, target any, strict bool) error {
	if clientCfg, ok := target.(*v1.ClientConfig); ok {
		decoded, err := v1.DecodeClientConfigJSON(content, v1.DecodeOptions{
			DisallowUnknownFields: strict,
		})
		if err != nil {
			return err
		}
		*clientCfg = decoded
		return nil
	}

	return jsonx.UnmarshalWithOptions(content, target, jsonx.DecodeOptions{
		RejectUnknownMembers: strict,
	})
}

// LoadConfigure loads configuration from bytes and unmarshal into c.
// Now it supports json, yaml and toml format.
// An optional format hint (e.g. "toml", "yaml", "json") can be provided
// to enable better error messages with line number information.
func LoadConfigure(b []byte, c any, strict bool, formats ...string) error {
	format := ""
	if len(formats) > 0 {
		format = formats[0]
	}
	if format == "ini" {
		return errors.New("INI configuration support has been removed; please migrate to TOML, YAML, or JSON")
	}

	originalBytes := b
	parsedFromTOML := false

	var tomlObj any
	tomlErr := toml.Unmarshal(b, &tomlObj)
	if tomlErr == nil {
		parsedFromTOML = true
		var err error
		b, err = jsonx.Marshal(&tomlObj)
		if err != nil {
			return err
		}
	} else if format == "toml" {
		// File is known to be TOML but has syntax errors.
		return formatTOMLError(tomlErr)
	}

	if err := rejectRemovedManagementConfigFields(b, c); err != nil {
		return err
	}

	// If the buffer smells like JSON (first non-whitespace character is '{'), unmarshal as JSON directly.
	if yaml.IsJSONBuffer(b) {
		if err := decodeJSONContent(b, c, strict); err != nil {
			return enhanceDecodeError(err, originalBytes, !parsedFromTOML)
		}
		return nil
	}

	// Handle YAML content
	if strict {
		// In strict mode, always use our custom handler to support YAML merge
		if err := parseYAMLWithDotFieldsHandling(b, c); err != nil {
			return enhanceDecodeError(err, originalBytes, !parsedFromTOML)
		}
		return nil
	}
	// Non-strict mode, parse normally
	return yaml.Unmarshal(b, c)
}

// formatTOMLError extracts line/column information from TOML decode errors.
func rejectRemovedManagementConfigFields(content []byte, target any) error {
	var fields []string
	switch target.(type) {
	case *v1.ServerConfig:
		fields = []string{
			"webServer.user",
			"webServer.password",
			"webServer.pprofEnable",
			"metricsServer",
			"enablePrometheus",
		}
	case *v1.ClientConfig, *v1.ClientCommonConfig:
		fields = []string{
			"webServer.user",
			"webServer.password",
			"webServer.pprofEnable",
			"metricsServer",
		}
	default:
		return nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal(content, &raw); err != nil {
		// The regular config decoder will return the format-specific error.
		return nil
	}
	for _, field := range fields {
		if !hasConfigField(raw, field) {
			continue
		}
		switch field {
		case "webServer.user", "webServer.password":
			return fmt.Errorf("configuration field %q has been removed; use FRP_ADMIN_USERNAME and FRP_ADMIN_PASSWORD for session authentication", field)
		case "webServer.pprofEnable":
			return fmt.Errorf("configuration field %q has been removed; use the authenticated diagnostics page instead", field)
		default:
			return fmt.Errorf("configuration field %q has been removed; use the authenticated dashboard monitoring and diagnostics instead", field)
		}
	}
	return nil
}

func hasConfigField(raw map[string]any, field string) bool {
	if _, ok := raw[field]; ok {
		return true
	}
	var current any = raw
	for part := range strings.SplitSeq(field, ".") {
		values, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = values[part]
		if !ok {
			return false
		}
	}
	return true
}

func formatTOMLError(err error) error {
	var decErr *toml.DecodeError
	if errors.As(err, &decErr) {
		row, col := decErr.Position()
		return fmt.Errorf("toml: line %d, column %d: %s", row, col, decErr.Error())
	}
	var strictErr *toml.StrictMissingError
	if errors.As(err, &strictErr) {
		return strictErr
	}
	return err
}

// enhanceDecodeError tries to add field path and line number information to JSON/YAML decode errors.
func enhanceDecodeError(err error, originalContent []byte, includeLine bool) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		if includeLine {
			line := findFieldLineInContent(originalContent, typeErr.Field)
			if line > 0 {
				return fmt.Errorf("line %d: field \"%s\": cannot unmarshal %s into %s", line, typeErr.Field, typeErr.Value, typeErr.Type)
			}
		}
		return fmt.Errorf("field \"%s\": cannot unmarshal %s into %s", typeErr.Field, typeErr.Value, typeErr.Type)
	}
	return err
}

// findFieldLineInContent searches the original config content for a field name
// and returns the 1-indexed line number where it appears, or 0 if not found.
func findFieldLineInContent(content []byte, fieldPath string) int {
	if fieldPath == "" {
		return 0
	}

	// Use the last component of the field path (e.g. "proxies" from "proxies" or
	// "protocol" from "transport.protocol").
	parts := strings.Split(fieldPath, ".")
	searchKey := parts[len(parts)-1]

	lines := bytes.Split(content, []byte("\n"))
	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)
		// Match TOML key assignments like: key = ...
		if bytes.HasPrefix(trimmed, []byte(searchKey)) {
			rest := bytes.TrimSpace(trimmed[len(searchKey):])
			if len(rest) > 0 && rest[0] == '=' {
				return i + 1
			}
		}
		// Match TOML table array headers like: [[proxies]]
		if bytes.Contains(trimmed, []byte("[["+searchKey+"]]")) {
			return i + 1
		}
	}
	return 0
}

func NewProxyConfigurerFromMsg(m *msg.NewProxy, serverCfg *v1.ServerConfig) (v1.ProxyConfigurer, error) {
	m.ProxyType = util.EmptyOr(m.ProxyType, string(v1.ProxyTypeTCP))

	configurer := v1.NewProxyConfigurerByType(v1.ProxyType(m.ProxyType))
	if configurer == nil {
		return nil, fmt.Errorf("unknown proxy type: %s", m.ProxyType)
	}

	configurer.UnmarshalFromMsg(m)
	configurer.Complete()

	if err := validation.ValidateProxyConfigurerForServer(configurer, serverCfg); err != nil {
		return nil, err
	}
	return configurer, nil
}

func LoadServerConfig(path string, strict bool) (*v1.ServerConfig, error) {
	svrCfg := &v1.ServerConfig{}
	if err := LoadConfigureFromFile(path, svrCfg, strict); err != nil {
		return nil, err
	}
	if err := svrCfg.Complete(); err != nil {
		return nil, err
	}
	return svrCfg, nil
}

// ClientConfigLoadResult contains the result of loading a client configuration file.
type ClientConfigLoadResult struct {
	// Common contains the common client configuration.
	Common *v1.ClientCommonConfig

	// Proxies contains proxy configurations from inline [[proxies]] and includeConfigFiles.
	// These are NOT completed (user prefix not added).
	Proxies []v1.ProxyConfigurer

	// Visitors contains visitor configurations from inline [[visitors]] and includeConfigFiles.
	// These are NOT completed.
	Visitors []v1.VisitorConfigurer
}

// LoadClientConfigResult loads and parses a client configuration file.
// It returns the raw configuration without completing proxies/visitors.
func LoadClientConfigResult(path string, strict bool) (*ClientConfigLoadResult, error) {
	result := &ClientConfigLoadResult{
		Proxies:  make([]v1.ProxyConfigurer, 0),
		Visitors: make([]v1.VisitorConfigurer, 0),
	}

	allCfg := v1.ClientConfig{}
	if err := LoadConfigureFromFile(path, &allCfg, strict); err != nil {
		return nil, err
	}
	result.Common = &allCfg.ClientCommonConfig
	for _, c := range allCfg.Proxies {
		result.Proxies = append(result.Proxies, c.ProxyConfigurer)
	}
	for _, c := range allCfg.Visitors {
		result.Visitors = append(result.Visitors, c.VisitorConfigurer)
	}

	// Load additional config from includes.
	if len(result.Common.IncludeConfigFiles) > 0 {
		extProxyCfgs, extVisitorCfgs, err := LoadAdditionalClientConfigs(result.Common.IncludeConfigFiles, strict)
		if err != nil {
			return nil, err
		}
		result.Proxies = append(result.Proxies, extProxyCfgs...)
		result.Visitors = append(result.Visitors, extVisitorCfgs...)
	}

	// Complete the common config
	if result.Common != nil {
		if err := result.Common.Complete(); err != nil {
			return nil, err
		}
	}

	if err := validateNoDuplicateNames(result.Proxies, result.Visitors); err != nil {
		return nil, err
	}

	return result, nil
}

func LoadClientConfig(path string, strict bool) (
	*v1.ClientCommonConfig,
	[]v1.ProxyConfigurer,
	[]v1.VisitorConfigurer,
	error,
) {
	result, err := LoadClientConfigResult(path, strict)
	if err != nil {
		return nil, nil, nil, err
	}

	proxyCfgs := result.Proxies
	visitorCfgs := result.Visitors

	proxyCfgs, visitorCfgs = FilterClientConfigurers(result.Common, proxyCfgs, visitorCfgs)
	proxyCfgs = CompleteProxyConfigurers(proxyCfgs)
	visitorCfgs = CompleteVisitorConfigurers(visitorCfgs)
	return result.Common, proxyCfgs, visitorCfgs, nil
}

// validateNoDuplicateNames rejects proxies or visitors that share a name. They are
// keyed by name in the config sources, so a duplicate would otherwise be silently
// overwritten and never started, with no error or log.
func validateNoDuplicateNames(proxies []v1.ProxyConfigurer, visitors []v1.VisitorConfigurer) error {
	proxyNames := make(map[string]struct{}, len(proxies))
	for _, p := range proxies {
		name := p.GetBaseConfig().Name
		if _, ok := proxyNames[name]; ok {
			return fmt.Errorf("proxy name [%s] is duplicated", name)
		}
		proxyNames[name] = struct{}{}
	}

	visitorNames := make(map[string]struct{}, len(visitors))
	for _, v := range visitors {
		name := v.GetBaseConfig().Name
		if _, ok := visitorNames[name]; ok {
			return fmt.Errorf("visitor name [%s] is duplicated", name)
		}
		visitorNames[name] = struct{}{}
	}

	return nil
}

func CompleteProxyConfigurers(proxies []v1.ProxyConfigurer) []v1.ProxyConfigurer {
	proxyCfgs := proxies
	for _, c := range proxyCfgs {
		c.Complete()
	}
	return proxyCfgs
}

func CompleteVisitorConfigurers(visitors []v1.VisitorConfigurer) []v1.VisitorConfigurer {
	visitorCfgs := visitors
	for _, c := range visitorCfgs {
		c.Complete()
	}
	return visitorCfgs
}

func FilterClientConfigurers(
	common *v1.ClientCommonConfig,
	proxies []v1.ProxyConfigurer,
	visitors []v1.VisitorConfigurer,
) ([]v1.ProxyConfigurer, []v1.VisitorConfigurer) {
	if common == nil {
		common = &v1.ClientCommonConfig{}
	}

	proxyCfgs := proxies
	visitorCfgs := visitors

	// Filter by start across merged configurers from all sources.
	// For example, store entries are also filtered by this set.
	if len(common.Start) > 0 {
		startSet := sets.New(common.Start...)
		proxyCfgs = lo.Filter(proxyCfgs, func(c v1.ProxyConfigurer, _ int) bool {
			return startSet.Has(c.GetBaseConfig().Name)
		})
		visitorCfgs = lo.Filter(visitorCfgs, func(c v1.VisitorConfigurer, _ int) bool {
			return startSet.Has(c.GetBaseConfig().Name)
		})
	}

	// Filter by enabled field in each proxy
	// nil or true means enabled, false means disabled
	proxyCfgs = lo.Filter(proxyCfgs, func(c v1.ProxyConfigurer, _ int) bool {
		enabled := c.GetBaseConfig().Enabled
		return enabled == nil || *enabled
	})
	visitorCfgs = lo.Filter(visitorCfgs, func(c v1.VisitorConfigurer, _ int) bool {
		enabled := c.GetBaseConfig().Enabled
		return enabled == nil || *enabled
	})
	return proxyCfgs, visitorCfgs
}

func LoadAdditionalClientConfigs(paths []string, strict bool) ([]v1.ProxyConfigurer, []v1.VisitorConfigurer, error) {
	proxyCfgs := make([]v1.ProxyConfigurer, 0)
	visitorCfgs := make([]v1.VisitorConfigurer, 0)
	for _, path := range paths {
		absDir, err := filepath.Abs(filepath.Dir(path))
		if err != nil {
			return nil, nil, err
		}
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			return nil, nil, err
		}
		files, err := os.ReadDir(absDir)
		if err != nil {
			return nil, nil, err
		}
		for _, fi := range files {
			if fi.IsDir() {
				continue
			}
			absFile := filepath.Join(absDir, fi.Name())
			if matched, _ := filepath.Match(filepath.Join(absDir, filepath.Base(path)), absFile); matched {
				// support yaml/json/toml
				cfg := v1.ClientConfig{}
				if err := LoadConfigureFromFile(absFile, &cfg, strict); err != nil {
					return nil, nil, fmt.Errorf("load additional config from %s error: %v", absFile, err)
				}
				for _, c := range cfg.Proxies {
					proxyCfgs = append(proxyCfgs, c.ProxyConfigurer)
				}
				for _, c := range cfg.Visitors {
					visitorCfgs = append(visitorCfgs, c.VisitorConfigurer)
				}
			}
		}
	}
	return proxyCfgs, visitorCfgs, nil
}
