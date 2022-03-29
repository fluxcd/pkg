/*
Copyright 2020 The Kubernetes Authors.
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

This file is modified from the source at
https://github.com/kubernetes-sigs/cluster-api/tree/7478817225e0a75acb6e14fc7b438231578073d2/util/conditions/setter_test.go,
and initially adapted to work with the `metav1.Condition` and `metav1.ConditionStatus` types.
More concretely, this includes the removal of "condition severity" related functionalities, as this is not supported by
the `metav1.Condition` type.
*/

package conditions

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func TestHasSameState(t *testing.T) {
	g := NewWithT(t)

	// same condition
	true2 := true1.DeepCopy()
	g.Expect(hasSameState(true1, true2)).To(BeTrue())

	// different LastTransitionTime does not impact state
	true2 = true1.DeepCopy()
	true2.LastTransitionTime = metav1.NewTime(time.Date(1900, time.November, 10, 23, 0, 0, 0, time.UTC))
	g.Expect(hasSameState(true1, true2)).To(BeTrue())

	// different ObservedGeneration does not impact state
	true2 = true1.DeepCopy()
	true2.ObservedGeneration = 1
	g.Expect(hasSameState(true1, true2)).To(BeTrue())

	// different Type, Status, Reason, and Message determine
	// different state
	true2 = true1.DeepCopy()
	true2.Type = "another type"
	g.Expect(hasSameState(true1, true2)).To(BeFalse())

	true2 = true1.DeepCopy()
	true2.Status = metav1.ConditionFalse
	g.Expect(hasSameState(true1, true2)).To(BeFalse())

	true2 = true1.DeepCopy()
	true2.Message = "another message"
	g.Expect(hasSameState(true1, true2)).To(BeFalse())
}

func TestLexicographicLess(t *testing.T) {
	g := NewWithT(t)

	// alphabetical order of Type is respected
	a := TrueCondition("A", "", "")
	b := TrueCondition("B", "", "")
	g.Expect(lexicographicLess(a, b)).To(BeTrue())

	a = TrueCondition("B", "", "")
	b = TrueCondition("A", "", "")
	g.Expect(lexicographicLess(a, b)).To(BeFalse())

	// observed generation is respected
	a = TrueCondition("A", "", "")
	a.ObservedGeneration = 2
	b = TrueCondition("B", "", "")
	b.ObservedGeneration = 2
	g.Expect(lexicographicLess(a, b)).To(BeTrue())

	a = TrueCondition("A", "", "")
	a.ObservedGeneration = 1
	b = TrueCondition("B", "", "")
	b.ObservedGeneration = 2
	g.Expect(lexicographicLess(a, b)).To(BeFalse())

	a = TrueCondition("A", "", "")
	a.ObservedGeneration = 1
	b = TrueCondition("B", "", "")
	b.ObservedGeneration = 0
	g.Expect(lexicographicLess(a, b)).To(BeTrue())

	// Disregard Type when observed generations aren't equal.
	c := TrueCondition("C", "", "")
	c.ObservedGeneration = 1
	b = TrueCondition("B", "", "")
	b.ObservedGeneration = 0
	g.Expect(lexicographicLess(c, b)).To(BeTrue())

	// Stalled, Ready, and Reconciling conditions are threaded as an
	// exception and always go first.
	stalled := TrueCondition(meta.StalledCondition, "", "")
	ready := FalseCondition(meta.ReadyCondition, "", "")
	reconciling := TrueCondition(meta.ReconcilingCondition, "", "")

	g.Expect(lexicographicLess(stalled, ready)).To(BeTrue())
	g.Expect(lexicographicLess(ready, stalled)).To(BeFalse())

	g.Expect(lexicographicLess(reconciling, ready)).To(BeTrue())
	g.Expect(lexicographicLess(ready, reconciling)).To(BeFalse())

	g.Expect(lexicographicLess(stalled, reconciling)).To(BeTrue())
	g.Expect(lexicographicLess(reconciling, stalled)).To(BeFalse())

	g.Expect(lexicographicLess(ready, b)).To(BeTrue())
	g.Expect(lexicographicLess(b, ready)).To(BeFalse())

	ready.ObservedGeneration = 1
	b.ObservedGeneration = 2
	g.Expect(lexicographicLess(ready, b)).To(BeTrue())
}

func TestSet(t *testing.T) {
	a := TrueCondition("a", "", "")
	b := TrueCondition("b", "", "")
	ready := TrueCondition(meta.ReadyCondition, "", "")

	tests := []struct {
		name      string
		to        Setter
		condition *metav1.Condition
		want      []metav1.Condition
	}{
		{
			name:      "Set adds a condition",
			to:        setterWithConditions(),
			condition: a,
			want:      conditionList(a),
		},
		{
			name:      "Set adds more conditions",
			to:        setterWithConditions(a),
			condition: b,
			want:      conditionList(a, b),
		},
		{
			name:      "Set does not duplicate existing conditions",
			to:        setterWithConditions(a, b),
			condition: a,
			want:      conditionList(a, b),
		},
		{
			name:      "Set sorts conditions in lexicographic order",
			to:        setterWithConditions(b, a),
			condition: ready,
			want:      conditionList(ready, a, b),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			Set(tt.to, tt.condition)

			g.Expect(tt.to.GetConditions()).To(haveSameConditionsOf(tt.want))
		})
	}
}

func TestSetLastTransitionTime(t *testing.T) {
	x := metav1.Date(2012, time.January, 1, 12, 15, 30, 5e8, time.UTC)

	foo := FalseCondition("foo", "reason foo", "message foo")
	fooWithLastTransitionTime := FalseCondition("foo", "reason foo", "message foo")
	fooWithLastTransitionTime.LastTransitionTime = x
	fooWithAnotherState := TrueCondition("foo", "", "")

	tests := []struct {
		name                    string
		to                      Setter
		new                     *metav1.Condition
		LastTransitionTimeCheck func(*WithT, metav1.Time)
	}{
		{
			name: "Set a condition that does not exists should set the last transition time if not defined",
			to:   setterWithConditions(),
			new:  foo,
			LastTransitionTimeCheck: func(g *WithT, lastTransitionTime metav1.Time) {
				g.Expect(lastTransitionTime).ToNot(BeZero())
			},
		},
		{
			name: "Set a condition that does not exists should preserve the last transition time if defined",
			to:   setterWithConditions(),
			new:  fooWithLastTransitionTime,
			LastTransitionTimeCheck: func(g *WithT, lastTransitionTime metav1.Time) {
				g.Expect(lastTransitionTime).To(Equal(x))
			},
		},
		{
			name: "Set a condition that already exists with the same state should preserves the last transition time",
			to:   setterWithConditions(fooWithLastTransitionTime),
			new:  foo,
			LastTransitionTimeCheck: func(g *WithT, lastTransitionTime metav1.Time) {
				g.Expect(lastTransitionTime).To(Equal(x))
			},
		},
		{
			name: "Set a condition that already exists but with different state should changes the last transition time",
			to:   setterWithConditions(fooWithLastTransitionTime),
			new:  fooWithAnotherState,
			LastTransitionTimeCheck: func(g *WithT, lastTransitionTime metav1.Time) {
				g.Expect(lastTransitionTime).ToNot(Equal(x))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			Set(tt.to, tt.new)

			tt.LastTransitionTimeCheck(g, Get(tt.to, "foo").LastTransitionTime)
		})
	}
}

func TestSetObservedGeneration(t *testing.T) {
	g := NewWithT(t)

	obj := &testdata.Fake{}
	x := metav1.Date(2012, time.January, 1, 12, 15, 30, 5e8, time.UTC)

	// Conditions with stale observed generation.
	foo1 := FalseCondition("foo1", "reasonFoo1", "messageFoo1")
	foo1.ObservedGeneration = 2
	foo1.LastTransitionTime = x
	foo2 := TrueCondition("foo2", "reasonFoo2", "messageFoo2")
	foo2.ObservedGeneration = 3
	foo2.LastTransitionTime = x

	// Object with higher generation and stale conditions.
	obj.Generation = 4
	obj.SetConditions([]metav1.Condition{*foo1, *foo2})

	// Ensure the conditions haven't updated.
	g.Expect(Get(obj, "foo1").ObservedGeneration).To(BeEquivalentTo(2))
	g.Expect(Get(obj, "foo2").ObservedGeneration).To(BeEquivalentTo(3))

	// Update the object's generation and Set the conditions without any state
	// change.
	obj.Generation = 5
	Set(obj, foo1)
	Set(obj, foo2)

	// ObservedGeneration is updated but not the LastTransitionTime.
	g.Expect(Get(obj, "foo1").ObservedGeneration).To(BeEquivalentTo(5))
	g.Expect(Get(obj, "foo1").LastTransitionTime).To(Equal(x))
	g.Expect(Get(obj, "foo2").ObservedGeneration).To(BeEquivalentTo(5))
	g.Expect(Get(obj, "foo2").LastTransitionTime).To(Equal(x))
}

func TestMarkMethods(t *testing.T) {
	g := NewWithT(t)

	obj := &testdata.Fake{}

	// test MarkTrue
	MarkTrue(obj, "conditionFoo", "reasonFoo", "messageFoo")
	g.Expect(Get(obj, "conditionFoo")).To(HaveSameStateOf(&metav1.Condition{
		Type:    "conditionFoo",
		Status:  metav1.ConditionTrue,
		Reason:  "reasonFoo",
		Message: "messageFoo",
	}))

	// test MarkFalse
	MarkFalse(obj, "conditionBar", "reasonBar", "messageBar")
	g.Expect(Get(obj, "conditionBar")).To(HaveSameStateOf(&metav1.Condition{
		Type:    "conditionBar",
		Status:  metav1.ConditionFalse,
		Reason:  "reasonBar",
		Message: "messageBar",
	}))

	// test MarkUnknown
	MarkUnknown(obj, "conditionBaz", "reasonBaz", "messageBaz")
	g.Expect(Get(obj, "conditionBaz")).To(HaveSameStateOf(&metav1.Condition{
		Type:    "conditionBaz",
		Status:  metav1.ConditionUnknown,
		Reason:  "reasonBaz",
		Message: "messageBaz",
	}))

	// test MarkReconciling
	MarkTrue(obj, meta.StalledCondition, "reasonStalled", "messageStalled")
	MarkReconciling(obj, "reasonReconciling", "messageReconciling")
	g.Expect(Get(obj, meta.ReconcilingCondition)).To(HaveSameStateOf(&metav1.Condition{
		Type:    meta.ReconcilingCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "reasonReconciling",
		Message: "messageReconciling",
	}))
	g.Expect(IsUnknown(obj, meta.StalledCondition)).To(BeTrue())

	// test MarkStalled
	MarkTrue(obj, meta.ReconcilingCondition, "reasonReconciling", "messageReconciling")
	MarkStalled(obj, "reasonStalled", "messageStalled")
	g.Expect(Get(obj, meta.StalledCondition)).To(HaveSameStateOf(&metav1.Condition{
		Type:    meta.StalledCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "reasonStalled",
		Message: "messageStalled",
	}))
	g.Expect(IsUnknown(obj, meta.ReconcilingCondition)).To(BeTrue())
}

func TestSetSummary(t *testing.T) {
	g := NewWithT(t)
	target := setterWithConditions(TrueCondition("foo", "", ""))

	SetSummary(target, "test")

	g.Expect(Has(target, "test")).To(BeTrue())
}

func TestSetMirror(t *testing.T) {
	g := NewWithT(t)
	source := getterWithConditions(TrueCondition(meta.ReadyCondition, "", ""))
	target := setterWithConditions()

	SetMirror(target, "foo", source)

	g.Expect(Has(target, "foo")).To(BeTrue())
}

func TestSetAggregate(t *testing.T) {
	g := NewWithT(t)
	source1 := getterWithConditions(TrueCondition(meta.ReadyCondition, "", ""))
	source2 := getterWithConditions(TrueCondition(meta.ReadyCondition, "", ""))
	target := setterWithConditions()

	SetAggregate(target, "foo", []Getter{source1, source2})

	g.Expect(Has(target, "foo")).To(BeTrue())
}

func setterWithConditions(conditions ...*metav1.Condition) Setter {
	obj := &testdata.Fake{}
	obj.SetConditions(conditionList(conditions...))
	return obj
}

func haveSameConditionsOf(expected []metav1.Condition) types.GomegaMatcher {
	return &ConditionsMatcher{
		Expected: expected,
	}
}

type ConditionsMatcher struct {
	Expected []metav1.Condition
}

func (matcher *ConditionsMatcher) Match(actual interface{}) (success bool, err error) {
	actualConditions, ok := actual.([]metav1.Condition)
	if !ok {
		return false, errors.New("Value should be a conditions list")
	}

	if len(actualConditions) != len(matcher.Expected) {
		return false, nil
	}

	for i := range actualConditions {
		if !hasSameState(&actualConditions[i], &matcher.Expected[i]) {
			return false, nil
		}
	}
	return true, nil
}

func (matcher *ConditionsMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to have the same conditions of", matcher.Expected)
}
func (matcher *ConditionsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to have the same conditions of", matcher.Expected)
}
