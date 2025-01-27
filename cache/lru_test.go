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

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func Test_LRU(t *testing.T) {
	type keyVal struct {
		key   string
		value string
	}
	testCases := []struct {
		name          string
		inputs        []keyVal
		expectedCache map[string]*node[string]
	}{
		{
			name:          "empty cache",
			inputs:        []keyVal{},
			expectedCache: map[string]*node[string]{},
		},
		{
			name: "add one node",
			inputs: []keyVal{
				{
					key:   "test",
					value: "test-token",
				},
			},
			expectedCache: map[string]*node[string]{
				"test": {
					key:   "test",
					value: "test-token",
				},
			},
		},
		{
			name: "add seven nodes",
			inputs: []keyVal{
				{key: "test", value: "test-token"},
				{key: "test2", value: "test-token"},
				{key: "test3", value: "test-token"},
				{key: "test4", value: "test-token"},
				{key: "test5", value: "test-token"},
				{key: "test6", value: "test-token"},
				{key: "test7", value: "test-token"},
			},
			expectedCache: map[string]*node[string]{
				"test3": {key: "test3", value: "test-token"},
				"test4": {key: "test4", value: "test-token"},
				"test5": {key: "test5", value: "test-token"},
				"test6": {key: "test6", value: "test-token"},
				"test7": {key: "test7", value: "test-token"},
			},
		},
	}

	for _, v := range testCases {
		t.Run(v.name, func(t *testing.T) {
			g := NewWithT(t)
			cache, err := NewLRU[string](5,
				WithMetricsRegisterer(prometheus.NewPedanticRegistry()))
			g.Expect(err).ToNot(HaveOccurred())
			for _, input := range v.inputs {
				err := cache.Set(input.key, input.value)
				g.Expect(err).ToNot(HaveOccurred())
			}

			g.Expect(cache.cache).To(HaveLen(len(v.expectedCache)))
			for k, v := range v.expectedCache {
				if node, ok := cache.cache[k]; !ok {
					t.Errorf("Expected key %s, got %s", k, node.key)
				}
				g.Expect(cache.cache[k].key).To(Equal(v.key))
			}
		})
	}
}

func Test_LRU_Set(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[string](1,
		WithMetricsRegisterer(reg),
		WithMetricsPrefix("gotk_"))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	key1 := "key1"
	value1 := "val1"
	err = cache.Set(key1, value1)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf(key1))

	// try adding the same object again, it should overwrite the existing one
	err = cache.Set(key1, value1)
	g.Expect(err).ToNot(HaveOccurred())

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
	g.Expect(cache.cache[key2].value).To(Equal(value3))

	// validate metrics
	validateMetrics(reg, `
	# HELP gotk_cache_evictions_total Total number of cache evictions.
	# TYPE gotk_cache_evictions_total counter
	gotk_cache_evictions_total 1
	# HELP gotk_cache_requests_total Total number of cache requests partioned by success or failure.
	# TYPE gotk_cache_requests_total counter
	gotk_cache_requests_total{status="success"} 7
	# HELP gotk_cached_items Total number of items in the cache.
	# TYPE gotk_cached_items gauge
	gotk_cached_items 1
`, t)
}

func Test_LRU_Get(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[string](5,
		WithMetricsRegisterer(reg),
		WithMetricsPrefix("gotk_"))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconciling object label values for cache event metric.
	recObjKind := "TestObject"
	recObjName := "test"
	recObjNamespace := "test-ns"

	// Add an object representing an expiring token
	key1 := "key1"
	value1 := "val1"
	got, err := cache.Get(key1)
	g.Expect(err).To(Equal(ErrNotFound))
	g.Expect(got).To(BeEmpty())
	cache.RecordCacheEvent(CacheEventTypeMiss, recObjKind, recObjName, recObjNamespace)

	err = cache.Set(key1, value1)
	g.Expect(err).ToNot(HaveOccurred())

	got, err = cache.Get(key1)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(got).To(Equal(value1))
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

func Test_LRU_Delete(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[string](5,
		WithMetricsRegisterer(reg),
		WithMetricsPrefix("gotk_"))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	key := "key1"
	value := "val1"
	err = cache.Set(key, value)
	g.Expect(err).ToNot(HaveOccurred())

	err = cache.Delete(key)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(BeEmpty())

	validateMetrics(reg, `
	# HELP gotk_cache_evictions_total Total number of cache evictions.
	# TYPE gotk_cache_evictions_total counter
	gotk_cache_evictions_total 0
	# HELP gotk_cache_requests_total Total number of cache requests partioned by success or failure.
	# TYPE gotk_cache_requests_total counter
	gotk_cache_requests_total{status="success"} 3
	# HELP gotk_cached_items Total number of items in the cache.
	# TYPE gotk_cached_items gauge
	gotk_cached_items 0
`, t)
}

func Test_LRU_Resize(t *testing.T) {
	n := 100
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[string](n,
		WithMetricsRegisterer(reg))
	g.Expect(err).ToNot(HaveOccurred())

	for i := range n {
		key := fmt.Sprintf("test-%d", i)
		err = cache.Set(key, "test-token")
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

func TestLRU_Concurrent(t *testing.T) {
	const (
		concurrency = 500
		keysNum     = 10
	)
	g := NewWithT(t)
	// create a cache that can hold 10 items and have no cleanup
	cache, err := NewLRU[string](10,
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
		g.Expect(val).To(Equal("test-token"))
	}
}

func TestLRU_int(t *testing.T) {
	g := NewWithT(t)

	cache, err := NewLRU[int](3, WithMetricsRegisterer(prometheus.NewPedanticRegistry()))
	g.Expect(err).ToNot(HaveOccurred())

	key := "key1"
	g.Expect(cache.Set(key, 4)).To(Succeed())

	got, err := cache.Get(key)
	g.Expect(err).To(Succeed())
	g.Expect(got).To(Equal(4))
}
