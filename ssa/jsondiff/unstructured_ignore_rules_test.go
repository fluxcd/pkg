/*
Copyright 2026 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jsondiff

import (
	"reflect"
	"testing"
)

func TestCompileIgnoreRules(t *testing.T) {
	tests := []struct {
		name    string
		rules   []IgnoreRule
		wantErr bool
	}{
		{
			name:  "nil rules",
			rules: nil,
		},
		{
			name: "valid pointer",
			rules: []IgnoreRule{
				{Paths: []string{"/spec/replicas"}},
			},
		},
		{
			name: "multiple valid pointers in one rule",
			rules: []IgnoreRule{
				{Paths: []string{"/spec/replicas", "/metadata/labels/foo"}},
			},
		},
		{
			name: "root sentinel is valid",
			rules: []IgnoreRule{
				{Paths: []string{IgnorePathRoot}},
			},
		},
		{
			name: "escaped tokens are valid",
			rules: []IgnoreRule{
				{Paths: []string{"/metadata/annotations/example.com~1config"}},
			},
		},
		{
			name: "valid selector and pointer",
			rules: []IgnoreRule{
				{
					Paths:    []string{"/spec/replicas"},
					Selector: &Selector{Kind: "Deployment"},
				},
			},
		},
		{
			name: "non-empty pointer missing leading slash",
			rules: []IgnoreRule{
				{Paths: []string{"spec/replicas"}},
			},
			wantErr: true,
		},
		{
			name: "one invalid pointer among valid ones",
			rules: []IgnoreRule{
				{Paths: []string{"/spec/replicas", "metadata/labels"}},
			},
			wantErr: true,
		},
		{
			name: "invalid pointer in second rule",
			rules: []IgnoreRule{
				{Paths: []string{"/spec/replicas"}},
				{Paths: []string{"bad"}},
			},
			wantErr: true,
		},
		{
			name: "invalid selector",
			rules: []IgnoreRule{
				{
					Paths:    []string{"/spec/replicas"},
					Selector: &Selector{LabelSelector: "!!!"},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := CompileIgnoreRules(tt.rules)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CompileIgnoreRules() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if compiled != nil {
					t.Errorf("CompileIgnoreRules() = %v, want nil on error", compiled)
				}
				return
			}
			if got := len(compiled); got != len(tt.rules) {
				t.Errorf("CompileIgnoreRules() compiled %d rules, want %d", got, len(tt.rules))
			}
			// Each compiled rule must preserve its original paths.
			gotPaths := make([][]string, 0, len(compiled))
			for _, paths := range compiled {
				gotPaths = append(gotPaths, paths)
			}
			for _, rule := range tt.rules {
				found := false
				for _, paths := range gotPaths {
					if reflect.DeepEqual(paths, rule.Paths) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("CompileIgnoreRules() missing paths %v in result", rule.Paths)
				}
			}
		})
	}
}
