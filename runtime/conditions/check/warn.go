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

	"github.com/kylelemons/godebug/pretty"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
)

// Negative polarity condition present when Ready condition is True.
func check_WARN0001(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.IsTrue(obj, meta.ReadyCondition) {
		return nil
	}
	// Return if no negative polarity context is provided.
	if len(condns.NegativePolarity) == 0 {
		return nil
	}
	// Collect problematic conditions.
	probConditions := []string{}
	for _, cond := range condns.NegativePolarity {
		if c := conditions.Get(obj, cond); c != nil {
			probConditions = append(probConditions, c.Type)
		}
	}
	if len(probConditions) > 0 {
		return fmt.Errorf(
			"Negative polarity condition present when Ready condition is True: %v",
			probConditions)
	}
	return nil
}

// Ready condition should have the value of the negative polarity conditon
// that's present with the highest priority.
func check_WARN0002(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if conditions.IsTrue(obj, meta.ReadyCondition) {
		return nil
	}
	// Return if no negative polarity context is provided.
	if len(condns.NegativePolarity) == 0 {
		return nil
	}
	ready := conditions.Get(obj, meta.ReadyCondition)
	hnpc, err := HighestNegativePriorityCondition(condns, obj.GetConditions())
	if err != nil {
		return err
	}
	// Return if no negative polarity condition was found.
	if hnpc == nil {
		return nil
	}
	// Return if the highest negative polarity condition is Reconciling or
	// Stalled condition.
	// NOTE: This is needed to preserve the Reconciling or Stalled and Ready
	// values in situations where there's no custom negative polarity conditions
	// and Stalled and Reconciling are the only negative conditions.
	if hnpc.Type == meta.ReconcilingCondition || hnpc.Type == meta.StalledCondition {
		return nil
	}
	if ready.Message != hnpc.Message || ready.Reason != hnpc.Reason {
		return fmt.Errorf(
			"Ready condition should have the value of the negative polarity conditon that's present with the highest priority: Ready != %s\nDiff:\n%v",
			hnpc.Type, compareAndDiffConditions(ready, hnpc))
	}
	return nil
}

// Reconciling condition can be removed when its value is False.
func check_WARN0003(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.Has(obj, meta.ReconcilingCondition) {
		return nil
	}
	rec := conditions.Get(obj, meta.ReconcilingCondition)
	if rec.Status == metav1.ConditionFalse {
		return fmt.Errorf("Reconciling condition can be removed when its value is False")
	}
	return nil
}

// Stalled condition can be removed when its value is False.
func check_WARN0004(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	if !conditions.Has(obj, meta.StalledCondition) {
		return nil
	}
	rec := conditions.Get(obj, meta.StalledCondition)
	if rec.Status == metav1.ConditionFalse {
		return fmt.Errorf("Stalled condition can be removed when its value is False")
	}
	return nil
}

// Missing ObservedGeneration from status condition.
func check_WARN0005(ctx context.Context, obj conditions.Getter, condns *Conditions) error {
	probConditions := []string{}
	for _, c := range obj.GetConditions() {
		if c.ObservedGeneration < 1 {
			probConditions = append(probConditions, c.Type)
		}
	}
	if len(probConditions) > 0 {
		return fmt.Errorf("Missing ObservedGeneration from status condition: %v", probConditions)
	}
	return nil
}

// compareAndDiffConditions returns a pretty printed diff of the values of two
// conditions.
func compareAndDiffConditions(a, b *metav1.Condition) string {
	// Intermediate representation of Condition, focusing only on Reason and
	// Message, to create diffs.
	type conditionValues struct {
		Reason  string
		Message string
	}
	acv := conditionValues{
		Reason:  a.Reason,
		Message: a.Message,
	}
	bcv := conditionValues{
		Reason:  b.Reason,
		Message: b.Message,
	}
	return pretty.Compare(acv, bcv)
}
