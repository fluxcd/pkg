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

type cacheMetrics struct {
	// cacheEventsCounter is a counter for cache events.
	cacheEventsCounter   *prometheus.CounterVec
	cacheItemsGauge      prometheus.Gauge
	cacheRequestsCounter *prometheus.CounterVec
	cacheEvictionCounter prometheus.Counter
	extraLabels          []string
}

// newcacheMetrics returns a new cacheMetrics.
func newCacheMetrics(reg prometheus.Registerer, extraLabels ...string) *cacheMetrics {
	labels := append([]string{"event_type"}, extraLabels...)
	return &cacheMetrics{
		cacheEventsCounter: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "gotk_cache_events_total",
				Help: "Total number of cache retrieval events for a Gitops Toolkit resource reconciliation.",
			},
			labels,
		),
		cacheItemsGauge: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{
				Name: "gotk_cached_items",
				Help: "Total number of items in the cache.",
			},
		),
		cacheRequestsCounter: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "gotk_cache_requests_total",
				Help: "Total number of cache requests partioned by success or failure.",
			},
			[]string{"status"},
		),
		cacheEvictionCounter: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Name: "gotk_cache_evictions_total",
				Help: "Total number of cache evictions.",
			},
		),
		extraLabels: extraLabels,
	}
}

func (m *cacheMetrics) getExtraLabels() []string {
	return m.extraLabels
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

func recordEvent(metrics *cacheMetrics, event string, lvs ...string) {
	if metrics != nil {
		metrics.incCacheEvents(event, lvs...)
	}
}

func recordItemIncrement(metrics *cacheMetrics) {
	if metrics != nil {
		metrics.incCacheItems()
	}
}

// IdentifiableObjectLabels are the labels for an IdentifiableObject.
var IdentifiableObjectLabels []string = []string{"name", "namespace", "kind"}

// GetLvsFunc is a function that returns the label's values for a metric.
type GetLvsFunc[T any] func(obj T, cardinality int) ([]string, error)

// IdentifiableObjectLVSFunc returns the label's values for a metric for an IdentifiableObject.
func IdentifiableObjectLVSFunc[T any](object T, cardinality int) ([]string, error) {
	n, ok := any(object).(IdentifiableObject)
	if !ok {
		return nil, fmt.Errorf("object is not an IdentifiableObject")
	}
	lvs := []string{n.Name, n.Namespace, n.GroupKind.Kind}
	if len(lvs) != cardinality {
		return nil, fmt.Errorf("expected cardinality %d, got %d", cardinality, len(lvs))
	}

	return []string{n.Name, n.Namespace, n.GroupKind.Kind}, nil
}
