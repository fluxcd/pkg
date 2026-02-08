/*
Copyright 2025 The Flux authors

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
	"net/url"
	"testing"
	"time"

	"github.com/fluxcd/pkg/auth"
)

func TestOptions_GetHTTPClient(t *testing.T) {
	tests := []struct {
		name        string
		options     auth.Options
		expectProxy bool
	}{
		{
			name:        "no proxy configured",
			options:     auth.Options{},
			expectProxy: false,
		},
		{
			name: "proxy configured",
			options: auth.Options{
				ProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com:8080"},
			},
			expectProxy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.options.GetHTTPClient()

			if client == nil {
				t.Error("GetHTTPClient() returned nil, expected non-nil client")
				return // fix linter error
			}

			expectedTimeout := 10 * time.Second
			if client.Timeout != expectedTimeout {
				t.Errorf("GetHTTPClient() timeout = %v, want %v", client.Timeout, expectedTimeout)
			}

			if client.Transport == nil {
				t.Error("GetHTTPClient() transport is nil")
				return
			}

			if tt.expectProxy && tt.options.ProxyURL == nil {
				t.Error("Expected proxy but ProxyURL is nil")
			}
		})
	}
}

type testIdentity string

func (t testIdentity) String() string { return string(t) }

func TestOptions_WithIdentityForOIDCImpersonation(t *testing.T) {
	var o auth.Options
	identity := testIdentity("oidc-identity")
	o.Apply(auth.WithIdentityForOIDCImpersonation(identity))

	if o.IdentityForOIDCImpersonation == nil {
		t.Fatal("IdentityForOIDCImpersonation should not be nil")
	}
	if o.IdentityForOIDCImpersonation.String() != "oidc-identity" {
		t.Errorf("IdentityForOIDCImpersonation = %v, want %v", o.IdentityForOIDCImpersonation, "oidc-identity")
	}
}

func TestOptions_WithIdentityForImpersonation(t *testing.T) {
	var o auth.Options
	identity := testIdentity("impersonation-identity")
	o.Apply(auth.WithIdentityForImpersonation(identity))

	if o.IdentityForImpersonation == nil {
		t.Fatal("IdentityForImpersonation should not be nil")
	}
	if o.IdentityForImpersonation.String() != "impersonation-identity" {
		t.Errorf("IdentityForImpersonation = %v, want %v", o.IdentityForImpersonation, "impersonation-identity")
	}
}

func TestOptions_ShouldGetServiceAccount(t *testing.T) {
	tests := []struct {
		name     string
		opts     []auth.Option
		expected bool
	}{
		{
			name: "service account name provided",
			opts: []auth.Option{
				auth.WithServiceAccountName("test-sa"),
			},
			expected: true,
		},
		{
			name: "default service account provided",
			opts: []auth.Option{
				auth.WithDefaultServiceAccount("default-sa"),
			},
			expected: true,
		},
		{
			name: "both name and default provided",
			opts: []auth.Option{
				auth.WithServiceAccountName("test-sa"),
				auth.WithDefaultServiceAccount("default-sa"),
			},
			expected: true,
		},
		{
			name:     "neither provided",
			opts:     []auth.Option{},
			expected: false,
		},
		{
			name: "only namespace provided",
			opts: []auth.Option{
				auth.WithServiceAccountNamespace("default"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var o auth.Options
			o.Apply(tt.opts...)

			result := o.ShouldGetServiceAccount()
			if result != tt.expected {
				t.Errorf("ShouldGetServiceAccount() = %v, want %v", result, tt.expected)
			}
		})
	}
}
