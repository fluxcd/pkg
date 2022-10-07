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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// Conditions contain the list of status conditions supported by a controller
// for an object kind.
type Conditions struct {
	// NegativePolarity conditions are conditions that have abnormal-true
	// nature.
	NegativePolarity []string `json:"negativePolarity"`
	// PositivePolarity conditions are conditions that have normal-true nature.
	// (Optional)
	PositivePolarity []string `json:"positivePolarity"`
}

// ParseConditions parses a given byte slice input into a Conditions object.
func ParseConditions(input []byte) (*Conditions, error) {
	var c *Conditions
	err := yaml.Unmarshal(input, &c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// HighestNegativePriorityCondition returns the negative priority condition
// supported by the controller which has the highest priority among the
// provided conditions.
func HighestNegativePriorityCondition(conditions *Conditions, condns []metav1.Condition) (*metav1.Condition, error) {
	if conditions == nil || len(conditions.NegativePolarity) == 0 {
		return nil, fmt.Errorf("no negative polarity conditions defined in the checker, can't prioritize")
	}

	// Iterate through the negative polarity conditions in order to find the
	// condition with the highest priority in the given set of conditions.
	for _, nc := range conditions.NegativePolarity {
		for _, c := range condns {
			if nc == c.Type {
				return &c, nil
			}
		}
	}
	return nil, nil
}

// getStatusObservedGeneration returns the status.observedGeneration of an
// object.
func getStatusObservedGeneration(obj client.Object) (int64, error) {
	u, err := toUnstructured(obj)
	if err != nil {
		return 0, err
	}
	observedGen, _, err := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	if err != nil {
		return 0, err
	}
	return observedGen, nil
}

// isNegativePolarityCondition determines if a given condition has negative
// polarity based on the given negative polarity context.
func isNegativePolarityCondition(context []string, condn metav1.Condition) bool {
	for _, c := range context {
		if c == condn.Type {
			return true
		}
	}
	return false
}

// toUnstructured converts a runtime object into Unstructured.
func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	// If the incoming object is already unstructured, perform a deep copy first
	// otherwise DefaultUnstructuredConverter ends up returning the inner map
	// without making a copy.
	if _, ok := obj.(runtime.Unstructured); ok {
		obj = obj.DeepCopyObject()
	}
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: rawMap}, nil
}
