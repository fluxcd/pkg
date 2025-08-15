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

package auth

import (
	"net/url"
	"testing"
	"time"
)

func TestOptions_GetHTTPClient(t *testing.T) {
	tests := []struct {
		name        string
		options     Options
		expectProxy bool
	}{
		{
			name:        "no proxy configured",
			options:     Options{},
			expectProxy: false,
		},
		{
			name: "proxy configured",
			options: Options{
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

func TestOptions_ShouldGetServiceAccountToken(t *testing.T) {
	tests := []struct {
		name     string
		opts     []Option
		expected bool
	}{
		{
			name: "both name and namespace provided",
			opts: []Option{
				WithServiceAccountName("test-sa"),
				WithServiceAccountNamespace("default"),
			},
			expected: true,
		},
		{
			name: "only namespace provided - no env vars",
			opts: []Option{
				WithServiceAccountNamespace("default"),
				func(o *Options) {
					t.Setenv("DEFAULT_SERVICE_ACCOUNT", "")
					t.Setenv("DEFAULT_KUBECONFIG_SERVICE_ACCOUNT", "")
				},
			},
			expected: false,
		},
		{
			name: "only namespace provided - with DEFAULT_SERVICE_ACCOUNT",
			opts: []Option{
				WithServiceAccountNamespace("default"),
				func(o *Options) {
					t.Setenv("DEFAULT_SERVICE_ACCOUNT", "default-sa")
				},
			},
			expected: true,
		},
		{
			name: "only namespace provided - with DEFAULT_KUBECONFIG_SERVICE_ACCOUNT",
			opts: []Option{
				WithServiceAccountNamespace("default"),
				func(o *Options) {
					t.Setenv("DEFAULT_KUBECONFIG_SERVICE_ACCOUNT", "default-kubeconfig-sa")
				},
			},
			expected: true,
		},
		{
			name: "only name provided",
			opts: []Option{
				WithServiceAccountName("test-sa"),
			},
			expected: false,
		},
		{
			name:     "neither provided",
			opts:     []Option{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var o Options
			o.Apply(tt.opts...)
			result := o.ShouldGetServiceAccountToken()
			if result != tt.expected {
				t.Errorf("ShouldGetServiceAccountToken() = %v, want %v", result, tt.expected)
			}
		})
	}
}
