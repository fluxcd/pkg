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
	"reflect"
	"testing"

	"github.com/wI2L/jsondiff"
)

func TestMaskSecretPatchData(t *testing.T) {
	tests := []struct {
		name  string
		patch jsondiff.Patch
		want  jsondiff.Patch
	}{
		{
			name: "masks replace data values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/data/foo", OldValue: "bar", Value: "baz"},
				{Type: jsondiff.OperationReplace, Path: "/data/bar", OldValue: "foo", Value: "baz"},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/data/foo", OldValue: sensitiveMaskBefore, Value: sensitiveMaskAfter},
				{Type: jsondiff.OperationReplace, Path: "/data/bar", OldValue: sensitiveMaskBefore, Value: sensitiveMaskAfter},
			},
		},
		{
			name: "masks add data values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/data/foo", Value: "baz"},
				{Type: jsondiff.OperationAdd, Path: "/data/bar", Value: "baz"},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/data/foo", Value: sensitiveMaskDefault},
				{Type: jsondiff.OperationAdd, Path: "/data/bar", Value: sensitiveMaskDefault},
			},
		},
		{
			name: "masks remove data values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationRemove, Path: "/data/foo", OldValue: "bar"},
				{Type: jsondiff.OperationRemove, Path: "/data/bar", OldValue: "foo"},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationRemove, Path: "/data/foo", OldValue: sensitiveMaskDefault},
				{Type: jsondiff.OperationRemove, Path: "/data/bar", OldValue: sensitiveMaskDefault},
			},
		},
		{
			name: "masks rationalized replace data values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/data", OldValue: map[string]interface{}{
					"foo": "bar",
					"bar": "foo",
				}, Value: map[string]interface{}{
					"foo": "baz",
					"bar": "baz",
				}},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/data", OldValue: map[string]interface{}{
					"foo": sensitiveMaskBefore,
					"bar": sensitiveMaskBefore,
				}, Value: map[string]interface{}{
					"foo": sensitiveMaskAfter,
					"bar": sensitiveMaskAfter,
				},
				}},
		},
		{
			name: "masks rationalized add data values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/data", Value: map[string]interface{}{
					"foo": "baz",
					"bar": "baz",
				}},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/data", Value: map[string]interface{}{
					"foo": sensitiveMaskDefault,
					"bar": sensitiveMaskDefault,
				}},
			},
		},
		{
			name: "masks rationalized remove data values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationRemove, Path: "/data", OldValue: map[string]interface{}{
					"foo": "bar",
					"bar": "foo",
				}},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationRemove, Path: "/data", OldValue: map[string]interface{}{
					"foo": sensitiveMaskDefault,
					"bar": sensitiveMaskDefault,
				}},
			},
		},
		{
			name: "masks rationalized replace complex data values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/data", OldValue: map[string]interface{}{
					// Changed key
					"foo": "bar",
					// Removed key
					"bar": "baz",
				}, Value: map[string]interface{}{
					"foo": "baz",
					// Added key
					"baz": "bar",
				}},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/data", OldValue: map[string]interface{}{
					"foo": sensitiveMaskBefore,
					"bar": sensitiveMaskDefault,
				}, Value: map[string]interface{}{
					"foo": sensitiveMaskAfter,
					"baz": sensitiveMaskDefault,
				}},
			},
		},
		{
			name: "masks replace stringData values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/stringData/foo", OldValue: "bar", Value: "baz"},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/stringData/foo", OldValue: sensitiveMaskBefore, Value: sensitiveMaskAfter},
			},
		},
		{
			name: "masks add stringData values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/stringData/foo", Value: "baz"},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/stringData/foo", Value: sensitiveMaskDefault},
			},
		},
		{
			name: "masks remove stringData values",
			patch: jsondiff.Patch{
				{Type: jsondiff.OperationRemove, Path: "/stringData/foo", OldValue: "bar"},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationRemove, Path: "/stringData/foo", OldValue: sensitiveMaskDefault},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskSecretPatchData(tt.patch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("maskUnstructuredSecretData() = %v, want %v", got, tt.want)
			}
		})
	}
}
