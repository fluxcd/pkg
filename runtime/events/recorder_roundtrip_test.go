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

package events

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/runtime/testenv"
)

// TestEventRecorder_RoundTrip records events through the recorder's two sinks
// and asserts both preserve the event: the core/v1 Kubernetes Event on the
// apiserver and the Flux event/v1 payload on the notification webhook.
//
// The critical case is a message longer than the apiserver's note-length limit.
// The recorder deliberately uses the legacy core/v1 event recorder, which does
// not set EventTime and so skips the strict note-length validation; the full
// message must survive on the Kubernetes Event. If the recorder is ever
// switched to a path that sets EventTime, the apiserver rejects the long note
// during admission, the event never lands, and no error is surfaced to the
// recorder — so only this round-trip assertion against the apiserver catches
// it. The webhook payload must carry the full message regardless.
func TestEventRecorder_RoundTrip(t *testing.T) {
	const namespace = "gitops-system"

	for _, tt := range []struct {
		name    string
		message string
	}{
		{
			name:    "short message survives both sinks",
			message: "sync object",
		},
		{
			name: "long message survives both sinks",
			// Well beyond the apiserver's note-length limit. A recorder that
			// set EventTime would have this note rejected by admission and the
			// Kubernetes Event would never appear.
			message: strings.Repeat("a very long reconciliation message. ", 200),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := require.New(t)

			sink := NewTestSink()
			t.Cleanup(sink.Close)

			recorder, err := NewRecorder(ctrl.Log, sink.URL(), "test-controller", WithManager(env))
			g.NoError(err)

			objName := "webapp-" + sanitize(tt.name)
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      objName,
					Namespace: namespace,
				},
			}
			g.NoError(env.Client.Create(ctx, obj))
			t.Cleanup(func() { _ = env.Client.Delete(ctx, obj) })

			recorder.Eventf(obj, nil, corev1.EventTypeNormal, "sync", testAction, "%s", tt.message)

			// Kubernetes sink (core/v1): the event must land on the apiserver
			// with the full message. If admission drops it (e.g. note too
			// long), WaitForEvents times out and fails.
			evs, err := testenv.WaitForEvents(ctx, env.Client, objName, namespace, nil, 1, 10*time.Second)
			g.NoError(err)
			g.NotEmpty(evs)
			g.Equal(tt.message, evs[0].Message)

			// Webhook sink (Flux event/v1): the payload must carry the full
			// message intact.
			g.Eventually(func() bool { return sink.Len() >= 1 }, 5*time.Second, 50*time.Millisecond)
			webhookEvents := sink.Events()
			g.NotEmpty(webhookEvents)
			g.Equal(tt.message, webhookEvents[0].Message)
			g.Equal(objName, webhookEvents[0].InvolvedObject.Name)
		})
	}
}

// sanitize turns a test case name into a DNS-1123 compatible object name suffix.
func sanitize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
