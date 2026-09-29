/*
Copyright 2026 The Flux authors

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

package eventstest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

// post marshals ev and POSTs it to the sink, returning the response status
// code. It fails the test on transport errors.
func post(t *testing.T, url string, ev eventv1.Event) int {
	t.Helper()
	body, err := json.Marshal(ev)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp.StatusCode
}

func eventFor(kind, namespace, name, msg string) eventv1.Event {
	return eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      kind,
			Namespace: namespace,
			Name:      name,
		},
		Message: msg,
	}
}

func TestSink_EventsAndEventsFor(t *testing.T) {
	sink := NewSink(t)

	require.Equal(t, http.StatusOK, post(t, sink.URL(), eventFor("Kustomization", "flux-system", "apps", "one")))
	require.Equal(t, http.StatusOK, post(t, sink.URL(), eventFor("HelmRelease", "flux-system", "podinfo", "two")))
	require.Equal(t, http.StatusOK, post(t, sink.URL(), eventFor("Kustomization", "flux-system", "apps", "three")))

	// Events returns everything in arrival order.
	all := sink.Events()
	require.Len(t, all, 3)
	require.Equal(t, "one", all[0].Message)
	require.Equal(t, "two", all[1].Message)
	require.Equal(t, "three", all[2].Message)

	// EventsFor filters by InvolvedObject kind/namespace/name, preserving order.
	apps := sink.EventsFor("Kustomization", "flux-system", "apps")
	require.Len(t, apps, 2)
	require.Equal(t, "one", apps[0].Message)
	require.Equal(t, "three", apps[1].Message)

	// A non-matching selector returns nothing.
	require.Empty(t, sink.EventsFor("Kustomization", "flux-system", "other"))
	require.Empty(t, sink.EventsFor("HelmRelease", "flux-system", "apps"))

	// Events returns a snapshot copy: mutating it does not affect the sink.
	all[0].Message = "mutated"
	require.Equal(t, "one", sink.Events()[0].Message)
}

func TestSink_Reset(t *testing.T) {
	sink := NewSink(t)

	require.Equal(t, http.StatusOK, post(t, sink.URL(), eventFor("Kustomization", "flux-system", "apps", "one")))
	require.Len(t, sink.Events(), 1)

	sink.Reset()
	require.Empty(t, sink.Events())

	// The sink is still usable after a reset.
	require.Equal(t, http.StatusOK, post(t, sink.URL(), eventFor("Kustomization", "flux-system", "apps", "two")))
	require.Len(t, sink.Events(), 1)
}

func TestSink_SetStatusCode(t *testing.T) {
	sink := NewSink(t)

	// Default is 200.
	require.Equal(t, http.StatusOK, post(t, sink.URL(), eventFor("Kustomization", "flux-system", "apps", "one")))

	// SetStatusCode can be called mid-test to change the response.
	sink.SetStatusCode(http.StatusTooManyRequests)
	require.Equal(t, http.StatusTooManyRequests, post(t, sink.URL(), eventFor("Kustomization", "flux-system", "apps", "two")))

	// Events are still captured regardless of the returned status code.
	require.Len(t, sink.Events(), 2)

	// Resetting to zero restores the default 200.
	sink.SetStatusCode(0)
	require.Equal(t, http.StatusOK, post(t, sink.URL(), eventFor("Kustomization", "flux-system", "apps", "three")))
}
