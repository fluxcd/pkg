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
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/ssa/jsondiff"
	"github.com/fluxcd/pkg/ssa/normalize"
)

func TestDiff_DriftIgnoreRules_IgnoredFieldReturnsUnchanged(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff-ign")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	_, deployObject := getFirstObject(objects, "Deployment", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	// Mutate an in-cluster field that will be covered by the ignore rule.
	existing := deployObject.DeepCopy()
	if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
		t.Fatal(err)
	}

	vpaObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"spec": map[string]interface{}{
				"replicas": int64(5),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": id,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": id,
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "podinfod",
								"image": "ghcr.io/stefanprodan/podinfo:6.0.0",
							},
						},
					},
				},
			},
		},
	}
	if err := manager.client.Patch(ctx, vpaObj, client.Apply,
		client.FieldOwner("vpa-controller"), client.ForceOwnership); err != nil {
		t.Fatal(err)
	}

	opts := DefaultDiffOptions()
	opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/replicas"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("returns unchanged when only ignored field drifted", func(t *testing.T) {
		entry, liveObj, mergedObj, err := manager.Diff(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(UnchangedAction, entry.Action); diff != "" {
			t.Errorf("expected UnchangedAction when only ignored fields differ (-want +got):\n%s", diff)
		}

		if liveObj != nil {
			t.Errorf("expected nil liveObject for unchanged diff, got non-nil")
		}
		if mergedObj != nil {
			t.Errorf("expected nil mergedObject for unchanged diff, got non-nil")
		}
	})
}

func TestDiff_DriftIgnoreRules_NonIgnoredFieldReturnsConfigured(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff-nonign")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	_, deployObject := getFirstObject(objects, "Deployment", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	opts := DefaultDiffOptions()
	opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/replicas"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("returns configured when non-ignored field changed", func(t *testing.T) {
		modified := deployObject.DeepCopy()
		if err := unstructured.SetNestedField(modified.Object, int64(10), "spec", "minReadySeconds"); err != nil {
			t.Fatal(err)
		}

		entry, liveObj, mergedObj, err := manager.Diff(ctx, modified, opts)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
			t.Errorf("expected ConfiguredAction for non-ignored field change (-want +got):\n%s", diff)
		}

		if liveObj == nil {
			t.Fatal("expected non-nil liveObject for configured diff")
		}
		if mergedObj == nil {
			t.Fatal("expected non-nil mergedObject for configured diff")
		}
	})
}

func TestDiff_DriftIgnoreRules_StripsIgnoredFieldsFromReturnedObjects(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff-strip")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	_, deployObject := getFirstObject(objects, "Deployment", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	// Cause drift in the ignored field (replicas) via another controller.
	vpaObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"spec": map[string]interface{}{
				"replicas": int64(10),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": id,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": id,
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "podinfod",
								"image": "ghcr.io/stefanprodan/podinfo:6.0.0",
							},
						},
					},
				},
			},
		},
	}
	if err := manager.client.Patch(ctx, vpaObj, client.Apply,
		client.FieldOwner("vpa-controller"), client.ForceOwnership); err != nil {
		t.Fatal(err)
	}

	opts := DefaultDiffOptions()
	opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/replicas"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("strips ignored fields from returned objects when other drift exists", func(t *testing.T) {
		// Change a non-ignored field so the diff returns ConfiguredAction.
		modified := deployObject.DeepCopy()
		if err := unstructured.SetNestedField(modified.Object, int64(42), "spec", "minReadySeconds"); err != nil {
			t.Fatal(err)
		}

		entry, liveObj, mergedObj, err := manager.Diff(ctx, modified, opts)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
			t.Errorf("expected ConfiguredAction (-want +got):\n%s", diff)
		}

		// The returned objects should have the ignored spec.replicas field stripped.
		if liveObj != nil {
			_, found, _ := unstructured.NestedInt64(liveObj.Object, "spec", "replicas")
			if found {
				t.Errorf("expected spec.replicas to be stripped from liveObject, but it was present")
			}
		}

		if mergedObj != nil {
			_, found, _ := unstructured.NestedInt64(mergedObj.Object, "spec", "replicas")
			if found {
				t.Errorf("expected spec.replicas to be stripped from mergedObject, but it was present")
			}
		}
	})
}

func TestDiff_DriftIgnoreRules_EmptyRulesFallsBack(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff-empty")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	// Empty DriftIgnoreRules should behave like DefaultDiffOptions.
	opts := DefaultDiffOptions()
	opts.DriftIgnoreRules = nil

	t.Run("unchanged object returns unchanged with nil rules", func(t *testing.T) {
		entry, _, _, err := manager.Diff(ctx, configMap, opts)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(UnchangedAction, entry.Action); diff != "" {
			t.Errorf("expected UnchangedAction with nil rules (-want +got):\n%s", diff)
		}
	})

	t.Run("changed object returns configured with nil rules", func(t *testing.T) {
		modified := configMap.DeepCopy()
		if err := unstructured.SetNestedField(modified.Object, "new-value", "data", "key"); err != nil {
			t.Fatal(err)
		}

		entry, _, _, err := manager.Diff(ctx, modified, opts)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
			t.Errorf("expected ConfiguredAction with nil rules (-want +got):\n%s", diff)
		}
	})

	t.Run("changed object returns configured with empty slice rules", func(t *testing.T) {
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{}

		modified := configMap.DeepCopy()
		if err := unstructured.SetNestedField(modified.Object, "another-value", "data", "key"); err != nil {
			t.Fatal(err)
		}

		entry, _, _, err := manager.Diff(ctx, modified, opts)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
			t.Errorf("expected ConfiguredAction with empty rules (-want +got):\n%s", diff)
		}
	})
}

func TestDiff_DriftIgnoreRules_SelectorScoping(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("diff-sel")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	// Ignore rule targets Deployments only, not ConfigMaps.
	opts := DefaultDiffOptions()
	opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/data/key"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("selector-scoped rule does not suppress drift on non-matching kind", func(t *testing.T) {
		modified := configMap.DeepCopy()
		if err := unstructured.SetNestedField(modified.Object, "changed-value", "data", "key"); err != nil {
			t.Fatal(err)
		}

		entry, _, _, err := manager.Diff(ctx, modified, opts)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
			t.Errorf("expected ConfiguredAction for ConfigMap change with Deployment-scoped rule (-want +got):\n%s", diff)
		}
	})
}
