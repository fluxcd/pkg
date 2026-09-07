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

// TestApply_FieldManagersBeforeDryRun reproduces the "break-glass" wedge:
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
func TestApply_FieldManagersBeforeDryRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const (
		fluxFM   = "resource-manager" // matches manager.owner.Field
		legacyFM = "legacy-controller"
		gitImage = "registry.k8s.io/pause:3.9"
		bgImage  = "registry.k8s.io/pause:3.8"
	)

	id := generateName("takeover")

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
	if _, err := manager.Apply(ctx, daemonset([2]string{"main", gitImage}, [2]string{"pdh", gitImage}), DefaultApplyOptions()); err != nil {
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
	if _, err := manager.Apply(ctx, daemonset([2]string{"main", gitImage}, [2]string{"pdh", gitImage}), forceOpts); err != nil {
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

		_, err := manager.Apply(ctx, daemonset([2]string{"main", gitImage}), opts)
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

		entry, err := manager.Apply(ctx, daemonset([2]string{"main", gitImage}), opts)
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
