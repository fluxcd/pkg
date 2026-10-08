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
	"fmt"
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

// applyMode runs a single-object apply through either Apply or ApplyAll and
// returns the resulting ChangeSetEntry, so a test can assert identical behavior
// on both code paths. Apply and ApplyAll carry duplicated before-dry-run reshape
// logic kept in sync only by a comment; running every before-dry-run test under
// both modes ensures a bug introduced in just one copy is caught.
type applyMode struct {
	name  string
	apply func(ctx context.Context, obj *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error)
}

// applyModes returns the two apply entrypoints that must behave identically for
// the before-dry-run reshape.
func applyModes() []applyMode {
	return []applyMode{
		{
			name: "Apply",
			apply: func(ctx context.Context, obj *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error) {
				return manager.Apply(ctx, obj, opts)
			},
		},
		{
			name: "ApplyAll",
			apply: func(ctx context.Context, obj *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error) {
				cs, err := manager.ApplyAll(ctx, []*unstructured.Unstructured{obj}, opts)
				if err != nil {
					return nil, err
				}
				if cs == nil || len(cs.Entries) == 0 {
					return nil, fmt.Errorf("ApplyAll returned no change set entries")
				}
				return &cs.Entries[0], nil
			},
		},
	}
}

// createBeforeDryRunNamespace creates a namespace used by the inline fixtures
// below. The inline objects (ConfigMap, PVC) don't come from a manifest, so we
// create their namespace explicitly.
func createBeforeDryRunNamespace(ctx context.Context, t *testing.T, name string) {
	t.Helper()
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
	if err := manager.client.Patch(ctx, ns, client.Apply, client.FieldOwner("test")); err != nil {
		t.Fatal(err)
	}
}

// TestApplyBeforeDryRun_AdoptUnblocksForbiddenDowngrade is the faithful repro of
// https://github.com/fluxcd/kustomize-controller/issues/1739.
//
// A PersistentVolumeClaim's spec.resources.requests.storage can only grow; the
// API server rejects any decrease at (dry-run) update validation with a
// Forbidden error. This is the same shape as the Gardener version-downgrade
// case: the desired value in Git is rejected by the API server, so the released
// post-dry-run ignore path can never reach the point where it would resolve the
// ignore rule — the object is wedged. Resolving the ignore rule BEFORE the
// dry-run (adopting the live value) lets the dry-run validate a value the API
// server accepts, unblocking reconciliation.
func TestApplyBeforeDryRun_AdoptUnblocksForbiddenDowngrade(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	storagePath := "/spec/resources/requests/storage"

	// The rule targets the storage request. It is the same rule for both the
	// post- and before-dry-run subtests; only the option field it is placed in
	// differs, which is exactly the knob IgnoreRule.beforeDryRun toggles.
	ignoreRule := jsondiff.IgnoreRule{
		Paths:    []string{storagePath},
		Selector: &jsondiff.Selector{Kind: "PersistentVolumeClaim"},
	}

	// Run the whole scenario through both Apply and ApplyAll, each on its own
	// object so the two runs never share cluster state.
	for _, mode := range applyModes() {
		t.Run(mode.name, func(t *testing.T) {
			id := generateName("bdr-adopt")
			createBeforeDryRunNamespace(ctx, t, id)

			newPVC := func(storage string) *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "PersistentVolumeClaim",
						"metadata": map[string]interface{}{
							"name":      id,
							"namespace": id,
						},
						"spec": map[string]interface{}{
							"accessModes": []interface{}{"ReadWriteOnce"},
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{
									"storage": storage,
								},
							},
						},
					},
				}
			}

			t.Run("create with 2Gi", func(t *testing.T) {
				// On create there is no in-cluster object, so ignore rules are
				// skipped and Flux applies (and owns) the full desired storage
				// request.
				optsBefore := DefaultApplyOptions()
				optsBefore.DriftIgnoreRulesBeforeDryRun = []jsondiff.IgnoreRule{ignoreRule}

				entry, err := mode.apply(ctx, newPVC("2Gi"), optsBefore)
				if err != nil {
					t.Fatal(err)
				}
				if entry.Action != CreatedAction {
					t.Fatalf("expected CreatedAction, got %s", entry.Action)
				}
			})

			t.Run("post-dry-run rule wedges on the forbidden downgrade", func(t *testing.T) {
				// Baseline that demonstrates the bug the feature fixes: with the
				// ignore rule resolved AFTER the dry-run, the dry-run runs against
				// the raw desired value (1Gi), the API server rejects the
				// decrease, and the ignore rule never gets a chance to run. The
				// apply fails.
				optsPost := DefaultApplyOptions()
				optsPost.DriftIgnoreRules = []jsondiff.IgnoreRule{ignoreRule}

				_, err := mode.apply(ctx, newPVC("1Gi"), optsPost)
				if err == nil {
					t.Fatal("expected the forbidden storage downgrade to fail the dry-run, got no error")
				}
				// The apiserver rejects a PVC storage decrease; the exact wording
				// can vary across versions, so match loosely.
				if !strings.Contains(err.Error(), "less than previous value") &&
					!strings.Contains(err.Error(), "Forbidden") {
					t.Fatalf("expected a forbidden-downgrade dry-run error, got: %v", err)
				}
			})

			t.Run("before-dry-run rule adopts the live value and unblocks", func(t *testing.T) {
				// With the ignore rule resolved BEFORE the dry-run: Flux is the
				// sole owner of the drifted storage request, so the reshape adopts
				// the live value (2Gi) into the payload. The dry-run then
				// validates 2Gi, which equals the in-cluster value, so there is no
				// decrease and the apply succeeds. The manifest's 1Gi never
				// reaches the API server.
				optsBefore := DefaultApplyOptions()
				optsBefore.DriftIgnoreRulesBeforeDryRun = []jsondiff.IgnoreRule{ignoreRule}

				entry, err := mode.apply(ctx, newPVC("1Gi"), optsBefore)
				if err != nil {
					t.Fatalf("expected the before-dry-run adopt to unblock the apply, got: %v", err)
				}
				// Only the ignored field differs, so drift detection sees no
				// change and the apply is a no-op (UnchangedAction). The important
				// assertion is that it did not error.
				if entry.Action != UnchangedAction && entry.Action != ConfiguredAction {
					t.Fatalf("expected UnchangedAction or ConfiguredAction, got %s", entry.Action)
				}

				// The live value must be preserved: adopt copied 2Gi, so the
				// stored value is still 2Gi, never the rejected 1Gi.
				existing := newPVC("")
				if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
					t.Fatal(err)
				}
				storage, found, _ := unstructured.NestedString(existing.Object, "spec", "resources", "requests", "storage")
				if !found || storage != "2Gi" {
					t.Fatalf("expected live storage to stay 2Gi after before-dry-run adopt, got %q (found=%v)", storage, found)
				}
			})
		})
	}
}

// TestApplyBeforeDryRun_CreateSkipsIgnore verifies that before-dry-run rules,
// like the post-dry-run rules, are skipped on create: with no in-cluster object
// there is nothing to strip or adopt against, so the full desired object is
// applied. This is the invariant that also makes the immutable-recreate path
// safe (after a delete, adopt has no live value and falls back to desired).
func TestApplyBeforeDryRun_CreateSkipsIgnore(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, mode := range applyModes() {
		t.Run(mode.name, func(t *testing.T) {
			id := generateName("bdr-create")
			createBeforeDryRunNamespace(ctx, t, id)

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
					},
				},
			}

			opts := DefaultApplyOptions()
			opts.DriftIgnoreRulesBeforeDryRun = []jsondiff.IgnoreRule{
				{
					Paths:    []string{"/data/key1"},
					Selector: &jsondiff.Selector{Kind: "ConfigMap"},
				},
			}

			entry, err := mode.apply(ctx, cm, opts)
			if err != nil {
				t.Fatal(err)
			}
			if entry.Action != CreatedAction {
				t.Fatalf("expected CreatedAction, got %s", entry.Action)
			}

			existing := cm.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			val, found, _ := unstructured.NestedString(existing.Object, "data", "key1")
			if !found || val != "value1" {
				t.Fatalf("expected data.key1=value1 on create (ignore rules must not strip on create), got %q (found=%v)", val, found)
			}
		})
	}
}

// TestApplyBeforeDryRun_AdoptSkippedWhenAbsentInCluster covers the b2 guard on
// the before-dry-run path: an ignored path that is present in the desired object
// but absent in-cluster must NOT be adopted. Adopting would copy the absent
// (nil) value over the value the user declared and apply null, the inverse of
// the crossplane "field set to null" failure. The guard skips the adopt and
// leaves the desired value in place.
func TestApplyBeforeDryRun_AdoptSkippedWhenAbsentInCluster(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, mode := range applyModes() {
		t.Run(mode.name, func(t *testing.T) {
			id := generateName("bdr-absent")
			createBeforeDryRunNamespace(ctx, t, id)

			newCM := func(data map[string]interface{}) *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      id,
							"namespace": id,
						},
						"data": data,
					},
				}
			}

			opts := DefaultApplyOptions()
			// The ignored path /data/optional is absent in-cluster after create.
			opts.DriftIgnoreRulesBeforeDryRun = []jsondiff.IgnoreRule{
				{
					Paths:    []string{"/data/optional"},
					Selector: &jsondiff.Selector{Kind: "ConfigMap"},
				},
			}

			// Create with only data.key1 (no data.optional in-cluster).
			if _, err := mode.apply(ctx, newCM(map[string]interface{}{"key1": "v1"}), opts); err != nil {
				t.Fatal(err)
			}

			// Now apply desired that adds data.optional (absent in-cluster) and
			// drifts a non-ignored field (data.key1) so the apply actually runs.
			entry, err := mode.apply(ctx, newCM(map[string]interface{}{
				"key1":     "v2",
				"optional": "should-stay",
			}), opts)
			if err != nil {
				t.Fatal(err)
			}
			if entry.Action != ConfiguredAction {
				t.Fatalf("expected ConfiguredAction, got %s", entry.Action)
			}

			existing := newCM(nil)
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			// The guard means the desired value is applied, not nulled/dropped.
			optional, found, _ := unstructured.NestedString(existing.Object, "data", "optional")
			if !found || optional != "should-stay" {
				t.Fatalf("expected data.optional=should-stay (adopt must be skipped when absent in-cluster), got %q (found=%v)", optional, found)
			}
			key1, _, _ := unstructured.NestedString(existing.Object, "data", "key1")
			if key1 != "v2" {
				t.Fatalf("expected non-ignored data.key1 to be updated to v2, got %q", key1)
			}
		})
	}
}

// TestApplyBeforeDryRun_StripCoOwnedField verifies the strip branch on the
// before-dry-run path: when the ignored field is owned by another Apply manager,
// the reshape strips it from the payload before the dry-run, so Flux releases
// ownership and the other controller's value is preserved.
func TestApplyBeforeDryRun_StripCoOwnedField(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, mode := range applyModes() {
		t.Run(mode.name, func(t *testing.T) {
			id := generateName("bdr-strip")
			objects, err := readManifest("testdata/test2.yaml", id)
			if err != nil {
				t.Fatal(err)
			}
			manager.SetOwnerLabels(objects, "app1", "default")
			if err := normalize.UnstructuredList(objects); err != nil {
				t.Fatal(err)
			}
			_, deployObject := getFirstObject(objects, "Deployment", id)

			opts := DefaultApplyOptions()
			opts.DriftIgnoreRulesBeforeDryRun = []jsondiff.IgnoreRule{
				{
					Paths:    []string{"/spec/replicas"},
					Selector: &jsondiff.Selector{Kind: "Deployment"},
				},
			}

			// Create the Deployment with replicas=2 (Flux owns replicas at this
			// point, but create skips the ignore rule).
			if err := unstructured.SetNestedField(deployObject.Object, int64(2), "spec", "replicas"); err != nil {
				t.Fatal(err)
			}
			if _, err := manager.ApplyAllStaged(ctx, objects, opts); err != nil {
				t.Fatal(err)
			}

			// An autoscaler claims spec.replicas with a different value via ForceOwnership.
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
					},
				},
			}
			if err := manager.client.Patch(ctx, vpaObj, client.Apply,
				client.FieldOwner("vpa-controller"), client.ForceOwnership); err != nil {
				t.Fatal(err)
			}

			// Drift a non-ignored field so the apply runs, and keep desired
			// replicas=2 (drifted vs the live 5). Because vpa-controller owns
			// replicas, the before-dry-run reshape strips it from the payload.
			if err := unstructured.SetNestedField(deployObject.Object, int64(11), "spec", "minReadySeconds"); err != nil {
				t.Fatal(err)
			}
			entry, err := mode.apply(ctx, deployObject, opts)
			if err != nil {
				t.Fatal(err)
			}
			if entry.Action != ConfiguredAction {
				t.Fatalf("expected ConfiguredAction, got %s", entry.Action)
			}

			existing := deployObject.DeepCopy()
			if err := manager.client.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
				t.Fatal(err)
			}
			// The autoscaler's replicas value must be preserved (Flux stripped it).
			replicas, found, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
			if !found || replicas != 5 {
				t.Fatalf("expected vpa-controller's replicas=5 to be preserved, got %d (found=%v)", replicas, found)
			}
			// Flux must no longer own spec.replicas.
			for _, mf := range existing.GetManagedFields() {
				if mf.Manager == manager.owner.Field && mf.Operation == metav1.ManagedFieldsOperationApply && mf.FieldsV1 != nil {
					if strings.Contains(string(mf.FieldsV1.Raw), "f:replicas") {
						t.Errorf("expected Flux to no longer own spec.replicas after before-dry-run strip")
					}
				}
			}
		})
	}
}

// TestApplyBeforeDryRun_NoSpuriousDrift verifies that a before-dry-run ignored
// field never drives an apply on its own: drift detection excludes the union of
// before- and after-dry-run ignored paths, so when only the ignored field
// differs the apply is a no-op (UnchangedAction) and the object's
// resourceVersion is not bumped.
func TestApplyBeforeDryRun_NoSpuriousDrift(t *testing.T) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, mode := range applyModes() {
		t.Run(mode.name, func(t *testing.T) {
			id := generateName("bdr-nodrift")
			objects, err := readManifest("testdata/test2.yaml", id)
			if err != nil {
				t.Fatal(err)
			}
			manager.SetOwnerLabels(objects, "app1", "default")
			if err := normalize.UnstructuredList(objects); err != nil {
				t.Fatal(err)
			}
			_, deployObject := getFirstObject(objects, "Deployment", id)

			opts := DefaultApplyOptions()
			opts.DriftIgnoreRulesBeforeDryRun = []jsondiff.IgnoreRule{
				{
					Paths:    []string{"/spec/replicas"},
					Selector: &jsondiff.Selector{Kind: "Deployment"},
				},
			}

			// Create with replicas=2.
			if err := unstructured.SetNestedField(deployObject.Object, int64(2), "spec", "replicas"); err != nil {
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

			// Re-apply with only the ignored field changed (replicas 2 -> 4). No
			// other field drifts, so drift detection must report no change.
			if err := unstructured.SetNestedField(deployObject.Object, int64(4), "spec", "replicas"); err != nil {
				t.Fatal(err)
			}
			entry, err := mode.apply(ctx, deployObject, opts)
			if err != nil {
				t.Fatal(err)
			}
			if entry.Action != UnchangedAction {
				t.Fatalf("expected UnchangedAction when only the before-dry-run ignored field differs, got %s", entry.Action)
			}
		})
	}
}
