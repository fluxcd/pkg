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
	object T
	key    string
	prev   *node[T]
	next   *node[T]
}

func (n *node[T]) addNext(node *node[T]) {
	n.next = node
}

func (n *node[T]) addPrev(node *node[T]) {
	n.prev = node
}

// LRU is a thread-safe in-memory key/object store.
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
// A function to extract the key from the object must be provided.
// Use the NewLRU function to create a new cache that is ready to use.
type LRU[T any] struct {
	cache    map[string]*node[T]
	capacity int
	// keyFunc is used to make the key for objects stored in and retrieved from items, and
	// should be deterministic.
	keyFunc    KeyFunc[T]
	metrics    *cacheMetrics
	labelsFunc GetLvsFunc[T]
	head       *node[T]
	tail       *node[T]
	mu         sync.RWMutex
}

var _ Store[any] = &LRU[any]{}

// NewLRU creates a new LRU cache with the given capacity and keyFunc.
func NewLRU[T any](capacity int, keyFunc KeyFunc[T], opts ...Options[T]) (*LRU[T], error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	head := &node[T]{}
	tail := &node[T]{}
	head.addNext(tail)
	tail.addPrev(head)

	lru := &LRU[T]{
		cache:      make(map[string]*node[T]),
		keyFunc:    keyFunc,
		labelsFunc: opt.labelsFunc,
		capacity:   capacity,
		head:       head,
		tail:       tail,
	}

	if opt.registerer != nil {
		lru.metrics = newCacheMetrics(opt.registerer, opt.extraLabels...)
	}

	return lru, nil
}

// Set an item in the cache, existing index will be overwritten.
func (c *LRU[T]) Set(object T) error {
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return &CacheError{Reason: ErrInvalidKey, Err: err}
	}

	// if node is already in cache, return error
	c.mu.Lock()
	newNode, ok := c.cache[key]
	if ok {
		c.delete(newNode)
		_ = c.add(&node[T]{key: key, object: object})
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return nil
	}

	evicted := c.add(&node[T]{key: key, object: object})
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
func (c *LRU[T]) Delete(object T) error {
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return &CacheError{Reason: ErrInvalidKey, Err: err}
	}

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

// Get returns the given object from the cache.
// If the object is not in the cache, it returns false.
func (c *LRU[T]) Get(object T) (item T, exists bool, err error) {
	var res T
	lvs := []string{}
	if c.labelsFunc != nil {
		lvs, err = c.labelsFunc(object, len(c.metrics.getExtraLabels()))
		if err != nil {
			recordRequest(c.metrics, StatusFailure)
			return res, false, &CacheError{Reason: ErrInvalidLabels, Err: err}
		}
	}
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return item, false, &CacheError{Reason: ErrInvalidKey, Err: err}
	}

	item, exists, err = c.get(key)
	if err != nil {
		return res, false, ErrInvalidKey
	}
	if !exists {
		recordEvent(c.metrics, CacheEventTypeMiss, lvs...)
		return res, false, nil
	}
	recordEvent(c.metrics, CacheEventTypeHit, lvs...)
	return item, true, nil
}

// GetByKey returns the object for the given key.
func (c *LRU[T]) GetByKey(key string) (T, bool, error) {
	var res T
	item, found, err := c.get(key)
	if err != nil {
		return res, false, err
	}
	if !found {
		recordEvent(c.metrics, CacheEventTypeMiss)
		return res, false, nil
	}

	recordEvent(c.metrics, CacheEventTypeHit)
	return item, true, nil
}

func (c *LRU[T]) get(key string) (item T, exists bool, err error) {
	var res T
	c.mu.Lock()
	node, ok := c.cache[key]
	if !ok {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return res, false, nil
	}
	c.delete(node)
	_ = c.add(node)
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return node.object, true, nil
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
