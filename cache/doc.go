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

// Package cache provides a Store interface for a cache store, along with two
// implementations of this interface for expiring cache(Cache) and least
// recently used cache(LRU). Expirable defines an interface for cache with
// expiring items. This is also implemented by the expiring cache
// implementation, Cache.
// The Cache and LRU cache implementations are generic cache. The data type of
// the value stored in the cache has to be defined when creating the cache. For
// example, for storing string values in Cache create a string type Cache
//
//	cache, err := New[string](10)
//
// The cache implementations are self-instrumenting and export metrics about the
// internal operations of the cache if it is configured with a metrics
// registerer.
//
//	cache, err := New[string](10, WithMetricsRegisterer(reg))
//
// For recording cache hit/miss metrics associated with a Flux object for which
// the cache is used, the caller must explicitly record the cache event based on
// the result of the operation along with the object in the context
//
//	got, err := cache.Get("foo")
//	// Handle any error.
//	...
//
//	if err == ErrNotFound {
//	  cache.RecordCacheEvent(CacheEventTypeMiss, "GitRepository", "repoA", "testNS")
//	} else {
//	  cache.RecordCacheEvent(CacheEventTypeHit, "GitRepository", "repoA", "testNS")
//	}
//
// When the Flux object associated with the cache metrics is deleted, the
// metrics can be deleted as follows
//
//	cache.DeleteCacheEvent(CacheEventTypeHit, "GitRepository", "repoA", "testNS")
//	cache.DeleteCacheEvent(CacheEventTypeMiss, "GitRepository", "repoA", "testNS")
package cache
