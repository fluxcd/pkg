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

func TestSelectorRegex_MatchGVK(t *testing.T) {
	tests := []struct {
		name     string
		selector *Selector
		group    string
		version  string
		kind     string
		want     bool
	}{
		{
			name: "valid match",
			selector: &Selector{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			group:   "apps",
			version: "v1",
			kind:    "Deployment",
			want:    true,
		},
		{
			name: "valid match with regex",
			selector: &Selector{
				Group:   "apps",
				Version: "v.*",
				Kind:    "Deployment",
			},
			group:   "apps",
			version: "v1",
			kind:    "Deployment",
			want:    true,
		},
		{
			name:     "valid match without regex",
			selector: &Selector{},
			group:    "apps",
			version:  "v1",
			kind:     "Deployment",
			want:     true,
		},
		{
			name: "invalid group",
			selector: &Selector{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			group:   "extensions",
			version: "v1",
			kind:    "Deployment",
			want:    false,
		},
		{
			name: "invalid version",
			selector: &Selector{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			group:   "apps",
			version: "v2",
			kind:    "Deployment",
			want:    false,
		},
		{
			name: "invalid kind",
			selector: &Selector{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			group:   "apps",
			version: "v1",
			kind:    "StatefulSet",
			want:    false,
		},
		{
			name: "partial selector",
			selector: &Selector{
				Group: "apps",
				Kind:  "Deployment",
			},
			group:   "apps",
			version: "v2",
			kind:    "Deployment",
			want:    true,
		},
		{
			name:     "empty selector",
			selector: &Selector{},
			want:     true,
		},
		{
			name:     "nil selector",
			selector: nil,
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

			if got := s.MatchGVK(tt.group, tt.version, tt.kind); got != tt.want {
				t.Errorf("MatchGVK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectorRegex_MatchName(t *testing.T) {
	tests := []struct {
		name     string
		selector *Selector
		n        string
		want     bool
	}{
		{
			name: "valid match",
			selector: &Selector{
				Name: "name-1",
			},
			n:    "name-1",
			want: true,
		},
		{
			name: "valid match with regex",
			selector: &Selector{
				Name: "name-.*",
			},
			n:    "name-2",
			want: true,
		},
		{
			name: "invalid name",
			selector: &Selector{
				Name: "name-.*",
			},
			n:    "other-name-1",
			want: false,
		},
		{
			name:     "empty selector",
			selector: &Selector{},
			n:        "any-name",
			want:     true,
		},
		{
			name:     "nil selector",
			selector: nil,
			n:        "any-name",
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

			if got := s.MatchName(tt.n); got != tt.want {
				t.Errorf("MatchName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectorRegex_MatchNamespace(t *testing.T) {
	tests := []struct {
		name     string
		selector *Selector
		ns       string
		want     bool
	}{
		{
			name: "valid match",
			selector: &Selector{
				Namespace: "ns-1",
			},
			ns:   "ns-1",
			want: true,
		},
		{
			name: "valid match with regex",
			selector: &Selector{
				Namespace: "ns-.*",
			},
			ns:   "ns-2",
			want: true,
		},
		{
			name: "invalid namespace",
			selector: &Selector{
				Namespace: "ns-.*",
			},
			ns:   "other-ns-1",
			want: false,
		},
		{
			name:     "empty selector",
			selector: &Selector{},
			ns:       "any-ns",
			want:     true,
		},
		{
			name:     "nil selector",
			selector: nil,
			ns:       "any-ns",
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

			if got := s.MatchNamespace(tt.ns); got != tt.want {
				t.Errorf("MatchNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectorRegex_MatchAnnotationSelector(t *testing.T) {
	tests := []struct {
		name        string
		selector    *Selector
		annotations map[string]string
		want        bool
	}{
		{
			name: "valid match",
			selector: &Selector{
				AnnotationSelector: "key-1=value-1",
			},
			annotations: map[string]string{
				"key-1": "value-1",
				"key-2": "value-2",
			},
			want: true,
		},
		{
			name: "invalid annotation",
			selector: &Selector{
				AnnotationSelector: "key-1 in(value-1)",
			},
			annotations: map[string]string{
				"key-1": "value-2",
			},
			want: false,
		},
		{
			name:     "empty selector",
			selector: &Selector{},
			annotations: map[string]string{
				"key-1": "value-1",
			},
			want: true,
		},
		{
			name:     "nil selector",
			selector: nil,
			annotations: map[string]string{
				"key-2": "value-2",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSelectorRegex(tt.selector)
			if err != nil {
				t.Errorf("NewSelectorRegex() error = %v", err)
				return
			}

			if got := s.MatchAnnotationSelector(tt.annotations); got != tt.want {
				t.Errorf("MatchAnnotationSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectorRegex_MatchLabelSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector *Selector
		labels   map[string]string
		want     bool
	}{
		{
			name: "valid match",
			selector: &Selector{
				LabelSelector: "key-1=value-1",
			},
			labels: map[string]string{
				"key-1": "value-1",
				"key-2": "value-2",
			},
			want: true,
		},
		{
			name: "invalid label",
			selector: &Selector{
				LabelSelector: "key-1 in(value-1)",
			},
			labels: map[string]string{
				"key-1": "value-2",
			},
			want: false,
		},
		{
			name:     "empty selector",
			selector: &Selector{},
			labels: map[string]string{
				"key-1": "value-1",
			},
			want: true,
		},
		{
			name:     "nil selector",
			selector: nil,
			labels: map[string]string{
				"key-2": "value-2",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSelectorRegex(tt.selector)
			if err != nil {
				t.Errorf("NewSelectorRegex() error = %v", err)
				return
			}

			if got := s.MatchLabelSelector(tt.labels); got != tt.want {
				t.Errorf("MatchLabelSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}
