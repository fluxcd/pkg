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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/runtime/events"
)

// Events is a helper struct that adds the capability of sending events to the Kubernetes API and an external event
// recorder, like the GitOps Toolkit notification-controller.
//
// Use it by embedding it in your reconciler struct:
//
//	type MyTypeReconciler {
//  	client.Client
//      // ... etc.
//      controller.Events
//	}
//
// Use MakeEvents to create a working Events value; in most cases the value needs to be initialised just once per
// controller, as the specialised logger and object reference data are gathered from the arguments provided to the
// Eventf method.
type Events struct {
	Scheme                *runtime.Scheme
	EventRecorder         kuberecorder.EventRecorder
	ExternalEventRecorder *events.Recorder
}

// MakeEvents creates a new Events, with the Events.Scheme set to that of the given mgr and a newly initialised
// Events.EventRecorder for the given controllerName.
func MakeEvents(mgr ctrl.Manager, controllerName string, ext *events.Recorder) Events {
	return Events{
		Scheme:                mgr.GetScheme(),
		EventRecorder:         mgr.GetEventRecorderFor(controllerName),
		ExternalEventRecorder: ext,
	}
}

// Event emits a Kubernetes event, and forwards the event to the ExternalEventRecorder if configured.
// Use EventWithMeta or EventWithMetaf if you want to attach metadata to the external event.
func (e Events) Event(ctx context.Context, obj client.Object, severity, reason, msg string) {
	e.EventWithMetaf(ctx, obj, nil, severity, reason, msg)
}

// Eventf emits a Kubernetes event, and forwards the event to the ExternalEventRecorder if configured.
// Use EventWithMeta or EventWithMetaf if you want to attach metadata to the external event.
func (e Events) Eventf(ctx context.Context, obj client.Object, severity, reason, msgFmt string, args ...interface{}) {
	e.EventWithMetaf(ctx, obj, nil, severity, reason, msgFmt, args...)
}

// EventWithMeta emits a Kubernetes event, and forwards the event and metadata to the ExternalEventRecorder if configured.
func (e Events) EventWithMeta(ctx context.Context, obj client.Object, metadata map[string]string, severity, reason, msg string) {
	e.EventWithMetaf(ctx, obj, metadata, severity, reason, msg)
}

// EventWithMetaf emits a Kubernetes event, and forwards the event and metadata to the ExternalEventRecorder if configured.
func (e Events) EventWithMetaf(ctx context.Context, obj client.Object, metadata map[string]string, severity, reason, msgFmt string, args ...interface{}) {
	if e.EventRecorder != nil {
		e.EventRecorder.Eventf(obj, severityToEventType(severity), reason, msgFmt, args...)
	}
	if e.ExternalEventRecorder != nil {
		ref, err := reference.GetReference(e.Scheme, obj)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to send event")
			return
		}
		if err := e.ExternalEventRecorder.Eventf(*ref, metadata, severity, reason, msgFmt, args...); err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to send event")
			return
		}
	}
}

// severityToEventType maps the given severity string to a corev1 EventType.
// In case of an unrecognised severity, EventTypeNormal is returned.
func severityToEventType(severity string) string {
	switch severity {
	case events.EventSeverityError:
		return corev1.EventTypeWarning
	default:
		return corev1.EventTypeNormal
	}
}
