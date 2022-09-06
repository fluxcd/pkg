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
https://github.com/kubernetes-sigs/cluster-api/tree/7478817225e0a75acb6e14fc7b438231578073d2/util/conditions/patch_test.go,
and initially adapted to work with the `metav1.Condition` and `metav1.ConditionStatus` types.
More concretely, this includes the removal of "condition severity" related functionalities, as this is not supported by
the `metav1.Condition` type.
*/

package conditions

import (
	"testing"
	"time"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func TestNewPatch(t *testing.T) {
	fooTrue := TrueCondition("foo", "reason true", "message true")
	fooFalse := FalseCondition("foo", "reason false", "message false")

	tests := []struct {
		name   string
		before Getter
		after  Getter
		want   Patch
	}{
		{
			name:   "No changes return empty patch",
			before: getterWithConditions(),
			after:  getterWithConditions(),
			want:   nil,
		},
		{
			name:   "No changes return empty patch",
			before: getterWithConditions(fooTrue),
			after:  getterWithConditions(fooTrue),
			want:   nil,
		},
		{
			name:   "Detects AddConditionPatch",
			before: getterWithConditions(),
			after:  getterWithConditions(fooTrue),
			want: Patch{
				{
					Before: nil,
					After:  fooTrue,
					Op:     AddConditionPatch,
				},
			},
		},
		{
			name:   "Detects ChangeConditionPatch",
			before: getterWithConditions(fooTrue),
			after:  getterWithConditions(fooFalse),
			want: Patch{
				{
					Before: fooTrue,
					After:  fooFalse,
					Op:     ChangeConditionPatch,
				},
			},
		},
		{
			name:   "Detects RemoveConditionPatch",
			before: getterWithConditions(fooTrue),
			after:  getterWithConditions(),
			want: Patch{
				{
					Before: fooTrue,
					After:  nil,
					Op:     RemoveConditionPatch,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := NewPatch(tt.before, tt.after)

			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestApply(t *testing.T) {
	fooTrue := TrueCondition("foo", "reason true", "message true")
	fooFalse := FalseCondition("foo", "reason false", "message false")
	fooUnknown := UnknownCondition("foo", "reason unknown", "message unknown")

	tests := []struct {
		name    string
		before  Getter
		after   Getter
		latest  Setter
		options []ApplyOption
		want    []metav1.Condition
		wantErr bool
	}{
		{
			name:    "No patch return same list",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(fooTrue),
			latest:  setterWithConditions(fooTrue),
			want:    conditionList(fooTrue),
			wantErr: false,
		},
		{
			name:    "Add: When a condition does not exists, it should add",
			before:  getterWithConditions(),
			after:   getterWithConditions(fooTrue),
			latest:  setterWithConditions(),
			want:    conditionList(fooTrue),
			wantErr: false,
		},
		{
			name:    "Add: When a condition already exists but without conflicts, it should add",
			before:  getterWithConditions(),
			after:   getterWithConditions(fooTrue),
			latest:  setterWithConditions(fooTrue),
			want:    conditionList(fooTrue),
			wantErr: false,
		},
		{
			name:    "Add: When a condition already exists but with conflicts, it should error",
			before:  getterWithConditions(),
			after:   getterWithConditions(fooTrue),
			latest:  setterWithConditions(fooFalse),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Add: When a condition already exists but with conflicts, it should not error if the condition is owned",
			before:  getterWithConditions(),
			after:   getterWithConditions(fooTrue),
			latest:  setterWithConditions(fooFalse),
			options: []ApplyOption{WithOwnedConditions("foo")},
			want:    conditionList(fooTrue), // after condition should be kept in case of error
			wantErr: false,
		},
		{
			name:    "Remove: When a condition was already deleted, it should pass",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(),
			latest:  setterWithConditions(),
			want:    conditionList(),
			wantErr: false,
		},
		{
			name:    "Remove: When a condition already exists but without conflicts, it should delete",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(),
			latest:  setterWithConditions(fooTrue),
			want:    conditionList(),
			wantErr: false,
		},
		{
			name:    "Remove: When a condition already exists but with conflicts, it should error",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(),
			latest:  setterWithConditions(fooFalse),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Remove: When a condition already exists but with conflicts, it should not error if the condition is owned",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(),
			latest:  setterWithConditions(fooFalse),
			options: []ApplyOption{WithOwnedConditions("foo")},
			want:    conditionList(), // after condition should be kept in case of error
			wantErr: false,
		},
		{
			name:    "Change: When a condition exists without conflicts, it should change",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(fooFalse),
			latest:  setterWithConditions(fooTrue),
			want:    conditionList(fooFalse),
			wantErr: false,
		},
		{
			name:    "Change: When a condition exists with conflicts but there is agreement on the final state, it should change",
			before:  getterWithConditions(fooFalse),
			after:   getterWithConditions(fooTrue),
			latest:  setterWithConditions(fooTrue),
			want:    conditionList(fooTrue),
			wantErr: false,
		},
		{
			name:    "Change: When a condition exists with conflicts but there is no agreement on the final state, it should error",
			before:  getterWithConditions(fooUnknown),
			after:   getterWithConditions(fooFalse),
			latest:  setterWithConditions(fooTrue),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Change: When a condition exists with conflicts but there is no agreement on the final state, it should not error if the condition is owned",
			before:  getterWithConditions(fooUnknown),
			after:   getterWithConditions(fooFalse),
			latest:  setterWithConditions(fooTrue),
			options: []ApplyOption{WithOwnedConditions("foo")},
			want:    conditionList(fooFalse), // after condition should be kept in case of error
			wantErr: false,
		},
		{
			name:    "Change: When a condition was deleted, it should error",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(fooFalse),
			latest:  setterWithConditions(),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Change: When a condition was deleted, it should not error if the condition is owned",
			before:  getterWithConditions(fooTrue),
			after:   getterWithConditions(fooFalse),
			latest:  setterWithConditions(),
			options: []ApplyOption{WithOwnedConditions("foo")},
			want:    conditionList(fooFalse), // after condition should be kept in case of error
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			patch := NewPatch(tt.before, tt.after)

			err := patch.Apply(tt.latest, tt.options...)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(tt.latest.GetConditions()).To(haveSameConditionsOf(tt.want))
		})
	}
}

func TestApplyDoesNotAlterLastTransitionTime(t *testing.T) {
	g := NewWithT(t)

	before := &testdata.Fake{}
	after := &testdata.Fake{
		Status: testdata.FakeStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "foo",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
				},
			},
		},
	}
	latest := &testdata.Fake{}

	// latest has no conditions, so we are actually adding the
	// condition but in this case we should not set the LastTransitionTime
	// but we should preserve the LastTransition set in after

	diff := NewPatch(before, after)
	err := diff.Apply(latest)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(latest.GetConditions()).To(Equal(after.GetConditions()))
}

func Fuzz_PatchApply(f *testing.F) {
	f.Fuzz(func(t *testing.T,
		beforeData, afterData, setterData []byte) {

		before, err := newFake(fuzz.NewConsumer(beforeData))
		if err != nil {
			return
		}

		after, err := newFake(fuzz.NewConsumer(afterData))
		if err != nil {
			return
		}

		patch := NewPatch(before, after)
		setter, err := newFake(fuzz.NewConsumer(setterData))
		if err != nil {
			return
		}

		_ = patch.Apply(setter)
	})
}

func newFake(fc *fuzz.ConsumeFuzzer) (*testdata.Fake, error) {
	obj := &testdata.Fake{}
	noOfConditions, err := fc.GetInt()
	if err != nil {
		return obj, err
	}

	maxNoOfConditions := 30
	conditions := make([]metav1.Condition, 0)
	for i := 0; i < noOfConditions%maxNoOfConditions; i++ {
		c := metav1.Condition{}
		err = fc.GenerateStruct(&c)
		if err != nil {
			return obj, err
		}

		conditions = append(conditions, c)
	}
	obj.SetConditions(conditions)
	return obj, nil
}
