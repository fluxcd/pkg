/*
Copyright 2021 The Flux authors

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

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Recorder is a struct for recording GitOps Toolkit metrics for a controller.
//
// Use NewRecorder to initialise it with properly configured metric names.
type Recorder struct {
	conditionGauge    *prometheus.GaugeVec
	suspendGauge      *prometheus.GaugeVec
	durationHistogram *prometheus.HistogramVec
}

// MustMakeRecorder attempts to register the metrics collectors in the
// controller-runtime metrics registry, which panics upon the first registration
// that causes an error. Which usually happens if you try to initialise a
// Metrics value twice for your controller.
func MustMakeRecorder() *Recorder {
	metricsRecorder := NewRecorder()
	crtlmetrics.Registry.MustRegister(metricsRecorder.Collectors()...)
	return metricsRecorder
}

// NewRecorder returns a new Recorder with all metric names configured confirm GitOps Toolkit standards.
func NewRecorder() *Recorder {
	return &Recorder{
		conditionGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gotk_reconcile_condition",
				Help: "The current condition status of a GitOps Toolkit resource reconciliation.",
			},
			[]string{"kind", "name", "namespace", "type", "status"},
		),
		suspendGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gotk_suspend_status",
				Help: "The current suspend status of a GitOps Toolkit resource.",
			},
			[]string{"kind", "name", "namespace"},
		),
		durationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "gotk_reconcile_duration_seconds",
				Help: "The duration in seconds of a GitOps Toolkit resource reconciliation.",
				// Use a histogram with 10 count buckets between 1ms - 1hour
				Buckets: prometheus.ExponentialBucketsRange(10e-3, 1800, 10),
			},
			[]string{"kind", "name", "namespace"},
		),
	}
}

// Collectors returns a slice of Prometheus collectors, which can be used to register them in a metrics registry.
func (r *Recorder) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		r.conditionGauge,
		r.suspendGauge,
		r.durationHistogram,
	}
}

// RecordCondition records the condition as given for the ref.
func (r *Recorder) RecordCondition(ref corev1.ObjectReference, condition metav1.Condition) {
	for _, status := range []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown} {
		var value float64
		if status == condition.Status {
			value = 1
		}
		r.conditionGauge.WithLabelValues(ref.Kind, ref.Name, ref.Namespace, condition.Type, string(status)).Set(value)
	}
}

// DeleteCondition deletes the condition metrics for the ref.
func (r *Recorder) DeleteCondition(ref corev1.ObjectReference, conditionType string) {
	for _, status := range []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown} {
		r.conditionGauge.DeleteLabelValues(ref.Kind, ref.Name, ref.Namespace, conditionType, string(status))
	}
}

// RecordSuspend records the suspend status as given for the ref.
func (r *Recorder) RecordSuspend(ref corev1.ObjectReference, suspend bool) {
	var value float64
	if suspend {
		value = 1
	}
	r.suspendGauge.WithLabelValues(ref.Kind, ref.Name, ref.Namespace).Set(value)
}

// DeleteSuspend deletes the suspend metric for the ref.
func (r *Recorder) DeleteSuspend(ref corev1.ObjectReference) {
	r.suspendGauge.DeleteLabelValues(ref.Kind, ref.Name, ref.Namespace)
}

// RecordDuration records the duration since start for the given ref.
func (r *Recorder) RecordDuration(ref corev1.ObjectReference, start time.Time) {
	r.durationHistogram.WithLabelValues(ref.Kind, ref.Name, ref.Namespace).Observe(time.Since(start).Seconds())
}

// DeleteDuration deletes the duration metric for the ref.
func (r *Recorder) DeleteDuration(ref corev1.ObjectReference) {
	r.durationHistogram.DeleteLabelValues(ref.Kind, ref.Name, ref.Namespace)
}
