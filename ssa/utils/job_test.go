/*
Copyright 2026 The Flux authors

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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestIsJob(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		want       bool
	}{
		{
			name:       "batch/v1 Job",
			apiVersion: "batch/v1",
			kind:       "Job",
			want:       true,
		},
		{
			name:       "batch/v1beta1 Job",
			apiVersion: "batch/v1beta1",
			kind:       "Job",
			want:       true,
		},
		{
			name:       "apps/v1 Deployment",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			want:       false,
		},
		{
			name:       "v1 ConfigMap",
			apiVersion: "v1",
			kind:       "ConfigMap",
			want:       false,
		},
		{
			name:       "batch/v1 CronJob",
			apiVersion: "batch/v1",
			kind:       "CronJob",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetAPIVersion(tt.apiVersion)
			obj.SetKind(tt.kind)

			if got := IsJob(obj); got != tt.want {
				t.Errorf("IsJob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractJobsWithTTL(t *testing.T) {
	tests := []struct {
		name    string
		objects []*unstructured.Unstructured
		want    int
	}{
		{
			name:    "empty list",
			objects: []*unstructured.Unstructured{},
			want:    0,
		},
		{
			name: "Job with ttlSecondsAfterFinished: 0",
			objects: []*unstructured.Unstructured{
				makeJob("test-job", "default", int64Ptr(0)),
			},
			want: 1,
		},
		{
			name: "Job with ttlSecondsAfterFinished: 300",
			objects: []*unstructured.Unstructured{
				makeJob("test-job", "default", int64Ptr(300)),
			},
			want: 1,
		},
		{
			name: "Job without ttlSecondsAfterFinished",
			objects: []*unstructured.Unstructured{
				makeJob("test-job", "default", nil),
			},
			want: 0,
		},
		{
			name: "non-Job resources",
			objects: []*unstructured.Unstructured{
				makeDeployment("test-deploy", "default"),
				makeConfigMap("test-cm", "default"),
			},
			want: 0,
		},
		{
			name: "mixed list of resources",
			objects: []*unstructured.Unstructured{
				makeJob("job-with-ttl", "default", int64Ptr(60)),
				makeJob("job-without-ttl", "default", nil),
				makeDeployment("test-deploy", "default"),
				makeJob("another-job-with-ttl", "other", int64Ptr(0)),
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJobsWithTTL(tt.objects)
			if len(result) != tt.want {
				t.Errorf("ExtractJobsWithTTL() returned %d items, want %d", len(result), tt.want)
			}
		})
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func makeJob(name, namespace string, ttlSecondsAfterFinished *int64) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("batch/v1")
	obj.SetKind("Job")
	obj.SetName(name)
	obj.SetNamespace(namespace)

	if ttlSecondsAfterFinished != nil {
		_ = unstructured.SetNestedField(obj.Object, *ttlSecondsAfterFinished, "spec", "ttlSecondsAfterFinished")
	}

	return obj
}

func makeDeployment(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

func makeConfigMap(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}
