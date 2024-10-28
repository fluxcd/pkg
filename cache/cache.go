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
	"slices"
	"sort"
	"sync"
	"time"
)

const (
	// noExpiration is a sentinel value used to indicate no expiration time.
	// It is used instead of 0, to be able to sort items by expiration time ascending.
	noExpiration = time.Second * 86400 * 365 * 10 // 10 years
	// defaultInterval is the default interval for the janitor to run.
	defaultInterval = time.Minute
)

// Cache[T] is a thread-safe in-memory key/value store.
// It can be used to store items with optional expiration.
type Cache[T any] struct {
	*cache[T]
}

// item is an item stored in the cache.
type item[T any] struct {
	key string
	// value is the item's value.
	value T
	// expiresAt is the item's expiration time.
	expiresAt time.Time
}

type cache[T any] struct {
	// index holds the cache index.
	index map[string]*item[T]
	// items is the store of elements in the cache.
	items []*item[T]
	// sorted indicates whether the items are sorted by expiration time.
	// It is initially true, and set to false when the items are not sorted.
	sorted bool
	// capacity is the maximum number of index the cache can hold.
	capacity int
	metrics  *cacheMetrics
	janitor  *janitor[T]
	closed   bool

	mu sync.RWMutex
}

var _ Expirable[any] = &Cache[any]{}

// New creates a new cache with the given configuration.
func New[T any](capacity int, opts ...Options) (*Cache[T], error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	c := &cache[T]{
		index:    make(map[string]*item[T]),
		items:    make([]*item[T], 0, capacity),
		sorted:   true,
		capacity: capacity,
		janitor: &janitor[T]{
			interval: opt.interval,
			stop:     make(chan bool),
		},
	}

	if opt.registerer != nil {
		c.metrics = newCacheMetrics(opt.metricsPrefix, opt.registerer)
	}

	C := &Cache[T]{cache: c}

	if opt.interval > 0 {
		go c.janitor.run(c)
	}

	return C, nil
}

func makeOptions(opts ...Options) (*storeOptions, error) {
	opt := storeOptions{}
	for _, o := range opts {
		err := o(&opt)
		if err != nil {
			return nil, err
		}
	}
	if opt.interval <= 0 {
		opt.interval = defaultInterval
	}
	return &opt, nil
}

// Close closes the cache. It also stops the expiration eviction process.
func (c *Cache[T]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrCacheClosed
	}
	c.janitor.stop <- true
	c.closed = true
	return nil
}

// Set an item in the cache, existing index will be overwritten.
// If the cache is full, an error is returned.
func (c *Cache[T]) Set(key string, value T) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrCacheClosed
	}
	_, found := c.index[key]
	if found {
		c.set(key, value)
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return nil
	}

	if c.capacity > 0 && len(c.index) < c.capacity {
		c.set(key, value)
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		recordItemIncrement(c.metrics)
		return nil
	}
	c.mu.Unlock()
	recordRequest(c.metrics, StatusFailure)
	return ErrCacheFull
}

func (c *cache[T]) set(key string, value T) {
	item := item[T]{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(noExpiration),
	}

	if _, found := c.index[key]; found {
		// item already exists, update it only
		c.index[key] = &item
		return
	}
	c.index[key] = &item
	c.items = append(c.items, &item)
}

// Get returns a pointer to an item in the cache for the given key. If no item
// is found, it's a nil pointer.
// The caller can record cache hit or miss based on the result with
// Cache.RecordCacheEvent().
func (c *Cache[T]) Get(key string) (*T, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return nil, ErrCacheClosed
	}
	item, found := c.index[key]
	if !found {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return nil, nil
	}
	if !item.expiresAt.IsZero() {
		if item.expiresAt.Compare(time.Now()) < 0 {
			c.mu.RUnlock()
			recordRequest(c.metrics, StatusSuccess)
			return nil, nil
		}
	}
	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	// Copy the value to prevent writes to the cached item.
	r := item.value
	return &r, nil
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
// It actually sets the item expiration to `now“, so that it will be deleted at
// the cleanup.
func (c *Cache[T]) Delete(key string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrCacheClosed
	}
	if item, ok := c.index[key]; ok {
		// set the item expiration to now
		// so that it will be removed by the janitor
		item.expiresAt = time.Now()
	}
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return nil
}

// Clear all index from the cache.
// This reallocates the underlying array holding the index,
// so that the memory used by the index is reclaimed.
// A closed cache cannot be cleared.
func (c *cache[T]) Clear() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.index = make(map[string]*item[T])
	c.mu.Unlock()
}

// ListKeys returns a slice of the keys in the cache.
func (c *cache[T]) ListKeys() ([]string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return nil, ErrCacheClosed
	}
	keys := make([]string, 0, len(c.index))
	for k := range c.index {
		keys = append(keys, k)
	}
	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return keys, nil
}

// Resize resizes the cache and returns the number of index removed.
// Size must be greater than zero.
func (c *cache[T]) Resize(size int) (int, error) {
	if size <= 0 {
		recordRequest(c.metrics, StatusFailure)
		return 0, ErrInvalidSize
	}

	c.mu.Lock()
	overflow := len(c.items) - size
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return 0, ErrCacheClosed
	}

	// set the new capacity
	c.capacity = size

	if overflow <= 0 {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return 0, nil
	}

	if !c.sorted {
		// sort the slice of index by expiration time
		slices.SortFunc(c.items, func(i, j *item[T]) int {
			return i.expiresAt.Compare(j.expiresAt)
		})
		c.sorted = true
	}

	// delete the overflow indexes
	for _, v := range c.items[:overflow] {
		delete(c.index, v.key)
		recordEviction(c.metrics)
		recordDecrement(c.metrics)
	}
	// remove the overflow indexes from the slice
	c.items = c.items[overflow:]
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return overflow, nil
}

// HasExpired returns true if the item has expired.
func (c *Cache[T]) HasExpired(key string) (bool, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return false, ErrCacheClosed
	}
	item, ok := c.index[key]
	if !ok {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return true, nil
	}

	if item.expiresAt.Compare(time.Now()) < 0 {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return true, nil
	}

	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return false, nil
}

// SetExpiration sets the expiration for the given key.
func (c *Cache[T]) SetExpiration(key string, expiration time.Time) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrCacheClosed
	}
	item, ok := c.index[key]
	if !ok {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrNotFound
	}
	item.expiresAt = expiration
	// mark the items as not sorted
	c.sorted = false
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return nil
}

// GetExpiration returns the expiration for the given key.
// Returns zero if the key is not in the cache or the item
// has already expired.
func (c *Cache[T]) GetExpiration(key string) (time.Time, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return time.Time{}, ErrCacheClosed
	}
	item, ok := c.index[key]
	if !ok {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return time.Time{}, ErrNotFound
	}
	if !item.expiresAt.IsZero() {
		if item.expiresAt.Compare(time.Now()) < 0 {
			c.mu.RUnlock()
			recordRequest(c.metrics, StatusSuccess)
			return time.Time{}, nil
		}
	}
	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return item.expiresAt, nil
}

// deleteExpired deletes all expired index from the cache.
// It is called by the janitor.
func (c *cache[T]) deleteExpired() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}

	if !c.sorted {
		// sort the slice of index by expiration time
		slices.SortFunc(c.items, func(i, j *item[T]) int {
			return i.expiresAt.Compare(j.expiresAt)
		})
		c.sorted = true
	}

	t := time.Now()
	index := sort.Search(len(c.items), func(i int) bool {
		// smallest index with an expiration greater than t
		return c.items[i].expiresAt.Compare(t) > 0
	})

	// delete the expired indexes
	for _, v := range c.items[:index] {
		delete(c.index, v.key)
		recordEviction(c.metrics)
		recordDecrement(c.metrics)
	}
	// remove the expired indexes from the slice
	c.items = c.items[index:]
	c.mu.Unlock()
}

// RecordCacheEvent records a cache event (cache_miss or cache_hit) with kind,
// name and namespace of the associated object being reconciled.
func (c *Cache[T]) RecordCacheEvent(event, kind, name, namespace string) {
	recordCacheEvent(c.metrics, event, kind, name, namespace)
}

// DeleteCacheEvent deletes the cache event (cache_miss or cache_hit) metric for
// the associated object being reconciled, given their kind, name and namespace.
func (c *Cache[T]) DeleteCacheEvent(event, kind, name, namespace string) {
	deleteCacheEvent(c.metrics, event, kind, name, namespace)
}

type janitor[T any] struct {
	interval time.Duration
	stop     chan bool
}

func (j *janitor[T]) run(c *cache[T]) {
	ticker := time.NewTicker(j.interval)
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}
