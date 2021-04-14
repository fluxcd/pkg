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
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/runtime/events"
)

// Events is a helper struct that adds the capability of sending
// events to the Kubernetes API and to the GitOps Toolkit notification
// controller. You use it by embedding it in your reconciler struct:
//
//     type MyTypeReconciler {
//         client.Client
//         // ... etc.
//         controller.Events
//     }
//
//  You initialise a suitable value with MakeEvents(); each reconciler
//  will probably need its own value, since it's specialised to a
//  particular controller and log.
type Events struct {
	EventRecorder         kuberecorder.EventRecorder
	ExternalEventRecorder *events.Recorder
	Log                   logr.Logger
}

func MakeEvents(mgr ctrl.Manager, controllerName string, ext *events.Recorder, log logr.Logger) Events {
	return Events{
		EventRecorder:         mgr.GetEventRecorderFor(controllerName),
		ExternalEventRecorder: ext,
		Log:                   log,
	}
}

type runtimeAndMetaObject interface {
	runtime.Object
	metav1.Object
}

// Event emits a Kubernetes event, and forwards the event to the
// notification controller if configured.
func (e Events) Event(ref *corev1.ObjectReference, obj runtimeAndMetaObject, severity, reason, msg string) {
	e.Eventf(ref, obj, severity, reason, "%s", msg)
}

// Eventf emits a Kubernetes event, and forwards the event to the
// notification controller if configured.
func (e Events) Eventf(ref *corev1.ObjectReference, obj runtimeAndMetaObject, severity, reason, msgFmt string, args ...interface{}) {
	if e.EventRecorder != nil {
		e.EventRecorder.Eventf(obj, severityToEventType(severity), reason, msgFmt, args)
	}
	if e.ExternalEventRecorder != nil {
		if err := e.ExternalEventRecorder.Eventf(*ref, nil, severity, reason, msgFmt, args); err != nil {
			e.Log.WithValues(
				"request",
				fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName()),
			).Error(err, "unable to send event")
			return
		}
	}
}

func severityToEventType(severity string) string {
	switch severity {
	case events.EventSeverityError:
		return corev1.EventTypeWarning
	default:
		return corev1.EventTypeNormal
	}
}
