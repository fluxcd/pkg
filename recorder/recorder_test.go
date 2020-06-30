/*
Copyright 2020 The Flux CD contributors.

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

package recorder

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestEventRecorder_Eventf(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		var payload Event
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "GitRepository", payload.InvolvedObject.Kind)
		require.Equal(t, "webapp", payload.InvolvedObject.Name)
		require.Equal(t, "gitops-system", payload.InvolvedObject.Namespace)
		require.Equal(t, "true", payload.Metadata["test"])
		require.Equal(t, "sync", payload.Reason)

	}))
	defer ts.Close()

	eventRecorder, err := NewEventRecorder(ts.URL, "test-controller")
	require.NoError(t, err)

	obj := corev1.ObjectReference{
		Kind:      "GitRepository",
		Namespace: "gitops-system",
		Name:      "webapp",
	}
	meta := map[string]string{
		"test": "true",
	}

	err = eventRecorder.EventInfof(obj, meta, "sync", "sync %s", obj.Name)
	require.NoError(t, err)
}

func TestEventRecorder_Eventf_Retry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		var payload Event
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	eventRecorder, err := NewEventRecorder(ts.URL, "test-controller")
	require.NoError(t, err)
	eventRecorder.Client.RetryMax = 2

	obj := corev1.ObjectReference{
		Kind:      "GitRepository",
		Namespace: "gitops-system",
		Name:      "webapp",
	}

	err = eventRecorder.EventErrorf(obj, nil, "sync", "sync %s", obj.Name)
	require.EqualError(t, err, fmt.Sprintf("POST %s giving up after 3 attempts", ts.URL))
}
