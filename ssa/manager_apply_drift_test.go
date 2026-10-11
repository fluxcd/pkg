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

package ssa

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestApplyDriftResultSkipsUndeclaredPath(t *testing.T) {
	target := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name": "app",
			},
			"spec": map[string]any{
				"minReadySeconds": int64(1),
			},
		},
	}

	err := applyDriftResult(target, driftResult{
		entries: []driftEntry{{
			path:   "/spec/replicas",
			action: driftAdopt,
			value:  int64(2),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := unstructured.NestedFieldNoCopy(target.Object, "spec", "replicas"); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("replicas was added")
	}

	err = applyDriftResult(target, driftResult{
		entries: []driftEntry{{
			path:   "/spec/minReadySeconds",
			action: driftAdopt,
			value:  int64(7),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := unstructured.NestedFieldNoCopy(target.Object, "spec", "minReadySeconds")
	if err != nil || !found {
		t.Fatalf("minReadySeconds found=%v err=%v", found, err)
	}
	if fmt.Sprint(got) != "7" {
		t.Fatalf("minReadySeconds = %v (%T), want 7", got, got)
	}
}
