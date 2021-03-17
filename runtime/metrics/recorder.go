package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionDeleted = "Deleted"
)

type Recorder struct {
	conditionGauge    *prometheus.GaugeVec
	suspendGauge      *prometheus.GaugeVec
	durationHistogram *prometheus.HistogramVec
}

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
				Name:    "gotk_reconcile_duration_seconds",
				Help:    "The duration in seconds of a GitOps Toolkit resource reconciliation.",
				Buckets: prometheus.ExponentialBuckets(10e-9, 10, 10),
			},
			[]string{"kind", "name", "namespace"},
		),
	}
}

func (r *Recorder) Collectors() []prometheus.Collector {
	return []prometheus.Collector{r.conditionGauge, r.suspendGauge, r.durationHistogram}
}

func (r *Recorder) RecordCondition(ref corev1.ObjectReference, condition metav1.Condition, deleted bool) {
	for _, status := range []string{string(metav1.ConditionTrue), string(metav1.ConditionFalse), string(metav1.ConditionUnknown), ConditionDeleted} {
		var value float64
		if deleted {
			if status == ConditionDeleted {
				value = 1
			}
		} else {
			if status == string(condition.Status) {
				value = 1
			}
		}

		r.conditionGauge.WithLabelValues(ref.Kind, ref.Name, ref.Namespace, condition.Type, status).Set(value)
	}
}

func (r *Recorder) RecordSuspend(ref corev1.ObjectReference, suspend bool) {
	var value float64

	if suspend {
		value = 1
	}

	r.suspendGauge.WithLabelValues(ref.Kind, ref.Name, ref.Namespace).Set(value)
}

func (r *Recorder) RecordDuration(ref corev1.ObjectReference, start time.Time) {
	r.durationHistogram.WithLabelValues(ref.Kind, ref.Name, ref.Namespace).Observe(time.Since(start).Seconds())
}
