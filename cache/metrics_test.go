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
	"bytes"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCacheMetrics(t *testing.T) {
	g := NewWithT(t)
	reg := prometheus.NewPedanticRegistry()
	m := newCacheMetrics(reg, IdentifiableObjectLabels...)
	g.Expect(m).ToNot(BeNil())

	// CounterVec is a collection of counters and is not exported until it has counters in it.
	m.incCacheEvents(CacheEventTypeHit, []string{"test", "test-ns", "TestObject"}...)
	m.incCacheEvents(CacheEventTypeMiss, []string{"test", "test-ns", "TestObject"}...)
	m.incCacheRequests("success")
	m.incCacheRequests("failure")

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
		gotk_cache_requests_total{status="failure"} 1
		gotk_cache_requests_total{status="success"} 1
		# HELP gotk_cached_items Total number of items in the cache.
		# TYPE gotk_cached_items gauge
		gotk_cached_items 0
	`, t)

	res, err := testutil.GatherAndLint(reg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(BeEmpty())
}

func validateMetrics(reg prometheus.Gatherer, expected string, t *testing.T) {
	g := NewWithT(t)
	err := testutil.GatherAndCompare(reg, bytes.NewBufferString(expected))
	g.Expect(err).ToNot(HaveOccurred())
}
