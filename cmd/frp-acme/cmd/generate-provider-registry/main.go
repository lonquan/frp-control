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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const legoModule = "github.com/go-acme/lego/v5"

type providerDocument struct {
	Name          string `toml:"Name"`
	Code          string `toml:"Code"`
	URL           string `toml:"URL"`
	Configuration providerConfiguration
}

type providerConfiguration struct {
	Credentials map[string]string `toml:"Credentials"`
	Additional  map[string]string `toml:"Additional"`
}

type providerRegistry struct {
	Version   int                `json:"version"`
	LegoVer   string             `json:"legoVersion"`
	Providers []providerMetadata `json:"providers"`
}

type providerMetadata struct {
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	URL         string          `json:"url,omitempty"`
	Fields      []providerField `json:"fields,omitempty"`
	Environment []string        `json:"environment,omitempty"`
}

type providerField struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

func main() {
	output := flag.String("output", "../../pkg/admin/dns_providers_generated.json", "generated registry path")
	check := flag.Bool("check", false, "check that the generated registry is current")
	flag.Parse()

	registry, err := generate()
	if err != nil {
		panic(err)
	}
	content, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		panic(err)
	}
	content = append(content, '\n')

	if *check {
		current, err := os.ReadFile(*output)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(current, content) {
			panic("DNS provider registry is stale; run make generate-dns-providers")
		}
		return
	}
	if err := os.WriteFile(*output, content, 0o644); err != nil {
		panic(err)
	}
}

func generate() (*providerRegistry, error) {
	legoDir, version, err := moduleInfo()
	if err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(legoDir, "providers", "dns", "*", "*.toml"))
	if err != nil {
		return nil, err
	}
	providers := make([]providerMetadata, 0, len(paths))
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var doc providerDocument
		if err := toml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if doc.Code == "" || doc.Code == "manual" || doc.Code == "exec" {
			continue
		}
		fields := make([]providerField, 0, len(doc.Configuration.Credentials))
		for name, label := range doc.Configuration.Credentials {
			fields = append(fields, providerField{
				Name:  name,
				Label: strings.TrimSpace(label),
			})
		}
		sort.Slice(fields, func(i, j int) bool {
			return fields[i].Name < fields[j].Name
		})

		environment := make([]string, 0, len(doc.Configuration.Credentials)+len(doc.Configuration.Additional))
		for name := range doc.Configuration.Credentials {
			environment = append(environment, name)
		}
		for name := range doc.Configuration.Additional {
			environment = append(environment, name)
		}
		sort.Strings(environment)
		environment = compactStrings(environment)

		providers = append(providers, providerMetadata{
			Code:        doc.Code,
			Name:        strings.TrimSpace(doc.Name),
			URL:         strings.TrimSpace(doc.URL),
			Fields:      fields,
			Environment: environment,
		})
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Code < providers[j].Code
	})
	return &providerRegistry{
		Version:   1,
		LegoVer:   version,
		Providers: providers,
	}, nil
}

func moduleInfo() (string, string, error) {
	output, err := exec.Command(
		"go",
		"list",
		"-m",
		"-f",
		"{{.Dir}}\t{{.Version}}",
		legoModule,
	).Output()
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected lego module information: %q", output)
	}
	return parts[0], parts[1], nil
}

func compactStrings(values []string) []string {
	result := values[:0]
	for i, value := range values {
		if i == 0 || value != values[i-1] {
			result = append(result, value)
		}
	}
	return result
}
