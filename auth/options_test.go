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
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOptions_CacheKey(t *testing.T) {
	const expectedHashLength = 16     // 8 bytes as hex string
	var k8sClient client.Client = nil // Not needed in test environment

	tests := []struct {
		name         string
		opts         []Option
		expectedBase string
		hasHash      bool
	}{
		{
			name:         "controller-level no options",
			opts:         []Option{},
			expectedBase: "controller",
			hasHash:      false,
		},
		{
			name: "object-level basic",
			opts: []Option{WithServiceAccount(client.ObjectKey{
				Namespace: "test-ns",
				Name:      "test-sa",
			}, k8sClient)},
			expectedBase: "sa:test-ns/test-sa",
			hasHash:      false,
		},
		{
			name:         "controller-level with proxy",
			opts:         []Option{WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"})},
			expectedBase: "controller",
			hasHash:      true,
		},
		{
			name:         "controller-level with STS endpoint",
			opts:         []Option{WithSTSEndpoint("https://sts.example.com")},
			expectedBase: "controller",
			hasHash:      true,
		},
		{
			name:         "controller-level with single scope",
			opts:         []Option{WithScopes("https://www.googleapis.com/auth/cloud-platform")},
			expectedBase: "controller",
			hasHash:      true,
		},
		{
			name:         "controller-level with multiple scopes",
			opts:         []Option{WithScopes("https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/userinfo.email")},
			expectedBase: "controller",
			hasHash:      true,
		},
		{
			name: "controller-level with all options",
			opts: []Option{
				WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				WithSTSEndpoint("https://sts.example.com"),
				WithScopes("https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/userinfo.email"),
			},
			expectedBase: "controller",
			hasHash:      true,
		},
		{
			name: "object-level with proxy",
			opts: []Option{
				WithServiceAccount(client.ObjectKey{Namespace: "test-ns", Name: "test-sa"}, k8sClient),
				WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
			},
			expectedBase: "sa:test-ns/test-sa",
			hasHash:      true,
		},
		{
			name: "object-level with all options",
			opts: []Option{
				WithServiceAccount(client.ObjectKey{Namespace: "test-ns", Name: "test-sa"}, k8sClient),
				WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				WithSTSEndpoint("https://sts.example.com"),
				WithScopes("https://www.googleapis.com/auth/cloud-platform"),
			},
			expectedBase: "sa:test-ns/test-sa",
			hasHash:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var o Options
			o.Apply(tt.opts...)
			key := o.CacheKey()

			if tt.hasHash {
				// Should contain base + hash
				if !strings.HasPrefix(key, tt.expectedBase+"-") {
					t.Errorf("CacheKey() = %v, expected to start with %v-", key, tt.expectedBase)
				}
				// Extract hash part - everything after the last "-"
				lastDashIndex := strings.LastIndex(key, "-")
				if lastDashIndex == -1 {
					t.Errorf("CacheKey() = %v, expected to contain hash part after last dash", key)
				} else {
					hashPart := key[lastDashIndex+1:]
					if len(hashPart) != expectedHashLength {
						t.Errorf("CacheKey() hash part = %v, expected %d characters", hashPart, expectedHashLength)
					}
				}
			} else {
				if key != tt.expectedBase {
					t.Errorf("CacheKey() = %v, want %v", key, tt.expectedBase)
				}
			}
		})
	}
}
