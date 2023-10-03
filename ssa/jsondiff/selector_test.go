/*
Copyright 2023 The Flux authors

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

package jsondiff

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSelectorRegex_MatchUnstructured(t *testing.T) {
	tests := []struct {
		name     string
		selector *Selector
		u        *unstructured.Unstructured
		want     bool
	}{
		{
			name: "valid input",
			selector: &Selector{
				Group:              "apps",
				Version:            "v1",
				Kind:               "Deployment",
				Name:               "name-.*",
				Namespace:          "namespace-.*",
				LabelSelector:      "foo.bar/label in (a, b)",
				AnnotationSelector: "foo.bar/annotation notin (c)",
			},
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "name-1",
						"namespace": "namespace-1",
						"labels": map[string]interface{}{
							"foo.bar/label": "a",
						},
						"annotations": map[string]interface{}{
							"foo.bar/annotation": "d",
						},
					},
				},
			},
			want: true,
		},
		{
			name:     "nil selector",
			selector: nil,
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "anything",
						"namespace": "anything",
					},
				},
			},
			want: true,
		},
		{
			name: "mismatched namespace",
			selector: &Selector{
				Namespace: "exact-namespace",
			},
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "other-namespace",
					},
				},
			},
			want: false,
		},
		{
			name: "mismatched name",
			selector: &Selector{
				Name: "exact-name",
			},
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "other-name",
					},
				},
			},
			want: false,
		},
		{
			name: "mismatched GVK",
			selector: &Selector{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
				},
			},
			want: false,
		},
		{
			name: "mismatched label",
			selector: &Selector{
				LabelSelector: "foo.bar/label in (a, b)",
			},
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo.bar/label": "c",
						},
					},
				},
			},
		},
		{
			name: "mismatched annotation",
			selector: &Selector{
				AnnotationSelector: "foo.bar/annotation notin (c)",
			},
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"foo.bar/annotation": "c",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "combination of mismatches",
			selector: &Selector{
				Group:              "apps",
				Version:            "v1",
				Kind:               "Deployment",
				Name:               "name-.*",
				Namespace:          "namespace-.*",
				LabelSelector:      "foo.bar/label in (a, b)",
				AnnotationSelector: "foo.bar/annotation notin (c)",
			},
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "other-name-1",
						"namespace": "other-namespace-1",
						"labels": map[string]interface{}{
							"foo.bar/label": "c",
						},
						"annotations": map[string]interface{}{
							"foo.bar/annotation": "c",
						},
					},
				},
			},
			want: false,
		},
		{
			name:     "empty input object",
			selector: &Selector{},
			u:        &unstructured.Unstructured{},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSelectorRegex(tt.selector)
			if err != nil {
				t.Errorf("NewSelectorRegex() error = %v", err)
				return
			}

			if got := s.MatchUnstructured(tt.u); got != tt.want {
				t.Errorf("MatchUnstructured(%v) = %v, want %v", tt.u.Object, got, tt.want)
			}
		})
	}
}
