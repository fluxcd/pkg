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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func Test_check_FAIL0001(t *testing.T) {
	tests := []struct {
		name             string
		negativePolarity []string
		addConditions    func(obj conditions.Setter)
		wantErr          bool
		errCheck         func(t *WithT, err error)
	}{
		{
			name: "Ready False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
			},
		},
		{
			name: "Ready True, no negative polarity",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkFalse(obj, "TestCondition1", "Foo1", "Bar1")
			},
		},
		{
			name:             "Ready True, with negative polarity but not True",
			negativePolarity: []string{"TestCondition1", "TestCondition2"},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkFalse(obj, "TestCondition1", "Foo1", "Bar1")
				conditions.MarkFalse(obj, "TestCondition2", "Foo2", "Bar2")
			},
		},
		{
			name:             "Ready True, with negative polarity True",
			negativePolarity: []string{"TestCondition1", "TestCondition2", "TestCondition3"},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkFalse(obj, "TestCondition1", "Foo1", "Bar1")
				conditions.MarkTrue(obj, "TestCondition2", "Foo2", "Bar2")
				conditions.MarkTrue(obj, "TestCondition3", "Foo3", "Bar3")
			},
			wantErr: true,
			errCheck: func(t *WithT, err error) {
				t.Expect(err.Error()).To(ContainSubstring("TestCondition2"))
				t.Expect(err.Error()).To(ContainSubstring("TestCondition3"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &testdata.Fake{}

			if tt.addConditions != nil {
				tt.addConditions(obj)
			}

			condns := &Conditions{NegativePolarity: tt.negativePolarity}
			err := check_FAIL0001(context.TODO(), obj, condns)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.errCheck != nil {
				tt.errCheck(g, err)
			}
		})
	}
}

func Test_check_FAIL0002(t *testing.T) {
	tests := []struct {
		name          string
		addConditions func(obj conditions.Setter)
		wantErr       bool
	}{
		{
			name: "no Ready condition",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
			},
			wantErr: true,
		},
		{
			name: "with Ready condition",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "FooMsg")
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &testdata.Fake{}

			if tt.addConditions != nil {
				tt.addConditions(obj)
			}

			err := check_FAIL0002(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_FAIL0003(t *testing.T) {
	tests := []struct {
		name          string
		addConditions func(obj conditions.Setter)
		wantErr       bool
	}{
		{
			name: "No Reconciling",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooY", "BarY")
			},
		},
		{
			name: "Reconciling False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReconcilingCondition, "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooY", "BarY")
			},
		},
		{
			name: "Reconciling False, no Ready",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReconcilingCondition, "FooX", "BarX")
				conditions.MarkTrue(obj, "TestCondition1", "FooY", "BarY")
			},
		},
		{
			name: "Reconciling True, Ready True",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooY", "BarY")
			},
			wantErr: true,
		},
		{
			name: "Reconciling True, Ready False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "FooX", "BarX")
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooY", "BarY")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &testdata.Fake{}
			if tt.addConditions != nil {
				tt.addConditions(obj)
			}
			err := check_FAIL0003(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_FAIL0004(t *testing.T) {
	tests := []struct {
		name          string
		addConditions func(obj conditions.Setter)
		wantErr       bool
	}{
		{
			name: "No Stalled",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooY", "BarY")
			},
		},
		{
			name: "Stalled False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.StalledCondition, "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooY", "BarY")
			},
		},
		{
			name: "Stalled False, no Ready",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.StalledCondition, "FooX", "BarX")
				conditions.MarkTrue(obj, "TestCondition1", "FooY", "BarY")
			},
		},
		{
			name: "Stalled True, Ready True",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.StalledCondition, "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooY", "BarY")
			},
			wantErr: true,
		},
		{
			name: "Stalled True, Ready False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.StalledCondition, "FooX", "BarX")
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooY", "BarY")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &testdata.Fake{}
			if tt.addConditions != nil {
				tt.addConditions(obj)
			}
			err := check_FAIL0004(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_FAIL0005(t *testing.T) {
	tests := []struct {
		name          string
		addConditions func(obj conditions.Setter)
		wantErr       bool
	}{
		{
			name: "no Reconciling, no Stalled",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
			},
		},
		{
			name: "Reconciling present, no Stalled",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "FooX", "BarX")
			},
		},
		{
			name: "no Reconciling, Stalled present",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
				conditions.MarkTrue(obj, meta.StalledCondition, "FooX", "BarX")
			},
		},
		{
			name: "Reconciling present, Stalled present",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "FooX", "BarX")
				conditions.MarkTrue(obj, meta.StalledCondition, "FooX", "BarX")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &testdata.Fake{}
			if tt.addConditions != nil {
				tt.addConditions(obj)
			}
			err := check_FAIL0005(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_FAIL0006(t *testing.T) {
	tests := []struct {
		name               string
		objectGeneration   int64
		observedGeneration int64
		wantErr            bool
	}{
		{
			name:               "ObservedGeneration < object generation",
			objectGeneration:   3,
			observedGeneration: 2,
		},
		{
			name:               "ObservedGeneration = object generation",
			objectGeneration:   3,
			observedGeneration: 3,
		},
		{
			name:               "ObservedGeneration > object generation",
			objectGeneration:   3,
			observedGeneration: 4,
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(testdata.AddFakeToScheme(scheme)).To(Succeed())

			obj := &testdata.Fake{}
			obj.SetGeneration(tt.objectGeneration)
			obj.Status.ObservedGeneration = tt.observedGeneration

			err := check_FAIL0006(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}

}

func Test_check_FAIL0007(t *testing.T) {
	tests := []struct {
		name               string
		objectGeneration   int64
		observedGeneration int64
		addConditions      func(obj conditions.Setter)
		wantErr            bool
	}{
		{
			name:               "ObservedGeneration = object Generation, Ready False",
			objectGeneration:   4,
			observedGeneration: 4,
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
		},
		{
			name:               "ObservedGeneration < object Generation, Ready True",
			objectGeneration:   4,
			observedGeneration: 3,
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
		},
		{
			name:               "ObservedGeneration < object Generation, Ready True",
			objectGeneration:   4,
			observedGeneration: 3,
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(testdata.AddFakeToScheme(scheme)).To(Succeed())

			obj := &testdata.Fake{}
			obj.SetGeneration(tt.objectGeneration)
			obj.Status.ObservedGeneration = tt.observedGeneration

			if tt.addConditions != nil {
				tt.addConditions(obj)
			}

			err := check_FAIL0007(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_FAIL0008(t *testing.T) {
	tests := []struct {
		name             string
		objectGeneration int64
		conditions       []metav1.Condition
		wantErr          bool
		errCheck         func(t *WithT, err error)
	}{
		{
			name:             "Ready False",
			objectGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
				},
			},
		},
		{
			name:             "conditions ObservedGeneration = object Generation, Ready True",
			objectGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
				},
				{
					Type:               "TestCondition1",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
				},
			},
		},
		{
			name:             "conditions ObservedGeneration < object Generation, Ready True",
			objectGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
				},
				{
					Type:               "TestCondition1",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 2,
				},
				{
					Type:               "TestCondition2",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 2,
				},
			},
			wantErr: true,
			errCheck: func(t *WithT, err error) {
				t.Expect(err.Error()).To(ContainSubstring("TestCondition1"))
				t.Expect(err.Error()).To(ContainSubstring("TestCondition2"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &testdata.Fake{}
			obj.SetGeneration(tt.objectGeneration)
			obj.SetConditions(tt.conditions)

			err := check_FAIL0008(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.errCheck != nil {
				tt.errCheck(g, err)
			}
		})
	}
}

func Test_check_FAIL0009(t *testing.T) {
	tests := []struct {
		name                   string
		rootObservedGeneration int64
		conditions             []metav1.Condition
		wantErr                bool
		errCheck               func(t *WithT, err error)
	}{
		{
			name:                   "Ready False",
			rootObservedGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
				},
			},
		},
		{
			name:                   "Conditions ObservedGeneration = Root ObservedGeneration, Ready True",
			rootObservedGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
				},
				{
					Type:               "TestCondition1",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
				},
			},
		},
		{
			name:                   "Conditions ObservedGeneration != Root ObservedGeneration, Ready True",
			rootObservedGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
				},
				{
					Type:               "TestCondition1",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 2,
				},
				{
					Type:               "TestCondition2",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 2,
				},
			},
			wantErr: true,
			errCheck: func(t *WithT, err error) {
				t.Expect(err.Error()).To(ContainSubstring("TestCondition1"))
				t.Expect(err.Error()).To(ContainSubstring("TestCondition2"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(testdata.AddFakeToScheme(scheme)).To(Succeed())

			obj := &testdata.Fake{}
			obj.Status.ObservedGeneration = tt.rootObservedGeneration
			obj.SetConditions(tt.conditions)

			err := check_FAIL0009(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.errCheck != nil {
				tt.errCheck(g, err)
			}
		})
	}
}

func Test_check_FAIL0010(t *testing.T) {
	tests := []struct {
		name                   string
		rootObservedGeneration int64
		conditions             []metav1.Condition
		wantErr                bool
	}{
		{
			name:                   "Reconciling False",
			rootObservedGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReconcilingCondition,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
				},
			},
		},
		{
			name:                   "Reconciling True, root ObservedGeneration < Reconciling ObservedGeneration",
			rootObservedGeneration: 2,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReconcilingCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
				},
			},
		},
		{
			name:                   "Reconciling True, root ObservedGeneration = Reconciling ObservedGeneration",
			rootObservedGeneration: 3,
			conditions: []metav1.Condition{
				{
					Type:               meta.ReconcilingCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(testdata.AddFakeToScheme(scheme)).To(Succeed())

			obj := &testdata.Fake{}
			obj.Status.ObservedGeneration = tt.rootObservedGeneration
			obj.SetConditions(tt.conditions)

			err := check_FAIL0010(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}

}
