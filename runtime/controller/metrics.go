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

	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	crtlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/metrics"
)

// Metrics is a helper struct that adds the capability for recording GitOps Toolkit standard metrics to a reconciler.
//
// Use it by embedding it in your reconciler struct:
//
//		type MyTypeReconciler {
//	 	client.Client
//	     // ... etc.
//	     controller.Metrics
//		}
//
// Following the GitOps Toolkit conventions, API types used in GOTK SHOULD implement conditions.Getter to work with
// status condition types, and this convention MUST be followed to be able to record metrics using this helper.
//
// Use MustMakeMetrics to create a working Metrics value; you can supply the same value to all reconcilers.
//
// Once initialised, metrics can be recorded by calling one of the available `Record*` methods.
type Metrics struct {
	Scheme          *runtime.Scheme
	MetricsRecorder *metrics.Recorder
}

// MustMakeMetrics creates a new Metrics with a new metrics.Recorder, and the Metrics.Scheme set to that of the given
// mgr.
// It attempts to register the metrics collectors in the controller-runtime metrics registry, which panics upon the
// first registration that causes an error. Which usually happens if you try to initialise a Metrics value twice for
// your controller.
func MustMakeMetrics(mgr ctrl.Manager) Metrics {
	metricsRecorder := metrics.NewRecorder()
	crtlmetrics.Registry.MustRegister(metricsRecorder.Collectors()...)

	return Metrics{
		Scheme:          mgr.GetScheme(),
		MetricsRecorder: metricsRecorder,
	}
}

// RecordDuration records the duration of a reconcile attempt for the given obj based on the given startTime.
func (m Metrics) RecordDuration(ctx context.Context, obj conditions.Getter, startTime time.Time) {
	if m.MetricsRecorder != nil {
		ref, err := reference.GetReference(m.Scheme, obj)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record duration")
			return
		}
		m.MetricsRecorder.RecordDuration(*ref, startTime)
	}
}

// RecordSuspend records the suspension of the given obj based on the given suspend value.
func (m Metrics) RecordSuspend(ctx context.Context, obj conditions.Getter, suspend bool) {
	if m.MetricsRecorder != nil {
		ref, err := reference.GetReference(m.Scheme, obj)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record suspend")
			return
		}
		m.MetricsRecorder.RecordSuspend(*ref, suspend)
	}
}

// RecordReadiness records the meta.ReadyCondition status for the given obj.
func (m Metrics) RecordReadiness(ctx context.Context, obj conditions.Getter) {
	m.RecordCondition(ctx, obj, meta.ReadyCondition)
}

// RecordReconciling records the meta.ReconcilingCondition status for the given obj.
func (m Metrics) RecordReconciling(ctx context.Context, obj conditions.Getter) {
	m.RecordCondition(ctx, obj, meta.ReconcilingCondition)
}

// RecordStalled records the meta.StalledCondition status for the given obj.
func (m Metrics) RecordStalled(ctx context.Context, obj conditions.Getter) {
	m.RecordCondition(ctx, obj, meta.StalledCondition)
}

// RecordCondition records the status of the given conditionType for the given obj.
func (m Metrics) RecordCondition(ctx context.Context, obj conditions.Getter, conditionType string) {
	if m.MetricsRecorder == nil {
		return
	}
	ref, err := reference.GetReference(m.Scheme, obj)
	if err != nil {
		logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record condition metric")
		return
	}
	rc := conditions.Get(obj, conditionType)
	if rc == nil {
		rc = conditions.UnknownCondition(conditionType, "", "")
	}
	m.MetricsRecorder.RecordCondition(*ref, *rc, !obj.GetDeletionTimestamp().IsZero())
}
