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

package ssa

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSanitizeUnstructuredData(t *testing.T) {
	type diff struct {
		old *unstructured.Unstructured
		new *unstructured.Unstructured
	}

	tests := []struct {
		name    string
		input   diff
		want    diff
		wantErr bool
	}{
		{
			name: "sanitizes data",
			input: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    "abc",
							"password": "123",
						},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    "abc",
							"password": "123",
						},
					},
				},
			},
			want: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    sanitizeMaskDefault,
							"password": sanitizeMaskDefault,
						},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    sanitizeMaskDefault,
							"password": sanitizeMaskDefault,
						},
					},
				},
			},
		},
		{
			name: "sanitizes data with different values",
			input: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    "abc",
							"password": "123",
						},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    "def",
							"password": "456",
						},
					},
				},
			},
			want: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    sanitizeMaskBefore,
							"password": sanitizeMaskBefore,
						},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    sanitizeMaskAfter,
							"password": sanitizeMaskAfter,
						},
					},
				},
			},
		},
		{
			name: "sanitizes data with different keys",
			input: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    "abc",
							"password": "123",
						},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"username": "hello",
							"token":    "abc",
						},
					},
				},
			},
			want: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token":    sanitizeMaskDefault,
							"password": sanitizeMaskDefault,
						},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"username": sanitizeMaskDefault,
							"token":    sanitizeMaskDefault,
						},
					},
				},
			},
		},
		{
			name: "sanitizes new object without old object",
			input: diff{
				old: nil,
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token": "abc",
						},
					},
				},
			},
			want: diff{
				old: nil,
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token": sanitizeMaskDefault,
						},
					},
				},
			},
		},
		{
			name: "sanitizes old object without new object",
			input: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token": "abc",
						},
					},
				},
				new: nil,
			},
			want: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"token": sanitizeMaskDefault,
						},
					},
				},
			},
		},
		{
			name: "sanitizes empty objecct",
			input: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{},
					},
				},
			},
			want: diff{
				old: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{},
					},
				},
				new: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"data": map[string]interface{}{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SanitizeUnstructuredData(tt.input.old, tt.input.new); (err != nil) != tt.wantErr {
				t.Errorf("SanitizeUnstructuredData() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.input.old != nil && tt.want.old != nil {
				if diff := cmp.Diff(tt.input.old, tt.want.old); diff != "" {
					t.Errorf("SanitizeUnstructuredData() old mismatch (-want +got):\n%s", diff)
				}
			}

			if tt.input.new != nil && tt.want.new != nil {
				if diff := cmp.Diff(tt.input.new, tt.want.new); diff != "" {
					t.Errorf("SanitizeUnstructuredData() new mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
