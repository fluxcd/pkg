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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRecorder_RecordCondition(t *testing.T) {
	rec := NewRecorder()
	reg := prometheus.NewRegistry()
	reg.MustRegister(rec.conditionGauge)

	ref := corev1.ObjectReference{
		Kind:      "Kustomization",
		Namespace: "default",
		Name:      "test",
	}

	cond := metav1.Condition{
		Type:   meta.ReadyCondition,
		Status: metav1.ConditionTrue,
	}

	rec.RecordCondition(ref, cond)

	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	require.Equal(t, len(metricFamilies), 1)
	require.Equal(t, len(metricFamilies[0].Metric), 3)

	var conditionTrueValue float64
	for _, m := range metricFamilies[0].Metric {
		for _, pair := range m.GetLabel() {
			if *pair.Name == "type" && *pair.Value != meta.ReadyCondition {
				t.Errorf("expected condition type to be %s, got %s", meta.ReadyCondition, *pair.Value)
			}
			if *pair.Name == "status" && *pair.Value == string(metav1.ConditionTrue) {
				conditionTrueValue = *m.GetGauge().Value
			} else if *pair.Name == "status" && *m.GetGauge().Value != 0 {
				t.Errorf("expected guage value to be 0, got %v", *m.GetGauge().Value)
			}
		}
	}

	require.Equal(t, conditionTrueValue, float64(1))

	// Delete metrics.
	rec.DeleteCondition(ref, cond.Type)

	metricFamilies, err = reg.Gather()
	require.NoError(t, err)
	require.Equal(t, len(metricFamilies), 0)
}

func TestRecorder_RecordDuration(t *testing.T) {
	rec := NewRecorder()
	reg := prometheus.NewRegistry()
	reg.MustRegister(rec.durationHistogram)

	ref := corev1.ObjectReference{
		Kind:      "GitRepository",
		Namespace: "default",
		Name:      "test",
	}

	reconcileStart := time.Now().Add(-time.Second)
	rec.RecordDuration(ref, reconcileStart)

	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	require.Equal(t, len(metricFamilies), 1)
	require.Equal(t, len(metricFamilies[0].Metric), 1)

	sampleCount := metricFamilies[0].Metric[0].Histogram.GetSampleCount()
	require.Equal(t, sampleCount, uint64(1))

	labels := metricFamilies[0].Metric[0].GetLabel()
	require.Equal(t, len(labels), 3)

	for _, pair := range labels {
		if *pair.Name == "kind" && *pair.Value != ref.Kind {
			t.Errorf("expected kind label to be %s, got %s", ref.Kind, *pair.Value)
		}
		if *pair.Name == "name" && *pair.Value != ref.Name {
			t.Errorf("expected name label to be %s, got %s", ref.Name, *pair.Value)
		}
		if *pair.Name == "namespace" && *pair.Value != ref.Namespace {
			t.Errorf("expected namespace label to be %s, got %s", ref.Namespace, *pair.Value)
		}
	}

	// Delete metrics.
	rec.DeleteDuration(ref)

	metricFamilies, err = reg.Gather()
	require.NoError(t, err)
	require.Equal(t, len(metricFamilies), 0)
}

func TestRecorder_RecordSuspend(t *testing.T) {
	rec := NewRecorder()
	reg := prometheus.NewRegistry()
	reg.MustRegister(rec.suspendGauge)

	ref := corev1.ObjectReference{
		Kind:      "GitRepository",
		Namespace: "default",
		Name:      "test",
	}

	rec.RecordSuspend(ref, true)

	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	require.Equal(t, len(metricFamilies), 1)
	require.Equal(t, len(metricFamilies[0].Metric), 1)

	value := *metricFamilies[0].Metric[0].GetGauge().Value
	require.EqualValues(t, value, 1, "expected gauge value")

	for _, pair := range metricFamilies[0].Metric[0].GetLabel() {
		if *pair.Name == "kind" {
			require.Equal(t, *pair.Value, ref.Kind, "unexpected kind")
		}
		if *pair.Name == "name" {
			require.Equal(t, *pair.Value, ref.Name, "unexpected name")
		}
		if *pair.Name == "namespace" {
			require.Equal(t, *pair.Value, ref.Namespace, "unexpected namespace")
		}
	}

	// Delete metrics.
	rec.DeleteSuspend(ref)

	metricFamilies, err = reg.Gather()
	require.NoError(t, err)
	require.Equal(t, len(metricFamilies), 0)
}
