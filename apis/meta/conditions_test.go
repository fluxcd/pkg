/*
Copyright 2020 The Flux authors

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

package meta

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInReadyCondition(t *testing.T) {
	var conditions []metav1.Condition

	found := InReadyCondition(conditions)
	if found {
		t.Error("expected InReadyCondition to return false when no conditions")
	}

	fake := metav1.Condition{
		Type: "FakeCondition",
	}
	conditions = append(conditions, fake)
	found = InReadyCondition(conditions)
	if found {
		t.Error("expected InReadyCondition to return false when no ReadyCondition exists")
	}

	ready := metav1.Condition{
		Type:   ReadyCondition,
		Status: metav1.ConditionFalse,
	}
	conditions = append(conditions, ready)
	found = InReadyCondition(conditions)
	if found {
		t.Error("expected InReadyCondition to return false if ReadyCondition Status is false")
	}

	ready.Status = metav1.ConditionTrue
	conditions = []metav1.Condition{ready}
	found = InReadyCondition(conditions)
	if !found {
		t.Error("expected InReadyCondition to return true")
	}
}

func TestFilterOutCondition(t *testing.T) {
	const FakeCondition = "FakeCondition"
	var conditions []metav1.Condition

	filtered := FilterOutCondition(conditions, "")
	if len(filtered) > 0 {
		t.Error("expected FilterOutCondition to return empty slice")
	}

	fake := metav1.Condition{
		Type: FakeCondition,
	}
	conditions = append(conditions, fake)
	filtered = FilterOutCondition(conditions, FakeCondition)
	if len(filtered) > 0 {
		t.Error("expected FilterOutCondition to return empty slice")
	}

	ready := metav1.Condition{
		Type: ReadyCondition,
	}
	conditions = append(conditions, ready)
	filtered = FilterOutCondition(conditions, FakeCondition)
	if filtered[0] != conditions[1] {
		t.Error("expected FilterOutCondition to return the ready condition")
	}

	test := metav1.Condition{
		Type: "TestCondition",
	}
	conditions = append(conditions, test)
	filtered = FilterOutCondition(conditions, FakeCondition)
	if len(filtered) != 2 {
		t.Error("expected FilterOutCondition to have two elements")
	}
}
