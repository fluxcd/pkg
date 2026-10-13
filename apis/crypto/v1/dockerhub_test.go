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

package v1

import "testing"

func TestDockerHubOIDCConnectionIDExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantError bool
	}{
		{
			name: "valid",
			ann:  map[string]string{AnnotationDockerHubOIDCConnectionID.String(): testUUID},
			want: testUUID,
		},
		{name: "missing", ann: map[string]string{}, wantError: true},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationDockerHubOIDCConnectionID.String(): "nope"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnnotationDockerHubOIDCConnectionID.Extract(tt.ann)
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v), want (%q, err=%v)", got, err, tt.want, tt.wantError)
			}
		})
	}
}
