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

package predicates

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	metav1 "github.com/fluxcd/pkg/apis/meta"
)

// ReconcileRequestedPredicate implements an update predicate function for meta.ReconcileRequestAnnotation changes.
// This predicate will skip update events that have no meta.ReconcileRequestAnnotation change.
//
// It is intended to be used in conjunction with the predicate.GenerationChangedPredicate, as in the following example:
//
//	Controller.Watch(
//		&source.Kind{Type: v1.MyCustomKind},
//		&handler.EnqueueRequestForObject{},
//		predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{}))
type ReconcileRequestedPredicate struct {
	predicate.Funcs
}

// Update implements the default UpdateEvent filter for validating meta.ReconcileRequestAnnotation changes.
func (ReconcileRequestedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	if val, ok := metav1.ReconcileAnnotationValue(e.ObjectNew.GetAnnotations()); ok {
		if valOld, okOld := metav1.ReconcileAnnotationValue(e.ObjectOld.GetAnnotations()); okOld {
			return val != valOld
		}
		return true
	}
	return false
}
