/*
Copyright 2023 The Flux authors

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

import "testing"

func TestDiffSet_HasType(t *testing.T) {
	tests := []struct {
		name string
		ds   DiffSet
		t    DiffType
		want bool
	}{
		{"contains", DiffSet{{Type: DiffTypeExclude}, {Type: DiffTypeCreate}}, DiffTypeCreate, true},
		{"does not contain", DiffSet{{Type: DiffTypeExclude}, {Type: DiffTypeCreate}}, DiffTypeUpdate, false},
		{"empty", DiffSet{}, DiffTypeNone, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ds.HasType(tt.t); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiffSet_HasChanges(t *testing.T) {
	tests := []struct {
		name string
		ds   DiffSet
		want bool
	}{
		{"has create", DiffSet{{Type: DiffTypeExclude}, {Type: DiffTypeCreate}}, true},
		{"has update", DiffSet{{Type: DiffTypeNone}, {Type: DiffTypeUpdate}}, true},
		{"no changes", DiffSet{{Type: DiffTypeNone}, {Type: DiffTypeExclude}}, false},
		{"empty", DiffSet{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ds.HasChanges(); got != tt.want {
				t.Errorf("HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}
