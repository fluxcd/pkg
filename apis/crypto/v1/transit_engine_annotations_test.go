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

func TestTransitEngineLoginPathExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "valid",
			ann:    map[string]string{AnnotationTransitEngineLoginPath.String(): "auth/jwt/login"},
			want:   "auth/jwt/login",
			wantOk: true,
		},
		{name: "missing", ann: map[string]string{}},
		{
			name:      "leading slash",
			ann:       map[string]string{AnnotationTransitEngineLoginPath.String(): "/auth/jwt/login"},
			wantError: true,
		},
		{
			name:      "trailing slash",
			ann:       map[string]string{AnnotationTransitEngineLoginPath.String(): "auth/jwt/login/"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationTransitEngineLoginPath.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestTransitEngineRoleExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "valid",
			ann:    map[string]string{AnnotationTransitEngineRole.String(): "my-role"},
			want:   "my-role",
			wantOk: true,
		},
		{name: "missing", ann: map[string]string{}},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationTransitEngineRole.String(): "my role"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationTransitEngineRole.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestExtractTransitEngineLoginPathAndRole(t *testing.T) {
	tests := []struct {
		name          string
		ann           map[string]string
		sopsVaultConf bool
		wantLoginPath string
		wantRole      string
		wantOk        bool
		wantError     bool
	}{
		{
			name:          "legacy flag with rfc0014 annotation",
			ann:           map[string]string{AnnotationTransitEngineLoginPath.String(): "auth/jwt/login"},
			sopsVaultConf: true,
			wantError:     true,
		},
		{
			name:          "legacy flag without annotations",
			ann:           map[string]string{},
			sopsVaultConf: true,
		},
		{
			name:          "legacy flag with excluded gcp annotation",
			ann:           map[string]string{AnnotationGCPWorkloadIdentityProvider.String(): "projects/1/locations/global/workloadIdentityPools/p/providers/x"},
			sopsVaultConf: true,
		},
		{
			name:          "annotations",
			ann:           map[string]string{AnnotationTransitEngineLoginPath.String(): "auth/jwt/login", AnnotationTransitEngineRole.String(): "my-role"},
			sopsVaultConf: false,
			wantLoginPath: "auth/jwt/login",
			wantRole:      "my-role",
			wantOk:        true,
		},
		{
			name:          "only login path",
			ann:           map[string]string{AnnotationTransitEngineLoginPath.String(): "auth/jwt/login"},
			sopsVaultConf: false,
			wantError:     true,
		},
		{
			name:          "only role",
			ann:           map[string]string{AnnotationTransitEngineRole.String(): "my-role"},
			sopsVaultConf: false,
			wantError:     true,
		},
		{
			name:          "invalid login path",
			ann:           map[string]string{AnnotationTransitEngineLoginPath.String(): "/nope", AnnotationTransitEngineRole.String(): "my-role"},
			sopsVaultConf: false,
			wantError:     true,
		},
		{
			name:          "invalid role",
			ann:           map[string]string{AnnotationTransitEngineLoginPath.String(): "auth/jwt/login", AnnotationTransitEngineRole.String(): "my role"},
			sopsVaultConf: false,
			wantError:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginPath, role, ok, err := ExtractTransitEngineLoginPathAndRole(tt.ann, tt.sopsVaultConf)
			if loginPath != tt.wantLoginPath || role != tt.wantRole || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %q, %v, %v), want (%q, %q, %v, err=%v)",
					loginPath, role, ok, err, tt.wantLoginPath, tt.wantRole, tt.wantOk, tt.wantError)
			}
		})
	}
}
