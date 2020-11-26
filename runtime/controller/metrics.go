/*
Copyright 2020 The Flux authors

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

package controller

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/metrics"
)

// Metrics adds the capability for recording GOTK-standard metrics to
// a reconciler. Use by embedding into the reconciler struct:
//
//     type MyTypeReconciler struct {
//       client.Client
//       // ...
//       controller.Metrics
//     }
//
// then you can call either or both of `RecordDuration` and
// `RecordReadinessMetric`. API types used in GOTK will usually
// already be suitable for passing (as a pointer) as the second
// argument to `RecordReadinessMetric`.
//
// When initialising controllers in main.go, use `MustMakeMetrics` to
// create a working Metrics value; you can supply the same value to
// all reconcilers.
type Metrics struct {
	MetricsRecorder *metrics.Recorder
}

func MustMakeMetrics() Metrics {
	metricsRecorder := metrics.NewRecorder()
	crtlmetrics.Registry.MustRegister(metricsRecorder.Collectors()...)
	return Metrics{MetricsRecorder: metricsRecorder}
}

func (m Metrics) RecordDuration(ref *corev1.ObjectReference, startTime time.Time) {
	if m.MetricsRecorder != nil {
		m.MetricsRecorder.RecordDuration(*ref, startTime)
	}
}

type readinessMetricsable interface {
	metav1.Object
	meta.ObjectWithStatusConditions
}

func (m Metrics) RecordReadinessMetric(ref *corev1.ObjectReference, obj readinessMetricsable) {
	if m.MetricsRecorder == nil {
		return
	}
	if rc := apimeta.FindStatusCondition(*obj.GetStatusConditions(), meta.ReadyCondition); rc != nil {
		m.MetricsRecorder.RecordCondition(*ref, *rc, !obj.GetDeletionTimestamp().IsZero())
	} else {
		m.MetricsRecorder.RecordCondition(*ref, metav1.Condition{
			Type:   meta.ReadyCondition,
			Status: metav1.ConditionUnknown,
		}, !obj.GetDeletionTimestamp().IsZero())
	}
}
