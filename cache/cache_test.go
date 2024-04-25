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

	"github.com/fluxcd/cli-utils/pkg/object"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kc "k8s.io/client-go/tools/cache"
)

func TestCache(t *testing.T) {
	t.Run("Add and update keys", func(t *testing.T) {
		g := NewWithT(t)
		// create a cache that can hold 2 items and have no cleanup
		cache, err := New(3, kc.MetaNamespaceKeyFunc,
			WithMetricsRegisterer[any](prometheus.NewPedanticRegistry()),
			WithCleanupInterval[any](1*time.Second))
		g.Expect(err).ToNot(HaveOccurred())

		obj := &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				Kind:       "TestObject",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-ns",
				Name:      "test",
			},
		}

		// Get an Item from the cache
		_, found, err := cache.Get(obj)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeFalse())

		// Add an item to the cache
		err = cache.Set(obj)
		g.Expect(err).ToNot(HaveOccurred())

		// Get the item from the cache
		item, found, err := cache.Get(obj)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(item).To(Equal(obj))

		obj2 := &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				Kind:       "TestObject",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-ns",
				Name:      "test2",
			},
		}
		// Add another item to the cache
		err = cache.Set(obj2)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cache.ListKeys()).To(ConsistOf("test-ns/test", "test-ns/test2"))

		// Get the item from the cache
		item, found, err = cache.GetByKey("test-ns/test2")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(item).To(Equal(obj2))

		//Update an item in the cache
		obj3 := &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				Kind:       "TestObject",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-ns",
				Name:      "test3",
			},
		}
		err = cache.Set(obj3)
		g.Expect(err).ToNot(HaveOccurred())

		// Replace an item in the cache
		obj3.Labels = map[string]string{"pp.kubernetes.io/created-by: ": "flux"}
		err = cache.Set(obj3)
		g.Expect(err).ToNot(HaveOccurred())

		// Get the item from the cache
		item, found, err = cache.Get(obj3)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(item).To(Equal(obj3))

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
		cache, err := New(2, IdentifiableObjectKeyFunc,
			WithCleanupInterval[IdentifiableObject](1*time.Second),
			WithMetricsRegisterer[IdentifiableObject](prometheus.NewPedanticRegistry()))
		g.Expect(err).ToNot(HaveOccurred())

		// Add an object representing an expiring token
		obj := IdentifiableObject{
			ObjMetadata: object.ObjMetadata{
				Namespace: "test-ns",
				Name:      "test",
				GroupKind: schema.GroupKind{
					Group: "test-group",
					Kind:  "TestObject",
				},
			},
			Object: struct {
				token string
			}{
				token: "test-token",
			},
		}

		err = cache.Set(obj)
		g.Expect(err).ToNot(HaveOccurred())

		// set expiration time to 2 seconds
		err = cache.SetExpiration(obj, time.Now().Add(2*time.Second))
		g.Expect(err).ToNot(HaveOccurred())

		// Get the item from the cache
		item, found, err := cache.Get(obj)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(item).To(Equal(obj))

		// wait for the item to expire
		time.Sleep(3 * time.Second)

		// Get the item from the cache
		item, found, err = cache.Get(obj)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeFalse())
		g.Expect(item.Object).To(BeNil())
	})
}

func Test_Cache_Add(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := New[IdentifiableObject](1, IdentifiableObjectKeyFunc,
		WithMetricsRegisterer[IdentifiableObject](reg),
		WithCleanupInterval[IdentifiableObject](10*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	obj := IdentifiableObject{
		ObjMetadata: object.ObjMetadata{
			Namespace: "test-ns",
			Name:      "test",
			GroupKind: schema.GroupKind{
				Group: "test-group",
				Kind:  "TestObject",
			},
		},
		Object: "test-token",
	}
	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())
	err = cache.SetExpiration(obj, time.Now().Add(10*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf("test-ns_test_test-group_TestObject"))

	// try adding the same object again, it should overwrite the existing one
	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())

	// wait for the item to expire
	time.Sleep(20 * time.Millisecond)
	ok, err := cache.HasExpired(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ok).To(BeTrue())

	// add another object
	obj.Name = "test2"
	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf("test-ns_test2_test-group_TestObject"))

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

func Test_Cache_Update(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := New[IdentifiableObject](1, IdentifiableObjectKeyFunc,
		WithMetricsRegisterer[IdentifiableObject](reg))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	obj := IdentifiableObject{
		ObjMetadata: object.ObjMetadata{
			Namespace: "test-ns",
			Name:      "test",
			GroupKind: schema.GroupKind{
				Group: "test-group",
				Kind:  "TestObject",
			},
		},
		Object: "test-token",
	}
	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf("test-ns_test_test-group_TestObject"))

	obj.Object = "test-token2"
	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf("test-ns_test_test-group_TestObject"))
	g.Expect(cache.index["test-ns_test_test-group_TestObject"].object.Object).To(Equal("test-token2"))

	// validate metrics
	validateMetrics(reg, `
		# HELP gotk_cache_evictions_total Total number of cache evictions.
		# TYPE gotk_cache_evictions_total counter
		gotk_cache_evictions_total 0
		# HELP gotk_cache_requests_total Total number of cache requests partioned by success or failure.
		# TYPE gotk_cache_requests_total counter
		gotk_cache_requests_total{status="success"} 4
		# HELP gotk_cached_items Total number of items in the cache.
		# TYPE gotk_cached_items gauge
		gotk_cached_items 1
	`, t)
}

func Test_Cache_Get(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := New[IdentifiableObject](5, IdentifiableObjectKeyFunc,
		WithMetricsRegisterer[IdentifiableObject](reg),
		WithMetricsLabels[IdentifiableObject](IdentifiableObjectLabels, IdentifiableObjectLVSFunc))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	obj := IdentifiableObject{
		ObjMetadata: object.ObjMetadata{
			Namespace: "test-ns",
			Name:      "test",
			GroupKind: schema.GroupKind{
				Group: "test-group",
				Kind:  "TestObject",
			},
		},
		Object: "test-token",
	}

	_, found, err := cache.Get(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeFalse())

	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())

	item, found, err := cache.Get(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(item).To(Equal(obj))

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
}

func Test_Cache_Delete(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := New[IdentifiableObject](5, IdentifiableObjectKeyFunc,
		WithMetricsRegisterer[IdentifiableObject](reg),
		WithCleanupInterval[IdentifiableObject](1*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())

	// Add an object representing an expiring token
	obj := IdentifiableObject{
		ObjMetadata: object.ObjMetadata{
			Namespace: "test-ns",
			Name:      "test",
			GroupKind: schema.GroupKind{
				Group: "test-group",
				Kind:  "TestObject",
			},
		},
		Object: "test-token",
	}

	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cache.ListKeys()).To(ConsistOf("test-ns_test_test-group_TestObject"))

	err = cache.Delete(obj)
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
		object    StoreObject[string]
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
					object: StoreObject[string]{
						Object: "test-token",
						Key:    "test",
					},
					expiresAt: time.Now().Add(noExpiration),
				},
				{
					object: StoreObject[string]{
						Object: "test-token2",
						Key:    "test2",
					},
					expiresAt: time.Now().Add(noExpiration),
				},
			},
			nonExpiredKeys: []string{"test", "test2"},
		},
		{
			name: "expiring items",
			items: []expiringItem{
				{
					object: StoreObject[string]{
						Object: "test-token",
						Key:    "test",
					},
					expiresAt: time.Now().Add(1 * time.Millisecond),
					expire:    true,
				},
				{
					object: StoreObject[string]{
						Object: "test-token2",
						Key:    "test2",
					},
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
					object: StoreObject[string]{
						Object: "test-token",
						Key:    "test",
					},
					expiresAt: time.Now().Add(1 * time.Millisecond),
					expire:    true,
				},
				{
					object: StoreObject[string]{
						Object: "test-token2",
						Key:    "test2",
					},
					expiresAt: time.Now().Add(noExpiration),
				},
				{
					object: StoreObject[string]{
						Object: "test-token3",
						Key:    "test3",
					},
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
			cache, err := New[StoreObject[string]](5, StoreObjectKeyFunc,
				WithMetricsRegisterer[StoreObject[string]](reg),
				WithCleanupInterval[StoreObject[string]](1*time.Millisecond))
			g.Expect(err).ToNot(HaveOccurred())

			for _, item := range tt.items {
				err := cache.Set(item.object)
				g.Expect(err).ToNot(HaveOccurred())
				if item.expire {
					err = cache.SetExpiration(item.object, item.expiresAt)
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
	cache, err := New[IdentifiableObject](n, IdentifiableObjectKeyFunc,
		WithMetricsRegisterer[IdentifiableObject](reg),
		WithCleanupInterval[IdentifiableObject](10*time.Millisecond))
	g.Expect(err).ToNot(HaveOccurred())

	for i := range n {
		obj := IdentifiableObject{
			ObjMetadata: object.ObjMetadata{
				Namespace: "test-ns",
				Name:      fmt.Sprintf("test-%d", i),
				GroupKind: schema.GroupKind{
					Group: "test-group",
					Kind:  "TestObject",
				},
			},
			Object: "test-token",
		}
		err = cache.Set(obj)
		g.Expect(err).ToNot(HaveOccurred())
		err = cache.SetExpiration(obj, time.Now().Add(10*time.Minute))
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
	cache, err := New(10, IdentifiableObjectKeyFunc,
		WithCleanupInterval[IdentifiableObject](1*time.Second),
		WithMetricsRegisterer[IdentifiableObject](prometheus.NewPedanticRegistry()))
	g.Expect(err).ToNot(HaveOccurred())

	objmap := createObjectMap(keysNum)

	wg := sync.WaitGroup{}
	run := make(chan bool)

	// simulate concurrent read and write
	for i := 0; i < concurrency; i++ {
		key := rand.IntN(keysNum)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = cache.Set(objmap[key])
		}()
		go func() {
			defer wg.Done()
			<-run
			_, _, _ = cache.Get(objmap[key])
			_ = cache.SetExpiration(objmap[key], time.Now().Add(noExpiration))
		}()
	}
	close(run)
	wg.Wait()

	keys, err := cache.ListKeys()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(keys)).To(Equal(len(objmap)))

	for _, obj := range objmap {
		val, found, err := cache.Get(obj)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue(), "object %s not found", obj.Name)
		g.Expect(val).To(Equal(obj))
	}
}

func createObjectMap(num int) map[int]IdentifiableObject {
	objMap := make(map[int]IdentifiableObject)
	for i := 0; i < num; i++ {
		obj := IdentifiableObject{
			ObjMetadata: object.ObjMetadata{
				Namespace: "test-ns",
				Name:      fmt.Sprintf("test-%d", i),
				GroupKind: schema.GroupKind{
					Group: "test-group",
					Kind:  "TestObject",
				},
			},
			Object: struct {
				token string
			}{
				token: "test-token",
			},
		}
		objMap[i] = obj
	}
	return objMap
}
