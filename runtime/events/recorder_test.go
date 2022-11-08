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
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

func TestEventRecorder_AnnotatedEventf(t *testing.T) {
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
		require.Equal(t, "true", payload.Metadata["test"])
		require.Equal(t, "sync", payload.Reason)

	}))
	defer ts.Close()

	eventRecorder, err := NewRecorder(env, ctrl.Log, ts.URL, "test-controller")
	require.NoError(t, err)

	obj := &corev1.ConfigMap{}
	obj.Namespace = "gitops-system"
	obj.Name = "webapp"

	meta := map[string]string{
		"test": "true",
	}

	eventRecorder.AnnotatedEventf(obj, meta, corev1.EventTypeNormal, "sync", "sync %s", obj.Name)
	require.Equal(t, 2, requestCount)

	// When a trace event is sent, it's dropped, no new request.
	eventRecorder.AnnotatedEventf(obj, meta, eventv1.EventTypeTrace, "sync", "sync %s", obj.Name)
	require.Equal(t, 2, requestCount)
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
