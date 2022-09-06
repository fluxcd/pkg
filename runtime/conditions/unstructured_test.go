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
https://github.com/kubernetes-sigs/cluster-api/tree/7478817225e0a75acb6e14fc7b438231578073d2/util/conditions/unstructured_test.go,
and initially adapted to work with the `metav1.Condition` and `metav1.ConditionStatus` types.
More concretely, this includes the removal of "condition severity" related functionalities, as this is not supported by
the `metav1.Condition` type.
*/

package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func TestUnstructuredGetConditions(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(testdata.AddFakeToScheme(scheme)).To(Succeed())

	// GetConditions should return conditions from an unstructured object
	c := &testdata.Fake{}
	c.SetConditions(conditionList(true1))
	u := &unstructured.Unstructured{}
	g.Expect(scheme.Convert(c, u, nil)).To(Succeed())

	g.Expect(UnstructuredGetter(u).GetConditions()).To(haveSameConditionsOf(conditionList(true1)))

	// GetConditions should return nil for an unstructured object with empty conditions
	c = &testdata.Fake{}
	u = &unstructured.Unstructured{}
	g.Expect(scheme.Convert(c, u, nil)).To(Succeed())

	g.Expect(UnstructuredGetter(u).GetConditions()).To(BeNil())

	// GetConditions should return nil for an unstructured object without conditions
	e := &corev1.Endpoints{}
	u = &unstructured.Unstructured{}
	g.Expect(scheme.Convert(e, u, nil)).To(Succeed())

	g.Expect(UnstructuredGetter(u).GetConditions()).To(BeNil())

	// GetConditions should return conditions from an unstructured object with a different type of conditions.
	p := &corev1.Pod{Status: corev1.PodStatus{
		Conditions: []corev1.PodCondition{
			{
				Type:               "foo",
				Status:             "foo",
				LastProbeTime:      metav1.Time{},
				LastTransitionTime: metav1.Time{},
				Reason:             "foo",
				Message:            "foo",
			},
		},
	}}
	u = &unstructured.Unstructured{}
	g.Expect(scheme.Convert(p, u, nil)).To(Succeed())

	g.Expect(UnstructuredGetter(u).GetConditions()).To(HaveLen(1))
}

func TestUnstructuredSetConditions(t *testing.T) {
	g := NewWithT(t)

	// gets an unstructured with empty conditions
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(testdata.AddFakeToScheme(scheme)).To(Succeed())

	c := &testdata.Fake{}
	u := &unstructured.Unstructured{}
	g.Expect(scheme.Convert(c, u, nil)).To(Succeed())

	// set conditions
	conditions := conditionList(true1, false1)

	s := UnstructuredSetter(u)
	s.SetConditions(conditions)
	g.Expect(s.GetConditions()).To(Equal(conditions))
}

func Fuzz_Unstructured(f *testing.F) {
	f.Add("type", "reason true", "condition message")

	f.Fuzz(func(t *testing.T,
		ct, reason, message string) {

		cs := []metav1.Condition{{
			Type:    ct,
			Status:  metav1.ConditionUnknown,
			Reason:  reason,
			Message: message,
		}}

		u := &unstructured.Unstructured{
			Object: map[string]interface{}{},
		}
		s := UnstructuredSetter(u)
		s.SetConditions(cs)

		g := UnstructuredGetter(u)
		_ = g.GetConditions()
	})
}
