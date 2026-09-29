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
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestApply_FieldManagersBeforeDryRun and TestApplyAll_FieldManagersBeforeDryRun
// reproduce the "break-glass" wedge:
//
// A workload is managed by Flux via server-side apply. During an incident an
// operator disables Flux and hand-patches the workload with client-side PUTs
// from a legacy controller (operation: Update). Re-enabling Flux leaves a stale
// `legacy-controller / Update` managedFields entry co-owning a container's
// identity ("." + f:name) but NOT its required f:image. When a later build
// removes that container, server-side apply cannot prune it (the legacy Update
// manager still owns the element) but DOES drop the field Flux solely owned
// (f:image), leaving a half-removed, invalid container:
//
//	spec.template.spec.containers[N].image: Required value
//
// This fails at the dry-run on every reconcile, before any write. The standard
// post-dry-run FieldManagers cleanup never runs, so --override-manager cannot
// recover it. FieldManagersBeforeDryRun hoists the takeover before the dry-run,
// which repairs ownership and lets the container prune cleanly.
//
// The scenario is run through both Apply and ApplyAll, since kustomize-controller
// always uses ApplyAll.
func TestApply_FieldManagersBeforeDryRun(t *testing.T) {
	runFieldManagersBeforeDryRunScenario(t, generateName("takeover"), applyOneViaApply)
}

func TestApplyAll_FieldManagersBeforeDryRun(t *testing.T) {
	runFieldManagersBeforeDryRunScenario(t, generateName("takeover-all"), applyOneViaApplyAll)
}

func runFieldManagersBeforeDryRunScenario(t *testing.T, id string, apply applyFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const (
		fluxFM   = "resource-manager" // matches manager.owner.Field
		legacyFM = "legacy-controller"
		gitImage = "registry.k8s.io/pause:3.9"
		bgImage  = "registry.k8s.io/pause:3.8"
	)

	// Namespace for the workload.
	ns := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]interface{}{"name": id},
	}}
	if err := manager.client.Patch(ctx, ns, client.Apply, client.FieldOwner(fluxFM)); err != nil {
		t.Fatal(err)
	}

	// daemonset builds a DaemonSet with the given container name/image pairs.
	daemonset := func(containers ...[2]string) *unstructured.Unstructured {
		cs := make([]interface{}, 0, len(containers))
		for _, c := range containers {
			cs = append(cs, map[string]interface{}{"name": c[0], "image": c[1]})
		}
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": id},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{"app": id},
					},
					"spec": map[string]interface{}{"containers": cs},
				},
			},
		}}
	}

	// legacyPut emulates a break-glass client-side PUT: read the in-cluster
	// object, replace its containers, and Update it under the legacy manager
	// (operation: Update).
	legacyPut := func(t *testing.T, containers ...[2]string) {
		t.Helper()
		cur := daemonset()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cur), cur); err != nil {
			t.Fatal(err)
		}
		cs := make([]interface{}, 0, len(containers))
		for _, c := range containers {
			cs = append(cs, map[string]interface{}{"name": c[0], "image": c[1]})
		}
		if err := unstructured.SetNestedSlice(cur.Object, cs, "spec", "template", "spec", "containers"); err != nil {
			t.Fatal(err)
		}
		if err := manager.client.Update(ctx, cur, client.FieldOwner(legacyFM)); err != nil {
			t.Fatal(err)
		}
	}

	// pdhOwnership reports, per manager/operation, whether it owns the pdh
	// container element ("."), its f:name, and its f:image.
	type ownership struct{ self, name, image bool }
	pdhOwnership := func(t *testing.T) map[string]ownership {
		t.Helper()
		cur := daemonset()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cur), cur); err != nil {
			t.Fatal(err)
		}
		out := map[string]ownership{}
		for _, e := range cur.GetManagedFields() {
			if e.Subresource != "" || e.FieldsV1 == nil {
				continue
			}
			var raw map[string]interface{}
			if err := json.Unmarshal(e.FieldsV1.Raw, &raw); err != nil {
				t.Fatal(err)
			}
			pdh := digPath(raw, "f:spec", "f:template", "f:spec", "f:containers", `k:{"name":"pdh"}`)
			if pdh == nil {
				continue
			}
			o := ownership{}
			if _, ok := pdh["."]; ok {
				o.self = true
			}
			if _, ok := pdh["f:name"]; ok {
				o.name = true
			}
			if _, ok := pdh["f:image"]; ok {
				o.image = true
			}
			out[string(e.Manager)+"/"+string(e.Operation)] = o
		}
		return out
	}

	// --- Set up the exact production ownership shape ---------------------------

	// 1. GitOps healthy: Flux SSA-applies main + pdh (git image).
	if _, err := apply(ctx, daemonset([2]string{"main", gitImage}, [2]string{"pdh", gitImage}), DefaultApplyOptions()); err != nil {
		t.Fatalf("initial flux apply: %v", err)
	}

	// 2. Break-glass: operator removes broken pdh, then re-adds it (hotfix image).
	//    The legacy Update manager that re-creates pdh takes the element "." + f:name.
	legacyPut(t, [2]string{"main", gitImage})
	legacyPut(t, [2]string{"main", gitImage}, [2]string{"pdh", bgImage})

	// 3. Re-enable Flux: it reasserts the git manifest. Because the image value
	//    differs, Flux's apply takes f:image exclusively; the legacy Update keeps
	//    "." + f:name but loses f:image
	forceOpts := DefaultApplyOptions()
	forceOpts.Force = false
	if _, err := apply(ctx, daemonset([2]string{"main", gitImage}, [2]string{"pdh", gitImage}), forceOpts); err != nil {
		t.Fatalf("re-enable flux apply: %v", err)
	}

	own := pdhOwnership(t)
	flux, okFlux := own[fluxFM+"/Apply"]
	legacy, okLegacy := own[legacyFM+"/Update"]
	if !okFlux || !flux.image {
		t.Fatalf("expected %s/Apply to own pdh f:image, ownership=%+v", fluxFM, own)
	}
	if !okLegacy || !legacy.self || legacy.image {
		t.Fatalf("expected %s/Update to anchor pdh (self, not image), ownership=%+v", legacyFM, own)
	}
	t.Logf("ownership set up: flux=%+v legacy=%+v", flux, legacy)

	// FieldManagers to take over (the legacy break-glass manager).
	fms := []FieldManager{
		{Name: legacyFM, OperationType: metav1.ManagedFieldsOperationUpdate},
		{Name: legacyFM, OperationType: metav1.ManagedFieldsOperationApply},
	}

	// --- The wedge and the fix -------------------------------------------------

	t.Run("without pre-dry-run takeover, dry-run is Invalid", func(t *testing.T) {
		opts := DefaultApplyOptions()
		// Post-dry-run cleanup only (current --override-manager behavior): the
		// takeover is gated behind a passing dry-run, so it never runs here.
		opts.Cleanup.FieldManagers = fms

		_, err := apply(ctx, daemonset([2]string{"main", gitImage}), opts)
		if err == nil {
			t.Fatal("expected dry-run to fail with Required value, got nil")
		}
		if !strings.Contains(err.Error(), "Required value") {
			t.Fatalf("expected 'Required value' dry-run error, got: %v", err)
		}
		t.Logf("reproduced wedge: %v", err)

		// pdh must still be present (nothing was applied).
		cur := daemonset()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cur), cur); err != nil {
			t.Fatal(err)
		}
		if !hasContainer(t, cur, "pdh") {
			t.Fatal("expected pdh to still be present after failed dry-run")
		}
	})

	t.Run("with pre-dry-run takeover, apply succeeds and pdh is pruned", func(t *testing.T) {
		opts := DefaultApplyOptions()
		opts.Cleanup.FieldManagersBeforeDryRun = fms

		entry, err := apply(ctx, daemonset([2]string{"main", gitImage}), opts)
		if err != nil {
			t.Fatalf("expected apply to succeed after takeover, got: %v", err)
		}
		if entry.Action != ConfiguredAction {
			t.Errorf("expected ConfiguredAction, got %s", entry.Action)
		}

		cur := daemonset()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cur), cur); err != nil {
			t.Fatal(err)
		}
		if hasContainer(t, cur, "pdh") {
			t.Fatal("expected pdh to be pruned after takeover apply")
		}
		if !hasContainer(t, cur, "main") {
			t.Fatal("expected main container to remain")
		}
	})
}

// TestApply_FieldManagersBeforeDryRun_MutatesEvenOnUnrelatedFailure and
// TestApplyAll_FieldManagersBeforeDryRun_MutatesEvenOnUnrelatedFailure document
// the non-transactional tradeoff called out in
// ApplyCleanupOptions.FieldManagersBeforeDryRun: the pre-dry-run takeover PATCH
// is sent regardless of whether the apply that triggered it goes on to
// succeed. If the dry-run subsequently fails for a reason entirely unrelated
// to the takeover (here, an out-of-range port that fails API validation), the
// managedFields mutation has already happened and is not rolled back, even
// though the apply returns an error and no other write occurs.
//
// The scenario is run through both Apply and ApplyAll, since kustomize-controller
// always uses ApplyAll.
func TestApply_FieldManagersBeforeDryRun_MutatesEvenOnUnrelatedFailure(t *testing.T) {
	runFieldManagersBeforeDryRunMutatesEvenOnUnrelatedFailureScenario(
		t, generateName("takeover-unrelated-failure"), applyOneViaApply)
}

func TestApplyAll_FieldManagersBeforeDryRun_MutatesEvenOnUnrelatedFailure(t *testing.T) {
	runFieldManagersBeforeDryRunMutatesEvenOnUnrelatedFailureScenario(
		t, generateName("takeover-unrelated-failure-all"), applyOneViaApplyAll)
}

func runFieldManagersBeforeDryRunMutatesEvenOnUnrelatedFailureScenario(t *testing.T, id string, apply applyFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const (
		fluxFM   = "resource-manager" // matches manager.owner.Field
		legacyFM = "legacy-controller"
		noteKey  = "legacy.example.com/note"
	)

	ns := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]interface{}{"name": id},
	}}
	if err := manager.client.Patch(ctx, ns, client.Apply, client.FieldOwner(fluxFM)); err != nil {
		t.Fatal(err)
	}

	// service builds a Service with the given port. An out-of-range port fails
	// API server validation independently of any field manager ownership.
	service := func(port int64) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{"app": id},
				"ports": []interface{}{
					map[string]interface{}{"port": port, "targetPort": port},
				},
			},
		}}
	}

	// annotationOwner returns the manager owning noteKey, if any.
	annotationOwner := func(t *testing.T) (string, bool) {
		t.Helper()
		obj := service(80)
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			t.Fatal(err)
		}
		for _, e := range obj.GetManagedFields() {
			if e.Subresource != "" || e.FieldsV1 == nil {
				continue
			}
			var raw map[string]interface{}
			if err := json.Unmarshal(e.FieldsV1.Raw, &raw); err != nil {
				t.Fatal(err)
			}
			if digPath(raw, "f:metadata", "f:annotations", "f:"+noteKey) != nil {
				return string(e.Manager), true
			}
		}
		return "", false
	}

	// 1. Flux creates the Service with a valid port, owning everything via Apply.
	if _, err := apply(ctx, service(80), DefaultApplyOptions()); err != nil {
		t.Fatalf("initial flux apply: %v", err)
	}

	// 2. Break-glass: legacy controller PUTs the object, adding an annotation
	//    that has nothing to do with the port validation failure below.
	cur := service(80)
	if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cur), cur); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(cur.Object, "keep-me", "metadata", "annotations", noteKey); err != nil {
		t.Fatal(err)
	}
	if err := manager.client.Update(ctx, cur, client.FieldOwner(legacyFM)); err != nil {
		t.Fatal(err)
	}
	if owner, ok := annotationOwner(t); !ok || owner != legacyFM {
		t.Fatalf("expected %s to own the annotation before takeover, got owner=%q ok=%v", legacyFM, owner, ok)
	}

	// 3. Apply with FieldManagersBeforeDryRun scoped to legacyFM, but submit a
	// desired Service with an invalid port so the dry-run fails for a reason
	// entirely unrelated to field manager ownership.
	fms := []FieldManager{
		{Name: legacyFM, OperationType: metav1.ManagedFieldsOperationUpdate},
		{Name: legacyFM, OperationType: metav1.ManagedFieldsOperationApply},
	}
	opts := DefaultApplyOptions()
	opts.Cleanup.FieldManagersBeforeDryRun = fms

	const invalidPort = 70000 // out of the valid 1-65535 range
	_, err := apply(ctx, service(invalidPort), opts)
	if err == nil {
		t.Fatal("expected dry-run to fail due to the invalid port, got nil")
	}
	if strings.Contains(err.Error(), "Required value") {
		t.Fatalf("expected an unrelated validation error, got what looks like the co-ownership wedge error: %v", err)
	}
	t.Logf("apply failed as expected, for a reason unrelated to field manager ownership: %v", err)

	// 4. Even though the apply never went through, the pre-dry-run takeover has
	// already reassigned the annotation's ownership to Flux. This is the
	// documented tradeoff: FieldManagersBeforeDryRun mutates managedFields even
	// when the apply it runs within does not succeed.
	if owner, ok := annotationOwner(t); !ok || owner != fluxFM {
		t.Fatalf("expected the takeover to have reassigned the annotation to %s despite the failed apply, got owner=%q ok=%v",
			fluxFM, owner, ok)
	}
}

// TestApply_FieldManagersBeforeDryRun_ReclaimsUnrelatedFields and
// TestApplyAll_FieldManagersBeforeDryRun_ReclaimsUnrelatedFields document that
// the takeover is whole-manager, not per-field: reclaiming a stale manager to
// fix a specific wedged field also reassigns every other field that manager
// owns to the applier. If the applier's desired state does not declare those
// other fields, they are pruned by the very same apply that fixes the wedge.
//
// The scenario is run through both Apply and ApplyAll, since kustomize-controller
// always uses ApplyAll.
func TestApply_FieldManagersBeforeDryRun_ReclaimsUnrelatedFields(t *testing.T) {
	runFieldManagersBeforeDryRunReclaimsUnrelatedFieldsScenario(
		t, generateName("takeover-collateral"), applyOneViaApply)
}

func TestApplyAll_FieldManagersBeforeDryRun_ReclaimsUnrelatedFields(t *testing.T) {
	runFieldManagersBeforeDryRunReclaimsUnrelatedFieldsScenario(
		t, generateName("takeover-collateral-all"), applyOneViaApplyAll)
}

func runFieldManagersBeforeDryRunReclaimsUnrelatedFieldsScenario(t *testing.T, id string, apply applyFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const (
		fluxFM   = "resource-manager"
		legacyFM = "legacy-controller"
		gitImage = "registry.k8s.io/pause:3.9"
		bgImage  = "registry.k8s.io/pause:3.8"
		noteKey  = "legacy.example.com/note"
	)

	ns := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]interface{}{"name": id},
	}}
	if err := manager.client.Patch(ctx, ns, client.Apply, client.FieldOwner(fluxFM)); err != nil {
		t.Fatal(err)
	}

	daemonset := func(containers ...[2]string) *unstructured.Unstructured {
		cs := make([]interface{}, 0, len(containers))
		for _, c := range containers {
			cs = append(cs, map[string]interface{}{"name": c[0], "image": c[1]})
		}
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"name":      id,
				"namespace": id,
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": id},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{"app": id},
					},
					"spec": map[string]interface{}{"containers": cs},
				},
			},
		}}
	}

	// annotationOwner returns the manager owning noteKey on the DaemonSet, if any.
	annotationOwner := func(t *testing.T) (string, bool) {
		t.Helper()
		obj := daemonset()
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			t.Fatal(err)
		}
		for _, e := range obj.GetManagedFields() {
			if e.Subresource != "" || e.FieldsV1 == nil {
				continue
			}
			var raw map[string]interface{}
			if err := json.Unmarshal(e.FieldsV1.Raw, &raw); err != nil {
				t.Fatal(err)
			}
			if digPath(raw, "f:metadata", "f:annotations", "f:"+noteKey) != nil {
				return string(e.Manager), true
			}
		}
		return "", false
	}

	// 1. GitOps healthy: Flux applies main + pdh at the git image.
	if _, err := apply(ctx, daemonset([2]string{"main", gitImage}, [2]string{"pdh", gitImage}), DefaultApplyOptions()); err != nil {
		t.Fatalf("initial flux apply: %v", err)
	}

	// 2. Break-glass: legacy controller PUTs the whole object, re-creating pdh
	//    at a hotfix image AND adding an annotation that has nothing to do with
	//    pdh or the eventual wedge, e.g. an operator's incident note.
	cur := daemonset()
	if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cur), cur); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedSlice(cur.Object,
		[]interface{}{
			map[string]interface{}{"name": "main", "image": gitImage},
			map[string]interface{}{"name": "pdh", "image": bgImage},
		}, "spec", "template", "spec", "containers"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(cur.Object, "keep-me", "metadata", "annotations", noteKey); err != nil {
		t.Fatal(err)
	}
	if err := manager.client.Update(ctx, cur, client.FieldOwner(legacyFM)); err != nil {
		t.Fatal(err)
	}

	// 3. Re-enable Flux: it reasserts the git manifest, taking ownership of
	//    pdh's image field but leaving legacy-controller anchoring pdh's
	//    identity and still owning the unrelated annotation.
	if _, err := apply(ctx, daemonset([2]string{"main", gitImage}, [2]string{"pdh", gitImage}), DefaultApplyOptions()); err != nil {
		t.Fatalf("re-enable flux apply: %v", err)
	}
	if owner, ok := annotationOwner(t); !ok || owner != legacyFM {
		t.Fatalf("expected %s to own the unrelated annotation before takeover, got owner=%q ok=%v", legacyFM, owner, ok)
	}

	// 4. A build removes pdh. FieldManagersBeforeDryRun is scoped to legacyFM
	// to fix the co-ownership wedge on pdh — the annotation is not the target
	// of this takeover, it is simply owned by the same manager.
	fms := []FieldManager{
		{Name: legacyFM, OperationType: metav1.ManagedFieldsOperationUpdate},
		{Name: legacyFM, OperationType: metav1.ManagedFieldsOperationApply},
	}
	opts := DefaultApplyOptions()
	opts.Cleanup.FieldManagersBeforeDryRun = fms

	if _, err := apply(ctx, daemonset([2]string{"main", gitImage}), opts); err != nil {
		t.Fatalf("expected apply to succeed after takeover, got: %v", err)
	}

	cur = daemonset()
	if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cur), cur); err != nil {
		t.Fatal(err)
	}
	if hasContainer(t, cur, "pdh") {
		t.Fatal("expected pdh to be pruned after takeover apply")
	}

	// The takeover is whole-manager, not per-field: reclaiming legacyFM's
	// ownership of pdh also reclaimed its ownership of the unrelated
	// annotation. Since Flux's desired manifest does not declare that
	// annotation, the applier's own apply prunes it in the same reconcile —
	// a side effect unrelated to the pdh wedge this takeover was scoped to
	// fix. If this assertion starts failing, the takeover has become
	// field-scoped and this test (and the doc comments referencing this
	// behavior) should be updated.
	if _, found, _ := unstructured.NestedString(cur.Object, "metadata", "annotations", noteKey); found {
		t.Fatal("expected the unrelated annotation to have been pruned as a collateral side effect " +
			"of the whole-manager field manager takeover")
	}
}

// hasContainer reports whether the workload has a container with the given name.
func hasContainer(t *testing.T, obj *unstructured.Unstructured, name string) bool {
	t.Helper()
	cs, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return false
	}
	for _, c := range cs {
		if m, ok := c.(map[string]interface{}); ok {
			if n, _, _ := unstructured.NestedString(m, "name"); n == name {
				return true
			}
		}
	}
	return false
}

// digPath walks nested map[string]interface{} by the given keys, returning the
// terminal map or nil.
func digPath(m map[string]interface{}, keys ...string) map[string]interface{} {
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}
