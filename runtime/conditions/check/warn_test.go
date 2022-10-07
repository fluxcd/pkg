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

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func Test_check_WARN0001(t *testing.T) {
	tests := []struct {
		name             string
		negativePolarity []string
		addConditions    func(obj conditions.Setter)
		wantErr          bool
	}{
		{
			name: "Ready False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMessage")
				conditions.MarkTrue(obj, "SomeCondition", "FooX", "BarY")
			},
		},
		{
			name: "Ready True, no negative polarity context",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMessage")
				conditions.MarkTrue(obj, "SomeCondition", "FooX", "BarY")
			},
		},
		{
			name:             "Ready True, with polarity context, no other negative conditions",
			negativePolarity: []string{"TestCondition1", "TestCondition2"},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMessage")
				conditions.MarkTrue(obj, "TestCondition3", "FooX", "BarY")
			},
		},
		{
			name:             "Ready True, with polarity context, with negative conditions",
			negativePolarity: []string{"TestCondition1", "TestCondition2"},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMessage")
				conditions.MarkTrue(obj, "TestCondition2", "FooX", "BarY")
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

			condns := &Conditions{NegativePolarity: tt.negativePolarity}
			err := check_WARN0001(context.TODO(), obj, condns)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_WARN0002(t *testing.T) {
	tests := []struct {
		name             string
		negativePolarity []string
		addConditions    func(obj conditions.Setter)
		wantErr          bool
		errCheck         func(t *WithT, err error)
	}{
		{
			name: "Ready True",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
		},
		{
			name: "Ready False, no negative polarity",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, "TestCondition1", "FooX", "BarX")
			},
		},
		{
			name:             "Ready False, with negative polarity",
			negativePolarity: []string{"TestCondition1"},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, "TestCondition1", "Foo1", "Bar1")
				conditions.MarkTrue(obj, "TestCondition2", "Foo2", "Bar2")
			},
			wantErr: true,
			errCheck: func(t *WithT, err error) {
				t.Expect(err.Error()).To(ContainSubstring("TestCondition1"))
				t.Expect(err.Error()).ToNot(ContainSubstring("TestCondition2"))
			},
		},
		{
			name:             "Ready False, with multiple negative polarity",
			negativePolarity: []string{"TestCondition2", "TestCondition1", "TestCondition3"},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, "TestCondition1", "FooReason", "Bar1")
				conditions.MarkTrue(obj, "TestCondition3", "Foo2", "Bar2")
			},
			wantErr: true,
			errCheck: func(t *WithT, err error) {
				t.Expect(err.Error()).To(ContainSubstring("TestCondition1"))
				t.Expect(err.Error()).ToNot(ContainSubstring("TestCondition2"))
				t.Expect(err.Error()).ToNot(ContainSubstring("TestCondition3"))
			},
		},
		{
			name:             "Ready False, with highest negative condition values",
			negativePolarity: []string{"TestCondition2", "TestCondition1", "TestCondition3"},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "Bar1")
				conditions.MarkTrue(obj, "TestCondition1", "FooReason", "Bar1")
				conditions.MarkTrue(obj, "TestCondition3", "Foo2", "Bar2")
			},
		},
		{
			name:             "Ready False, with different highest negative condition Reconciling",
			negativePolarity: []string{meta.StalledCondition, meta.ReconcilingCondition},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "NewGeneration", "reconciling new obj gen")
			},
			wantErr: false,
		},
		{
			name:             "Ready False, with different highest negative condition Stalled",
			negativePolarity: []string{meta.StalledCondition, meta.ReconcilingCondition},
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, meta.StalledCondition, "InvalidFoo", "invalid foo")
			},
			wantErr: false,
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
			err := check_WARN0002(context.TODO(), obj, condns)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.errCheck != nil {
				tt.errCheck(g, err)
			}
		})
	}
}

func Test_check_WARN0003(t *testing.T) {
	tests := []struct {
		name          string
		addConditions func(obj conditions.Setter)
		wantErr       bool
	}{
		{
			name: "No Reconciling",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
		},
		{
			name: "Reconciling True",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "FooReason", "BarMsg")
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
		},
		{
			name: "Reconciling False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReconcilingCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, "TestCondition1", "FooReason", "BarMsg")
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
			err := check_WARN0003(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_WARN0004(t *testing.T) {
	tests := []struct {
		name          string
		addConditions func(obj conditions.Setter)
		wantErr       bool
	}{
		{
			name: "No Stalled",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
		},
		{
			name: "Stalled True",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.StalledCondition, "FooReason", "BarMsg")
				conditions.MarkFalse(obj, meta.ReadyCondition, "FooReason", "BarMsg")
			},
		},
		{
			name: "Stalled False",
			addConditions: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.StalledCondition, "FooReason", "BarMsg")
				conditions.MarkTrue(obj, "TestCondition1", "FooReason", "BarMsg")
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
			err := check_WARN0004(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func Test_check_WARN0005(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		wantErr    bool
	}{
		{
			name: "With ObservedGeneration",
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 4,
				},
				{
					Type:               "TestCondition1",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 4,
				},
			},
		},
		{
			name: "Some without ObservedGeneration",
			conditions: []metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 4,
				},
				{
					Type:   "TestCondition1",
					Status: metav1.ConditionTrue,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &testdata.Fake{}
			obj.SetConditions(tt.conditions)
			err := check_WARN0005(context.TODO(), obj, nil)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}
