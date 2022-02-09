/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestDiff(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	configMapName, configMap := getFirstObject(objects, "ConfigMap", id)
	secretName, secret := getFirstObject(objects, "Secret", id)

	if err := unstructured.SetNestedField(secret.Object, false, "immutable"); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	t.Run("generates empty diff for unchanged object", func(t *testing.T) {
		changeSetEntry, _, _, err := manager.Diff(ctx, configMap, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(configMapName, changeSetEntry.Subject); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(string(UnchangedAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("generates diff for changed object", func(t *testing.T) {
		newVal := "diff-test"
		err = unstructured.SetNestedField(configMap.Object, newVal, "data", "key")
		if err != nil {
			t.Fatal(err)
		}

		changeSetEntry, _, mergedObj, err := manager.Diff(ctx, configMap, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(string(ConfiguredAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		mergedObjYaml, _ := yaml.Marshal(mergedObj)
		if !strings.Contains(string(mergedObjYaml), newVal) {
			t.Errorf("Mismatch from expected value, want %s", newVal)
		}
	})

	t.Run("generates diff for replaced key in stringData secret", func(t *testing.T) {
		// create a new stringData secret
		sec := secret.DeepCopy()
		if err := unstructured.SetNestedField(sec.Object, generateName("diff"), "metadata", "name"); err != nil {
			t.Fatal(err)
		}

		// copy the secret to simulate a replace of key
		diffSecret := sec.DeepCopy()

		// apply stringData conversion
		SetNativeKindsDefaults([]*unstructured.Unstructured{sec})

		if _, err = manager.Apply(ctx, sec, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		newVal := "diff-test"
		unstructured.RemoveNestedField(diffSecret.Object, "stringData", "key")

		newKey := "key.new"
		err = unstructured.SetNestedField(diffSecret.Object, newVal, "stringData", newKey)
		if err != nil {
			t.Fatal(err)
		}

		// apply stringData conversion
		SetNativeKindsDefaults([]*unstructured.Unstructured{diffSecret})

		_, liveObj, mergedObj, err := manager.Diff(ctx, diffSecret, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		liveKeys := getKeys(liveObj.Object["data"].(map[string]interface{}))
		mergedKeys := getKeys(mergedObj.Object["data"].(map[string]interface{}))

		if diff := cmp.Diff(liveKeys, mergedKeys); diff != "" && len(liveKeys) != len(mergedKeys) {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

	})

	t.Run("masks secret values", func(t *testing.T) {
		newVal := "diff-test"
		err = unstructured.SetNestedField(secret.Object, newVal, "stringData", "key")
		if err != nil {
			t.Fatal(err)
		}

		newKey := "key.new"
		err = unstructured.SetNestedField(secret.Object, newVal, "stringData", newKey)
		if err != nil {
			t.Fatal(err)
		}

		changeSetEntry, _, mergedObj, err := manager.Diff(ctx, secret, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		mergedObjYaml, _ := yaml.Marshal(mergedObj)

		if diff := cmp.Diff(secretName, changeSetEntry.Subject); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if !strings.Contains(string(mergedObjYaml), newKey) {
			t.Errorf("Mismatch from expected value, got %s", string(mergedObjYaml))
		}

		if strings.Contains(string(mergedObjYaml), newVal) {
			t.Errorf("Mismatch from expected value, got %s", string(mergedObjYaml))
		}
	})
}

func TestDiff_Removals(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff")
	objects, err := readManifest("testdata/test4.yaml", id)
	if err != nil {
		t.Fatal(err)
	}
	SetNativeKindsDefaults(objects)

	configMapName, configMap := getFirstObject(objects, "ConfigMap", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	t.Run("generates empty diff for unchanged object", func(t *testing.T) {
		changeSetEntry, _, _, err := manager.Diff(ctx, configMap, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(configMapName, changeSetEntry.Subject); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(string(UnchangedAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if _, err = manager.ApplyAll(ctx, []*unstructured.Unstructured{configMap}, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("generates diff for added map entry", func(t *testing.T) {
		newVal := "diff-test"
		err = unstructured.SetNestedField(configMap.Object, newVal, "data", "token")
		if err != nil {
			t.Fatal(err)
		}

		changeSetEntry, _, mergedObj, err := manager.Diff(ctx, configMap, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		mergedObjYaml, _ := yaml.Marshal(mergedObj)

		if diff := cmp.Diff(string(ConfiguredAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if !strings.Contains(string(mergedObjYaml), newVal) {
			t.Errorf("Mismatch from expected value, want %s", newVal)
		}

		if _, err = manager.ApplyAll(ctx, []*unstructured.Unstructured{configMap}, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("generates diff for removed map entry", func(t *testing.T) {
		unstructured.RemoveNestedField(configMap.Object, "data", "token")

		changeSetEntry, _, _, err := manager.Diff(ctx, configMap, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(string(ConfiguredAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if _, err = manager.ApplyAll(ctx, []*unstructured.Unstructured{configMap}, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}
	})

}

func TestDiffHPA(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff")
	objects, err := readManifest("testdata/test6.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	hpaName, hpa := getFirstObject(objects, "HorizontalPodAutoscaler", id)
	var metrics []interface{}

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	t.Run("generates empty diff for unchanged object", func(t *testing.T) {
		changeSetEntry, _, _, err := manager.Diff(ctx, hpa, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(hpaName, changeSetEntry.Subject); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(string(UnchangedAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("generates diff for removed metric", func(t *testing.T) {
		metrics, _, err = unstructured.NestedSlice(hpa.Object, "spec", "metrics")
		if err != nil {
			t.Fatal(err)
		}

		err = unstructured.SetNestedSlice(hpa.Object, metrics[:1], "spec", "metrics")
		if err != nil {
			t.Fatal(err)
		}

		changeSetEntry, _, _, err := manager.Diff(ctx, hpa, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(string(ConfiguredAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("generates empty diff for unchanged metric", func(t *testing.T) {
		changeSetEntry, _, _, err := manager.Diff(ctx, hpa, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(string(UnchangedAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("generates diff for added metric", func(t *testing.T) {
		err = unstructured.SetNestedSlice(hpa.Object, metrics, "spec", "metrics")
		if err != nil {
			t.Fatal(err)
		}

		changeSetEntry, _, _, err := manager.Diff(ctx, hpa, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(string(ConfiguredAction), changeSetEntry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHasDrifted_Metadata(t *testing.T) {
	id := generateName("drifted")
	objects, err := readManifest("testdata/test7.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, deploy := getFirstObject(objects, "Deployment", id)
	deploy.SetResourceVersion("1")

	annotatedDeploy := deploy.DeepCopy()
	unstructured.RemoveNestedField(deploy.Object, "metadata", "annotations", "annotated")

	labeledDeploy := deploy.DeepCopy()
	unstructured.RemoveNestedField(labeledDeploy.Object, "metadata", "labels", "labeled")

	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		drifted bool
	}{
		{
			name:    "returns false if object is unchanged",
			obj:     deploy,
			drifted: false,
		},
		{
			name:    "returns true if an annotation is removed",
			obj:     annotatedDeploy,
			drifted: true,
		},
		{
			name:    "returns true if an label is removed",
			obj:     labeledDeploy,
			drifted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasDrifted := manager.hasDrifted(tt.obj, deploy)
			if hasDrifted != tt.drifted {
				t.Errorf("expected hasDrifted to be %t but got %t\n objects.", tt.drifted, hasDrifted)
			}
		})
	}
}

func getKeys(m map[string]interface{}) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}
