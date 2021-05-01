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
	"context"
	"time"

	"github.com/go-logr/logr"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
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
// then you can call either or both of RecordDuration and
// RecordReadinessMetric. API types used in GOTK will usually
// already be suitable for passing (as a pointer) as the second
// argument to `RecordReadinessMetric`.
//
// When initialising controllers in main.go, use MustMakeMetrics to
// create a working Metrics value; you can supply the same value to
// all reconcilers.
type Metrics struct {
	Scheme          *runtime.Scheme
	MetricsRecorder *metrics.Recorder
}

func MustMakeMetrics(mgr ctrl.Manager) Metrics {
	metricsRecorder := metrics.NewRecorder()
	crtlmetrics.Registry.MustRegister(metricsRecorder.Collectors()...)

	return Metrics{
		Scheme:          mgr.GetScheme(),
		MetricsRecorder: metricsRecorder,
	}
}

func (m Metrics) RecordDuration(ctx context.Context, obj readinessMetricsable, startTime time.Time) {
	if m.MetricsRecorder != nil {
		ref, err := reference.GetReference(m.Scheme, obj)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record duration")
			return
		}
		m.MetricsRecorder.RecordDuration(*ref, startTime)
	}
}

func (m Metrics) RecordSuspend(ctx context.Context, obj readinessMetricsable, suspend bool) {
	if m.MetricsRecorder != nil {
		ref, err := reference.GetReference(m.Scheme, obj)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record suspend")
			return
		}
		m.MetricsRecorder.RecordSuspend(*ref, suspend)
	}
}

type readinessMetricsable interface {
	runtime.Object
	metav1.Object
	meta.ObjectWithStatusConditions
}

func (m Metrics) RecordReadinessMetric(ctx context.Context, obj readinessMetricsable) {
	m.RecordConditionMetric(ctx, obj, meta.ReadyCondition)
}

func (m Metrics) RecordConditionMetric(ctx context.Context, obj readinessMetricsable, conditionType string) {
	if m.MetricsRecorder == nil {
		return
	}
	ref, err := reference.GetReference(m.Scheme, obj)
	if err != nil {
		logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record condition metric")
		return
	}
	rc := apimeta.FindStatusCondition(*obj.GetStatusConditions(), conditionType)
	if rc == nil {
		rc = &metav1.Condition{
			Type:   conditionType,
			Status: metav1.ConditionUnknown,
		}
	}
	m.MetricsRecorder.RecordCondition(*ref, *rc, !obj.GetDeletionTimestamp().IsZero())
}
