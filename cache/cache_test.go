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
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func TestCache(t *testing.T) {
	t.Run("Add and update keys", func(t *testing.T) {
		g := NewWithT(t)
		// create a cache that can hold 2 items and have no cleanup
		cache, err := New[string](3,
			WithMetricsRegisterer(prometheus.NewPedanticRegistry()),
			WithCleanupInterval(1*time.Second))
		g.Expect(err).ToNot(HaveOccurred())

		// Get an Item from the cache
		key1 := "key1"
		value1 := "val1"
		got, err := cache.Get(key1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(BeNil())

		// Add an item to the cache
		err = cache.Set(key1, value1)
		g.Expect(err).ToNot(HaveOccurred())

		// Get the item from the cache
		got, err = cache.Get(key1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(*got).To(Equal(value1))

		// Writing to the obtained value doesn't update the cache.
		*got = "val2"
		got2, err := cache.Get(key1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(*got2).To(Equal(value1))

		// Add another item to the cache
		key2 := "key2"
		value2 := "val2"
		err = cache.Set(key2, value2)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cache.ListKeys()).To(ConsistOf(key1, key2))

		// Get the item from the cache
		got, err = cache.Get(key2)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(*got).To(Equal(value2))

		// Update an item in the cache
		key3 := "key3"
		value3 := "val3"
		value4 := "val4"
		err = cache.Set(key3, value3)
		g.Expect(err).ToNot(HaveOccurred())

		// Replace an item in the cache
		err = cache.Set(key3, value4)
		g.Expect(err).ToNot(HaveOccurred())

		// Get the item from the cache
		got, err = cache.Get(key3)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(*got).To(Equal(value4))

		// cleanup the cache
		cache.Clear()
		g.Expect(cache.ListKeys()).To(BeEmpty())

		// close the cache
		err = cache.Close()
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("Add expiring keys", func(t *testing.T) {
		g := NewWithT(t)
		// new cache with a cleanup interval of 1 second

		cache, err := New[string](2,
			WithCleanupInterval(1*time.Second),
			WithMetricsRegisterer(prometheus.NewPedanticRegistry()))
		g.Expect(err).ToNot(HaveOccurred())

		// Add an object representing an expiring token
		key := "key1"
		value := "val1"

		err = cache.Set(key, value)
		g.Expect(err).ToNot(HaveOccurred())

		// set expiration time to 2 seconds
		err = cache.SetExpiration(key, time.Now().Add(2*time.Second))
		g.Expect(err).ToNot(HaveOccurred())

		// Get the item from the cache
		item, err := cache.Get(key)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(*item).To(Equal(value))

		// wait for the item to expire
		time.Sleep(3 * time.Second)

		// Get the item from the cache
		item, err = cache.Get(key)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(item).To(BeNil())
	})

	t.Run("Cache of integer value", func(t *testing.T) {
		g := NewWithT(t)

		cache, err := New[int](3, WithMetricsRegisterer(prometheus.NewPedanticRegistry()))
		g.Expect(err).ToNot(HaveOccurred())

		key := "key1"
		g.Expect(cache.Set(key, 4)).To(Succeed())

		got, err := cache.Get(key)
		g.Expect(err).To(Succeed())
		g.Expect(*got).To(Equal(4))
	})
}

func Test_Cache_Set(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := New[string](1,
		WithMetricsRegisterer(reg),
		WithMetricsPrefix("gotk_"),
		WithCleanupInterval(10*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	key1 := "key1"
	value1 := "val1"
	err = cache.Set(key1, value1)
	g.Expect(err).ToNot(HaveOccurred())
	err = cache.SetExpiration(key1, time.Now().Add(10*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf(key1))

	// try adding the same object again, it should overwrite the existing one
	err = cache.Set(key1, value1)
	g.Expect(err).ToNot(HaveOccurred())

	// wait for the item to expire
	time.Sleep(20 * time.Millisecond)
	ok, err := cache.HasExpired(key1)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ok).To(BeTrue())

	// add another object
	key2 := "key2"
	value2 := "val2"
	err = cache.Set(key2, value2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf(key2))

	// Update the value of existing item.
	value3 := "val3"
	err = cache.Set(key2, value3)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf(key2))
	g.Expect(cache.index[key2].value).To(Equal(value3))

	// validate metrics
	validateMetrics(reg, `
		# HELP gotk_cache_evictions_total Total number of cache evictions.
		# TYPE gotk_cache_evictions_total counter
		gotk_cache_evictions_total 1
		# HELP gotk_cache_requests_total Total number of cache requests partioned by success or failure.
		# TYPE gotk_cache_requests_total counter
		gotk_cache_requests_total{status="success"} 9
		# HELP gotk_cached_items Total number of items in the cache.
		# TYPE gotk_cached_items gauge
		gotk_cached_items 1
`, t)
}

func Test_Cache_Get(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := New[string](5, WithMetricsRegisterer(reg), WithMetricsPrefix("gotk_"))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconciling object label values for cache event metric.
	recObjKind := "TestObject"
	recObjName := "test"
	recObjNamespace := "test-ns"

	// Add an object representing an expiring token
	key := "key1"
	value := "val1"

	got, err := cache.Get(key)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(got).To(BeNil())
	cache.RecordCacheEvent(CacheEventTypeMiss, recObjKind, recObjName, recObjNamespace)

	err = cache.Set(key, value)
	g.Expect(err).ToNot(HaveOccurred())

	got, err = cache.Get(key)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(*got).To(Equal(value))
	cache.RecordCacheEvent(CacheEventTypeHit, recObjKind, recObjName, recObjNamespace)

	validateMetrics(reg, `
	# HELP gotk_cache_events_total Total number of cache retrieval events for a Gitops Toolkit resource reconciliation.
	# TYPE gotk_cache_events_total counter
	gotk_cache_events_total{event_type="cache_hit",kind="TestObject",name="test",namespace="test-ns"} 1
	gotk_cache_events_total{event_type="cache_miss",kind="TestObject",name="test",namespace="test-ns"} 1
	# HELP gotk_cache_evictions_total Total number of cache evictions.
	# TYPE gotk_cache_evictions_total counter
	gotk_cache_evictions_total 0
	# HELP gotk_cache_requests_total Total number of cache requests partioned by success or failure.
	# TYPE gotk_cache_requests_total counter
	gotk_cache_requests_total{status="success"} 3
	# HELP gotk_cached_items Total number of items in the cache.
	# TYPE gotk_cached_items gauge
	gotk_cached_items 1
`, t)

	cache.DeleteCacheEvent(CacheEventTypeHit, recObjKind, recObjName, recObjNamespace)
	cache.DeleteCacheEvent(CacheEventTypeMiss, recObjKind, recObjName, recObjNamespace)

	validateMetrics(reg, `
	# HELP gotk_cache_evictions_total Total number of cache evictions.
	# TYPE gotk_cache_evictions_total counter
	gotk_cache_evictions_total 0
	# HELP gotk_cache_requests_total Total number of cache requests partioned by success or failure.
	# TYPE gotk_cache_requests_total counter
	gotk_cache_requests_total{status="success"} 3
	# HELP gotk_cached_items Total number of items in the cache.
	# TYPE gotk_cached_items gauge
	gotk_cached_items 1
`, t)

}

func Test_Cache_Delete(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := New[string](5,
		WithMetricsRegisterer(reg),
		WithMetricsPrefix("gotk_"),
		WithCleanupInterval(1*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	key := "key1"
	value := "value1"

	err = cache.Set(key, value)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf(key))

	err = cache.Delete(key)
	g.Expect(err).ToNot(HaveOccurred())

	time.Sleep(5 * time.Millisecond)
	g.Expect(cache.ListKeys()).To(BeEmpty())

	validateMetrics(reg, `
	# HELP gotk_cache_evictions_total Total number of cache evictions.
	# TYPE gotk_cache_evictions_total counter
	gotk_cache_evictions_total 1
	# HELP gotk_cache_requests_total Total number of cache requests partioned by success or failure.
	# TYPE gotk_cache_requests_total counter
	gotk_cache_requests_total{status="success"} 4
	# HELP gotk_cached_items Total number of items in the cache.
	# TYPE gotk_cached_items gauge
	gotk_cached_items 0
`, t)
}

func Test_Cache_deleteExpired(t *testing.T) {
	type expiringItem struct {
		key       string
		value     string
		expiresAt time.Time
		expire    bool
	}
	tests := []struct {
		name           string
		items          []expiringItem
		nonExpiredKeys []string
	}{
		{
			name: "non expiring items",
			items: []expiringItem{
				{
					key:       "test",
					value:     "test-token",
					expiresAt: time.Now().Add(noExpiration),
				},
				{
					key:       "test2",
					value:     "test-token2",
					expiresAt: time.Now().Add(noExpiration),
				},
			},
			nonExpiredKeys: []string{"test", "test2"},
		},
		{
			name: "expiring items",
			items: []expiringItem{
				{
					key:       "test",
					value:     "test-token",
					expiresAt: time.Now().Add(1 * time.Millisecond),
					expire:    true,
				},
				{
					key:       "test2",
					value:     "test-token2",
					expiresAt: time.Now().Add(1 * time.Millisecond),
					expire:    true,
				},
			},
			nonExpiredKeys: []string{},
		},
		{
			name: "mixed items",
			items: []expiringItem{
				{
					key:       "test",
					value:     "test-token",
					expiresAt: time.Now().Add(1 * time.Millisecond),
					expire:    true,
				},
				{
					key:       "test2",
					value:     "test-token2",
					expiresAt: time.Now().Add(noExpiration),
				},
				{
					key:       "test3",
					value:     "test-token3",
					expiresAt: time.Now().Add(1 * time.Minute),
					expire:    true,
				},
			},
			nonExpiredKeys: []string{"test2", "test3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			reg := prometheus.NewPedanticRegistry()
			cache, err := New[string](5,
				WithMetricsRegisterer(reg),
				WithCleanupInterval(1*time.Millisecond))
			g.Expect(err).ToNot(HaveOccurred())

			for _, item := range tt.items {
				err := cache.Set(item.key, item.value)
				g.Expect(err).ToNot(HaveOccurred())
				if item.expire {
					err = cache.SetExpiration(item.key, item.expiresAt)
					g.Expect(err).ToNot(HaveOccurred())
				}
			}

			time.Sleep(5 * time.Millisecond)
			keys, err := cache.ListKeys()
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(keys).To(ConsistOf(tt.nonExpiredKeys))
		})
	}
}

func Test_Cache_Resize(t *testing.T) {
	n := 100
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()

	cache, err := New[string](n,
		WithMetricsRegisterer(reg),
		WithCleanupInterval(10*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())

	for i := range n {
		key := fmt.Sprintf("test-%d", i)
		value := "test-token"
		err = cache.Set(key, value)
		g.Expect(err).ToNot(HaveOccurred())
		err = cache.SetExpiration(key, time.Now().Add(10*time.Minute))
		g.Expect(err).ToNot(HaveOccurred())
	}

	deleted, err := cache.Resize(10)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(deleted).To(Equal(n - 10))
	g.Expect(cache.ListKeys()).To(HaveLen(10))
	g.Expect(cache.capacity).To(Equal(10))

	deleted, err = cache.Resize(15)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(deleted).To(Equal(0))
	g.Expect(cache.ListKeys()).To(HaveLen(10))
	g.Expect(cache.capacity).To(Equal(15))
}

func TestCache_Concurrent(t *testing.T) {
	const (
		concurrency = 500
		keysNum     = 10
	)
	g := NewWithT(t)
	// create a cache that can hold 10 items and have no cleanup
	cache, err := New[string](10,
		WithCleanupInterval(1*time.Second),
		WithMetricsRegisterer(prometheus.NewPedanticRegistry()))
	g.Expect(err).ToNot(HaveOccurred())

	keymap := map[int]string{}
	for i := 0; i < keysNum; i++ {
		key := fmt.Sprintf("test-%d", i)
		keymap[i] = key
	}

	wg := sync.WaitGroup{}
	run := make(chan bool)

	// simulate concurrent read and write
	for i := 0; i < concurrency; i++ {
		key := rand.IntN(keysNum)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = cache.Set(keymap[key], "test-token")
		}()
		go func() {
			defer wg.Done()
			<-run
			_, _ = cache.Get(keymap[key])
			_ = cache.SetExpiration(keymap[key], time.Now().Add(noExpiration))
		}()
	}
	close(run)
	wg.Wait()

	keys, err := cache.ListKeys()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(keys)).To(Equal(len(keymap)))

	for _, key := range keymap {
		val, err := cache.Get(key)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(val).ToNot(BeNil(), "object %s not found", key)
		g.Expect(*val).To(Equal("test-token"))
	}
}
