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
)

func TestNewConditionsGroup(t *testing.T) {
	g := NewWithT(t)

	conditions := []*metav1.Condition{nil1, true1, true1, false1, unknown1}

	got := getConditionGroups(conditionsWithSource(&fake{}, conditions...))

	g.Expect(got).ToNot(BeNil())
	g.Expect(got).To(HaveLen(3))

	// The top group should be False and it should have one condition
	g.Expect(got.TopGroup().status).To(Equal(metav1.ConditionFalse))
	g.Expect(got.TopGroup().conditions).To(HaveLen(1))

	// The true group should be True and it should have two conditions
	g.Expect(got.TrueGroup().status).To(Equal(metav1.ConditionTrue))
	g.Expect(got.TrueGroup().conditions).To(HaveLen(2))

	// got[0] should be False and it should have one condition
	g.Expect(got[0].status).To(Equal(metav1.ConditionFalse))
	g.Expect(got[0].conditions).To(HaveLen(1))

	// got[1] should be True and it should have two conditions
	g.Expect(got[1].status).To(Equal(metav1.ConditionTrue))
	g.Expect(got[1].conditions).To(HaveLen(2))

	// got[2] should be Unknown and it should have one condition
	g.Expect(got[2].status).To(Equal(metav1.ConditionUnknown))
	g.Expect(got[2].conditions).To(HaveLen(1))

	// nil conditions are ignored
}

func TestMergeRespectPriority(t *testing.T) {
	tests := []struct {
		name       string
		conditions []*metav1.Condition
		want       *metav1.Condition
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
			name:       "When there is False it returns False",
			conditions: []*metav1.Condition{false1, unknown1, true1},
			want:       FalseCondition("foo", "reason false1","message false1"),
		},
		{
			name:       "When there is True and no False, it returns True",
			conditions: []*metav1.Condition{unknown1, true1},
			want:       TrueCondition("foo", "reason true1", "message true1"),
		},
		{
			name:       "When there is Unknown and no True or False, it returns Unknown",
			conditions: []*metav1.Condition{unknown1},
			want:       UnknownCondition("foo", "reason unknown1", "message unknown1"),
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

			got := merge(conditionsWithSource(&fake{}, tt.conditions...), "foo", &mergeOptions{})

			if tt.want == nil {
				g.Expect(got).To(BeNil())
				return
			}
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
