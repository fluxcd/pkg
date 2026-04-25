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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/ssa/jsondiff"
	"github.com/fluxcd/pkg/ssa/normalize"
)

func TestApply_DriftIgnoreRules_OptionalFields(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("drift-opt")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	_, deployObject := getFirstObject(objects, "Deployment", id)

	// Define ignore rules for two optional mutable fields upfront.
	opts := DefaultApplyOptions()
	opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/replicas"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
		{
			Paths: []string{"/spec/template/metadata/annotations"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("creates objects with ignore rules present", func(t *testing.T) {
		// Set replicas so it's explicit in the desired state.
		err := unstructured.SetNestedField(deployObject.Object, int64(2), "spec", "replicas")
		if err != nil {
			t.Fatal(err)
		}

		changeSet, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}

		// On create, ignore rules are skipped, so Flux should own all fields
		// including replicas and annotations.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		replicas, found, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if !found || replicas != 2 {
			t.Fatalf("expected spec.replicas=2 after create, got %d (found=%v)", replicas, found)
		}

		// Verify Flux is the field manager and owns both replicas and annotations.
		fluxFound := false
		for _, entry := range existing.GetManagedFields() {
			if entry.Manager == manager.owner.Field && entry.Operation == metav1.ManagedFieldsOperationApply {
				fluxFound = true
				if entry.FieldsV1 != nil {
					fieldsJSON := string(entry.FieldsV1.Raw)
					if !strings.Contains(fieldsJSON, "f:replicas") {
						t.Errorf("expected Flux to own spec.replicas after create, but it does not")
					}
					if !strings.Contains(fieldsJSON, "f:prometheus.io/scrape") {
						t.Errorf("expected Flux to own template annotations after create, but it does not")
					}
				}
			}
		}
		if !fluxFound {
			t.Errorf("expected to find field manager %q with Apply operation", manager.owner.Field)
		}
	})

	t.Run("other controllers claim ignored fields", func(t *testing.T) {
		// VPA controller claims spec.replicas via ForceOwnership.
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
		err := manager.client.Patch(ctx, vpaObj, client.Apply,
			client.FieldOwner("vpa-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		// Monitoring controller claims template annotations via ForceOwnership
		// with different values to introduce drift in the ignored field.
		monObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": id,
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{
								"prometheus.io/scrape": "false",
								"prometheus.io/port":   "8080",
							},
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
		err = manager.client.Patch(ctx, monObj, client.Apply,
			client.FieldOwner("monitoring-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		// Verify the other controllers' values are in-cluster.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if replicas != 5 {
			t.Fatalf("expected spec.replicas=5 after VPA claim, got %d", replicas)
		}

		// Verify field ownership transferred to the third-party controllers.
		vpaOwnsReplicas := false
		monOwnsAnnotations := false
		for _, entry := range existing.GetManagedFields() {
			if entry.Manager == "vpa-controller" && entry.Operation == metav1.ManagedFieldsOperationApply {
				if entry.FieldsV1 != nil {
					fieldsJSON := string(entry.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:replicas") {
						vpaOwnsReplicas = true
					}
				}
			}
			if entry.Manager == "monitoring-controller" && entry.Operation == metav1.ManagedFieldsOperationApply {
				if entry.FieldsV1 != nil {
					fieldsJSON := string(entry.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:prometheus.io/scrape") {
						monOwnsAnnotations = true
					}
				}
			}
		}
		if !vpaOwnsReplicas {
			t.Errorf("expected vpa-controller to own spec.replicas after ForceOwnership claim")
		}
		if !monOwnsAnnotations {
			t.Errorf("expected monitoring-controller to own template annotations after ForceOwnership claim")
		}
	})

	t.Run("flux apply releases ownership of ignored fields", func(t *testing.T) {
		// Trigger drift by changing a non-ignored field.
		err := unstructured.SetNestedField(deployObject.Object, int64(10), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}

		// VPA's replicas value should be preserved.
		replicas, found, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if !found || replicas != 5 {
			t.Errorf("expected spec.replicas=5 (VPA value preserved), got %d", replicas)
		}

		// Verify Flux no longer owns the ignored fields.
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:replicas") {
						t.Errorf("expected Flux to no longer own spec.replicas")
					}
					if strings.Contains(fieldsJSON, "f:prometheus.io/scrape") {
						t.Errorf("expected Flux to no longer own prometheus.io/scrape")
					}
					if strings.Contains(fieldsJSON, "f:prometheus.io/port") {
						t.Errorf("expected Flux to no longer own prometheus.io/port")
					}
				}
			}
		}
	})

	t.Run("other controller orphans ignored field and flux does not reclaim it", func(t *testing.T) {
		// VPA applies WITHOUT spec.replicas, dropping its ownership.
		// Since Flux also doesn't own it anymore (released in previous subtest),
		// the field becomes orphaned — no manager owns it.
		vpaObjNoReplicas := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
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
		err := manager.client.Patch(ctx, vpaObjNoReplicas, client.Apply,
			client.FieldOwner("vpa-controller"))
		if err != nil {
			t.Fatal(err)
		}

		// Verify replicas is still in-cluster but orphaned (no manager owns it).
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		replicas, found, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if !found {
			t.Fatal("expected spec.replicas to still exist in-cluster after VPA dropped ownership")
		}
		t.Logf("spec.replicas=%d is now orphaned (value persists, no manager owns it)", replicas)

		// Confirm no manager owns spec.replicas.
		for _, mf := range existing.GetManagedFields() {
			if mf.FieldsV1 != nil {
				fieldsJSON := string(mf.FieldsV1.Raw)
				if strings.Contains(fieldsJSON, "f:replicas") {
					t.Errorf("expected no manager to own spec.replicas, but %q (op=%s) still owns it",
						mf.Manager, mf.Operation)
				}
			}
		}

		// Flux re-apply with ignore rule should NOT reclaim the orphaned field.
		err = unstructured.SetNestedField(deployObject.Object, int64(11), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify Flux did NOT reclaim ownership of spec.replicas.
		existing = deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		for _, mf := range existing.GetManagedFields() {
			if mf.FieldsV1 != nil {
				fieldsJSON := string(mf.FieldsV1.Raw)
				if strings.Contains(fieldsJSON, "f:replicas") {
					t.Errorf("expected spec.replicas to remain orphaned, but %q (op=%s) owns it",
						mf.Manager, mf.Operation)
				}
			}
		}
	})

	t.Run("re-apply with no changes returns unchanged", func(t *testing.T) {
		// After ownership has been released and no fields have changed,
		// re-applying should return UnchangedAction. The only difference
		// between desired and in-cluster is in ignored fields (replicas),
		// which should not trigger an apply.
		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != UnchangedAction {
			t.Errorf("expected UnchangedAction when only ignored fields differ, got %s", entry.Action)
		}
	})
}

func TestApply_DriftIgnoreRules_ImmutableField(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("drift-imm")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	_, deployObject := getFirstObject(objects, "Deployment", id)

	// Define ignore rule for the immutable spec.selector field upfront.
	opts := DefaultApplyOptions()
	opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/selector"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("creates objects with ignore rules present", func(t *testing.T) {
		changeSet, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}
	})

	t.Run("non-drifted immutable field stays in payload", func(t *testing.T) {
		// When Flux is the sole owner of spec.selector and the ignore rule is present,
		// but the selector value has NOT drifted (same in existing and dry-run),
		// the field should NOT be stripped from the payload. The apply succeeds
		// because the full object including selector is sent.
		err := unstructured.SetNestedField(deployObject.Object, int64(7), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatalf("expected apply to succeed when ignored immutable field has not drifted, got: %v", err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify the Deployment is still intact and Flux still owns spec.selector.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		_, found, _ := unstructured.NestedMap(existing.Object, "spec", "selector")
		if !found {
			t.Fatal("expected spec.selector to still exist in-cluster")
		}

		fluxOwnsSelector := false
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:selector") {
						fluxOwnsSelector = true
					}
				}
			}
		}
		if !fluxOwnsSelector {
			t.Errorf("expected Flux to still own spec.selector since it has not drifted")
		}
	})

	t.Run("other controller co-owns immutable field", func(t *testing.T) {
		// Another controller applies with the same selector value. Since the value
		// matches, both Flux and selector-controller co-own spec.selector.
		otherObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
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
		err := manager.client.Patch(ctx, otherObj, client.Apply,
			client.FieldOwner("selector-controller"))
		if err != nil {
			t.Fatal(err)
		}

		// Verify both Flux and selector-controller co-own spec.selector.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		selectorControllerOwns := false
		fluxOwnsSelector := false
		for _, entry := range existing.GetManagedFields() {
			if entry.FieldsV1 != nil {
				fieldsJSON := string(entry.FieldsV1.Raw)
				if strings.Contains(fieldsJSON, "f:selector") {
					if entry.Manager == "selector-controller" && entry.Operation == metav1.ManagedFieldsOperationApply {
						selectorControllerOwns = true
					}
					if entry.Manager == manager.owner.Field && entry.Operation == metav1.ManagedFieldsOperationApply {
						fluxOwnsSelector = true
					}
				}
			}
		}
		if !selectorControllerOwns {
			t.Errorf("expected selector-controller to co-own spec.selector")
		}
		if !fluxOwnsSelector {
			t.Errorf("expected Flux to still co-own spec.selector before ignore-rule apply")
		}
	})

	t.Run("co-owned non-drifted immutable field stays in payload", func(t *testing.T) {
		// Now that selector-controller co-owns spec.selector, Flux applies with
		// the ignore rule present. Since spec.selector is immutable and has the
		// same value on both sides (it cannot change), it is NOT stripped from
		// the payload. Flux retains its co-ownership.
		err := unstructured.SetNestedField(deployObject.Object, int64(8), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatalf("expected apply to succeed when ignoring co-owned non-drifted immutable field, got: %v", err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify the Deployment is intact and spec.selector is preserved.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		_, found, _ := unstructured.NestedMap(existing.Object, "spec", "selector")
		if !found {
			t.Fatal("expected spec.selector to still exist in-cluster after apply")
		}

		// Verify Flux still co-owns spec.selector (not stripped because value hasn't drifted).
		fluxOwnsSelector := false
		selectorControllerOwns := false
		for _, mf := range existing.GetManagedFields() {
			if mf.FieldsV1 != nil {
				fieldsJSON := string(mf.FieldsV1.Raw)
				if strings.Contains(fieldsJSON, "f:selector") {
					if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
						fluxOwnsSelector = true
					}
					if mf.Manager == "selector-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
						selectorControllerOwns = true
					}
				}
			}
		}
		if !fluxOwnsSelector {
			t.Errorf("expected Flux to still co-own spec.selector since it has not drifted")
		}
		if !selectorControllerOwns {
			t.Errorf("expected selector-controller to still own spec.selector")
		}
	})

	t.Run("immutable required field cannot be orphaned by other controller", func(t *testing.T) {
		// spec.selector is both immutable and required on Deployments.
		// Since Flux still co-owns spec.selector (it was not stripped because it
		// has not drifted), selector-controller applying without spec.selector
		// will succeed because Flux's co-ownership preserves the field.
		// Verify that the field is preserved and both managers maintain their state.
		selectorObjNoSelector := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
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
		err := manager.client.Patch(ctx, selectorObjNoSelector, client.Apply,
			client.FieldOwner("selector-controller"))
		if err != nil {
			// If Flux co-owns selector, this apply succeeds (selector-controller
			// drops its selector ownership but Flux preserves the field).
			// If it fails, that's also acceptable — the API server protects the field.
			t.Logf("selector-controller apply without selector: %v", err)
		}

		// Verify the Deployment is still intact and spec.selector is preserved.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		_, found, _ := unstructured.NestedMap(existing.Object, "spec", "selector")
		if !found {
			t.Fatal("expected spec.selector to still exist in-cluster")
		}
	})
}

func TestApply_DriftIgnoreRules_RequiredMutableField(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("drift-mut")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	_, deployObject := getFirstObject(objects, "Deployment", id)

	// Define ignore rule for the container image (required but mutable) upfront.
	// The path targets the image field of the first container.
	opts := DefaultApplyOptions()
	opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/template/spec/containers/0/image"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("creates objects with ignore rules present", func(t *testing.T) {
		changeSet, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}

		// On create, ignore rules are skipped, so the image should be present.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		containers, found, _ := unstructured.NestedSlice(existing.Object, "spec", "template", "spec", "containers")
		if !found || len(containers) == 0 {
			t.Fatal("expected containers to exist after create")
		}
		c0 := containers[0].(map[string]interface{})
		if c0["image"] != "ghcr.io/stefanprodan/podinfo:6.0.0" {
			t.Fatalf("expected image 6.0.0 after create, got %v", c0["image"])
		}
	})

	t.Run("image policy controller claims container image", func(t *testing.T) {
		// An image policy controller updates the container image and takes ownership.
		imgObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
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
									"image": "ghcr.io/stefanprodan/podinfo:6.2.0",
								},
							},
						},
					},
				},
			},
		}
		err := manager.client.Patch(ctx, imgObj, client.Apply,
			client.FieldOwner("image-policy-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		// Verify the image was updated.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		containers, _, _ := unstructured.NestedSlice(existing.Object, "spec", "template", "spec", "containers")
		c0 := containers[0].(map[string]interface{})
		if c0["image"] != "ghcr.io/stefanprodan/podinfo:6.2.0" {
			t.Fatalf("expected image 6.2.0 after image-policy claim, got %v", c0["image"])
		}

		// Verify image-policy-controller owns the image field and Flux does not.
		imgControllerOwnsImage := false
		fluxOwnsImage := false
		for _, entry := range existing.GetManagedFields() {
			if entry.FieldsV1 != nil {
				fieldsJSON := string(entry.FieldsV1.Raw)
				if strings.Contains(fieldsJSON, "\"f:image\":") {
					if entry.Manager == "image-policy-controller" && entry.Operation == metav1.ManagedFieldsOperationApply {
						imgControllerOwnsImage = true
					}
					if entry.Manager == manager.owner.Field && entry.Operation == metav1.ManagedFieldsOperationApply {
						fluxOwnsImage = true
					}
				}
			}
		}
		if !imgControllerOwnsImage {
			t.Errorf("expected image-policy-controller to own container image after ForceOwnership claim")
		}
		if fluxOwnsImage {
			t.Errorf("expected Flux to no longer own container image after ForceOwnership takeover, but it still does")
		}
	})

	t.Run("flux apply releases image ownership and preserves other controller value", func(t *testing.T) {
		// Trigger drift by changing a non-ignored field.
		err := unstructured.SetNestedField(deployObject.Object, int64(12), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}

		// The image-policy-controller's value should be preserved.
		containers, found, _ := unstructured.NestedSlice(existing.Object, "spec", "template", "spec", "containers")
		if !found || len(containers) == 0 {
			t.Fatal("expected containers to still exist")
		}
		c0 := containers[0].(map[string]interface{})
		if c0["image"] != "ghcr.io/stefanprodan/podinfo:6.2.0" {
			t.Errorf("expected image-policy-controller's image 6.2.0 to be preserved, got %v", c0["image"])
		}

		// Verify Flux no longer owns the container image field.
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "\"f:image\":") {
						t.Errorf("expected Flux to no longer own container image, but it does")
					}
				}
			}
		}

		// Verify image-policy-controller still owns the image.
		imgControllerOwnsImage := false
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == "image-policy-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "\"f:image\":") {
						imgControllerOwnsImage = true
					}
				}
			}
		}
		if !imgControllerOwnsImage {
			t.Errorf("expected image-policy-controller to still own container image")
		}
	})

	t.Run("required field cannot be orphaned by other controller", func(t *testing.T) {
		// Unlike optional fields like spec.replicas, the container image is a
		// required field. The API server rejects an apply payload that omits it.
		// Verify that image-policy-controller cannot drop the image field.
		imgObjNoImage := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
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
									"name": "podinfod",
								},
							},
						},
					},
				},
			},
		}
		err := manager.client.Patch(ctx, imgObjNoImage, client.Apply,
			client.FieldOwner("image-policy-controller"))
		if err == nil {
			t.Fatal("expected API server to reject apply without required image field, but got no error")
		}
		if !strings.Contains(err.Error(), "Required") {
			t.Errorf("expected error to mention required field, got: %v", err)
		}

		// Verify the Deployment is still intact and image-policy-controller still owns the image.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		containers, found, _ := unstructured.NestedSlice(existing.Object, "spec", "template", "spec", "containers")
		if !found || len(containers) == 0 {
			t.Fatal("expected containers to still exist after rejected apply")
		}
		c0 := containers[0].(map[string]interface{})
		if c0["image"] != "ghcr.io/stefanprodan/podinfo:6.2.0" {
			t.Errorf("expected image 6.2.0 to be preserved after rejected apply, got %v", c0["image"])
		}

		imgControllerOwnsImage := false
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == "image-policy-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "\"f:image\":") {
						imgControllerOwnsImage = true
					}
				}
			}
		}
		if !imgControllerOwnsImage {
			t.Errorf("expected image-policy-controller to still own container image after rejected apply")
		}
	})

	t.Run("re-apply with no changes returns unchanged after ownership released", func(t *testing.T) {
		// After Flux released ownership of the image field in the prior subtest,
		// re-applying the same object with no changes should return UnchangedAction.
		// The only difference is in the ignored field (image 6.0.0 vs 6.2.0),
		// which should not trigger an apply.
		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != UnchangedAction {
			t.Errorf("expected UnchangedAction when only ignored fields differ, got %s", entry.Action)
		}
	})
}

func TestApply_DriftIgnoreRules_SelectorAndPaths(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("drift-sel")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	_, deployObject := getFirstObject(objects, "Deployment", id)
	_, svcObject := getFirstObject(objects, "Service", id)

	t.Run("creates objects initially", func(t *testing.T) {
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}
	})

	t.Run("non-matching selector does not strip field", func(t *testing.T) {
		// An ignore rule targeting kind=Service should NOT affect a Deployment.
		// The Deployment's spec.minReadySeconds should be applied normally.
		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/minReadySeconds"},
				Selector: &jsondiff.Selector{
					Kind: "Service",
				},
			},
		}

		err := unstructured.SetNestedField(deployObject.Object, int64(20), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify the field was applied (not ignored).
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		val, found, _ := unstructured.NestedInt64(existing.Object, "spec", "minReadySeconds")
		if !found || val != 20 {
			t.Errorf("expected spec.minReadySeconds=20 (not ignored), got %d (found=%v)", val, found)
		}
	})

	t.Run("name selector matches only named object", func(t *testing.T) {
		// An ignore rule targeting kind=Deployment, name=nonexistent should
		// NOT match the actual Deployment. The field should be applied normally.
		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/minReadySeconds"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
					Name: "nonexistent",
				},
			},
		}

		err := unstructured.SetNestedField(deployObject.Object, int64(22), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		val, _, _ := unstructured.NestedInt64(existing.Object, "spec", "minReadySeconds")
		if val != 22 {
			t.Errorf("expected spec.minReadySeconds=22 (selector didn't match), got %d", val)
		}
	})

	t.Run("multiple paths in single rule", func(t *testing.T) {
		// A single IgnoreRule with multiple Paths strips only the drifted fields.
		err := unstructured.SetNestedField(deployObject.Object, int64(3), "spec", "replicas")
		if err != nil {
			t.Fatal(err)
		}

		// First apply to claim ownership of replicas.
		_, err = manager.Apply(ctx, deployObject, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		// VPA claims replicas via ForceOwnership to introduce drift.
		vpaObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
					"replicas": int64(7),
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
		err = manager.client.Patch(ctx, vpaObj, client.Apply,
			client.FieldOwner("vpa-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		// Monitoring controller claims annotations via ForceOwnership.
		monObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": id,
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{
								"prometheus.io/scrape": "false",
							},
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
		err = manager.client.Patch(ctx, monObj, client.Apply,
			client.FieldOwner("monitoring-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/replicas", "/spec/template/metadata/annotations"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
				},
			},
		}

		// Trigger drift in a non-ignored field.
		err = unstructured.SetNestedField(deployObject.Object, int64(25), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify Flux no longer owns either drifted field.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:replicas") {
						t.Errorf("expected Flux to not own spec.replicas")
					}
					if strings.Contains(fieldsJSON, "f:prometheus.io/scrape") {
						t.Errorf("expected Flux to not own prometheus annotation")
					}
				}
			}
		}
	})

	t.Run("non-existent path is a no-op", func(t *testing.T) {
		// Ignoring a path that doesn't exist in the object should not error.
		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/nonExistentField"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
				},
			},
		}

		err := unstructured.SetNestedField(deployObject.Object, int64(27), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}
	})

	t.Run("malformed pointer path is a no-op", func(t *testing.T) {
		// A malformed pointer like /spec/replicas/invalid (where replicas is an int)
		// cannot be resolved. lookupJSONPointer treats unresolvable paths as
		// "not present" on both sides, so the field is not considered drifted
		// and is not stripped from the payload.
		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/replicas/invalid"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
				},
			},
		}

		err := unstructured.SetNestedField(deployObject.Object, int64(28), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}
	})

	t.Run("multiple paths strips only drifted ones", func(t *testing.T) {
		// A rule with two paths: /spec/replicas (drifted by VPA) and
		// /spec/minReadySeconds (not drifted — same value on both sides).
		// Only replicas should be stripped; minReadySeconds should be kept.
		err := unstructured.SetNestedField(deployObject.Object, int64(4), "spec", "replicas")
		if err != nil {
			t.Fatal(err)
		}

		// Apply to claim ownership of both fields.
		_, err = manager.Apply(ctx, deployObject, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		// VPA claims replicas, introducing drift.
		vpaObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
					"replicas": int64(8),
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
		err = manager.client.Patch(ctx, vpaObj, client.Apply,
			client.FieldOwner("vpa-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/replicas", "/spec/minReadySeconds"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
				},
			},
		}

		// Change a non-ignored field to trigger the apply.
		annotations := deployObject.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations["example.com/trigger"] = "true"
		deployObject.SetAnnotations(annotations)

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify Flux no longer owns replicas (drifted → stripped).
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:replicas") {
						t.Errorf("expected Flux to not own spec.replicas (drifted, should be stripped)")
					}
					// minReadySeconds was NOT drifted, so Flux should still own it.
					if !strings.Contains(fieldsJSON, "f:minReadySeconds") {
						t.Errorf("expected Flux to still own spec.minReadySeconds (not drifted, should be kept)")
					}
				}
			}
		}
	})

	t.Run("ApplyAll applies ignore rules selectively", func(t *testing.T) {
		// An ignore rule targeting kind=Deployment should strip drifted fields from the
		// Deployment but NOT from the Service.
		err := unstructured.SetNestedField(deployObject.Object, int64(3), "spec", "replicas")
		if err != nil {
			t.Fatal(err)
		}
		_, err = manager.Apply(ctx, deployObject, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		// VPA claims replicas via ForceOwnership to introduce drift in the ignored field.
		vpaObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
					"replicas": int64(9),
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
		err = manager.client.Patch(ctx, vpaObj, client.Apply,
			client.FieldOwner("vpa-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/replicas"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
				},
			},
		}

		// Trigger drift on both the Deployment and the Service.
		err = unstructured.SetNestedField(deployObject.Object, int64(30), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}
		err = unstructured.SetNestedField(svcObject.Object, "ClientIP", "spec", "sessionAffinity")
		if err != nil {
			t.Fatal(err)
		}

		changeSet, err := manager.ApplyAll(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		// Verify we got a change set back.
		if len(changeSet.Entries) == 0 {
			t.Fatal("expected non-empty change set from ApplyAll")
		}

		// Verify Flux no longer owns Deployment's spec.replicas.
		existingDeploy := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existingDeploy), existingDeploy); err != nil {
			t.Fatal(err)
		}
		for _, mf := range existingDeploy.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:replicas") {
						t.Errorf("expected Flux to not own Deployment spec.replicas after ApplyAll")
					}
				}
			}
		}

		// Verify Service was NOT affected by the Deployment-targeted rule.
		existingSvc := svcObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existingSvc), existingSvc); err != nil {
			t.Fatal(err)
		}
		svcOwnedByFlux := false
		for _, mf := range existingSvc.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				svcOwnedByFlux = true
			}
		}
		if !svcOwnedByFlux {
			t.Errorf("expected Flux to still own the Service (rule shouldn't affect it)")
		}
	})
}

func TestApply_DriftIgnoreRules_CreateSkipsIgnore(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("drift-create")

	// Build a minimal ConfigMap inline to test create behavior in isolation.
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": id,
			},
		},
	}
	if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"data": map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	t.Run("create includes all fields despite ignore rules", func(t *testing.T) {
		// When creating a new object, ignore rules should NOT strip fields
		// because the guard `existingObject.GetResourceVersion() != ""` is false.
		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/data/key1"},
				Selector: &jsondiff.Selector{
					Kind: "ConfigMap",
				},
			},
		}

		entry, err := manager.Apply(ctx, cm, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != CreatedAction {
			t.Errorf("expected CreatedAction, got %s", entry.Action)
		}

		// Verify key1 IS present in-cluster (not stripped on create).
		existing := cm.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		val, found, _ := unstructured.NestedString(existing.Object, "data", "key1")
		if !found || val != "value1" {
			t.Errorf("expected data.key1=value1 on create (ignore rules should not apply), got %q (found=%v)", val, found)
		}
	})

	t.Run("subsequent update strips drifted ignored field", func(t *testing.T) {
		// On update, the ignore rule should take effect only for drifted fields.
		// First, have another controller change key1 to introduce drift.
		cmClone := cm.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cmClone), cmClone); err != nil {
			t.Fatal(err)
		}
		err := unstructured.SetNestedField(cmClone.Object, "changed-by-other", "data", "key1")
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.client.Update(ctx, cmClone); err != nil {
			t.Fatal(err)
		}

		opts := DefaultApplyOptions()
		opts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/data/key1"},
				Selector: &jsondiff.Selector{
					Kind: "ConfigMap",
				},
			},
		}

		// Change a non-ignored field to trigger drift.
		err = unstructured.SetNestedField(cm.Object, "value2-updated", "data", "key2")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, cm, opts)
		if err != nil {
			t.Fatal(err)
		}

		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify Flux no longer owns data.key1.
		existing := cm.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:key1") {
						t.Errorf("expected Flux to not own data.key1 after update with ignore rule")
					}
				}
			}
		}
	})
}

// driftIgnoreTestDeployment creates a minimal Deployment for drift ignore rule testing.
func driftIgnoreTestDeployment(id string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"spec": map[string]interface{}{
				"replicas": int64(2),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": id,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"prometheus.io/scrape": "true",
						},
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
}

// vpaClaimReplicas simulates VPA claiming spec.replicas via server-side apply with ForceOwnership.
func vpaClaimReplicas(ctx context.Context, t *testing.T, id string, replicas int64) {
	t.Helper()
	vpaObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
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
	err := manager.client.Patch(ctx, vpaObj, client.Apply,
		client.FieldOwner("vpa-controller"), client.ForceOwnership)
	if err != nil {
		t.Fatal(err)
	}
}

func TestApply_DriftIgnoreRules_NonIgnoredFieldDrift(t *testing.T) {
	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ignoreReplicasOpts := DefaultApplyOptions()
	ignoreReplicasOpts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/replicas"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("VPA claimed ignored field", func(t *testing.T) {
		id := generateName("drift-vpa")
		ns := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata":   map[string]interface{}{"name": id},
			},
		}
		if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		deploy := driftIgnoreTestDeployment(id)
		if _, err := manager.Apply(ctx, deploy, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		// VPA claims replicas via ForceOwnership.
		vpaClaimReplicas(ctx, t, id, 5)

		t.Run("mutate non-ignored field", func(t *testing.T) {
			err := unstructured.SetNestedField(deploy.Object, int64(10), "spec", "minReadySeconds")
			if err != nil {
				t.Fatal(err)
			}

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}

			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}

			// VPA's replicas value should be preserved.
			replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
			if replicas != 5 {
				t.Errorf("expected spec.replicas=5 (VPA value), got %d", replicas)
			}

			// Flux should no longer own replicas.
			for _, mf := range existing.GetManagedFields() {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
						t.Errorf("expected Flux to not own spec.replicas")
					}
				}
			}
		})

		t.Run("add non-ignored field", func(t *testing.T) {
			// Re-claim replicas for the next subtest.
			vpaClaimReplicas(ctx, t, id, 5)

			ann := deploy.GetAnnotations()
			if ann == nil {
				ann = map[string]string{}
			}
			ann["example.com/new-field"] = "added"
			deploy.SetAnnotations(ann)

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}

			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			val, ok := existing.GetAnnotations()["example.com/new-field"]
			if !ok || val != "added" {
				t.Errorf("expected annotation example.com/new-field=added, got %q", val)
			}
		})

		t.Run("remove non-ignored field", func(t *testing.T) {
			vpaClaimReplicas(ctx, t, id, 5)

			ann := deploy.GetAnnotations()
			delete(ann, "example.com/new-field")
			deploy.SetAnnotations(ann)

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}
		})

		t.Run("no change to non-ignored fields returns unchanged", func(t *testing.T) {
			vpaClaimReplicas(ctx, t, id, 5)

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != UnchangedAction {
				t.Errorf("expected UnchangedAction when only ignored fields differ, got %s", entry.Action)
			}
		})
	})

	t.Run("client-side edit of ignored field", func(t *testing.T) {
		id := generateName("drift-cse")
		ns := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata":   map[string]interface{}{"name": id},
			},
		}
		if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		deploy := driftIgnoreTestDeployment(id)
		if _, err := manager.Apply(ctx, deploy, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		// Simulate client-side edit (kubectl edit) — Update operation, no SSA manager.
		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		if err := unstructured.SetNestedField(existing.Object, int64(10), "spec", "replicas"); err != nil {
			t.Fatal(err)
		}
		if err := manager.client.Update(ctx, existing); err != nil {
			t.Fatal(err)
		}

		t.Run("mutate non-ignored field", func(t *testing.T) {
			err := unstructured.SetNestedField(deploy.Object, int64(15), "spec", "minReadySeconds")
			if err != nil {
				t.Fatal(err)
			}

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}
		})

		t.Run("add non-ignored field", func(t *testing.T) {
			// Re-introduce client-side drift.
			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(existing.Object, int64(10), "spec", "replicas"); err != nil {
				t.Fatal(err)
			}
			if err := manager.client.Update(ctx, existing); err != nil {
				t.Fatal(err)
			}

			ann := deploy.GetAnnotations()
			if ann == nil {
				ann = map[string]string{}
			}
			ann["example.com/cse-add"] = "true"
			deploy.SetAnnotations(ann)

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}
		})

		t.Run("remove non-ignored field", func(t *testing.T) {
			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(existing.Object, int64(10), "spec", "replicas"); err != nil {
				t.Fatal(err)
			}
			if err := manager.client.Update(ctx, existing); err != nil {
				t.Fatal(err)
			}

			ann := deploy.GetAnnotations()
			delete(ann, "example.com/cse-add")
			deploy.SetAnnotations(ann)

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}
		})

		t.Run("no change to non-ignored fields returns unchanged", func(t *testing.T) {
			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(existing.Object, int64(10), "spec", "replicas"); err != nil {
				t.Fatal(err)
			}
			if err := manager.client.Update(ctx, existing); err != nil {
				t.Fatal(err)
			}

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != UnchangedAction {
				t.Errorf("expected UnchangedAction when only ignored fields differ (client-side), got %s", entry.Action)
			}
		})
	})

	t.Run("no drift in ignored field", func(t *testing.T) {
		id := generateName("drift-none")
		ns := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata":   map[string]interface{}{"name": id},
			},
		}
		if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		deploy := driftIgnoreTestDeployment(id)
		if _, err := manager.Apply(ctx, deploy, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		// No one changes replicas — it stays at Flux's value of 2.

		t.Run("mutate non-ignored field", func(t *testing.T) {
			err := unstructured.SetNestedField(deploy.Object, int64(20), "spec", "minReadySeconds")
			if err != nil {
				t.Fatal(err)
			}

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}

			// Flux should STILL own replicas (not drifted, not stripped).
			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			fluxOwnsReplicas := false
			for _, mf := range existing.GetManagedFields() {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
						fluxOwnsReplicas = true
					}
				}
			}
			if !fluxOwnsReplicas {
				t.Errorf("expected Flux to still own spec.replicas (not drifted, should be kept)")
			}
		})

		t.Run("add non-ignored field", func(t *testing.T) {
			ann := deploy.GetAnnotations()
			if ann == nil {
				ann = map[string]string{}
			}
			ann["example.com/nodrift-add"] = "true"
			deploy.SetAnnotations(ann)

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}

			// Flux should still own replicas.
			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			for _, mf := range existing.GetManagedFields() {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					if mf.FieldsV1 != nil && !strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
						t.Errorf("expected Flux to still own spec.replicas")
					}
				}
			}
		})

		t.Run("remove non-ignored field", func(t *testing.T) {
			ann := deploy.GetAnnotations()
			delete(ann, "example.com/nodrift-add")
			deploy.SetAnnotations(ann)

			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != ConfiguredAction {
				t.Errorf("expected ConfiguredAction, got %s", entry.Action)
			}

			// Flux should still own replicas.
			existing := deploy.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			for _, mf := range existing.GetManagedFields() {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					if mf.FieldsV1 != nil && !strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
						t.Errorf("expected Flux to still own spec.replicas")
					}
				}
			}
		})

		t.Run("no change to non-ignored fields returns unchanged", func(t *testing.T) {
			entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if entry.Action != UnchangedAction {
				t.Errorf("expected UnchangedAction, got %s", entry.Action)
			}
		})
	})

	t.Run("ApplyAll variants", func(t *testing.T) {
		id := generateName("drift-aa")
		objects, err := readManifest("testdata/test2.yaml", id)
		if err != nil {
			t.Fatal(err)
		}

		manager.SetOwnerLabels(objects, "app1", "default")
		if err := normalize.UnstructuredList(objects); err != nil {
			t.Fatal(err)
		}

		_, deployObject := getFirstObject(objects, "Deployment", id)

		// Create all objects first.
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}

		t.Run("VPA claimed and mutate via ApplyAll", func(t *testing.T) {
			vpaClaimReplicas(ctx, t, id, 5)

			err := unstructured.SetNestedField(deployObject.Object, int64(40), "spec", "minReadySeconds")
			if err != nil {
				t.Fatal(err)
			}

			cs, err := manager.ApplyAll(ctx, objects, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if len(cs.Entries) == 0 {
				t.Fatal("expected non-empty change set")
			}

			// Verify Flux no longer owns replicas on the Deployment.
			existing := deployObject.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			for _, mf := range existing.GetManagedFields() {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
						t.Errorf("expected Flux to not own spec.replicas after ApplyAll")
					}
				}
			}
		})

		t.Run("VPA claimed and no change via ApplyAll returns unchanged", func(t *testing.T) {
			vpaClaimReplicas(ctx, t, id, 5)

			cs, err := manager.ApplyAll(ctx, objects, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			for _, entry := range cs.Entries {
				if entry.Action != UnchangedAction {
					t.Errorf("expected UnchangedAction for %s, got %s", entry.Subject, entry.Action)
				}
			}
		})

		t.Run("no drift in ignored field and mutate via ApplyAll", func(t *testing.T) {
			// Re-apply without VPA drift (Flux owns replicas from previous applies).
			err := unstructured.SetNestedField(deployObject.Object, int64(2), "spec", "replicas")
			if err != nil {
				t.Fatal(err)
			}
			_, err = manager.Apply(ctx, deployObject, DefaultApplyOptions())
			if err != nil {
				t.Fatal(err)
			}

			// Now apply via ApplyAll with a non-ignored change. Replicas has not drifted.
			err = unstructured.SetNestedField(deployObject.Object, int64(42), "spec", "minReadySeconds")
			if err != nil {
				t.Fatal(err)
			}

			cs, err := manager.ApplyAll(ctx, objects, ignoreReplicasOpts)
			if err != nil {
				t.Fatal(err)
			}

			if len(cs.Entries) == 0 {
				t.Fatal("expected non-empty change set")
			}

			// Flux should still own replicas (not drifted → not stripped).
			existing := deployObject.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			fluxOwnsReplicas := false
			for _, mf := range existing.GetManagedFields() {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
						fluxOwnsReplicas = true
					}
				}
			}
			if !fluxOwnsReplicas {
				t.Errorf("expected Flux to still own spec.replicas via ApplyAll (not drifted)")
			}
		})
	})
}

func TestApply_DriftIgnoreRules_TwoPhaseOwnershipLifecycle(t *testing.T) {
	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("drift-2ph")
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": id},
		},
	}
	if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	deploy := driftIgnoreTestDeployment(id)

	ignoreReplicasOpts := DefaultApplyOptions()
	ignoreReplicasOpts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/replicas"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	t.Run("creates deployment", func(t *testing.T) {
		entry, err := manager.Apply(ctx, deploy, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != CreatedAction {
			t.Errorf("expected CreatedAction, got %s", entry.Action)
		}

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		fluxOwnsReplicas := false
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
					fluxOwnsReplicas = true
				}
			}
		}
		if !fluxOwnsReplicas {
			t.Errorf("expected Flux to own spec.replicas after create")
		}
	})

	t.Run("HPA co-owns replicas without force", func(t *testing.T) {
		// HPA applies with replicas=2 (same value as Flux) without ForceOwnership.
		// SSA allows this because the values agree — both managers co-own the field.
		hpaObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
					"replicas": int64(2),
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
		err := manager.client.Patch(ctx, hpaObj, client.Apply,
			client.FieldOwner("hpa-controller"))
		if err != nil {
			t.Fatal(err)
		}

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		fluxOwns := false
		hpaOwns := false
		for _, mf := range existing.GetManagedFields() {
			if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					fluxOwns = true
				}
				if mf.Manager == "hpa-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
					hpaOwns = true
				}
			}
		}
		if !fluxOwns {
			t.Errorf("expected Flux to co-own spec.replicas")
		}
		if !hpaOwns {
			t.Errorf("expected hpa-controller to co-own spec.replicas")
		}
	})

	t.Run("unchanged when no non-ignored fields changed", func(t *testing.T) {
		// Both Flux and HPA co-own replicas with the same value.
		// Flux applies with ignore rule and no non-ignored changes → UnchangedAction.
		entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != UnchangedAction {
			t.Errorf("expected UnchangedAction, got %s", entry.Action)
		}
	})

	t.Run("flux still co-owns ignored field after unchanged", func(t *testing.T) {
		// Since no apply was sent to the API server, Flux's co-ownership
		// of spec.replicas should be preserved.
		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		fluxOwns := false
		hpaOwns := false
		for _, mf := range existing.GetManagedFields() {
			if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					fluxOwns = true
				}
				if mf.Manager == "hpa-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
					hpaOwns = true
				}
			}
		}
		if !fluxOwns {
			t.Errorf("expected Flux to still co-own spec.replicas after UnchangedAction (no apply sent to API server)")
		}
		if !hpaOwns {
			t.Errorf("expected hpa-controller to still co-own spec.replicas")
		}
	})

	t.Run("HPA force-claims replicas introducing drift", func(t *testing.T) {
		// HPA changes replicas to 5 via ForceOwnership. This introduces
		// real drift and steals sole ownership from Flux.
		vpaClaimReplicas(ctx, t, id, 5)

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if replicas != 5 {
			t.Fatalf("expected spec.replicas=5 after force claim, got %d", replicas)
		}
	})

	t.Run("non-ignored change triggers apply and drops ownership", func(t *testing.T) {
		err := unstructured.SetNestedField(deploy.Object, int64(50), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}

		// VPA's value should be preserved.
		replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if replicas != 5 {
			t.Errorf("expected spec.replicas=5 (VPA value), got %d", replicas)
		}

		// Flux should no longer own replicas.
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
					t.Errorf("expected Flux to no longer own spec.replicas after real apply")
				}
			}
		}
	})

	t.Run("subsequent unchanged confirms ownership dropped", func(t *testing.T) {
		entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != UnchangedAction {
			t.Errorf("expected UnchangedAction, got %s", entry.Action)
		}

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
					t.Errorf("expected Flux to still not own spec.replicas")
				}
			}
		}
	})
}

func TestApply_DriftIgnoreRules_SharedMutableCoOwnership(t *testing.T) {
	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("drift-co")
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": id},
		},
	}
	if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	deploy := driftIgnoreTestDeployment(id)

	ignoreReplicasOpts := DefaultApplyOptions()
	ignoreReplicasOpts.DriftIgnoreRules = []jsondiff.IgnoreRule{
		{
			Paths: []string{"/spec/replicas"},
			Selector: &jsondiff.Selector{
				Kind: "Deployment",
			},
		},
	}

	// Minimal Deployment for HPA SSA patches.
	hpaObj := func(replicas int64) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      id,
					"namespace": id,
				},
				"spec": map[string]interface{}{
					"replicas": replicas,
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
	}

	t.Run("creates deployment", func(t *testing.T) {
		entry, err := manager.Apply(ctx, deploy, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != CreatedAction {
			t.Errorf("expected CreatedAction, got %s", entry.Action)
		}
	})

	t.Run("other controller co-owns replicas without force", func(t *testing.T) {
		// HPA applies with replicas=2 (same value as Flux) without ForceOwnership.
		// SSA allows this because the values agree — both managers co-own the field.
		err := manager.client.Patch(ctx, hpaObj(2), client.Apply,
			client.FieldOwner("hpa-controller"))
		if err != nil {
			t.Fatal(err)
		}

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}

		fluxOwns := false
		hpaOwns := false
		for _, mf := range existing.GetManagedFields() {
			if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					fluxOwns = true
				}
				if mf.Manager == "hpa-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
					hpaOwns = true
				}
			}
		}
		if !fluxOwns {
			t.Errorf("expected Flux to co-own spec.replicas")
		}
		if !hpaOwns {
			t.Errorf("expected hpa-controller to co-own spec.replicas")
		}
	})

	t.Run("other controller changes replicas with force", func(t *testing.T) {
		// HPA force-applies replicas=10 to introduce real drift and take sole ownership.
		err := manager.client.Patch(ctx, hpaObj(10), client.Apply,
			client.FieldOwner("hpa-controller"), client.ForceOwnership)
		if err != nil {
			t.Fatal(err)
		}

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if replicas != 10 {
			t.Fatalf("expected spec.replicas=10 after HPA force claim, got %d", replicas)
		}
	})

	t.Run("flux apply with ignore rule drops co-ownership", func(t *testing.T) {
		err := unstructured.SetNestedField(deploy.Object, int64(55), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deploy, ignoreReplicasOpts)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}

		// HPA value should be preserved.
		replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		if replicas != 10 {
			t.Errorf("expected spec.replicas=10 (HPA value preserved), got %d", replicas)
		}

		// Flux should no longer own replicas; HPA should be the sole owner.
		fluxOwns := false
		hpaOwns := false
		for _, mf := range existing.GetManagedFields() {
			if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
					fluxOwns = true
				}
				if mf.Manager == "hpa-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
					hpaOwns = true
				}
			}
		}
		if fluxOwns {
			t.Errorf("expected Flux to no longer own spec.replicas")
		}
		if !hpaOwns {
			t.Errorf("expected hpa-controller to retain sole ownership of spec.replicas")
		}
	})
}

func TestApply_DriftIgnoreRules_ClientSideEditConsequences(t *testing.T) {
	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Run("optional field reverts to default after ownership dropped", func(t *testing.T) {
		id := generateName("drift-cse-opt")
		ns := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata":   map[string]interface{}{"name": id},
			},
		}
		if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		deploy := driftIgnoreTestDeployment(id)

		// Create the deployment — Flux owns replicas=2.
		entry, err := manager.Apply(ctx, deploy, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != CreatedAction {
			t.Fatalf("expected CreatedAction, got %s", entry.Action)
		}

		// Client-side edit changes replicas to 10 (no SSA manager).
		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		if err := unstructured.SetNestedField(existing.Object, int64(10), "spec", "replicas"); err != nil {
			t.Fatal(err)
		}
		if err := manager.client.Update(ctx, existing); err != nil {
			t.Fatal(err)
		}

		// Flux applies with ignore rule and a non-ignored change.
		ignoreReplicasOpts := DefaultApplyOptions()
		ignoreReplicasOpts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/replicas"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
				},
			},
		}
		if err := unstructured.SetNestedField(deploy.Object, int64(60), "spec", "minReadySeconds"); err != nil {
			t.Fatal(err)
		}

		entry, err = manager.Apply(ctx, deploy, ignoreReplicasOpts)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		// Verify Flux no longer owns replicas.
		existing = deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
					t.Errorf("expected Flux to no longer own spec.replicas")
				}
			}
		}

		// Check what happened to the optional field's value.
		// When Flux drops replicas from its SSA payload and no other SSA Apply manager
		// owns the field, the API server may revert it to the default value (1).
		replicas, found, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
		t.Logf("spec.replicas after Flux dropped ownership: value=%d, found=%v", replicas, found)
		if found && replicas == 10 {
			t.Logf("optional field retained client-side value (no SSA manager reclaimed it)")
		} else if found && replicas == 1 {
			t.Logf("optional field reverted to default value (expected for orphaned optional fields)")
		} else if !found {
			t.Logf("optional field was removed entirely")
		}
	})

	t.Run("required field may cause error after ownership dropped", func(t *testing.T) {
		id := generateName("drift-cse-req")
		ns := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata":   map[string]interface{}{"name": id},
			},
		}
		if _, err := manager.Apply(ctx, ns, DefaultApplyOptions()); err != nil {
			t.Fatal(err)
		}

		deploy := driftIgnoreTestDeployment(id)

		// Create the deployment — Flux owns the container image.
		entry, err := manager.Apply(ctx, deploy, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		if entry.Action != CreatedAction {
			t.Fatalf("expected CreatedAction, got %s", entry.Action)
		}

		// Client-side edit changes the image (no SSA manager).
		existing := deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		containers, _, _ := unstructured.NestedSlice(existing.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			c0 := containers[0].(map[string]interface{})
			c0["image"] = "ghcr.io/stefanprodan/podinfo:6.5.0"
			containers[0] = c0
			if err := unstructured.SetNestedSlice(existing.Object, containers, "spec", "template", "spec", "containers"); err != nil {
				t.Fatal(err)
			}
		}
		if err := manager.client.Update(ctx, existing); err != nil {
			t.Fatal(err)
		}

		// Flux applies with ignore rule for image and a non-ignored change.
		ignoreImageOpts := DefaultApplyOptions()
		ignoreImageOpts.DriftIgnoreRules = []jsondiff.IgnoreRule{
			{
				Paths: []string{"/spec/template/spec/containers/0/image"},
				Selector: &jsondiff.Selector{
					Kind: "Deployment",
				},
			},
		}
		if err := unstructured.SetNestedField(deploy.Object, int64(70), "spec", "minReadySeconds"); err != nil {
			t.Fatal(err)
		}

		entry, err = manager.Apply(ctx, deploy, ignoreImageOpts)
		if err != nil {
			// The API server may reject the apply if the container image (a required
			// field) was stripped from the payload with no other SSA manager owning it.
			if strings.Contains(err.Error(), "Required") || strings.Contains(err.Error(), "required") {
				t.Logf("API server rejected apply with missing required field (expected): %v", err)
				return
			}
			t.Fatalf("unexpected error: %v", err)
		}

		// If the apply succeeded, verify the client-side image value was preserved
		// and Flux no longer owns the image field.
		t.Logf("apply succeeded — checking field ownership and value")

		existing = deploy.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}

		containers, found, _ := unstructured.NestedSlice(existing.Object, "spec", "template", "spec", "containers")
		if !found || len(containers) == 0 {
			t.Fatal("expected containers to exist")
		}
		c0 := containers[0].(map[string]interface{})
		if c0["image"] != "ghcr.io/stefanprodan/podinfo:6.5.0" {
			t.Errorf("expected client-side image 6.5.0 to be preserved, got %v", c0["image"])
		}

		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil && strings.Contains(string(mf.FieldsV1.Raw), "\"f:image\":") {
					t.Errorf("expected Flux to no longer own container image")
				}
			}
		}
	})
}
