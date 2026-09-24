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
	"testing"

	"github.com/stretchr/testify/require"
	eventsv1 "k8s.io/client-go/tools/events"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

// TestNewRecorder_ResolveKubeSink verifies which Kubernetes Event backend
// NewRecorder wires up for each combination of options, and that conflicting
// or invalid selections are rejected.
func TestNewRecorder_ResolveKubeSink(t *testing.T) {
	for _, tt := range []struct {
		name    string
		opts    []RecorderOption
		wantErr string
		// wantSink is the concrete kubeSink type expected on success, or nil
		// when the recorder should have no Kubernetes backend.
		wantSink kubeSink
	}{
		{
			name:     "manager default is core/v1",
			opts:     []RecorderOption{WithManager(env)},
			wantSink: &coreV1Sink{},
		},
		{
			name:     "manager with explicit core/v1",
			opts:     []RecorderOption{WithManager(env), WithEventsAPI(EventsAPICoreV1)},
			wantSink: &coreV1Sink{},
		},
		{
			name:     "manager with explicit events.k8s.io/v1",
			opts:     []RecorderOption{WithManager(env), WithEventsAPI(EventsAPIEventsV1)},
			wantSink: &eventsV1Sink{},
		},
		{
			name:     "injected core/v1 recorder",
			opts:     []RecorderOption{WithLegacyEventRecorder(kuberecorder.NewFakeRecorder(1))},
			wantSink: &coreV1Sink{},
		},
		{
			name:     "injected events.k8s.io/v1 recorder",
			opts:     []RecorderOption{WithEventRecorder(eventsv1.NewFakeRecorder(1))},
			wantSink: &eventsV1Sink{},
		},
		{
			name:     "injected core/v1 recorder with matching API",
			opts:     []RecorderOption{WithLegacyEventRecorder(kuberecorder.NewFakeRecorder(1)), WithEventsAPI(EventsAPICoreV1)},
			wantSink: &coreV1Sink{},
		},
		{
			name:     "injected events.k8s.io/v1 recorder with matching API",
			opts:     []RecorderOption{WithEventRecorder(eventsv1.NewFakeRecorder(1)), WithEventsAPI(EventsAPIEventsV1)},
			wantSink: &eventsV1Sink{},
		},
		{
			name:    "injected core/v1 recorder conflicts with events.k8s.io/v1 API",
			opts:    []RecorderOption{WithLegacyEventRecorder(kuberecorder.NewFakeRecorder(1)), WithEventsAPI(EventsAPIEventsV1)},
			wantErr: `events API "events.k8s.io/v1" conflicts with injected "v1" recorder`,
		},
		{
			name:    "injected events.k8s.io/v1 recorder conflicts with core/v1 API",
			opts:    []RecorderOption{WithEventRecorder(eventsv1.NewFakeRecorder(1)), WithEventsAPI(EventsAPICoreV1)},
			wantErr: `events API "v1" conflicts with injected "events.k8s.io/v1" recorder`,
		},
		{
			// Without a manager or injected recorder there is no backend to
			// build, so the selected API is never consulted; the recorder is
			// webhook-only with no kube sink.
			name:     "unknown API without a manager is webhook-only",
			opts:     []RecorderOption{WithScheme(env.GetScheme()), WithEventsAPI("bogus")},
			wantSink: nil,
		},
		{
			name:    "unknown API is rejected with a manager",
			opts:    []RecorderOption{WithManager(env), WithEventsAPI("bogus")},
			wantErr: `unsupported events API "bogus"`,
		},
		{
			name:     "webhook-only setup has no kube backend",
			opts:     []RecorderOption{WithScheme(env.GetScheme())},
			wantSink: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := NewRecorder(ctrl.Log, "", "test-controller", tt.opts...)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				require.Nil(t, rec)
				return
			}
			require.NoError(t, err)

			got := rec.(*recorder).kube
			if tt.wantSink == nil {
				require.Nil(t, got)
				return
			}
			require.IsType(t, tt.wantSink, got)
			require.Equal(t, tt.wantSink.api(), got.api())
		})
	}
}
