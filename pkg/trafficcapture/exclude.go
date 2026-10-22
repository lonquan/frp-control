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
	"fmt"
	"regexp"
	"strings"
)

const (
	maxExcludePatterns    = 100
	maxExcludePatternSize = 256
)

type excludeMatcher []*regexp.Regexp

func normalizeExcludePatterns(patterns []string) ([]string, error) {
	normalized := make([]string, 0, len(patterns))
	seen := make(map[string]struct{}, len(patterns))
	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		if len([]rune(pattern)) > maxExcludePatternSize {
			return nil, fmt.Errorf("排除关键字单条不能超过 %d 个字符", maxExcludePatternSize)
		}
		key := strings.ToLower(pattern)
		if _, ok := seen[key]; ok {
			continue
		}
		if len(normalized) >= maxExcludePatterns {
			return nil, fmt.Errorf("排除关键字最多支持 %d 条", maxExcludePatterns)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, pattern)
	}
	return normalized, nil
}

func compileExcludePatterns(patterns []string) excludeMatcher {
	matcher := make(excludeMatcher, 0, len(patterns))
	for _, pattern := range patterns {
		expression := regexp.QuoteMeta(pattern)
		expression = strings.ReplaceAll(expression, `\*`, `.*`)
		expression = strings.ReplaceAll(expression, `\?`, `.`)
		matcher = append(matcher, regexp.MustCompile(`(?i)`+expression))
	}
	return matcher
}

func (m excludeMatcher) matches(path, rawQuery string) bool {
	target := path
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	for _, pattern := range m {
		if pattern.MatchString(target) {
			return true
		}
	}
	return false
}
