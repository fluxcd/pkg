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

package auth_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fluxcd/pkg/auth"
)

func TestFindPodServiceAccount(t *testing.T) {
	tests := []struct {
		name          string
		readFile      func(string) ([]byte, error)
		wantNamespace string
		wantName      string
		wantErr       string
	}{
		{
			name: "valid token with correct subject",
			readFile: fakeReadFile(t, makeToken(t, jwt.MapClaims{
				"sub": "system:serviceaccount:flux-system:source-controller",
			})),
			wantNamespace: "flux-system",
			wantName:      "source-controller",
		},
		{
			name: "read file error",
			readFile: func(string) ([]byte, error) {
				return nil, fmt.Errorf("file not found")
			},
			wantErr: "failed to read service account token file",
		},
		{
			name:     "invalid JWT",
			readFile: fakeReadFile(t, "not-a-jwt"),
			wantErr:  "failed to parse service account token",
		},
		{
			name: "subject with too few parts",
			readFile: fakeReadFile(t, makeToken(t, jwt.MapClaims{
				"sub": "system:serviceaccount",
			})),
			wantErr: "invalid subject format",
		},
		{
			name: "subject with too many parts",
			readFile: fakeReadFile(t, makeToken(t, jwt.MapClaims{
				"sub": "system:serviceaccount:ns:name:extra",
			})),
			wantErr: "invalid subject format",
		},
		{
			name: "empty subject",
			readFile: fakeReadFile(t, makeToken(t, jwt.MapClaims{
				"sub": "",
			})),
			wantErr: "invalid subject format",
		},
		{
			name: "no subject claim",
			readFile: fakeReadFile(t, makeToken(t, jwt.MapClaims{
				"iss": "kubernetes/serviceaccount",
			})),
			wantErr: "invalid subject format",
		},
		{
			name: "non-string subject claim",
			readFile: fakeReadFile(t, makeToken(t, jwt.MapClaims{
				"iss": "kubernetes/serviceaccount",
				"sub": 12345,
			})),
			wantErr: "failed to get subject from service account token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := auth.FindPodServiceAccount(tt.readFile)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if got := err.Error(); !strings.Contains(got, tt.wantErr) {
					t.Errorf("error = %q, want it to contain %q", got, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Namespace != tt.wantNamespace {
				t.Errorf("namespace = %q, want %q", result.Namespace, tt.wantNamespace)
			}
			if result.Name != tt.wantName {
				t.Errorf("name = %q, want %q", result.Name, tt.wantName)
			}
		})
	}
}

func TestFindPodServiceAccount_ReadsCorrectPath(t *testing.T) {
	const expectedPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	var gotPath string
	readFile := func(name string) ([]byte, error) {
		gotPath = name
		return []byte(makeToken(t, jwt.MapClaims{
			"sub": "system:serviceaccount:default:my-sa",
		})), nil
	}

	_, err := auth.FindPodServiceAccount(readFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != expectedPath {
		t.Errorf("read path = %q, want %q", gotPath, expectedPath)
	}
}

// makeToken creates an unsigned JWT string with the given claims.
func makeToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return s
}

// fakeReadFile returns a readFile function that ignores the path
// and returns the given token string.
func fakeReadFile(t *testing.T, token string) func(string) ([]byte, error) {
	t.Helper()
	return func(string) ([]byte, error) {
		return []byte(token), nil
	}
}
