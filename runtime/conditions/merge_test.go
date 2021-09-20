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
https://github.com/kubernetes-sigs/cluster-api/tree/7478817225e0a75acb6e14fc7b438231578073d2/util/conditions/merge_test.go,
and initially adapted to work with the `metav1.Condition` and `metav1.ConditionStatus` types.
More concretely, this includes the removal of "condition severity" related functionalities, as this is not supported by
the `metav1.Condition` type.
*/

package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func TestNewConditionsGroup(t *testing.T) {
	g := NewWithT(t)

	negativeFalseReconciling := FalseCondition(meta.ReconcilingCondition, "reason reconciling1", "message reconciling1")
	negativeTrueStalled := TrueCondition(meta.StalledCondition, "reason stalled1", "message stalled1")
	negativeUnknownReconciling := UnknownCondition(meta.ReconcilingCondition, "reason reconciling2", "message reconciling2")

	conditions := []*metav1.Condition{nil1, true1, true1, false1, unknown1, negativeFalseReconciling, negativeTrueStalled, negativeUnknownReconciling}

	got := getConditionGroups(conditionsWithSource(&testdata.Fake{}, conditions...), &mergeOptions{
		negativePolarityConditionTypes: []string{meta.ReconcilingCondition, meta.StalledCondition},
	})

	g.Expect(got).ToNot(BeNil())
	g.Expect(got).To(HaveLen(6))

	// The TopGroup should be True/Negative and it should have one condition
	g.Expect(got.TopGroup().status).To(Equal(metav1.ConditionTrue))
	g.Expect(got.TopGroup().negativePolarity).To(BeTrue())
	g.Expect(got.TopGroup().conditions).To(HaveLen(1))

	// The TruePositivePolarityGroup should be True/Positive and it should have one condition
	g.Expect(got.TruePositivePolarityGroup().status).To(Equal(metav1.ConditionTrue))
	g.Expect(got.TruePositivePolarityGroup().negativePolarity).To(BeFalse())
	g.Expect(got.TruePositivePolarityGroup().conditions).To(HaveLen(2))

	// got[0] should be True/Negative and it should have one condition
	g.Expect(got[0].status).To(Equal(metav1.ConditionTrue))
	g.Expect(got[0].negativePolarity).To(BeTrue())
	g.Expect(got[0].conditions).To(HaveLen(1))

	// got[1] should be False/Positive and it should have one conditions
	g.Expect(got[1].status).To(Equal(metav1.ConditionFalse))
	g.Expect(got[1].negativePolarity).To(BeFalse())
	g.Expect(got[1].conditions).To(HaveLen(1))

	// got[2] should be True/Positive and it should have two conditions
	g.Expect(got[2].status).To(Equal(metav1.ConditionTrue))
	g.Expect(got[1].negativePolarity).To(BeFalse())
	g.Expect(got[2].conditions).To(HaveLen(2))

	// got[3] should be False/Negative and it should have one condition
	g.Expect(got[3].status).To(Equal(metav1.ConditionFalse))
	g.Expect(got[3].negativePolarity).To(BeTrue())
	g.Expect(got[3].conditions).To(HaveLen(1))

	// got[4] should be Unknown/Positive and it should have one condition
	g.Expect(got[4].status).To(Equal(metav1.ConditionUnknown))
	g.Expect(got[4].negativePolarity).To(BeFalse())
	g.Expect(got[4].conditions).To(HaveLen(1))

	// got[5] should be Unknown/Negative and it should have one condition
	g.Expect(got[5].status).To(Equal(metav1.ConditionUnknown))
	g.Expect(got[5].negativePolarity).To(BeTrue())
	g.Expect(got[3].conditions).To(HaveLen(1))

	// nil conditions are ignored
}

func TestMergeRespectPriority(t *testing.T) {
	tests := []struct {
		name               string
		negativeConditions []string
		conditions         []*metav1.Condition
		want               *metav1.Condition
	}{
		{
			name:       "aggregate nil list return nil",
			conditions: nil,
			want:       nil,
		},
		{
			name:       "aggregate empty list return nil",
			conditions: []*metav1.Condition{},
			want:       nil,
		},
		{
			name:               "When there is True/Negative it returns an inverted False/Positive",
			negativeConditions: []string{true1.Type},
			conditions:         []*metav1.Condition{false1, false1, false1, unknown1, true1},
			want:               FalseCondition("foo", "reason true1", "message true1"),
		},
		{
			name:       "When there is False/Positive and no True/Negative, it returns False/Positive",
			conditions: []*metav1.Condition{false1, false1, unknown1, true1},
			want:       FalseCondition("foo", "reason false1", "message false1"),
		},
		{
			name:               "When there is True/Positive and no True/Negative or False/Positive, it returns True/Positive",
			negativeConditions: []string{false1.Type},
			conditions:         []*metav1.Condition{false1, unknown1, true1},
			want:               TrueCondition("foo", "reason true1", "message true1"),
		},
		{
			name:               "When there is True/Positive and no False/Positive, it returns True/Positive",
			negativeConditions: []string{false1.Type},
			conditions:         []*metav1.Condition{unknown1, true1, false1},
			want:               TrueCondition("foo", "reason true1", "message true1"),
		},
		{
			name:               "When there is False/Negative and no True/* or False/Positive, it returns False/Negative",
			negativeConditions: []string{false1.Type},
			conditions:         []*metav1.Condition{unknown1, false1},
			want:               TrueCondition("foo", "reason false1", "message false1"),
		},
		{
			name:       "When there is Unknown/* but no False/*, it returns Unknown/*",
			conditions: []*metav1.Condition{unknown1},
			want:       UnknownCondition("foo", "reason unknown1", "message unknown1"),
		},
		{
			name:               "When the target condition is inverted, it returns an inverted condition",
			negativeConditions: []string{"foo"},
			conditions:         []*metav1.Condition{true1},
			want:               FalseCondition("foo", "reason true1", "message true1"),
		},
		{
			name:               "When the top and target conditions are inverted, it returns an equal condition",
			negativeConditions: []string{"foo", true1.Type},
			conditions:         []*metav1.Condition{true1},
			want:               TrueCondition("foo", "reason true1", "message true1"),
		},
		{
			name:       "nil conditions are ignored",
			conditions: []*metav1.Condition{nil1, nil1, nil1},
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := merge(conditionsWithSource(&testdata.Fake{}, tt.conditions...), "foo", &mergeOptions{
				negativePolarityConditionTypes: tt.negativeConditions,
			})

			if tt.want == nil {
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(got).To(HaveSameStateOf(tt.want))
		})
	}
}

func TestMergeRespectGeneration(t *testing.T) {
	tests := []struct {
		name               string
		negativeConditions []string
		conditions         []*metav1.Condition
		mergeOpts          []MergeOption
		want               *metav1.Condition
	}{
		{
			name:               "without generation",
			negativeConditions: []string{true1.Type},
			conditions: []*metav1.Condition{
				conditionWithGeneration(false1, 1),
				conditionWithGeneration(true1, 2),
				conditionWithGeneration(unknown1, 3),
			},
			want: FalseCondition("foo", "reason true1", "message true1"),
		},
		{
			name:               "with generation",
			negativeConditions: []string{true1.Type},
			conditions: []*metav1.Condition{
				conditionWithGeneration(false1, 1),
				conditionWithGeneration(unknown1, 4),
				conditionWithGeneration(true1, 4),
			},
			mergeOpts: []MergeOption{WithLatestGeneration()},
			want:      FalseCondition("foo", "reason true1", "message true1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			mo := &mergeOptions{
				negativePolarityConditionTypes: tt.negativeConditions,
			}
			for _, o := range tt.mergeOpts {
				o(mo)
			}
			got := merge(conditionsWithSource(&testdata.Fake{}, tt.conditions...), "foo", mo)
			g.Expect(got).To(HaveSameStateOf(tt.want))
		})
	}
}

func conditionsWithSource(obj Setter, conditions ...*metav1.Condition) []localizedCondition {
	obj.SetConditions(conditionList(conditions...))

	ret := []localizedCondition{}
	for i := range conditions {
		ret = append(ret, localizedCondition{
			Condition: conditions[i],
			Getter:    obj,
		})
	}

	return ret
}

func conditionWithGeneration(condition *metav1.Condition, generation int64) *metav1.Condition {
	condition.ObservedGeneration = generation
	return condition
}
