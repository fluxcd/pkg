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

		// Monitoring controller claims template annotations via ForceOwnership.
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
								"prometheus.io/scrape": "true",
								"prometheus.io/port":   "9797",
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

	t.Run("flux apply fails when sole owner ignores immutable field", func(t *testing.T) {
		// When Flux is the sole owner of spec.selector and the ignore rule strips
		// it from the payload, K8s tries to remove the field. Since it's immutable
		// and required, the apply is rejected.
		err := unstructured.SetNestedField(deployObject.Object, int64(7), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		_, applyErr := manager.Apply(ctx, deployObject, opts)
		if applyErr == nil {
			t.Fatal("expected apply to fail when Flux is sole owner and ignores immutable field spec.selector")
		}
		t.Logf("Apply correctly failed when sole owner ignores immutable field: %v", applyErr)

		if !strings.Contains(applyErr.Error(), "spec.selector") {
			t.Errorf("expected error to mention spec.selector, got: %v", applyErr)
		}

		// Verify the Deployment is still intact in-cluster.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		_, found, _ := unstructured.NestedMap(existing.Object, "spec", "selector")
		if !found {
			t.Fatal("expected spec.selector to still exist in-cluster after failed apply")
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

	t.Run("flux apply succeeds when co-owned immutable field is ignored", func(t *testing.T) {
		// Now that selector-controller co-owns spec.selector, Flux can safely
		// drop the field from its payload. K8s doesn't try to remove the field
		// because selector-controller still owns it. Flux just releases its
		// co-ownership.
		err := unstructured.SetNestedField(deployObject.Object, int64(8), "spec", "minReadySeconds")
		if err != nil {
			t.Fatal(err)
		}

		entry, err := manager.Apply(ctx, deployObject, opts)
		if err != nil {
			t.Fatalf("expected apply to succeed when ignoring co-owned immutable field, got: %v", err)
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

		// Verify Flux no longer owns spec.selector.
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
		if fluxOwnsSelector {
			t.Errorf("expected Flux to no longer own spec.selector after ignore-rule apply")
		}
		if !selectorControllerOwns {
			t.Errorf("expected selector-controller to still own spec.selector")
		}
	})

	t.Run("immutable required field cannot be orphaned by other controller", func(t *testing.T) {
		// spec.selector is both immutable and required on Deployments.
		// The API server rejects an apply payload that omits it.
		// Verify that selector-controller cannot drop spec.selector from its payload.
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
		if err == nil {
			t.Fatal("expected API server to reject apply without required spec.selector field, but got no error")
		}
		if !strings.Contains(err.Error(), "field is immutable") {
			t.Errorf("expected error to mention field is immutable, got: %v", err)
		}

		// Verify the Deployment is still intact and spec.selector is preserved.
		existing := deployObject.DeepCopy()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
			t.Fatal(err)
		}
		_, found, _ := unstructured.NestedMap(existing.Object, "spec", "selector")
		if !found {
			t.Fatal("expected spec.selector to still exist in-cluster after rejected apply")
		}

		// Verify selector-controller still owns spec.selector after the rejected apply.
		selectorControllerOwns := false
		for _, mf := range existing.GetManagedFields() {
			if mf.Manager == "selector-controller" && mf.Operation == metav1.ManagedFieldsOperationApply {
				if mf.FieldsV1 != nil {
					fieldsJSON := string(mf.FieldsV1.Raw)
					if strings.Contains(fieldsJSON, "f:selector") {
						selectorControllerOwns = true
					}
				}
			}
		}
		if !selectorControllerOwns {
			t.Errorf("expected selector-controller to still own spec.selector after rejected apply")
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
		// A single IgnoreRule with multiple Paths strips all listed fields.
		err := unstructured.SetNestedField(deployObject.Object, int64(3), "spec", "replicas")
		if err != nil {
			t.Fatal(err)
		}

		// First apply to claim ownership of replicas.
		_, err = manager.Apply(ctx, deployObject, DefaultApplyOptions())
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

		// Trigger drift.
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

		// Verify Flux no longer owns either field.
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

	t.Run("ApplyAll applies ignore rules selectively", func(t *testing.T) {
		// An ignore rule targeting kind=Deployment should strip fields from the
		// Deployment but NOT from the Service. We verify by checking that the
		// Service's ports are still owned by Flux (not stripped).
		err := unstructured.SetNestedField(deployObject.Object, int64(3), "spec", "replicas")
		if err != nil {
			t.Fatal(err)
		}
		_, err = manager.Apply(ctx, deployObject, DefaultApplyOptions())
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

	t.Run("subsequent update strips ignored field", func(t *testing.T) {
		// On update, the ignore rule should now take effect.
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
		err := unstructured.SetNestedField(cm.Object, "value2-updated", "data", "key2")
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
