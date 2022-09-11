/*
Copyright 2022 The Flux authors

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

package reconcile

import (
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/object"
	"github.com/fluxcd/pkg/runtime/patch"
)

// Conditions contains all the conditions information needed to summarize the
// target condition.
type Conditions struct {
	// Target is the target condition (e.g. Ready).
	Target string
	// Owned conditions are the conditions owned by the reconciler for this
	// target condition.
	Owned []string
	// Summarize conditions are the conditions that the target condition depends
	// on.
	Summarize []string
	// NegativePolarity conditions are the conditions in Summarize with negative
	// polarity.
	NegativePolarity []string
}

// SuccessType is the type of success reconciliation result for a reconciler.
type SuccessType int

const (
	// SuccessWithRequeue is the type of success result which requeues at a
	// constant period. Reconcilers of this success type reconcile on their own
	// at the period to make sure the world matches with the desired state.
	SuccessWithRequeue SuccessType = iota
	// SuccessNoRequeue is the type of success result which does not requeue.
	// Reconcilers of such success type reconcile only based on events.
	SuccessNoRequeue
)

// IsResultSuccess defines if a given ctrl.Result and error result in a
// successful reconciliation result.
type IsResultSuccess func(ctrl.Result, error) bool

// ResultFinalizer finalizes the results of reconciliation to provide a kstatus
// compliant object status and appropriate runtime results based on the status
// observations.
type ResultFinalizer struct {
	isSuccess       IsResultSuccess
	readySuccessMsg string
	conditions      []Conditions
}

// NewResultFinalizer returns a new ResultFinalizer.
func NewResultFinalizer(isSuccess IsResultSuccess, readySuccessMsg string, conditions ...Conditions) *ResultFinalizer {
	return &ResultFinalizer{
		isSuccess:       isSuccess,
		readySuccessMsg: readySuccessMsg,
		conditions:      conditions,
	}
}

// Finalize computes the result of reconciliation. It takes ctrl.Result, error from
// the reconciliation, and a conditions.Setter with conditions, and analyzes
// them to return a reconciliation error. It mutates the object status
// conditions based on the input to ensure the conditions are compliant with
// kstatus. If conditions are passed for summarization, it summarizes the status
// conditions such that the result is kstatus compliant. It also checks for any
// reconcile annotation in the object metadata and adds it to the status as
// LastHandledReconcileAt.
func (rs ResultFinalizer) Finalize(obj conditions.Setter, res ctrl.Result, recErr error) error {
	// Evaluate isSuccess to determine what success means for the reconciler.
	successType := determineSuccessType(rs.isSuccess)

	// Store the success result of the reconciliation taking the error value in
	// consideration.
	successResult := rs.isSuccess(res, recErr)

	// If reconcile error isn't nil, a retry needs to be attempted. Since
	// it's not stalled situation, ensure Stalled condition is removed.
	if recErr != nil {
		conditions.Delete(obj, meta.StalledCondition)
	}

	// Analyze the result if it's unsuccessful or it's a SuccessNoRequeue
	// reconciler. In case of SuccessNoRequeue, it's difficult to differentiate
	// between Success and Stalled result. Further analyze the result to
	// determine if it's Stalled and add the necessary changes to the status.
	if !successResult || successType == SuccessNoRequeue {
		// ctrl.Result is expected to be zero when stalled. If the result isn't
		// zero and not success even without considering the error value, a
		// requeue is requested in the ctrl.Result, it is not a stalled
		// situation. Ensure Stalled condition is removed.
		if !res.IsZero() && !rs.isSuccess(res, nil) {
			conditions.Delete(obj, meta.StalledCondition)
		}
		// If it's still Stalled and Ready is unset or True, ensure Ready value
		// matches with Stalled.
		overwriteReady := conditions.IsUnknown(obj, meta.ReadyCondition) || conditions.IsTrue(obj, meta.ReadyCondition)
		if conditions.IsTrue(obj, meta.StalledCondition) && overwriteReady {
			sc := conditions.Get(obj, meta.StalledCondition)
			conditions.MarkFalse(obj, meta.ReadyCondition, sc.Reason, sc.Message)
		}
	}

	// If it's a successful result or Stalled=True, ensure Reconciling is
	// removed.
	if successResult || conditions.IsTrue(obj, meta.StalledCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
	}

	// Since conditions.IsReady() depends on the values of Stalled and
	// Reconciling conditions, after resolving their values above, update Ready
	// condition based on the reconcile error.
	// If there's a reconcile error and Ready=True or Ready is unknown, mark
	// Ready=False with the reconcile error. If Ready is already False with a
	// reason, preserve the value.
	if recErr != nil {
		if conditions.IsUnknown(obj, meta.ReadyCondition) || conditions.IsReady(obj) {
			conditions.MarkFalse(obj, meta.ReadyCondition, meta.FailedReason, recErr.Error())
		}
	}

	// If custom conditions are provided, summarize them with the Reconciling
	// and Stalled condition changes above.
	for _, c := range rs.conditions {
		conditions.SetSummary(obj,
			c.Target,
			conditions.WithConditions(c.Summarize...),
			conditions.WithNegativePolarityConditions(c.NegativePolarity...),
		)
	}

	// If the result is success for SuccessWithRequeue type reconciler, but
	// Ready is explicitly False (not unknown, with not Ready condition
	// message), and it's not Stalled, set error value to be the Ready failure
	// message.
	if successResult && successType != SuccessNoRequeue &&
		!conditions.IsUnknown(obj, meta.ReadyCondition) &&
		conditions.IsFalse(obj, meta.ReadyCondition) && !conditions.IsStalled(obj) {
		recErr = errors.New(conditions.GetMessage(obj, meta.ReadyCondition))
	}

	// After the above, if Ready condition is not set, it's still a successful
	// reconciliation and it's not reconciling or stalled, mark Ready=True.
	// This tries to preserve any Ready value set previously.
	if conditions.IsUnknown(obj, meta.ReadyCondition) && rs.isSuccess(res, recErr) && !conditions.IsReconciling(obj) && !conditions.IsStalled(obj) {
		conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, rs.readySuccessMsg)
	}

	// TODO: When the Result requests a requeue and no Ready condition value
	// is set, the status condition won't have any Ready condition value.
	// It's difficult to assign a Ready condition value without an error or
	// an existing Reconciling condition.
	// Maybe add a default Ready=False value for safeguard in case this
	// situation becomes common.

	// If a reconcile annotation value is found, set it in the object status as
	// status.lastHandledReconcileAt.
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		object.SetStatusLastHandledReconcileAt(obj, v)
	}

	return recErr
}

// AddPatchOptions adds patch options to a given patch option based on the
// passed conditions.Setter, ownedConditions and fieldOwner, and returns the
// patch options.
// This must be run on a kstatus compliant status. Non-kstatus compliant status
// may result in unexpected patch option result.
func AddPatchOptions(obj conditions.Setter, opts []patch.Option, ownedConditions []string, fieldOwner string) []patch.Option {
	opts = append(opts,
		patch.WithOwnedConditions{Conditions: ownedConditions},
		patch.WithFieldOwner(fieldOwner),
	)
	// Set status observed generation option if the object is stalled, or
	// if the object is ready, i.e. success result.
	if conditions.IsStalled(obj) || conditions.IsReady(obj) {
		opts = append(opts, patch.WithStatusObservedGeneration{})
	}
	return opts
}

// determineSuccessType analyzes the given IsResultSuccess to determine the
// SuccessType of the reconciler.
func determineSuccessType(isSuccess IsResultSuccess) SuccessType {
	// If ctrl.Result is zero with no reconcile error is success, it's
	// SuccessNoRequeue.
	if isSuccess(ctrl.Result{}, nil) {
		return SuccessNoRequeue
	}
	return SuccessWithRequeue
}
