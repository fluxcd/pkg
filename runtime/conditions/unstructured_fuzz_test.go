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
*/

package conditions

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
