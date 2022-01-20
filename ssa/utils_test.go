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

package ssa

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCmpMaskData(t *testing.T) {
	testCases := []struct {
		name     string
		current  map[string]interface{}
		future   map[string]interface{}
		expected string
	}{
		{
			name:     "empty",
			current:  map[string]interface{}{},
			future:   map[string]interface{}{},
			expected: "",
		},
		{
			name: "no change",
			current: map[string]interface{}{
				"foo": "bar",
			},
			future: map[string]interface{}{
				"foo": "bar",
			},
			expected: "",
		},
		{
			name: "simple value changed",
			current: map[string]interface{}{
				"foo": "bar",
			},
			future: map[string]interface{}{
				"foo": "baz",
			},
			expected: fmt.Sprintf("foo\": string(\"%s\")", defaultMask),
		},
		{
			name: "simple value changed with different casing",
			current: map[string]interface{}{
				"foo": "bar",
			},
			future: map[string]interface{}{
				"FOO": "baz",
			},
			expected: fmt.Sprintf("foo\": string(\"%s\")", defaultMask),
		},
		{
			name: "value changed with different casing and different values",
			current: map[string]interface{}{
				"foo": "bar",
				"baz": "qux",
			},
			future: map[string]interface{}{
				"foo": "baz",
				"baz": "qux",
			},
			expected: fmt.Sprintf("baz\": string(\"%s\")", defaultMask),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, ft := cmpMaskData(tc.current, tc.future)
			if diff := cmp.Diff(c, ft); !strings.Contains(diff, tc.expected) {
				t.Errorf("expected %s in %s", tc.expected, diff)
			}
		})
	}

}

func TestReadKubernetesObjects(t *testing.T) {
	testCases := []struct {
		name        string
		resources   string
		expectError bool
	}{
		{
			name: "valid resources",
			resources: `
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-aws1
spec:
  package: crossplane/provider-aws:v0.23.0
  controllerConfigRef:
    name: provider-aws
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-aws2
spec:
  package: crossplane/provider-aws:v0.23.0
  controllerConfigRef:
    name: provider-aws
`,
			expectError: false,
		},
		{
			name: "some invalid resources",
			resources: `
---
piVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-aws1
spec:
  package: crossplane/provider-aws:v0.23.0
  controllerConfigRef:
    name: provider-aws
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-aws2
spec:
  package: crossplane/provider-aws:v0.23.0
  controllerConfigRef:
    name: provider-aws
`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadKubernetesObjects(strings.NewReader(tc.resources))
			if err != nil && !tc.expectError {
				t.Errorf("unexpected error: %v", err)
			}

			if err != nil && tc.expectError {
				validObj, readErr := ReadObjects(strings.NewReader(tc.resources))
				if readErr != nil {
					t.Errorf("unexpected error: %v", readErr)
				}

				if len(validObj) != 1 {
					t.Errorf("unexpected objects: %v", validObj)
				}
			}
		})
	}
}
