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
	"net/http"
	"sync"

	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
)

// TokenSourceManager manages TokenSource instances for GCP authentication.
// It maintains a cache of TokenSources keyed by ServiceAccount and ensures
// that each ServiceAccount has exactly one long-lived TokenSource, avoiding
// short-lived context issues and improving performance through reuse.
type TokenSourceManager struct {
	tokenSources map[string]oauth2.TokenSource
	mu           sync.RWMutex
	httpClient   *http.Client
	provider     Provider
}

// NewTokenSourceManager creates a new TokenSourceManager instance.
// Each manager instance maintains its own TokenSource cache and can be
// safely used across multiple goroutines.
func NewTokenSourceManager() *TokenSourceManager {
	return &TokenSourceManager{
		tokenSources: make(map[string]oauth2.TokenSource),
		provider:     Provider{},
	}
}

// GetOrCreateTokenSource returns a cached TokenSource based on the provided auth options.
// This is the primary API that automatically determines authentication type based on options:
// - If ServiceAccount option is provided: Object-level authentication
// - If no ServiceAccount option: Controller-level authentication using default GCP auth chain
// This ensures that each configuration has exactly one long-lived TokenSource, avoiding
// short-lived context issues and improving performance through reuse.
func (m *TokenSourceManager) GetOrCreateTokenSource(opts ...auth.Option) oauth2.TokenSource {
	var o auth.Options
	o.Apply(opts...)
	key := o.CacheKey()

	m.mu.RLock()
	if ts, exists := m.tokenSources[key]; exists {
		m.mu.RUnlock()
		return ts
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check to avoid race condition
	if ts, exists := m.tokenSources[key]; exists {
		return ts
	}

	tokenSource := NewPersistentTokenSource(opts...)
	m.tokenSources[key] = tokenSource

	return tokenSource
}

// InvalidateTokenSource removes a TokenSource from the cache for the specified ServiceAccount.
// This can be useful when ServiceAccount configuration changes and you want to force
// creation of a new TokenSource with updated settings.
func (m *TokenSourceManager) InvalidateTokenSource(serviceAccountRef client.ObjectKey) {
	var o auth.Options
	o.Apply(auth.WithServiceAccount(serviceAccountRef, nil))
	key := o.CacheKey()

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tokenSources, key)
}

// Close releases resources held by the manager.
// This should be called when the manager is no longer needed,
// typically when the controller manager is shutting down.
func (m *TokenSourceManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear the cache
	m.tokenSources = make(map[string]oauth2.TokenSource)

	// Close idle connections if we have a shared HTTP client
	if m.httpClient != nil && m.httpClient.Transport != nil {
		if transport, ok := m.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	return nil
}

// Provider returns the underlying GCP Provider.
// This can be useful when you need access to Provider methods
// for authentication operations that don't require TokenSource caching.
func (m *TokenSourceManager) Provider() Provider {
	return m.provider
}
