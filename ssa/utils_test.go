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
	. "github.com/onsi/gomega"
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

func TestReadObjects_DropsInvalid(t *testing.T) {
	testCases := []struct {
		name      string
		resources string
		expected  int
	}{
		{
			name: "valid resources",
			resources: `
---
apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: default
immutable: true
stringData:
  key: "private-key"
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
			expected: 2,
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
apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: default
immutable: true
stringData:
  key: "private-key"
`,
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objects, err := ReadObjects(strings.NewReader(tc.resources))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(objects) != tc.expected {
				t.Errorf("unexpected number of objects in %v", objects)
			}
		})
	}
}

func TestSetCommonMetadata(t *testing.T) {
	testCases := []struct {
		name        string
		resources   string
		labels      map[string]string
		annotations map[string]string
	}{
		{
			name: "adds metadata",
			resources: `
---
apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: default
stringData:
  key: "private-key"
`,
			labels: map[string]string{
				"test1": "lb1",
				"test2": "lb2",
			},
			annotations: map[string]string{
				"test1": "a1",
				"test2": "a2",
			},
		},
		{
			name: "overrides metadata",
			resources: `
---
apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: default
  labels:
    test1: over
  annotations:
    test2: over
stringData:
  key: "private-key"
`,
			labels: map[string]string{
				"test1": "lb1",
			},
			annotations: map[string]string{
				"test1": "an1",
				"test2": "an2",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			objects, err := ReadObjects(strings.NewReader(tc.resources))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			SetCommonMetadata(objects, tc.labels, tc.annotations)

			for _, object := range objects {
				for k, v := range tc.labels {
					g.Expect(object.GetLabels()).To(HaveKeyWithValue(k, v))
				}
				for k, v := range tc.annotations {
					g.Expect(object.GetAnnotations()).To(HaveKeyWithValue(k, v))
				}
			}
		})
	}
}

func TestIsImmutableError(t *testing.T) {
	testCases := []struct {
		name  string
		err   error
		match bool
	}{
		{
			name:  "CEL immutable error",
			err:   fmt.Errorf(`the ImmutableSinceFirstWrite "test1" is invalid: value: Invalid value: "string": Value is immutable`),
			match: true,
		},
		{
			name:  "Custom admission immutable error",
			err:   fmt.Errorf(`the IAMPolicyMember's spec is immutable: admission webhook "deny-immutable-field-updates.cnrm.cloud.google.com" denied the request: the IAMPolicyMember's spec is immutable`),
			match: true,
		},
		{
			name:  "Not immutable error",
			err:   fmt.Errorf(`is not immutable`),
			match: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(IsImmutableError(tc.err)).To(BeIdenticalTo(tc.match))
		})
	}
}
