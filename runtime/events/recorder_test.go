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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestEventRecorder_AnnotatedEventf(t *testing.T) {
	for _, tt := range []struct {
		name             string
		object           runtime.Object
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
			expectedMetadata: map[string]string{
				"test":                                 "true",
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
			expectedMetadata: map[string]string{
				"test": "true",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var payload eventv1.Event
				err = json.Unmarshal(b, &payload)
				require.NoError(t, err)

				require.Equal(t, "ConfigMap", payload.InvolvedObject.Kind)
				require.Equal(t, "webapp", payload.InvolvedObject.Name)
				require.Equal(t, "gitops-system", payload.InvolvedObject.Namespace)
				require.Equal(t, "sync", payload.Reason)
				require.Equal(t, "sync object", payload.Message)

				for k, v := range tt.expectedMetadata {
					require.Equal(t, v, payload.Metadata[k])
				}
			}))
			defer ts.Close()

			eventRecorder, err := NewRecorder(env, ctrl.Log, ts.URL, "test-controller")
			require.NoError(t, err)

			obj := tt.object

			meta := map[string]string{
				"test": "true",
			}

			const msg = "sync object"

			eventRecorder.AnnotatedEventf(obj, meta, corev1.EventTypeNormal, "sync", "%s", msg)
			require.Equal(t, 2, requestCount)

			// When a trace event is sent, it's dropped, no new request.
			eventRecorder.AnnotatedEventf(obj, meta, eventv1.EventTypeTrace, "sync", "%s", msg)
			require.Equal(t, 2, requestCount)
		})
	}
}

func TestEventRecorder_AnnotatedEventf_Retry(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload eventv1.Event
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	eventRecorder, err := NewRecorder(env, ctrl.Log, ts.URL, "test-controller")
	require.NoError(t, err)
	eventRecorder.Client.RetryMax = 2

	obj := &corev1.ConfigMap{}
	obj.Namespace = "gitops-system"
	obj.Name = "webapp"

	eventRecorder.AnnotatedEventf(obj, nil, corev1.EventTypeNormal, "sync", "sync %s", obj.Name)
	require.True(t, requestCount > 1)
}

func TestEventRecorder_AnnotatedEventf_RateLimited(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload eventv1.Event
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	eventRecorder, err := NewRecorder(env, ctrl.Log, ts.URL, "test-controller")
	require.NoError(t, err)
	eventRecorder.Client.RetryMax = 2

	obj := &corev1.ConfigMap{}
	obj.Namespace = "gitops-system"
	obj.Name = "webapp"

	eventRecorder.AnnotatedEventf(obj, nil, corev1.EventTypeNormal, "sync", "sync %s", obj.Name)
	require.Equal(t, 1, requestCount)
}

func TestEventRecorder_Webhook(t *testing.T) {
	_, err := NewRecorder(env, ctrl.Log, "", "test-controller")
	require.NoError(t, err)

	_, err = NewRecorder(env, ctrl.Log, " http://example.com", "test-controller")
	require.Error(t, err)

	_, err = NewRecorder(env, ctrl.Log, "http://example.com", "test-controller")
	require.NoError(t, err)
}
