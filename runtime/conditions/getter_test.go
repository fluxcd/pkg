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
https://github.com/kubernetes-sigs/cluster-api/tree/7478817225e0a75acb6e14fc7b438231578073d2/util/conditions/getter_test.go,
and initially adapted to work with the `metav1.Condition` and `metav1.ConditionStatus` types.
More concretely, this includes the removal of "condition severity" related functionalities, as this is not supported by
the `metav1.Condition` type.
*/

package conditions

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

var (
	nil1             *metav1.Condition
	true1            = TrueCondition("true1", "reason true1", "message true1")
	unknown1         = UnknownCondition("unknown1", "reason unknown1", "message unknown1")
	false1           = FalseCondition("false1", "reason false1", "message false1")
	readyTrue        = TrueCondition(meta.ReadyCondition, "reason readyTrue", "message readyTrue")
	readyFalse       = FalseCondition(meta.ReadyCondition, "reason readyFalse", "message readyFalse")
	stalledTrue      = TrueCondition(meta.StalledCondition, "reason stalledTrue", "message stalledTrue")
	stalledFalse     = FalseCondition(meta.StalledCondition, "reason stalledFalse", "message stalledFalse")
	reconcilingTrue  = TrueCondition(meta.ReconcilingCondition, "reason reconcilingTrue", "message reconcilingTrue")
	reconcilingFalse = TrueCondition(meta.ReconcilingCondition, "reason reconcilingFalse", "message reconcilingFalse")
)

func TestGetAndHas(t *testing.T) {
	g := NewWithT(t)

	obj := &testdata.Fake{}

	g.Expect(Has(obj, "conditionBaz")).To(BeFalse())
	g.Expect(Get(obj, "conditionBaz")).To(BeNil())

	obj.SetConditions(conditionList(TrueCondition("conditionBaz", "", "")))

	g.Expect(Has(obj, "conditionBaz")).To(BeTrue())
	g.Expect(Get(obj, "conditionBaz")).To(HaveSameStateOf(TrueCondition("conditionBaz", "", "")))
}

func TestIsMethods(t *testing.T) {
	g := NewWithT(t)

	false2 := false1.DeepCopy()
	false2.Type = "false2"
	false2.ObservedGeneration = 1

	obj := getterWithConditions(nil1, true1, unknown1, false1, false2)

	// test isTrue
	g.Expect(IsTrue(obj, "nil1")).To(BeFalse())
	g.Expect(IsTrue(obj, "true1")).To(BeTrue())
	g.Expect(IsTrue(obj, "false1")).To(BeFalse())
	g.Expect(IsTrue(obj, "unknown1")).To(BeFalse())

	// test isFalse
	g.Expect(IsFalse(obj, "nil1")).To(BeFalse())
	g.Expect(IsFalse(obj, "true1")).To(BeFalse())
	g.Expect(IsFalse(obj, "false1")).To(BeTrue())
	g.Expect(IsFalse(obj, "unknown1")).To(BeFalse())

	// test isUnknown
	g.Expect(IsUnknown(obj, "nil1")).To(BeTrue())
	g.Expect(IsUnknown(obj, "true1")).To(BeFalse())
	g.Expect(IsUnknown(obj, "false1")).To(BeFalse())
	g.Expect(IsUnknown(obj, "unknown1")).To(BeTrue())

	// test GetReason
	g.Expect(GetReason(obj, "nil1")).To(Equal(""))
	g.Expect(GetReason(obj, "false1")).To(Equal("reason false1"))

	// test GetMessage
	g.Expect(GetMessage(obj, "nil1")).To(Equal(""))
	g.Expect(GetMessage(obj, "false1")).To(Equal("message false1"))

	// test GetLastTransitionTime
	g.Expect(GetLastTransitionTime(obj, "nil1")).To(BeNil())
	g.Expect(GetLastTransitionTime(obj, "false1")).ToNot(BeNil())

	// test GetObservedGeneration
	g.Expect(GetObservedGeneration(obj, "nil1")).To(BeZero())
	g.Expect(GetObservedGeneration(obj, "false2")).ToNot(BeZero())
}

func TestIsReadyStalledReconciling(t *testing.T) {
	g := NewWithT(t)

	readyObj := getterWithConditions(readyTrue, stalledFalse)
	stalledObj := getterWithConditions(stalledTrue, readyFalse)
	reconcilingObj := getterWithConditions(reconcilingTrue, stalledFalse)

	// test IsReady
	g.Expect(IsReady(readyObj)).To(BeTrue())
	g.Expect(IsReady(stalledObj)).To(BeFalse())
	g.Expect(IsReady(reconcilingObj)).To(BeFalse())

	// test IsStalled
	g.Expect(IsStalled(stalledObj)).To(BeTrue())
	g.Expect(IsStalled(readyObj)).To(BeFalse())
	g.Expect(IsStalled(reconcilingObj)).To(BeFalse())

	// test IsReconciling
	g.Expect(IsReconciling(reconcilingObj)).To(BeTrue())
	g.Expect(IsReconciling(stalledObj)).To(BeFalse())
	g.Expect(IsReconciling(readyObj)).To(BeFalse())
}

func TestMirror(t *testing.T) {
	foo := FalseCondition("foo", "reason foo", "message foo")
	ready := TrueCondition(meta.ReadyCondition, "reason ready", "message ready")
	readyBar := ready.DeepCopy()
	readyBar.Type = "bar"

	tests := []struct {
		name string
		from Getter
		t    string
		want *metav1.Condition
	}{
		{
			name: "Returns nil when the ready condition does not exists",
			from: getterWithConditions(foo),
			want: nil,
		},
		{
			name: "Returns ready condition from source",
			from: getterWithConditions(ready, foo),
			t:    "bar",
			want: readyBar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := mirror(tt.from, tt.t)
			if tt.want == nil {
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(got).To(HaveSameStateOf(tt.want))
		})
	}
}

func TestSummary(t *testing.T) {
	foo := TrueCondition("foo", "reason trueFoo", "message trueFoo")
	bar := FalseCondition("bar", "reason falseBar", "message falseBar")
	baz := FalseCondition("baz", "reason falseBaz", "message falseBaz")
	existingReady := FalseCondition(meta.ReadyCondition, "reason falseReady", "message falseReady") //NB. existing ready has higher priority than other conditions

	tests := []struct {
		name    string
		from    Getter
		options []MergeOption
		want    *metav1.Condition
	}{
		{
			name: "Returns nil when there are no conditions to summarize",
			from: getterWithConditions(),
			want: nil,
		},
		{
			name: "Returns ready condition with the summary of existing conditions (with default options)",
			from: getterWithConditions(foo, bar),
			want: FalseCondition(meta.ReadyCondition, "reason falseBar", "message falseBar"),
		},
		{
			name:    "Returns ready condition with the summary of existing conditions (using WithStepCounter options)",
			from:    getterWithConditions(foo, bar),
			options: []MergeOption{WithStepCounter()},
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "1 of 2 completed"),
		},
		{
			name:    "Returns ready condition with the summary of existing conditions (using WithStepCounterIf options)",
			from:    getterWithConditions(foo, bar),
			options: []MergeOption{WithStepCounterIf(false)},
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "message falseBar"),
		},
		{
			name:    "Returns ready condition with the summary of existing conditions (using WithStepCounterIf options)",
			from:    getterWithConditions(foo, bar),
			options: []MergeOption{WithStepCounterIf(true)},
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "1 of 2 completed"),
		},
		{
			name:    "Returns ready condition with the summary of existing conditions (using WithStepCounterIf and WithStepCounterIfOnly options)",
			from:    getterWithConditions(bar),
			options: []MergeOption{WithStepCounter(), WithStepCounterIfOnly("bar")},
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "0 of 1 completed"),
		},
		{
			name:    "Returns ready condition with the summary of existing conditions (using WithStepCounterIf and WithStepCounterIfOnly options)",
			from:    getterWithConditions(foo, bar),
			options: []MergeOption{WithStepCounter(), WithStepCounterIfOnly("foo")},
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "message falseBar"),
		},
		{
			name:    "Returns ready condition with the summary of selected conditions (using WithConditions options)",
			from:    getterWithConditions(foo, bar),
			options: []MergeOption{WithConditions("foo")}, // bar should be ignored
			want:    TrueCondition(meta.ReadyCondition, "reason trueFoo", "message trueFoo"),
		},
		{
			name:    "Returns ready condition with the summary of selected conditions (using WithConditions and WithStepCounter options)",
			from:    getterWithConditions(foo, bar, baz),
			options: []MergeOption{WithConditions("foo", "bar"), WithStepCounter()}, // baz should be ignored, total steps should be 2
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "1 of 2 completed"),
		},
		{
			name:    "Returns ready condition with the summary of selected conditions (using WithConditions and WithStepCounterIfOnly options)",
			from:    getterWithConditions(bar),
			options: []MergeOption{WithConditions("bar", "baz"), WithStepCounter(), WithStepCounterIfOnly("bar")}, // there is only bar, the step counter should be set and counts only a subset of conditions
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "0 of 1 completed"),
		},
		{
			name:    "Returns ready condition with the summary of selected conditions (using WithConditions and WithStepCounterIfOnly options - with inconsistent order between the two)",
			from:    getterWithConditions(bar),
			options: []MergeOption{WithConditions("baz", "bar"), WithStepCounter(), WithStepCounterIfOnly("bar", "baz")}, // conditions in WithStepCounterIfOnly could be in different order than in WithConditions
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "0 of 2 completed"),
		},
		{
			name:    "Returns ready condition with the summary of selected conditions (using WithConditions and WithStepCounterIfOnly options)",
			from:    getterWithConditions(bar, baz),
			options: []MergeOption{WithConditions("bar", "baz"), WithStepCounter(), WithStepCounterIfOnly("bar")}, // there is also baz, so the step counter should not be set
			want:    FalseCondition(meta.ReadyCondition, "reason falseBar", "message falseBar"),
		},
		{
			name:    "Ready condition respects merge order",
			from:    getterWithConditions(bar, baz),
			options: []MergeOption{WithConditions("baz", "bar")}, // baz should take precedence on bar
			want:    FalseCondition(meta.ReadyCondition, "reason falseBaz", "message falseBaz"),
		},
		{
			name: "Ignores existing Ready condition when computing the summary",
			from: getterWithConditions(existingReady, foo, bar),
			want: FalseCondition(meta.ReadyCondition, "reason falseBar", "message falseBar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := summary(tt.from, meta.ReadyCondition, tt.options...)
			if tt.want == nil {
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(got).To(HaveSameStateOf(tt.want))
		})
	}
}

func TestAggregate(t *testing.T) {
	ready1 := TrueCondition(meta.ReadyCondition, "reason true1", "message true1")
	ready2 := FalseCondition(meta.ReadyCondition, "reason false1", "message false1")
	bar := FalseCondition("bar", "reason falseBar1", "message falseBar1") //NB. bar has higher priority than other conditions

	tests := []struct {
		name string
		from []Getter
		t    string
		opts []MergeOption
		want *metav1.Condition
	}{
		{
			name: "Returns nil when there are no conditions to aggregate",
			from: []Getter{},
			want: nil,
		},
		{
			name: "Returns foo condition with an aggregation of the object's top group conditions",
			from: []Getter{
				getterWithConditions(ready1),
				getterWithConditions(ready1),
				getterWithConditions(ready2, bar),
				getterWithConditions(),
				getterWithConditions(bar),
			},
			t:    "foo",
			want: FalseCondition("foo", "reason false1", "message false1"),
		},
		{
			name: "Returns foo condition with the aggregation of object's subset conditions",
			from: []Getter{
				getterWithConditions(ready1),
				getterWithConditions(ready1),
				getterWithConditions(ready2, bar),
				getterWithConditions(),
				getterWithConditions(bar),
			},
			opts: []MergeOption{
				WithConditions("bar"),
			},
			t:    "foo",
			want: FalseCondition("foo", "reason falseBar1", "message falseBar1"),
		},
		{
			name: "Returns foo condition with the aggregation of object's subset priority conditions",
			from: []Getter{
				getterWithConditions(ready1),
				getterWithConditions(ready1),
				getterWithConditions(ready2, bar),
				getterWithConditions(),
				getterWithConditions(bar),
			},
			opts: []MergeOption{
				WithConditions("bar", meta.ReadyCondition),
			},
			t:    "foo",
			want: FalseCondition("foo", "reason falseBar1", "message falseBar1"),
		},
		{
			name: "Returns foo condition with the aggregation of object's subset priority conditions (inverse)",
			from: []Getter{
				getterWithConditions(ready1),
				getterWithConditions(ready1),
				getterWithConditions(ready2, bar),
				getterWithConditions(),
				getterWithConditions(bar),
			},
			opts: []MergeOption{
				WithConditions(meta.ReadyCondition, "bar"),
			},
			t:    "foo",
			want: FalseCondition("foo", "reason false1", "message false1"),
		},
		{
			name: "Returns foo condition with source ref",
			from: []Getter{
				getterWithConditions(ready1),
				getterWithConditions(ready1),
				getterWithConditions(ready2, bar),
				getterWithConditions(),
				getterWithConditions(bar),
			},
			opts: []MergeOption{
				WithSourceRefIf(meta.ReadyCondition),
			},
			t:    "foo",
			want: FalseCondition("foo", "reason false1 @ /", "message false1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := aggregate(tt.from, tt.t, tt.opts...)
			if tt.want == nil {
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(got).To(HaveSameStateOf(tt.want))
		})
	}
}

func Fuzz_Getter(f *testing.F) {
	f.Fuzz(func(t *testing.T,
		data []byte) {
		fc := fuzz.NewConsumer(data)

		// Set a reproduceable amount of conditions.
		noOfConditions, err := fc.GetInt()
		if err != nil {
			return
		}
		maxNoOfConditions := 30
		conditions := make([]metav1.Condition, 0)

		// Add N conditions in the slice based on a reproduceable
		// state provided by fc.
		for i := 0; i < noOfConditions%maxNoOfConditions; i++ {
			c := metav1.Condition{}
			err = fc.GenerateStruct(&c)
			if err != nil {
				return
			}
			conditions = append(conditions, c)
		}
		obj := &testdata.Fake{}
		obj.SetConditions(conditions)

		targetCondition, err := fc.GetString()
		if err != nil {
			return
		}

		SetSummary(obj, targetCondition)
		return
	})
}

func getterWithConditions(conditions ...*metav1.Condition) Getter {
	obj := &testdata.Fake{}
	obj.SetConditions(conditionList(conditions...))
	return obj
}

func conditionList(conditions ...*metav1.Condition) []metav1.Condition {
	cs := []metav1.Condition{}
	for _, x := range conditions {
		if x != nil {
			cs = append(cs, *x)
		}
	}
	return cs
}
