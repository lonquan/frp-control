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

package version

import "testing"

func TestDisplay(t *testing.T) {
	original := distributionVersion
	t.Cleanup(func() {
		distributionVersion = original
	})

	tests := []struct {
		name         string
		distribution string
		wantFull     string
		wantDisplay  string
	}{
		{
			name:        "local build",
			wantFull:    "0.71.1",
			wantDisplay: "0.71.1",
		},
		{
			name:         "distribution build",
			distribution: "0.0.1",
			wantFull:     "0.71.1",
			wantDisplay:  "0.0.1 (frp 0.71.1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distributionVersion = tt.distribution
			if got := Full(); got != tt.wantFull {
				t.Fatalf("Full() = %q, want %q", got, tt.wantFull)
			}
			if got := Distribution(); got != tt.distribution {
				t.Fatalf("Distribution() = %q, want %q", got, tt.distribution)
			}
			if got := Display(); got != tt.wantDisplay {
				t.Fatalf("Display() = %q, want %q", got, tt.wantDisplay)
			}
		})
	}
}
