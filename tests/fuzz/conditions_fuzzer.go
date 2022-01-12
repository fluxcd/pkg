//go:build gofuzz
// +build gofuzz

/*
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
*/
package conditions

import (
	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FuzzGetterConditions implements a fuzzer that targets
// conditions.SetSummary().
func FuzzGetterConditions(data []byte) int {
	f := fuzz.NewConsumer(data)

	// Create slice of metav1.Condition
	noOfConditions, err := f.GetInt()
	if err != nil {
		return 0
	}
	maxNoOfConditions := 30
	conditions := make([]metav1.Condition, 0)

	// Add Conditions in the slice
	for i := 0; i < noOfConditions%maxNoOfConditions; i++ {
		c := metav1.Condition{}
		err = f.GenerateStruct(&c)
		if err != nil {
			return 0
		}
		conditions = append(conditions, c)
	}
	obj := &testdata.Fake{}
	obj.SetConditions(conditions)

	targetCondition, err := f.GetString()
	if err != nil {
		return 0
	}

	// Call the target
	SetSummary(obj, targetCondition)
	return 1
}

// FuzzConditionsMatch implements a fuzzer that that targets
// MatchCondition.Match().
func FuzzConditionsMatch(data []byte) int {
	f := fuzz.NewConsumer(data)
	condition := metav1.Condition{}
	err := f.GenerateStruct(&condition)
	if err != nil {
		return 0
	}
	m := MatchCondition(condition)

	actual := metav1.Condition{}
	err = f.GenerateStruct(&actual)
	if err != nil {
		return 0
	}

	// Call the target
	_, _ = m.Match(actual)
	return 1
}

// newGetter allows the fuzzer to create a Getter.
func newGetter(f *fuzz.ConsumeFuzzer) (Getter, error) {
	obj := &testdata.Fake{}
	noOfConditions, err := f.GetInt()
	if err != nil {
		return obj, err
	}
	maxNoOfConditions := 30
	conditions := make([]metav1.Condition, 0)
	for i := 0; i < noOfConditions%maxNoOfConditions; i++ {
		c := metav1.Condition{}
		err = f.GenerateStruct(&c)
		if err != nil {
			return obj, err
		}
		conditions = append(conditions, c)
	}

	obj.SetConditions(conditions)
	return obj, nil
}

// newSetter allows the fuzzer to create a Setter.
func newSetter(f *fuzz.ConsumeFuzzer) (Setter, error) {
	obj := &testdata.Fake{}
	noOfConditions, err := f.GetInt()
	if err != nil {
		return obj, err
	}
	maxNoOfConditions := 30
	conditions := make([]metav1.Condition, 0)
	for i := 0; i < noOfConditions%maxNoOfConditions; i++ {
		c := metav1.Condition{}
		err = f.GenerateStruct(&c)
		if err != nil {
			return obj, err
		}
		conditions = append(conditions, c)
	}
	obj.SetConditions(conditions)
	return obj, nil
}

// FuzzPatchApply implements a fuzzer that targets Patch.Apply.
func FuzzPatchApply(data []byte) int {
	f := fuzz.NewConsumer(data)

	before, err := newGetter(f)
	if err != nil {
		return 0
	}
	after, err := newGetter(f)
	if err != nil {
		return 0
	}
	patch := NewPatch(before, after)

	setter, err := newSetter(f)
	if err != nil {
		return 0
	}
	_ = patch.Apply(setter)
	return 1
}

// FuzzConditionsUnstructured implements a fuzzer that targets
// Getter.GetConditions.
func FuzzConditionsUnstructured(data []byte) int {
	u := &unstructured.Unstructured{}
	f := fuzz.NewConsumer(data)
	err := f.GenerateStruct(u)
	if err != nil {
		return 0
	}
	g := UnstructuredGetter(u)
	_ = g.GetConditions()
	return 1
}
