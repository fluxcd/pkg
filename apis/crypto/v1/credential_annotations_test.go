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
	"testing"
	"time"
)

func TestCredentialProviderExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "kubernetes",
			ann:    map[string]string{AnnotationCredentialProvider.String(): CredentialProviderKubernetes},
			want:   CredentialProviderKubernetes,
			wantOk: true,
		},
		{
			name:   "spiffe",
			ann:    map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE},
			want:   CredentialProviderSPIFFE,
			wantOk: true,
		},
		{name: "missing", ann: map[string]string{}},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationCredentialProvider.String(): "nope"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationCredentialProvider.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestCredentialTypeExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "jwt",
			ann:    map[string]string{AnnotationCredentialType.String(): CredentialTypeJWT},
			want:   CredentialTypeJWT,
			wantOk: true,
		},
		{
			name:   "x509",
			ann:    map[string]string{AnnotationCredentialType.String(): CredentialTypeX509},
			want:   CredentialTypeX509,
			wantOk: true,
		},
		{name: "missing", ann: map[string]string{}},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationCredentialType.String(): "nope"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationCredentialType.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestCredentialExpirationExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      time.Duration
		wantOk    bool
		wantError bool
	}{
		{name: "missing", ann: map[string]string{}},
		{
			name:   "valid",
			ann:    map[string]string{AnnotationCredentialExpiration.String(): "5m"},
			want:   5 * time.Minute,
			wantOk: true,
		},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationCredentialExpiration.String(): "nope"},
			wantError: true,
		},
		{
			name:      "zero",
			ann:       map[string]string{AnnotationCredentialExpiration.String(): "0s"},
			wantError: true,
		},
		{
			name:      "negative",
			ann:       map[string]string{AnnotationCredentialExpiration.String(): "-1s"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationCredentialExpiration.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%v, %v, %v), want (%v, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestCredentialUsernameExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "valid",
			ann:    map[string]string{AnnotationCredentialUsername.String(): "my-user"},
			want:   "my-user",
			wantOk: true,
		},
		{name: "missing", ann: map[string]string{}},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationCredentialUsername.String(): "my user"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationCredentialUsername.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestExtractCredentialProviderAndType(t *testing.T) {
	tests := []struct {
		name         string
		ann          map[string]string
		specProvider string
		wantProvider string
		wantType     string
		wantOk       bool
		wantError    bool
	}{
		{name: "neither", ann: map[string]string{}},
		{
			name:      "invalid provider",
			ann:       map[string]string{AnnotationCredentialProvider.String(): "nope", AnnotationCredentialType.String(): CredentialTypeJWT},
			wantError: true,
		},
		{
			name:      "invalid type",
			ann:       map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE, AnnotationCredentialType.String(): "nope"},
			wantError: true,
		},
		{
			name:      "provider only",
			ann:       map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE},
			wantError: true,
		},
		{
			name:      "type only",
			ann:       map[string]string{AnnotationCredentialType.String(): CredentialTypeJWT},
			wantError: true,
		},
		{
			name:         "kubernetes jwt",
			ann:          map[string]string{AnnotationCredentialProvider.String(): CredentialProviderKubernetes, AnnotationCredentialType.String(): CredentialTypeJWT},
			wantProvider: CredentialProviderKubernetes,
			wantType:     CredentialTypeJWT,
			wantOk:       true,
		},
		{
			name:      "kubernetes x509",
			ann:       map[string]string{AnnotationCredentialProvider.String(): CredentialProviderKubernetes, AnnotationCredentialType.String(): CredentialTypeX509},
			wantError: true,
		},
		{
			name:         "azure x509",
			ann:          map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE, AnnotationCredentialType.String(): CredentialTypeX509},
			specProvider: ProviderAzure,
			wantError:    true,
		},
		{
			name:         "aws x509",
			ann:          map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE, AnnotationCredentialType.String(): CredentialTypeX509},
			specProvider: ProviderAWS,
			wantProvider: CredentialProviderSPIFFE,
			wantType:     CredentialTypeX509,
			wantOk:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, credType, ok, err := ExtractCredentialProviderAndType(tt.ann, tt.specProvider)
			if provider != tt.wantProvider || credType != tt.wantType || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %q, %v, %v), want (%q, %q, %v, err=%v)",
					provider, credType, ok, err, tt.wantProvider, tt.wantType, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestCredentialAudiencesExtract(t *testing.T) {
	tests := []struct {
		name string
		ann  map[string]string
		want []string
	}{
		{name: "missing", ann: map[string]string{}},
		{
			name: "empty value",
			ann:  map[string]string{AnnotationCredentialAudiences.String(): ""},
		},
		{
			name: "multi line",
			ann:  map[string]string{AnnotationCredentialAudiences.String(): "audience1,audience2\n\naudience3\naudience4,audience5 , audience6"},
			want: []string{"audience1", "audience2", "audience3", "audience4", "audience5", "audience6"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnnotationCredentialAudiences.Extract(tt.ann)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExtractKubeConfigAudiences(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		cm        string
		want      []string
		wantError bool
	}{
		{name: "no legacy key and no annotation", ann: map[string]string{}},
		{
			name: "no legacy key with annotation",
			ann:  map[string]string{AnnotationCredentialAudiences.String(): "a,b"},
			want: []string{"a", "b"},
		},
		{
			name: "legacy key",
			ann:  map[string]string{},
			cm:   "c\nd",
			want: []string{"c", "d"},
		},
		{
			name:      "legacy key with rfc0014 annotation",
			ann:       map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE},
			cm:        "c",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnnotationCredentialAudiences.ExtractKubeConfigAudiences(tt.ann, tt.cm)
			if (err != nil) != tt.wantError {
				t.Fatalf("got error %v, want error=%v", err, tt.wantError)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}
