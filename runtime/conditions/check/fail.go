/*
Copyright 2022 The Flux authors

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

package check

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
)

// Negative polarity condition cannot be True when Ready condition is True.
func check_FAIL0001(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.IsTrue(obj, meta.ReadyCondition) {
		return nil
	}
	// Return if no negative polarity context is provided.
	if len(condns.NegativePolarity) == 0 {
		return nil
	}
	// Iterate through the negative conditions and collect the ones that have
	// are True value.
	probConditions := []string{}
	for _, c := range obj.GetConditions() {
		// Check if the condition has negative polarity.
		if isNegativePolarityCondition(condns.NegativePolarity, c) {
			if c.Status == metav1.ConditionTrue {
				probConditions = append(probConditions, c.Type)
			}
		}
	}
	if len(probConditions) > 0 {
		return fmt.Errorf("Negative polarity condition cannot be True when Ready condition is True: %v", probConditions)
	}
	return nil
}

// Ready condition must always be present.
func check_FAIL0002(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.Has(obj, meta.ReadyCondition) {
		return fmt.Errorf("Ready condition must always be present")
	}
	return nil
}

// Ready condition must be False when Reconciling condition is True.
func check_FAIL0003(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.Has(obj, meta.ReconcilingCondition) {
		return nil
	}
	ready := conditions.Get(obj, meta.ReadyCondition)
	// Return if Ready condition is not present, can't evaluate further.
	if ready == nil {
		return nil
	}
	rec := conditions.Get(obj, meta.ReconcilingCondition)
	if rec.Status == metav1.ConditionTrue && ready.Status != metav1.ConditionFalse {
		return fmt.Errorf("Ready condition must be False when Reconciling condition is True")
	}
	return nil
}

// Ready condition must be False when Stalled condition is True.
func check_FAIL0004(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.Has(obj, meta.StalledCondition) {
		return nil
	}
	ready := conditions.Get(obj, meta.ReadyCondition)
	// Return if Ready condition is not present, can't evaluate further.
	if ready == nil {
		return nil
	}
	stalled := conditions.Get(obj, meta.StalledCondition)
	if stalled.Status == metav1.ConditionTrue && ready.Status != metav1.ConditionFalse {
		return fmt.Errorf("Ready condition must be False when Stalled condition is True")
	}
	return nil
}

// Only one of Reconciling condition or Stalled condition must be present at a
// time.
func check_FAIL0005(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if conditions.Has(obj, meta.ReconcilingCondition) && conditions.Has(obj, meta.StalledCondition) {
		return fmt.Errorf("Only one of Reconciling condition or Stalled condition must be present at a time")
	}
	return nil
}

// The ObservedGeneration must be less than or equal to the object Generation.
func check_FAIL0006(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	og, err := getStatusObservedGeneration(obj)
	if err != nil {
		return fmt.Errorf("CHECK_FAIL0006: failed to get observed generation: %w", err)
	}
	if og > obj.GetGeneration() {
		return fmt.Errorf("The ObservedGeneration must be less than or equal to the object Generation")
	}
	return nil
}

// Ready condition must be False when the ObservedGeneration is less than the
// object Generation.
func check_FAIL0007(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	og, err := getStatusObservedGeneration(obj)
	if err != nil {
		return fmt.Errorf("CHECK_FAIL0007: failed to get observed generation: %w", err)
	}
	if og < obj.GetGeneration() {
		if conditions.IsReady(obj) {
			return fmt.Errorf("Ready condition must be False when the ObservedGeneration is less than the object Generation")
		}
	}
	return nil
}

// Ready condition must be False when any of the status condition's
// ObservedGeneration is less than the object Generation.
func check_FAIL0008(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.IsReady(obj) {
		return nil
	}
	objectGen := obj.GetGeneration()
	// Collect problematic conditions with ObservedGeneration != object Generation.
	probConditions := []string{}
	for _, c := range obj.GetConditions() {
		if c.ObservedGeneration < objectGen {
			probConditions = append(probConditions, c.Type)
		}
	}
	if len(probConditions) > 0 {
		return fmt.Errorf("Ready condition must be False when any of the status condition's ObservedGeneration is less than the object Generation: %v", probConditions)
	}
	return nil
}

// The status conditions' ObservedGenerations must be equal to the root
// ObservedGeneration when Ready condition is True.
func check_FAIL0009(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.IsReady(obj) {
		return nil
	}
	og, err := getStatusObservedGeneration(obj)
	if err != nil {
		return fmt.Errorf("CHECK_FAIL0009: failed to get observed generation: %w", err)
	}
	// Collect the problematic conditions with ObservedGeneration != og.
	probConditions := []string{}
	for _, c := range obj.GetConditions() {
		if c.ObservedGeneration != og {
			probConditions = append(probConditions, c.Type)
		}
	}
	if len(probConditions) > 0 {
		return fmt.Errorf("The status conditions' ObservedGenerations must be equal to the root ObservedGeneration when Ready condition is True: %v", probConditions)
	}
	return nil
}

// The root ObservedGeneration must be less than the Reconciling condition
// ObservedGeneration when Reconciling condition is True.
func check_FAIL0010(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.IsReconciling(obj) {
		return nil
	}
	og, err := getStatusObservedGeneration(obj)
	if err != nil {
		return fmt.Errorf("CHECK_FAIL0010: failed to get observed generation: %w", err)
	}
	rec := conditions.Get(obj, meta.ReconcilingCondition)
	if og >= rec.ObservedGeneration {
		return fmt.Errorf("The root ObservedGeneration must be less than the Reconciling condition ObservedGeneration when Reconciling condition is True")
	}
	return nil
}
