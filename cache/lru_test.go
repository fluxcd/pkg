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

	"github.com/fluxcd/cli-utils/pkg/object"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kc "k8s.io/client-go/tools/cache"
)

func Test_LRU(t *testing.T) {
	testCases := []struct {
		name          string
		inputs        []*metav1.PartialObjectMetadata
		expectedCache map[string]*node[metav1.PartialObjectMetadata]
	}{
		{
			name:          "empty cache",
			inputs:        []*metav1.PartialObjectMetadata{},
			expectedCache: map[string]*node[metav1.PartialObjectMetadata]{},
		},
		{
			name: "add one node",
			inputs: []*metav1.PartialObjectMetadata{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test",
					},
				},
			},
			expectedCache: map[string]*node[metav1.PartialObjectMetadata]{
				"test-ns/test": {
					object: metav1.PartialObjectMetadata{
						TypeMeta: metav1.TypeMeta{
							Kind:       "TestObject",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-ns",
							Name:      "test",
						},
					},
					key: "test-ns/test",
				},
			},
		},
		{
			name: "add seven nodes",
			inputs: []*metav1.PartialObjectMetadata{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test2",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test3",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test4",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test5",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test6",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TestObject",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-ns",
						Name:      "test7",
					},
				},
			},
			expectedCache: map[string]*node[metav1.PartialObjectMetadata]{
				"test-ns/test3": {
					object: metav1.PartialObjectMetadata{
						TypeMeta: metav1.TypeMeta{
							Kind:       "TestObject",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-ns",
							Name:      "test3",
						},
					},
					key: "test-ns/test3",
				},
				"test-ns/test4": {
					object: metav1.PartialObjectMetadata{
						TypeMeta: metav1.TypeMeta{
							Kind:       "TestObject",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-ns",
							Name:      "test4",
						},
					},
					key: "test-ns/test4",
				},
				"test-ns/test5": {
					object: metav1.PartialObjectMetadata{
						TypeMeta: metav1.TypeMeta{
							Kind:       "TestObject",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-ns",
							Name:      "test5",
						},
					},
					key: "test-ns/test5",
				},
				"test-ns/test6": {
					object: metav1.PartialObjectMetadata{
						TypeMeta: metav1.TypeMeta{
							Kind:       "TestObject",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-ns",
							Name:      "test6",
						},
					},
					key: "test-ns/test6",
				},
				"test-ns/test7": {
					object: metav1.PartialObjectMetadata{
						TypeMeta: metav1.TypeMeta{
							Kind:       "TestObject",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-ns",
							Name:      "test7",
						},
					},
					key: "test-ns/test7",
				},
			},
		},
	}

	for _, v := range testCases {
		t.Run(v.name, func(t *testing.T) {
			g := NewWithT(t)
			cache, err := NewLRU(5, kc.MetaNamespaceKeyFunc,
				WithMetricsRegisterer[any](prometheus.NewPedanticRegistry()))
			g.Expect(err).ToNot(HaveOccurred())
			for _, input := range v.inputs {
				err := cache.Set(input)
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

func Test_LRU_Add(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[IdentifiableObject](1, IdentifiableObjectKeyFunc,
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

	// try adding the same object again, it should overwrite the existing one
	err = cache.Set(obj)
	g.Expect(err).ToNot(HaveOccurred())

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
	gotk_cache_requests_total{status="success"} 5
	# HELP gotk_cached_items Total number of items in the cache.
	# TYPE gotk_cached_items gauge
	gotk_cached_items 1
`, t)
}

func Test_LRU_Update(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[IdentifiableObject](1, IdentifiableObjectKeyFunc,
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
	g.Expect(cache.cache["test-ns_test_test-group_TestObject"].object.Object).To(Equal("test-token2"))

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

func Test_LRU_Get(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[IdentifiableObject](5, IdentifiableObjectKeyFunc,
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

func Test_LRU_Delete(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	cache, err := NewLRU[IdentifiableObject](5, IdentifiableObjectKeyFunc,
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

	err = cache.Delete(obj)
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
	cache, err := NewLRU[IdentifiableObject](n, IdentifiableObjectKeyFunc,
		WithMetricsRegisterer[IdentifiableObject](reg))
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
	cache, err := NewLRU(10, IdentifiableObjectKeyFunc,
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
