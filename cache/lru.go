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
	"sync"
)

// node is a node in a doubly linked list
// that is used to implement an LRU cache
type node[T any] struct {
	value T
	key   string
	prev  *node[T]
	next  *node[T]
}

func (n *node[T]) addNext(node *node[T]) {
	n.next = node
}

func (n *node[T]) addPrev(node *node[T]) {
	n.prev = node
}

// LRU is a thread-safe in-memory key/value store.
// All methods are safe for concurrent use.
// All operations are O(1). The hash map lookup is O(1) and so is the doubly
// linked list insertion/deletion.
//
// The LRU is implemented as a doubly linked list, where the most recently accessed
// item is at the front of the list and the least recently accessed item is at
// the back. When an item is accessed, it is moved to the front of the list.
// When the cache is full, the least recently accessed item is removed from the
// back of the list.
//
//	                                  Cache
//	           ┌───────────────────────────────────────────────────┐
//	           │                                                   │
//	  empty    │     obj         obj          obj          obj     │    empty
//	┌───────┐  │  ┌───────┐   ┌───────┐     ┌───────┐   ┌───────┐  │  ┌───────┐
//	│       │  │  │       │   │       │ ... │       │   │       │  │  │       │
//	│ HEAD  │◄─┼─►│       │◄─►│       │◄───►│       │◄─►│       │◄─┼─►│ TAIL  │
//	│       │  │  │       │   │       │     │       │   │       │  │  │       │
//	└───────┘  │  └───────┘   └───────┘     └───────┘   └───────┘  │  └───────┘
//	           │                                                   │
//	           │                                                   │
//	           └───────────────────────────────────────────────────┘
//
// Use the NewLRU function to create a new cache that is ready to use.
type LRU[T any] struct {
	cache    map[string]*node[T]
	capacity int
	metrics  *cacheMetrics
	head     *node[T]
	tail     *node[T]
	mu       sync.RWMutex
}

var _ Store[any] = &LRU[any]{}

// NewLRU creates a new LRU cache with the given capacity.
func NewLRU[T any](capacity int, opts ...Options) (*LRU[T], error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	head := &node[T]{}
	tail := &node[T]{}
	head.addNext(tail)
	tail.addPrev(head)

	lru := &LRU[T]{
		cache:    make(map[string]*node[T]),
		capacity: capacity,
		head:     head,
		tail:     tail,
	}

	if opt.registerer != nil {
		lru.metrics = newCacheMetrics(opt.registerer)
	}

	return lru, nil
}

// Set an item in the cache, existing index will be overwritten.
func (c *LRU[T]) Set(key string, value T) error {
	// if node is already in cache, return error
	c.mu.Lock()
	newNode, ok := c.cache[key]
	if ok {
		c.delete(newNode)
		_ = c.add(&node[T]{key: key, value: value})
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return nil
	}

	evicted := c.add(&node[T]{key: key, value: value})
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	if evicted {
		recordEviction(c.metrics)
		return nil
	}
	recordItemIncrement(c.metrics)
	return nil
}

func (c *LRU[T]) add(node *node[T]) (evicted bool) {
	prev := c.tail.prev
	prev.addNext(node)
	c.tail.addPrev(node)
	node.addPrev(prev)
	node.addNext(c.tail)

	c.cache[node.key] = node

	if len(c.cache) > c.capacity {
		c.delete(c.head.next)
		return true
	}
	return false
}

// Delete removes a node from the list
func (c *LRU[T]) Delete(key string) error {
	// if node is head or tail, do nothing
	if key == c.head.key || key == c.tail.key {
		recordRequest(c.metrics, StatusSuccess)
		return nil
	}

	c.mu.Lock()
	// if node is not in cache, do nothing
	node, ok := c.cache[key]
	if !ok {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return nil
	}

	c.delete(node)
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	recordDecrement(c.metrics)
	return nil
}

func (c *LRU[T]) delete(node *node[T]) {
	node.prev.next, node.next.prev = node.next, node.prev
	node.next, node.prev = nil, nil // avoid memory leaks
	delete(c.cache, node.key)
}

// Get returns a pointer to an item in the cache for the given key. If no item
// is found, it's a nil pointer.
// The caller can record cache hit or miss based on the result with
// LRU.RecordCacheEvent().
func (c *LRU[T]) Get(key string) (*T, error) {
	c.mu.Lock()
	node, ok := c.cache[key]
	if !ok {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return nil, nil
	}
	c.delete(node)
	_ = c.add(node)
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	// Copy the value to prevent writes to the cached item.
	r := node.value
	return &r, nil
}

// ListKeys returns a list of keys in the cache.
func (c *LRU[T]) ListKeys() ([]string, error) {
	keys := make([]string, 0, len(c.cache))
	c.mu.RLock()
	for k := range c.cache {
		keys = append(keys, k)
	}
	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return keys, nil
}

// Resize resizes the cache and returns the number of items removed.
func (c *LRU[T]) Resize(size int) (int, error) {
	if size <= 0 {
		recordRequest(c.metrics, StatusFailure)
		return 0, ErrInvalidSize
	}

	c.mu.Lock()
	overflow := len(c.cache) - size
	// set the new capacity
	c.capacity = size
	if overflow <= 0 {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return 0, nil
	}

	for i := 0; i < overflow; i++ {
		c.delete(c.head.next)
		recordEviction(c.metrics)
	}
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return overflow, nil
}

// RecordCacheEvent records a cache event (cache_miss or cache_hit) with kind,
// name and namespace of the associated object being reconciled.
func (c *LRU[T]) RecordCacheEvent(event, kind, name, namespace string) {
	recordCacheEvent(c.metrics, event, kind, name, namespace)
}

// DeleteCacheEvent deletes the cache event (cache_miss or cache_hit) metric for
// the associated object being reconciled, given their kind, name and namespace.
func (c *LRU[T]) DeleteCacheEvent(event, kind, name, namespace string) {
	deleteCacheEvent(c.metrics, event, kind, name, namespace)
}
