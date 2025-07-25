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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/ssa/normalize"
	"github.com/fluxcd/pkg/ssa/utils"
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

		if diff := cmp.Diff(UnchangedAction, changeSetEntry.Action); diff != "" {
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

		if diff := cmp.Diff(ConfiguredAction, changeSetEntry.Action); diff != "" {
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
		if err = unstructured.SetNestedField(sec.Object, generateName("diff"), "metadata", "name"); err != nil {
			t.Fatal(err)
		}

		// copy the secret to simulate a replace of key
		diffSecret := sec.DeepCopy()

		// apply stringData conversion
		if err = normalize.Unstructured(sec); err != nil {
			t.Fatal(err)
		}

		if _, err = manager.Apply(ctx, sec, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		newVal := "diff-test"
		unstructured.RemoveNestedField(diffSecret.Object, "stringData", "key")

		newKey := "key.new"
		if err = unstructured.SetNestedField(diffSecret.Object, newVal, "stringData", newKey); err != nil {
			t.Fatal(err)
		}

		// apply stringData conversion
		if err = normalize.Unstructured(diffSecret); err != nil {
			t.Fatal(err)
		}

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

func TestDiff_Exclusions(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("ignore")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)
	_, secret := getFirstObject(objects, "Secret", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	meta := map[string]string{
		"fluxcd.io/ignore": "true",
	}
	opts := DefaultDiffOptions()
	opts.Exclusions = meta

	t.Run("diffs non-exclusion", func(t *testing.T) {
		entry, _, _, err := manager.Diff(ctx, secret, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != UnchangedAction && entry.Subject == utils.FmtUnstructured(secret) {
			t.Errorf("Expected %s, got %s", UnchangedAction, entry.Action)
		}
	})

	t.Run("skips diff exclusion", func(t *testing.T) {
		// mutate in-cluster object
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		configMapClone.SetAnnotations(meta)

		if err := manager.client.Update(ctx, configMapClone); err != nil {
			t.Fatal(err)
		}

		entry, _, _, err := manager.Diff(ctx, configMap, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != SkippedAction && entry.Subject == utils.FmtUnstructured(configMap) {
			t.Errorf("Expected %s, got %s", SkippedAction, entry.Action)
		}
	})
}

func TestDiff_IfNotPresent_OnExisting(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("ifnotpresentonexisting")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	meta := map[string]string{
		"fluxcd.io/ignore": "true",
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)
	configMap.SetAnnotations(meta)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	opts := DefaultDiffOptions()
	opts.IfNotPresentSelector = meta

	t.Run("diffs skips", func(t *testing.T) {
		entry, _, _, err := manager.Diff(ctx, configMap, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != SkippedAction && entry.Subject == utils.FmtUnstructured(configMap) {
			t.Errorf("Expected %s, got %s", SkippedAction, entry.Action)
		}
	})

	t.Run("diffs applies without meta", func(t *testing.T) {
		// mutate in-cluster object
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		err = unstructured.SetNestedField(configMapClone.Object, "public-second-key", "data", "secondKey")
		if err != nil {
			t.Fatal(err)
		}
		configMapClone.SetAnnotations(map[string]string{"fluxcd.io/ignore": ""})
		configMapClone.SetManagedFields(nil)
		entry, _, _, err := manager.Diff(ctx, configMapClone, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction && entry.Subject == utils.FmtUnstructured(configMapClone) {
			t.Errorf("Expected %s, got %s", ConfiguredAction, entry.Action)
		}
	})

	t.Run("diffs skips with meta", func(t *testing.T) {
		// mutate in-cluster object
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		err = unstructured.SetNestedField(configMapClone.Object, "public-second-key", "data", "secondKey")
		if err != nil {
			t.Fatal(err)
		}
		configMapClone.SetManagedFields(nil)

		entry, _, _, err := manager.Diff(ctx, configMapClone, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != SkippedAction && entry.Subject == utils.FmtUnstructured(configMapClone) {
			t.Errorf("Expected %s, got %s", SkippedAction, entry.Action)
		}
	})
}

func TestDiff_IfNotPresent_OnObject(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("ifnotpresentonobject")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	meta := map[string]string{
		"fluxcd.io/ignore": "true",
	}
	opts := DefaultDiffOptions()
	opts.IfNotPresentSelector = meta

	t.Run("diffs unchanged", func(t *testing.T) {
		entry, _, _, err := manager.Diff(ctx, configMap, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != UnchangedAction && entry.Subject == utils.FmtUnstructured(configMap) {
			t.Errorf("Expected %s, got %s", UnchangedAction, entry.Action)
		}
	})

	t.Run("diffs skips with meta", func(t *testing.T) {
		// mutate in-cluster object
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		configMapClone.SetAnnotations(meta)
		err = unstructured.SetNestedField(configMapClone.Object, "public-second-key", "data", "secondKey")
		if err != nil {
			t.Fatal(err)
		}

		entry, _, _, err := manager.Diff(ctx, configMapClone, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != SkippedAction && entry.Subject == utils.FmtUnstructured(configMapClone) {
			t.Errorf("Expected %s, got %s", SkippedAction, entry.Action)
		}
	})

	t.Run("diffs configures without meta", func(t *testing.T) {
		// mutate in-cluster object
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		err = unstructured.SetNestedField(configMapClone.Object, "public-second-key", "data", "secondKey")
		if err != nil {
			t.Fatal(err)
		}
		configMapClone.SetManagedFields(nil)

		entry, _, _, err := manager.Diff(ctx, configMapClone, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction && entry.Subject == utils.FmtUnstructured(configMapClone) {
			t.Errorf("Expected %s, got %s", ConfiguredAction, entry.Action)
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

	if err = normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

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

		if diff := cmp.Diff(UnchangedAction, changeSetEntry.Action); diff != "" {
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

		if diff := cmp.Diff(ConfiguredAction, changeSetEntry.Action); diff != "" {
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

		if diff := cmp.Diff(ConfiguredAction, changeSetEntry.Action); diff != "" {
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

		if diff := cmp.Diff(UnchangedAction, changeSetEntry.Action); diff != "" {
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

		if diff := cmp.Diff(ConfiguredAction, changeSetEntry.Action); diff != "" {
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

		if diff := cmp.Diff(UnchangedAction, changeSetEntry.Action); diff != "" {
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

		if diff := cmp.Diff(ConfiguredAction, changeSetEntry.Action); diff != "" {
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

func TestResourceManager_Diff_CRDWebhookCABundle(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Run("diff removes invalid CA bundle from CRD", func(t *testing.T) {
		id := generateName("diff-ca-bundle-invalid")

		objects, err := readManifest("testdata/test11.yaml", id)
		if err != nil {
			t.Fatal(err)
		}

		_, crd := getFirstObject(objects, "CustomResourceDefinition", "webhooks.example.com")
		uniqueGroup := fmt.Sprintf("%s.example.com", id)
		crdName := fmt.Sprintf("webhooks.%s", uniqueGroup)
		crd.SetName(crdName)
		err = unstructured.SetNestedField(crd.Object, uniqueGroup, "spec", "group")
		if err != nil {
			t.Fatal(err)
		}
		validCABundle := exampleCert
		err = unstructured.SetNestedField(crd.Object, validCABundle, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}

		manager.SetOwnerLabels(objects, "test", "default")

		_, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		modifiedCRD := crd.DeepCopy()
		err = unstructured.SetNestedField(modifiedCRD.Object, "invalid-cert-data", "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}

		changeSetEntry, existingObj, dryRunObj, err := manager.Diff(ctx, modifiedCRD, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if changeSetEntry.Action != ConfiguredAction {
			t.Errorf("Expected %s, got %s", ConfiguredAction, changeSetEntry.Action)
		}

		if dryRunObj != nil {
			dryRunCABundle, found, err := unstructured.NestedString(dryRunObj.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
			if err != nil {
				t.Fatal(err)
			}
			if found && dryRunCABundle != "" {
				t.Errorf("Expected invalid CA bundle to be removed in dry-run object, but found: %s", dryRunCABundle)
			}
		}

		if existingObj != nil {
			existingCABundle, found, err := unstructured.NestedString(existingObj.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
			if err != nil {
				t.Fatal(err)
			}
			if !found || existingCABundle != validCABundle {
				t.Errorf("Expected existing object to have valid CA bundle, got: %s", existingCABundle)
			}
		}
	})

	t.Run("diff preserves valid CA bundle in CRD", func(t *testing.T) {
		id := generateName("diff-ca-bundle-valid")

		objects, err := readManifest("testdata/test11.yaml", id)
		if err != nil {
			t.Fatal(err)
		}

		_, crd := getFirstObject(objects, "CustomResourceDefinition", "webhooks.example.com")
		uniqueGroup := fmt.Sprintf("%s.example.com", id)
		crdName := fmt.Sprintf("webhooks.%s", uniqueGroup)
		crd.SetName(crdName)
		err = unstructured.SetNestedField(crd.Object, uniqueGroup, "spec", "group")
		if err != nil {
			t.Fatal(err)
		}
		validCABundle := exampleCert
		err = unstructured.SetNestedField(crd.Object, validCABundle, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}

		manager.SetOwnerLabels(objects, "test", "default")

		_, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		changeSetEntry, existingObj, dryRunObj, err := manager.Diff(ctx, crd, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if changeSetEntry.Action != UnchangedAction {
			t.Errorf("Expected %s, got %s", UnchangedAction, changeSetEntry.Action)
		}

		if dryRunObj != nil || existingObj != nil {
			t.Error("Expected no diff objects for unchanged CRD")
		}
	})

	t.Run("diff with label change preserves valid CA bundle", func(t *testing.T) {
		id := generateName("diff-ca-bundle-change")

		objects, err := readManifest("testdata/test11.yaml", id)
		if err != nil {
			t.Fatal(err)
		}

		_, crd := getFirstObject(objects, "CustomResourceDefinition", "webhooks.example.com")
		uniqueGroup := fmt.Sprintf("%s.example.com", id)
		crdName := fmt.Sprintf("webhooks.%s", uniqueGroup)
		crd.SetName(crdName)
		err = unstructured.SetNestedField(crd.Object, uniqueGroup, "spec", "group")
		if err != nil {
			t.Fatal(err)
		}
		originalCABundle := exampleCert
		err = unstructured.SetNestedField(crd.Object, originalCABundle, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}

		manager.SetOwnerLabels(objects, "test", "default")

		_, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		modifiedCRD := crd.DeepCopy()
		labels := modifiedCRD.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["test-change"] = "true"
		modifiedCRD.SetLabels(labels)

		changeSetEntry, existingObj, dryRunObj, err := manager.Diff(ctx, modifiedCRD, DefaultDiffOptions())
		if err != nil {
			t.Fatal(err)
		}

		if changeSetEntry.Action != ConfiguredAction {
			t.Errorf("Expected %s, got %s", ConfiguredAction, changeSetEntry.Action)
		}

		if dryRunObj != nil {
			dryRunCABundle, found, err := unstructured.NestedString(dryRunObj.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
			if err != nil {
				t.Fatal(err)
			}
			if !found || dryRunCABundle != originalCABundle {
				t.Errorf("Expected dry-run object to preserve valid CA bundle, got: %s", dryRunCABundle)
			}
		}

		if existingObj != nil {
			existingCABundle, found, err := unstructured.NestedString(existingObj.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
			if err != nil {
				t.Fatal(err)
			}
			if !found || existingCABundle != originalCABundle {
				t.Errorf("Expected existing object to have original CA bundle, got: %s", existingCABundle)
			}
		}
	})
}
