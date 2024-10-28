/*
Copyright 2024 The Flux authors

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

package cache

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Store is an interface for a cache store.
type Store[T any] interface {
	// Set adds an item to the store for the given key.
	Set(key string, value T) error
	// Get returns an item stored in the store for the given key.
	Get(key string) (*T, error)
	// Delete deletes an item in the store for the given key.
	Delete(key string) error
}

// Expirable is an interface for a cache store that supports expiration.
type Expirable[T any] interface {
	Store[T]
	// SetExpiration sets the expiration time for a cached item.
	SetExpiration(key string, expiresAt time.Time) error
	// GetExpiration returns the expiration time of an item.
	GetExpiration(key string) (time.Time, error)
	// HasExpired returns if an item has expired.
	HasExpired(key string) (bool, error)
}

type storeOptions struct {
	interval      time.Duration
	registerer    prometheus.Registerer
	metricsPrefix string
}

// Options is a function that sets the store options.
type Options func(*storeOptions) error

// WithCleanupInterval sets the interval for the cache cleanup.
func WithCleanupInterval(interval time.Duration) Options {
	return func(o *storeOptions) error {
		o.interval = interval
		return nil
	}
}

// WithMetricsRegisterer sets the Prometheus registerer for the cache metrics.
func WithMetricsRegisterer(r prometheus.Registerer) Options {
	return func(o *storeOptions) error {
		o.registerer = r
		return nil
	}
}

// WithMetricsPrefix sets the metrics prefix for the cache metrics.
func WithMetricsPrefix(prefix string) Options {
	return func(o *storeOptions) error {
		o.metricsPrefix = prefix
		return nil
	}
}
