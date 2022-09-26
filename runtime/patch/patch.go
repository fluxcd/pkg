/*
Copyright 2017 The Kubernetes Authors.
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

This file is modified from the source at
https://github.com/kubernetes-sigs/cluster-api/tree/d2faf482116114c4075da1390d905742e524ff89/util/patch/patch.go,
and initially adapted to work with the `conditions` package and `metav1.Condition` types.
*/

package patch

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/fluxcd/pkg/runtime/conditions"
)

// Helper is a utility for ensuring the proper patching of objects.
//
// The Helper MUST be initialised before a set of modifications within the scope of an envisioned patch are made
// to an object, so that the difference in state can be utilised to calculate a patch that can be used on a new revision
// of the resource in case of conflicts.
//
// A common pattern for reconcilers is to initialise a NewHelper at the beginning of their Reconcile method, after
// having fetched the latest revision for the resource from the API server, and then defer the call of Helper.Patch.
// This ensures any modifications made to the spec and the status (conditions) object of the resource are always
// persisted at the end of a reconcile run.
//
// The example below assumes that you will use the Reconciling condition to signal that progress can be made; if it is
// not present, and the Ready condition is not true, the resource will be marked as stalled.
//
//	func (r *FooReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
//		// Retrieve the object from the API server
//		obj := &v1.Foo{}
//		if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
//			return ctrl.Result{}, client.IgnoreNotFound(err)
//		}
//
//		// Initialise the patch helper
//		patchHelper, err := patch.NewHelper(obj, r.Client)
//		if err != nil {
//			return ctrl.Result{}, err
//		}
//
//		// Always attempt to patch the object and status after each reconciliation
//		defer func() {
//			// Patch the object, ignoring conflicts on the conditions owned by this controller
//			patchOpts := []patch.Option{
//				patch.WithOwnedConditions{
//					Conditions: []string{
//						meta.ReadyCondition,
//						meta.ReconcilingCondition,
//						meta.StalledCondition,
//						// any other "owned conditions"
//					},
//				},
//			}
//
//			// On a clean exit, determine if the resource is still being reconciled, or if it has stalled, and record this observation
//			if retErr == nil && (result.IsZero() || !result.Requeue) {
//				// We have now observed this generation
//				patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
//
//				readyCondition := conditions.Get(obj, meta.ReadyCondition)
//				switch {
//				case readyCondition.Status == metav1.ConditionTrue:
//					// As we are no longer reconciling and the end-state is ready, the reconciliation is no longer stalled or progressing, so clear these
//					conditions.Delete(obj, meta.StalledCondition)
//					conditions.Delete(obj, meta.ReconcilingCondition)
//				case conditions.IsReconciling(obj):
//					// This implies stalling is not set; nothing to do
//					break
//				case readyCondition.Status == metav1.ConditionFalse:
//					// As we are no longer reconciling and the end-state is not ready, the reconciliation has stalled
//					conditions.MarkTrue(obj, meta.StalledCondition, readyCondition.Reason, readyCondition.Message)
//				}
//			}
//
//			// Finally, patch the resource
//			if err := patchHelper.Patch(ctx, obj, patchOpts...); err != nil {
//				retErr = kerrors.NewAggregate([]error{retErr, err})
//			}
//		}()
//
//		// ...start with actual reconciliation logic
//	}
//
// Using this pattern, one-off or scoped patches for a subset of a reconcile operation can be made by initialising a new
// Helper using NewHelper with the current state of the resource, making the modifications, and then directly applying
// the patch using Helper.Patch, for example:
//
//	func (r *FooReconciler) subsetReconcile(ctx context.Context, obj *v1.Foo) (ctrl.Result, error) {
//		patchHelper, err := patch.NewHelper(obj, r.Client)
//		if err != nil {
//			return ctrl.Result{}, err
//		}
//
//		// Set CustomField in status object of resource
//		obj.Status.CustomField = "value"
//
//		// Patch now only attempts to persist CustomField
//		patchHelper.Patch(ctx, obj, nil)
//	}
type Helper struct {
	client       client.Client
	gvk          schema.GroupVersionKind
	beforeObject client.Object
	before       *unstructured.Unstructured
	after        *unstructured.Unstructured
	changes      map[string]bool

	isConditionsSetter bool
}

// NewHelper returns an initialised Helper.
func NewHelper(obj client.Object, crClient client.Client) (*Helper, error) {
	// Get the GroupVersionKind of the object,
	// used to validate against later on.
	gvk, err := apiutil.GVKForObject(obj, crClient.Scheme())
	if err != nil {
		return nil, err
	}

	// Convert the object to unstructured to compare against our before copy.
	unstructuredObj, err := ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	// Check if the object satisfies the GitOps Toolkit API conditions contract.
	_, canInterfaceConditions := obj.(conditions.Setter)

	return &Helper{
		client:             crClient,
		gvk:                gvk,
		before:             unstructuredObj,
		beforeObject:       obj.DeepCopyObject().(client.Object),
		isConditionsSetter: canInterfaceConditions,
	}, nil
}

// Patch will attempt to patch the given object, including its status.
func (h *Helper) Patch(ctx context.Context, obj client.Object, opts ...Option) error {
	// Get the GroupVersionKind of the object that we want to patch.
	gvk, err := apiutil.GVKForObject(obj, h.client.Scheme())
	if err != nil {
		return err
	}
	if gvk != h.gvk {
		return errors.Errorf("unmatched GroupVersionKind, expected %q got %q", h.gvk, gvk)
	}

	// Calculate the options.
	options := &HelperOptions{}
	for _, opt := range opts {
		opt.ApplyToHelper(options)
	}

	// Convert the object to unstructured to compare against our before copy.
	h.after, err = ToUnstructured(obj)
	if err != nil {
		return err
	}

	// Determine if the object has status.
	if unstructuredHasStatus(h.after) {
		if options.IncludeStatusObservedGeneration {
			// Set status.observedGeneration if we're asked to do so.
			if err := unstructured.SetNestedField(h.after.Object, h.after.GetGeneration(), "status", "observedGeneration"); err != nil {
				return err
			}

			// Restore the changes back to the original object.
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(h.after.Object, obj); err != nil {
				return err
			}
		}
	}

	// Calculate and store the top-level field changes (e.g. "metadata", "spec", "status") we have before/after.
	h.changes, err = h.calculateChanges(obj)
	if err != nil {
		return err
	}

	// Define K8s client options
	var clientOpts []client.PatchOption
	if options.FieldOwner != "" {
		clientOpts = append(clientOpts, client.FieldOwner(options.FieldOwner))
	}

	// Issue patches and return errors in an aggregate.
	return kerrors.NewAggregate([]error{
		// Patch the conditions first.
		//
		// Given that we pass in metadata.resourceVersion to perform a 3-way-merge conflict resolution,
		// patching conditions first avoids an extra loop if spec or status patch succeeds first
		// given that causes the resourceVersion to mutate.
		h.patchStatusConditions(ctx, obj, options.ForceOverwriteConditions, options.OwnedConditions, clientOpts...),

		// Then proceed to patch the rest of the object.
		h.patch(ctx, obj, clientOpts...),
		h.patchStatus(ctx, obj, clientOpts...),
	})
}

// patch issues a patch for metadata and spec.
func (h *Helper) patch(ctx context.Context, obj client.Object, opts ...client.PatchOption) error {
	if !h.shouldPatch("metadata") && !h.shouldPatch("spec") {
		return nil
	}
	beforeObject, afterObject, err := h.calculatePatch(obj, specPatch)
	if err != nil {
		return err
	}
	return h.client.Patch(ctx, afterObject, client.MergeFromWithOptions(beforeObject), opts...)
}

// patchStatus issues a patch if the status has changed.
func (h *Helper) patchStatus(ctx context.Context, obj client.Object, opts ...client.PatchOption) error {
	if !h.shouldPatch("status") {
		return nil
	}
	beforeObject, afterObject, err := h.calculatePatch(obj, statusPatch)
	if err != nil {
		return err
	}
	return h.client.Status().Patch(ctx, afterObject, client.MergeFrom(beforeObject), opts...)
}

// patchStatusConditions issues a patch if there are any changes to the conditions slice under the status subresource.
// This is a special case and it's handled separately given that we allow different controllers to act on conditions of
// the same object.
//
// This method has an internal backoff loop. When a conflict is detected, the method asks the Client for the new
// version of the object we're trying to patch.
//
// Condition changes are then applied to the latest version of the object, and if there are no unresolvable conflicts,
// the patch is sent again.
func (h *Helper) patchStatusConditions(ctx context.Context, obj client.Object, forceOverwrite bool, ownedConditions []string, opts ...client.PatchOption) error {
	// Nothing to do if the object isn't a condition patcher.
	if !h.isConditionsSetter {
		return nil
	}

	// Make sure our before/after objects satisfy the proper interface before continuing.
	//
	// NOTE: The checks and error below are done so that we don't panic if any of the objects don't satisfy the
	// interface any longer, although this shouldn't happen because we already check when creating the patcher.
	before, ok := h.beforeObject.(conditions.Getter)
	if !ok {
		return errors.Errorf("object %s doesn't satisfy conditions.Getter, cannot patch", before.GetObjectKind())
	}
	after, ok := obj.(conditions.Getter)
	if !ok {
		return errors.Errorf("object %s doesn't satisfy conditions.Getter, cannot patch", after.GetObjectKind())
	}

	// Store the diff from the before/after object, and return early if there are no changes.
	diff := conditions.NewPatch(
		before,
		after,
	)
	if diff.IsZero() {
		return nil
	}

	// Make a copy of the object and store the key used if we have conflicts.
	key := client.ObjectKeyFromObject(after)

	// Define and start a backoff loop to handle conflicts
	// between controllers working on the same object.
	//
	// This has been copied from https://github.com/kubernetes/kubernetes/blob/release-1.16/pkg/controller/controller_utils.go#L86-L88.
	backoff := wait.Backoff{
		Steps:    5,
		Duration: 100 * time.Millisecond,
		Jitter:   1.0,
	}

	// Start the backoff loop and return errors if any.
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		latest, ok := before.DeepCopyObject().(conditions.Setter)
		if !ok {
			return false, errors.Errorf("object %s doesn't satisfy conditions.Setter, cannot patch", latest.GetObjectKind())
		}

		// Get a new copy of the object.
		if err := h.client.Get(ctx, key, latest); err != nil {
			return false, err
		}

		// Create the condition patch before merging conditions.
		conditionsPatch := client.MergeFromWithOptions(latest.DeepCopyObject().(conditions.Setter), client.MergeFromWithOptimisticLock{})

		// Set the condition patch previously created on the new object.
		if err := diff.Apply(latest, conditions.WithForceOverwrite(forceOverwrite), conditions.WithOwnedConditions(ownedConditions...)); err != nil {
			return false, err
		}

		// Issue the patch.
		err := h.client.Status().Patch(ctx, latest, conditionsPatch, opts...)
		switch {
		case apierrors.IsConflict(err):
			// Requeue.
			return false, nil
		case err != nil:
			return false, err
		default:
			return true, nil
		}
	})
}

// calculatePatch returns the before/after objects to be given in a controller-runtime patch, scoped down to the
// absolute necessary.
func (h *Helper) calculatePatch(afterObj client.Object, focus patchType) (client.Object, client.Object, error) {
	// Get a shallow unsafe copy of the before/after object in unstructured form.
	before := unsafeUnstructuredCopy(h.before, focus, h.isConditionsSetter)
	after := unsafeUnstructuredCopy(h.after, focus, h.isConditionsSetter)

	// We've now applied all modifications to local unstructured objects,
	// make copies of the original objects and convert them back.
	beforeObj := h.beforeObject.DeepCopyObject().(client.Object)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(before.Object, beforeObj); err != nil {
		return nil, nil, err
	}
	afterObj = afterObj.DeepCopyObject().(client.Object)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(after.Object, afterObj); err != nil {
		return nil, nil, err
	}
	return beforeObj, afterObj, nil
}

func (h *Helper) shouldPatch(in string) bool {
	return h.changes[in]
}

// calculate changes tries to build a patch from the before/after objects we have and store in a map which top-level
// fields (e.g. `metadata`, `spec`, `status`, etc.) have changed.
func (h *Helper) calculateChanges(after client.Object) (map[string]bool, error) {
	// Calculate patch data.
	patch := client.MergeFrom(h.beforeObject)
	diff, err := patch.Data(after)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate patch data")
	}

	// Unmarshal patch data into a local map.
	patchDiff := map[string]interface{}{}
	if err := json.Unmarshal(diff, &patchDiff); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal patch data into a map")
	}

	// Return the map.
	res := make(map[string]bool, len(patchDiff))
	for key := range patchDiff {
		res[key] = true
	}
	return res, nil
}
