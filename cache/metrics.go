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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// CacheEventTypeMiss is the event type for cache misses.
	CacheEventTypeMiss = "cache_miss"
	// CacheEventTypeHit is the event type for cache hits.
	CacheEventTypeHit = "cache_hit"
	// StatusSuccess is the status for successful cache requests.
	StatusSuccess = "success"
	// StatusFailure is the status for failed cache requests.
	StatusFailure = "failure"
)

var allEventTypes = []string{
	CacheEventTypeMiss,
	CacheEventTypeHit,
}

type cacheMetrics struct {
	// cacheEventsCounter is a counter for cache events.
	cacheEventsCounter   *prometheus.CounterVec
	cacheItemsGauge      prometheus.Gauge
	cacheRequestsCounter *prometheus.CounterVec
	cacheEvictionCounter prometheus.Counter
}

// newcacheMetrics returns a new cacheMetrics.
func newCacheMetrics(prefix string, reg prometheus.Registerer, opts ...Options) *cacheMetrics {
	o := storeOptions{eventNamespaceLabel: "namespace"}
	o.apply(opts...)

	return &cacheMetrics{
		cacheEventsCounter: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%scache_events_total", prefix),
				Help: "Total number of cache retrieval events for a Gitops Toolkit resource reconciliation.",
			},
			[]string{"event_type", "kind", "name", o.eventNamespaceLabel, "operation"},
		),
		cacheItemsGauge: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{
				Name: fmt.Sprintf("%scached_items", prefix),
				Help: "Total number of items in the cache.",
			},
		),
		cacheRequestsCounter: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%scache_requests_total", prefix),
				Help: "Total number of cache requests partioned by success or failure.",
			},
			[]string{"status"},
		),
		cacheEvictionCounter: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%scache_evictions_total", prefix),
				Help: "Total number of cache evictions.",
			},
		),
	}
}

// collectors returns the metrics.Collector objects for the cacheMetrics.
func (m *cacheMetrics) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.cacheEventsCounter,
		m.cacheItemsGauge,
		m.cacheRequestsCounter,
		m.cacheEvictionCounter,
	}
}

// incCacheEventCount increment by 1 the cache event count for the given event type, name and namespace.
func (m *cacheMetrics) incCacheEvents(event string, lvs ...string) {
	lvs = append([]string{event}, lvs...)
	m.cacheEventsCounter.WithLabelValues(lvs...).Inc()
}

// deleteCacheEvent deletes the cache event metric.
func (m *cacheMetrics) deleteCacheEvent(event string, lvs ...string) {
	lvs = append([]string{event}, lvs...)
	m.cacheEventsCounter.DeleteLabelValues(lvs...)
}

// SetCachedItems sets the number of cached items.
func (m *cacheMetrics) setCachedItems(value float64) {
	m.cacheItemsGauge.Set(value)
}

// incCacheItems increments the number of cached items by 1.
func (m *cacheMetrics) incCacheItems() {
	m.cacheItemsGauge.Inc()
}

// decCacheItems decrements the number of cached items by 1.
func (m *cacheMetrics) decCacheItems() {
	m.cacheItemsGauge.Dec()
}

// incCacheRequests increments the cache request count for the given status.
func (m *cacheMetrics) incCacheRequests(status string) {
	m.cacheRequestsCounter.WithLabelValues(status).Inc()
}

// incCacheEvictions increments the cache eviction count by 1.
func (m *cacheMetrics) incCacheEvictions() {
	m.cacheEvictionCounter.Inc()
}

// MustMakeMetrics registers the metrics collectors in the given registerer.
func MustMakeMetrics(r prometheus.Registerer, m *cacheMetrics) {
	r.MustRegister(m.collectors()...)
}

func recordRequest(metrics *cacheMetrics, status string) {
	if metrics != nil {
		metrics.incCacheRequests(status)
	}
}

func recordEviction(metrics *cacheMetrics) {
	if metrics != nil {
		metrics.incCacheEvictions()
	}
}

func recordDecrement(metrics *cacheMetrics) {
	if metrics != nil {
		metrics.decCacheItems()
	}
}

func recordItemIncrement(metrics *cacheMetrics) {
	if metrics != nil {
		metrics.incCacheItems()
	}
}

func recordCacheEvent(metrics *cacheMetrics, event, kind, name, namespace, operation string) {
	if metrics != nil {
		metrics.incCacheEvents(event, kind, name, namespace, operation)
	}
}

func deleteCacheEvent(metrics *cacheMetrics, event, kind, name, namespace, operation string) {
	if metrics != nil {
		metrics.deleteCacheEvent(event, kind, name, namespace, operation)
	}
}
