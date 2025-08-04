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

package gcp

import (
	"net/url"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
)

func TestTokenSourceManager_GetOrCreateTokenSource(t *testing.T) {
	const testNamespace = "test-namespace"
	var k8sClient client.Client = nil // Not needed in test environment

	tests := []struct {
		name       string
		opts1      []auth.Option
		opts2      []auth.Option
		expectSame bool
	}{
		{
			name:       "controller-level returns same instance",
			opts1:      []auth.Option{},
			opts2:      []auth.Option{},
			expectSame: true,
		},
		{
			name:       "object-level returns same instance for same ServiceAccount",
			opts1:      []auth.Option{auth.WithServiceAccount(client.ObjectKey{Namespace: testNamespace, Name: "test-sa"}, k8sClient)},
			opts2:      []auth.Option{auth.WithServiceAccount(client.ObjectKey{Namespace: testNamespace, Name: "test-sa"}, k8sClient)},
			expectSame: true,
		},
		{
			name:       "different ServiceAccounts get different TokenSources",
			opts1:      []auth.Option{auth.WithServiceAccount(client.ObjectKey{Namespace: testNamespace, Name: "sa1"}, k8sClient)},
			opts2:      []auth.Option{auth.WithServiceAccount(client.ObjectKey{Namespace: testNamespace, Name: "sa2"}, k8sClient)},
			expectSame: false,
		},
		{
			name:       "controller-level and object-level are different",
			opts1:      []auth.Option{},
			opts2:      []auth.Option{auth.WithServiceAccount(client.ObjectKey{Namespace: testNamespace, Name: "test-sa"}, k8sClient)},
			expectSame: false,
		},
		{
			name: "controller-level with same proxy returns same instance",
			opts1: []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint("https://sts.example.com"),
			},
			opts2: []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint("https://sts.example.com"),
			},
			expectSame: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewTokenSourceManager()

			ts1 := manager.GetOrCreateTokenSource(tt.opts1...)
			ts2 := manager.GetOrCreateTokenSource(tt.opts2...)

			same := ts1 == ts2

			if tt.expectSame != same {
				if tt.expectSame {
					t.Errorf("Expected same TokenSource instance, got different instances")
				} else {
					t.Errorf("Expected different TokenSource instances, got same instance")
				}
			}
		})
	}
}

func TestTokenSourceManager_InvalidateTokenSource(t *testing.T) {
	const testNamespace = "test-namespace"
	var k8sClient client.Client = nil // Not needed in test environment

	keyFor := func(saName string) string {
		var o auth.Options
		o.Apply(auth.WithServiceAccount(client.ObjectKey{
			Namespace: testNamespace, Name: saName,
		}, k8sClient))
		return o.CacheKey()
	}

	tests := []struct {
		name         string
		beforeCache  []string
		invalidateSA string
		afterCache   []string
	}{
		{
			name:         "invalidate existing cache entry",
			beforeCache:  []string{"other-sa", "test-sa"},
			invalidateSA: "test-sa",
			afterCache:   []string{"other-sa"},
		},
		{
			name:         "invalidate non-existing cache entry",
			beforeCache:  []string{"other-sa"},
			invalidateSA: "test-sa",
			afterCache:   []string{"other-sa"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testManager := NewTokenSourceManager()

			for _, saName := range tt.beforeCache {
				opts := []auth.Option{auth.WithServiceAccount(client.ObjectKey{
					Namespace: testNamespace, Name: saName,
				}, k8sClient)}
				testManager.GetOrCreateTokenSource(opts...)
			}

			testManager.InvalidateTokenSource(client.ObjectKey{
				Namespace: testNamespace, Name: tt.invalidateSA,
			})

			actualCacheKeys := make([]string, 0)
			testManager.mu.RLock()
			for key := range testManager.tokenSources {
				actualCacheKeys = append(actualCacheKeys, key)
			}
			testManager.mu.RUnlock()

			expectedCacheKeys := make([]string, 0)
			for _, saName := range tt.afterCache {
				expectedCacheKeys = append(expectedCacheKeys, keyFor(saName))
			}

			if len(actualCacheKeys) != len(expectedCacheKeys) {
				t.Errorf("Expected %d cache entries, got %d", len(expectedCacheKeys), len(actualCacheKeys))
			}

			for _, expectedKey := range expectedCacheKeys {
				found := false
				for _, actualKey := range actualCacheKeys {
					if actualKey == expectedKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected cache key %s not found", expectedKey)
				}
			}
		})
	}
}

func TestTokenSourceManager_Close(t *testing.T) {
	var k8sClient client.Client = nil // Not needed in test environment
	manager := NewTokenSourceManager()

	manager.GetOrCreateTokenSource()
	serviceAccountRef := client.ObjectKey{Namespace: "test-ns", Name: "test-sa"}
	manager.GetOrCreateTokenSource(auth.WithServiceAccount(serviceAccountRef, k8sClient))

	if err := manager.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	ts1 := manager.GetOrCreateTokenSource()
	ts2 := manager.GetOrCreateTokenSource()

	if ts1 != ts2 {
		t.Errorf("Expected same instance after Close() and re-creation, got different instances")
	}
}
