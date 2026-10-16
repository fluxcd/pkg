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

import (
	"reflect"
	"regexp"
	"testing"
)

func TestFindRFC0014Annotation(t *testing.T) {
	tests := []struct {
		name string
		ann  map[string]string
		want string
		ok   bool
	}{
		{name: "nil map"},
		{
			name: "rfc0014 auth annotation",
			ann:  map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE},
			want: AnnotationCredentialProvider.String(),
			ok:   true,
		},
		{
			name: "rfc0014 mtls annotation",
			ann:  map[string]string{AnnotationMTLSSPIFFEServerID.String(): "spiffe://example.org"},
			want: AnnotationMTLSSPIFFEServerID.String(),
			ok:   true,
		},
		{
			name: "gcp legacy annotation excluded",
			ann:  map[string]string{AnnotationGCPWorkloadIdentityProvider.String(): "x"},
		},
		{
			name: "non flux annotation",
			ann:  map[string]string{"example.com/foo": "x"},
		},
		{
			name: "key without slash",
			ann:  map[string]string{"justakey": "x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FindRFC0014Annotation(tt.ann)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestExtractServiceAccountAnnotation(t *testing.T) {
	regex := regexp.MustCompile(`^ok$`)
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{name: "valid", ann: map[string]string{"key": "ok"}, want: "ok", wantOk: true},
		{name: "missing", ann: map[string]string{}},
		{name: "invalid", ann: map[string]string{"key": "nope"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := extractServiceAccountAnnotation("key", tt.ann, regex)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestParseKubeConfigCluster(t *testing.T) {
	regex := regexp.MustCompile(`^(a)/(b)$`)
	tests := []struct {
		name      string
		value     string
		want      string
		wantMatch []string
		wantError bool
	}{
		{
			name:      "valid",
			value:     "a/b",
			want:      "a/b",
			wantMatch: []string{"a/b", "a", "b"},
		},
		{name: "empty", wantError: true},
		{name: "invalid", value: "a/c", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, matches, err := parseKubeConfigCluster(tt.value, regex)
			if got != tt.want || (err != nil) != tt.wantError ||
				!reflect.DeepEqual(matches, tt.wantMatch) {
				t.Fatalf("got (%q, %v, %v), want (%q, err=%v, matches=%v)",
					got, matches, err, tt.want, tt.wantError, tt.wantMatch)
			}
		})
	}
}
