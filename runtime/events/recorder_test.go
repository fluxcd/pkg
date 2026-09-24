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

package events

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

// testAction is a sample event action. Action values are controller-specific
// free-form strings (see eventv1.Event.Action), so tests use a literal here
// rather than a shared constant.
const testAction = "Reconciling"

func TestEventRecorder_AnnotatedEventf(t *testing.T) {
	for _, tt := range []struct {
		name             string
		object           runtime.Object
		inputAnnotations map[string]string
		expectedMetadata map[string]string
	}{
		{
			name: "event with ConfigMap",
			object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "webapp",
					Namespace: "gitops-system",
					Annotations: map[string]string{
						"event.toolkit.fluxcd.io/deploymentID": "e076e315-5a48-41c3-81c8-8d8bdee7d74d",
						"event.toolkit.fluxcd.io/image":        "ghcr.io/stefanprodan/podinfo:6.5.0",
					},
				},
			},
			inputAnnotations: map[string]string{"test": "true"},
			expectedMetadata: map[string]string{
				"test":                                 "true",
				"event.toolkit.fluxcd.io/deploymentID": "e076e315-5a48-41c3-81c8-8d8bdee7d74d",
				"event.toolkit.fluxcd.io/image":        "ghcr.io/stefanprodan/podinfo:6.5.0",
			},
		},
		{
			name: "event with ConfigMap does not panic with nil inputAnnotations",
			object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "webapp",
					Namespace: "gitops-system",
					Annotations: map[string]string{
						"event.toolkit.fluxcd.io/deploymentID": "e076e315-5a48-41c3-81c8-8d8bdee7d74d",
						"event.toolkit.fluxcd.io/image":        "ghcr.io/stefanprodan/podinfo:6.5.0",
					},
				},
			},
			inputAnnotations: nil,
			expectedMetadata: map[string]string{
				"event.toolkit.fluxcd.io/deploymentID": "e076e315-5a48-41c3-81c8-8d8bdee7d74d",
				"event.toolkit.fluxcd.io/image":        "ghcr.io/stefanprodan/podinfo:6.5.0",
			},
		},
		{
			name: "event with ObjectReference for ConfigMap (does not panic with runtime.Object without annotations)",
			object: &corev1.ObjectReference{
				Name:      "webapp",
				Namespace: "gitops-system",
				Kind:      "ConfigMap",
			},
			inputAnnotations: map[string]string{"test": "true"},
			expectedMetadata: map[string]string{"test": "true"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sink := NewTestSink()
			t.Cleanup(sink.Close)

			eventRecorder, err := NewRecorder(ctrl.Log, sink.URL(), "test-controller", WithManager(env))
			require.NoError(t, err)

			obj := tt.object

			const msg = "sync object"

			eventRecorder.AnnotatedEventf(obj, nil, tt.inputAnnotations, corev1.EventTypeNormal, "sync", testAction, "%s", msg)
			require.Eventually(t, func() bool { return sink.Len() == 1 }, time.Second, 10*time.Millisecond)

			// When a trace event is sent, it's dropped, no new request.
			eventRecorder.AnnotatedEventf(obj, nil, tt.inputAnnotations, eventv1.EventTypeTrace, "sync", testAction, "%s", msg)
			require.Never(t, func() bool { return sink.Len() > 1 }, 200*time.Millisecond, 20*time.Millisecond)

			payload := sink.Events()[0]
			require.Equal(t, "ConfigMap", payload.InvolvedObject.Kind)
			require.Equal(t, "webapp", payload.InvolvedObject.Name)
			require.Equal(t, "gitops-system", payload.InvolvedObject.Namespace)
			require.Equal(t, "sync", payload.Reason)
			require.Equal(t, testAction, payload.Action)
			require.Equal(t, "sync object", payload.Message)

			for k, v := range tt.expectedMetadata {
				require.Equal(t, v, payload.Metadata[k])
			}
		})
	}
}

func TestEventRecorder_AnnotatedEventf_Retry(t *testing.T) {
	sink := NewTestSink().WithStatusCode(http.StatusInternalServerError)
	t.Cleanup(sink.Close)

	eventRecorder, err := NewRecorder(ctrl.Log, sink.URL(), "test-controller", WithManager(env), WithRetryMax(2))
	require.NoError(t, err)

	obj := &corev1.ConfigMap{}
	obj.Namespace = "gitops-system"
	obj.Name = "webapp"

	eventRecorder.AnnotatedEventf(obj, nil, nil, corev1.EventTypeNormal, "sync", testAction, "sync %s", obj.Name)
	require.True(t, sink.Len() > 1)
}

func TestEventRecorder_AnnotatedEventf_RateLimited(t *testing.T) {
	sink := NewTestSink().WithStatusCode(http.StatusTooManyRequests)
	t.Cleanup(sink.Close)

	eventRecorder, err := NewRecorder(ctrl.Log, sink.URL(), "test-controller", WithManager(env), WithRetryMax(2))
	require.NoError(t, err)

	obj := &corev1.ConfigMap{}
	obj.Namespace = "gitops-system"
	obj.Name = "webapp"

	eventRecorder.AnnotatedEventf(obj, nil, nil, corev1.EventTypeNormal, "sync", testAction, "sync %s", obj.Name)
	require.Equal(t, 1, sink.Len())
}

func TestEventRecorder_Webhook(t *testing.T) {
	_, err := NewRecorder(ctrl.Log, "", "test-controller", WithManager(env))
	require.NoError(t, err)

	_, err = NewRecorder(ctrl.Log, " http://example.com", "test-controller", WithManager(env))
	require.Error(t, err)

	_, err = NewRecorder(ctrl.Log, "http://example.com", "test-controller", WithManager(env))
	require.NoError(t, err)
}
