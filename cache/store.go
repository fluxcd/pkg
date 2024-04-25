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
	"fmt"
	"time"

	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/prometheus/client_golang/prometheus"
)

// Store is an interface for a cache store.
// It is a generic version of the Kubernetes client-go cache.Store interface.
// See https://pkg.go.dev/k8s.io/client-go/tools/cache#Store
// The implementation should know how to extract a key from an object.
type Store[T any] interface {
	// Set adds an object to the store.
	// It will overwrite the item if it already exists.
	Set(object T) error
	// Delete deletes an object from the store.
	Delete(object T) error
	// ListKeys returns a list of keys in the store.
	ListKeys() ([]string, error)
	// Get returns the object stored in the store.
	Get(object T) (item T, exists bool, err error)
	// GetByKey returns the object stored in the store by key.
	GetByKey(key string) (item T, exists bool, err error)
	// Resize resizes the store and returns the number of items removed.
	Resize(int) (int, error)
}

// Expirable is an interface for a cache store that supports expiration.
type Expirable[T any] interface {
	Store[T]
	// SetExpiration sets the expiration time for the object.
	SetExpiration(object T, expiresAt time.Time) error
	// GetExpiration returns the expiration time for the object in unix time.
	GetExpiration(object T) (time.Time, error)
	// HasExpired returns true if the object has expired.
	HasExpired(object T) (bool, error)
}

type storeOptions[T any] struct {
	interval    time.Duration
	registerer  prometheus.Registerer
	extraLabels []string
	labelsFunc  GetLvsFunc[T]
}

// Options is a function that sets the store options.
type Options[T any] func(*storeOptions[T]) error

// WithMetricsLabels sets the extra labels for the cache metrics.
func WithMetricsLabels[T any](labels []string, f GetLvsFunc[T]) Options[T] {
	return func(o *storeOptions[T]) error {
		if labels != nil && f == nil {
			return fmt.Errorf("labelsFunc must be set if labels are provided")
		}
		o.extraLabels = labels
		o.labelsFunc = f
		return nil
	}
}

// WithCleanupInterval sets the interval for the cache cleanup.
func WithCleanupInterval[T any](interval time.Duration) Options[T] {
	return func(o *storeOptions[T]) error {
		o.interval = interval
		return nil
	}
}

// WithMetricsRegisterer sets the Prometheus registerer for the cache metrics.
func WithMetricsRegisterer[T any](r prometheus.Registerer) Options[T] {
	return func(o *storeOptions[T]) error {
		o.registerer = r
		return nil
	}
}

// KeyFunc knows how to make a key from an object. Implementations should be deterministic.
type KeyFunc[T any] func(object T) (string, error)

// IdentifiableObject is a wrapper for an object with its identifying metadata.
type IdentifiableObject struct {
	object.ObjMetadata
	// Object is the object that is being stored.
	Object any
}

// ExplicitKey can be passed to IdentifiableObjectKeyFunc if you have the key for
// the objectec but not the object itself.
type ExplicitKey string

// IdentifiableObjectKeyFunc is a convenient default KeyFunc which knows how to make
// keys from IdentifiableObject objects.
func IdentifiableObjectKeyFunc[T any](object T) (string, error) {
	if key, ok := any(object).(ExplicitKey); ok {
		return string(key), nil
	}
	n, ok := any(object).(IdentifiableObject)
	if !ok {
		return "", fmt.Errorf("object has no meta: %v", object)
	}
	return n.String(), nil
}

// StoreObject is a wrapper for an object with its identifying key.
// It is used to store objects in a Store.
// This helper is useful when the object does not have metadata to extract the key from.
// The supplied key can be retrieved with the StoreObjectKeyFunc.
// When the object has metadata, use IdentifiableObject instead if possible.
type StoreObject[T any] struct {
	// Object is the object that is being stored.
	Object T
	// Key is the key for the object.
	Key string
}

// StoreObjectKeyFunc returns the key for a StoreObject.
func StoreObjectKeyFunc[T any](object StoreObject[T]) (string, error) {
	return object.Key, nil
}
