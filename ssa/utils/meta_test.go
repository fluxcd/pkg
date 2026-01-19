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

package utils

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

func TestParseGroupKindSet(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected map[schema.GroupKind]struct{}
	}{
		{
			name:     "empty string returns empty set",
			input:    "",
			expected: map[schema.GroupKind]struct{}{},
		},
		{
			name:  "single kind without group (core API)",
			input: "Secret",
			expected: map[schema.GroupKind]struct{}{
				{Group: "", Kind: "Secret"}: {},
			},
		},
		{
			name:  "single kind with group",
			input: "apps/Deployment",
			expected: map[schema.GroupKind]struct{}{
				{Group: "apps", Kind: "Deployment"}: {},
			},
		},
		{
			name:  "multiple kinds with groups",
			input: "apps/Deployment,batch/Job",
			expected: map[schema.GroupKind]struct{}{
				{Group: "apps", Kind: "Deployment"}: {},
				{Group: "batch", Kind: "Job"}:       {},
			},
		},
		{
			name:  "mixed kinds with and without groups",
			input: "Secret,apps/Deployment,ConfigMap",
			expected: map[schema.GroupKind]struct{}{
				{Group: "", Kind: "Secret"}:         {},
				{Group: "apps", Kind: "Deployment"}: {},
				{Group: "", Kind: "ConfigMap"}:      {},
			},
		},
		{
			name:  "handles whitespace around items",
			input: " Secret , apps/Deployment ",
			expected: map[schema.GroupKind]struct{}{
				{Group: "", Kind: "Secret"}:         {},
				{Group: "apps", Kind: "Deployment"}: {},
			},
		},
		{
			name:  "handles kind with multi-part group",
			input: "gateway.networking.k8s.io/Gateway",
			expected: map[schema.GroupKind]struct{}{
				{Group: "gateway.networking.k8s.io", Kind: "Gateway"}: {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := ParseGroupKindSet(tc.input)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}
