/*
Copyright 2024 The Flux authors

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

package chartutil

import (
	"bytes"
	"testing"

	goyaml "go.yaml.in/yaml/v2"
	"sigs.k8s.io/yaml"
)

func TestSortMapSlice(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  map[string]interface{}
	}{
		{
			name:  "empty map",
			input: map[string]interface{}{},
			want:  map[string]interface{}{},
		},
		{
			name: "flat map",
			input: map[string]interface{}{
				"b": "value-b",
				"a": "value-a",
				"c": "value-c",
			},
			want: map[string]interface{}{
				"a": "value-a",
				"b": "value-b",
				"c": "value-c",
			},
		},
		{
			name: "nested map",
			input: map[string]interface{}{
				"b": "value-b",
				"a": "value-a",
				"c": map[string]interface{}{
					"z": "value-z",
					"y": "value-y",
				},
			},
			want: map[string]interface{}{
				"a": "value-a",
				"b": "value-b",
				"c": map[string]interface{}{
					"y": "value-y",
					"z": "value-z",
				},
			},
		},
		{
			name: "map with slices",
			input: map[string]interface{}{
				"b": []interface{}{"apple", "banana", "cherry"},
				"a": []interface{}{"orange", "grape"},
				"c": []interface{}{"strawberry"},
			},
			want: map[string]interface{}{
				"a": []interface{}{"orange", "grape"},
				"b": []interface{}{"apple", "banana", "cherry"},
				"c": []interface{}{"strawberry"},
			},
		},
		{
			name: "map with mixed data types",
			input: map[string]interface{}{
				"b": 50,
				"a": "value-a",
				"c": []interface{}{"strawberry", "banana"},
				"d": map[string]interface{}{
					"x": true,
					"y": 123,
				},
			},
			want: map[string]interface{}{
				"a": "value-a",
				"b": 50,
				"c": []interface{}{"strawberry", "banana"},
				"d": map[string]interface{}{
					"x": true,
					"y": 123,
				},
			},
		},
		{
			name: "map with complex structure",
			input: map[string]interface{}{
				"a": map[string]interface{}{
					"c": "value-c",
					"b": "value-b",
					"a": "value-a",
				},
				"b": "value-b",
				"c": map[string]interface{}{
					"z": map[string]interface{}{
						"a": "value-a",
						"b": "value-b",
						"c": "value-c",
					},
					"y": "value-y",
				},
				"d": map[string]interface{}{
					"q": "value-q",
					"p": "value-p",
					"r": "value-r",
				},
				"e": []interface{}{"strawberry", "banana"},
				"f": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"f1": map[string]interface{}{
								"f1q": "value-f1q",
								"f1p": "value-f1p",
								"f1r": "value-f1r",
							},
						},
						map[string]interface{}{
							"f2": map[string]interface{}{
								"f2q": "value-f2q",
								"f2p": "value-f2p",
								"f2r": "value-f2r",
							},
						},
						map[string]interface{}{
							"f3": map[string]interface{}{
								"f3q": "value-f3q",
								"f3p": "value-f3p",
								"f3r": "value-f3r",
							},
						},
					},
					[]interface{}{
						map[string]interface{}{
							"F1": map[string]interface{}{
								"F1q": "value-F1q",
								"F1p": "value-F1p",
								"F1r": "value-F1r",
							},
						},
						map[string]interface{}{
							"F2": map[string]interface{}{
								"F2q": "value-F2q",
								"F2p": "value-F2p",
								"F2r": "value-F2r",
							},
						},
						map[string]interface{}{
							"F3": map[string]interface{}{
								"F3q": "value-F3q",
								"F3p": "value-F3p",
								"F3r": "value-F3r",
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"a": "value-a",
					"b": "value-b",
					"c": "value-c",
				},
				"b": "value-b",
				"c": map[string]interface{}{
					"y": "value-y",
					"z": map[string]interface{}{
						"a": "value-a",
						"b": "value-b",
						"c": "value-c",
					},
				},
				"d": map[string]interface{}{
					"p": "value-p",
					"q": "value-q",
					"r": "value-r",
				},
				"e": []interface{}{"strawberry", "banana"},
				"f": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"f1": map[string]interface{}{
								"f1p": "value-f1p",
								"f1q": "value-f1q",
								"f1r": "value-f1r",
							},
						},
						map[string]interface{}{
							"f2": map[string]interface{}{
								"f2p": "value-f2p",
								"f2q": "value-f2q",
								"f2r": "value-f2r",
							},
						},
						map[string]interface{}{
							"f3": map[string]interface{}{
								"f3p": "value-f3p",
								"f3q": "value-f3q",
								"f3r": "value-f3r",
							},
						},
					},
					[]interface{}{
						map[string]interface{}{
							"F1": map[string]interface{}{
								"F1p": "value-F1p",
								"F1q": "value-F1q",
								"F1r": "value-F1r",
							},
						},
						map[string]interface{}{
							"F2": map[string]interface{}{
								"F2p": "value-F2p",
								"F2q": "value-F2q",
								"F2r": "value-F2r",
							},
						},
						map[string]interface{}{
							"F3": map[string]interface{}{
								"F3p": "value-F3p",
								"F3q": "value-F3q",
								"F3r": "value-F3r",
							},
						},
					},
				},
			},
		},
		{
			name: "map with empty slices and maps",
			input: map[string]interface{}{
				"b": []interface{}{},
				"a": map[string]interface{}{},
			},
			want: map[string]interface{}{
				"a": map[string]interface{}{},
				"b": []interface{}{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := yaml.JSONObjectToYAMLObject(tt.input)
			SortMapSlice(input)

			expect, err := goyaml.Marshal(input)
			if err != nil {
				t.Fatalf("error marshalling output: %v", err)
			}
			actual, err := goyaml.Marshal(tt.want)
			if err != nil {
				t.Fatalf("error marshalling want: %v", err)
			}

			if !bytes.Equal(expect, actual) {
				t.Errorf("SortMapSlice() = %s, want %s", expect, actual)
			}
		})
	}
}
