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

package leaderelection

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		additions []string
		want      string
	}{
		{
			name: "no additions",
			base: "foo",
			want: "foo",
		},
		{
			name:      "one addition",
			base:      "foo",
			additions: []string{"bar"},
			want:      "foo-fcde2b2e",
		},
		{
			name:      "multiple additions",
			base:      "foo",
			additions: []string{"bar", "baz"},
			want:      "foo-c8f8b724",
		},
		{
			name:      "multiple additions with empty string",
			base:      "foo",
			additions: []string{"bar", "", "baz"},
			want:      "foo-c8f8b724",
		},
		{
			name:      "multiple additions with empty base",
			additions: []string{"bar", "baz"},
			want:      "",
		},
		{
			name: "empty base and additions",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(GenerateID(tt.base, tt.additions...)).To(Equal(tt.want))
		})
	}
}
