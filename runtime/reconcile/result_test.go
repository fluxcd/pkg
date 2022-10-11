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
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	conditionscheck "github.com/fluxcd/pkg/runtime/conditions/check"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
	"github.com/fluxcd/pkg/runtime/patch"
)

const (
	fetchFailedCondition       = "FetchFailed"
	artifactOutdatedCondition  = "ArtifactOutdated"
	artifactInStorageCondition = "ArtifactInStorage"
)

func TestResultFinalizer(t *testing.T) {
	readySuccessMsg := "Success"
	successInterval := time.Minute
	arbitraryInterval := 5 * time.Second
	resultSuccess := ctrl.Result{RequeueAfter: successInterval}
	resultStalled := ctrl.Result{}
	resultFailed := ctrl.Result{}
	resultRequeue := ctrl.Result{Requeue: true}

	summarizeReadyConditions := Conditions{
		Target: meta.ReadyCondition,
		Owned: []string{
			fetchFailedCondition,
			artifactOutdatedCondition,
			artifactInStorageCondition,
			meta.ReadyCondition,
			meta.ReconcilingCondition,
			meta.StalledCondition,
		},
		Summarize: []string{
			fetchFailedCondition,
			artifactOutdatedCondition,
			artifactInStorageCondition,
			meta.StalledCondition,
			meta.ReconcilingCondition,
		},
		NegativePolarity: []string{
			fetchFailedCondition,
			artifactOutdatedCondition,
			meta.StalledCondition,
			meta.ReconcilingCondition,
		},
	}

	// Success is no error, no immediate or arbitrary requeue in the result.
	// Only requeue at the success interval.
	isSuccess := func(res ctrl.Result, err error) bool {
		if err != nil || res.RequeueAfter != successInterval || res.Requeue {
			return false
		}
		return true
	}

	tests := []struct {
		name                       string
		summarizeConditions        []Conditions
		beforeFunc                 func(obj conditions.Setter)
		result                     ctrl.Result
		recErr                     error
		statusObservedGen          int64
		wantErr                    bool
		wantLastHandledReconcileAt string
		assertConditions           []metav1.Condition
	}{
		{
			name: "result with error and stalled",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
			},
			result:  resultStalled,
			recErr:  errors.New("foo failed"),
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "foo failed"),
			},
		},
		{
			name: "result with error, reconciling and stalled",
			beforeFunc: func(obj conditions.Setter) {
				// Since MarkStalled() removes existing Reconciling condition,
				// use MarkTrue instead for setting Reconciling and Stalled.
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "SomeReasonX", "some msg X")
				conditions.MarkTrue(obj, meta.StalledCondition, "SomeReasonY", "some msg Y")
			},
			result:  resultStalled,
			recErr:  errors.New("foo failed"),
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.ReconcilingCondition, "SomeReasonX", "some msg X"),
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "foo failed"),
			},
		},
		{
			name:       "result with error, no ready value, set ready value",
			beforeFunc: func(obj conditions.Setter) {},
			result:     resultFailed,
			recErr:     errors.New("foo failed"),
			wantErr:    true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "foo failed"),
			},
		},
		{
			name: "result with error, false ready value, no overwrite",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonX", "fail-msg")
			},
			result:  resultFailed,
			recErr:  errors.New("foo failed"),
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonX", "fail-msg"),
			},
		},
		{
			name: "result with error, true ready value, overwrite",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, readySuccessMsg)
			},
			result:  resultFailed,
			recErr:  errors.New("foo failed"),
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "foo failed"),
			},
		},
		{
			name: "result with error, not ready and reconciling, no change",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkReconciling(obj, "SomeReasonX", "some msg X")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonY", "some msg Y")
			},
			result:  resultFailed,
			recErr:  errors.New("foo failed"),
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonY", "some msg Y"),
				*conditions.TrueCondition(meta.ReconcilingCondition, "SomeReasonX", "some msg X"),
			},
		},
		{
			name: "stalled and reconciling, Ready=False, remove reconciling, retain ready",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "SomeReasonX", "some msg X")
				conditions.MarkTrue(obj, meta.StalledCondition, "SomeReasonY", "some msg Y")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonZ", "some msg Z")
			},
			result: resultStalled,
			recErr: nil,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.StalledCondition, "SomeReasonY", "some msg Y"),
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonZ", "some msg Z"),
			},
		},
		{
			name: "stalled and reconciling, empty ready, remove reconciling, set ready",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "SomeReasonX", "some msg X")
				conditions.MarkTrue(obj, meta.StalledCondition, "SomeReasonY", "some msg Y")
			},
			result: resultStalled,
			recErr: nil,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.StalledCondition, "SomeReasonY", "some msg Y"),
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonY", "some msg Y"),
			},
		},
		{
			name: "stalled and reconciling, Ready=True, remove reconciling, overwrite ready",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "SomeReasonX", "some msg X")
				conditions.MarkTrue(obj, meta.StalledCondition, "SomeReasonY", "some msg Y")
				conditions.MarkTrue(obj, meta.ReadyCondition, "SomeReasonZ", "some msg Z")
			},
			result: resultStalled,
			recErr: nil,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.StalledCondition, "SomeReasonY", "some msg Y"),
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonY", "some msg Y"),
			},
		},
		{
			name: "not success result due to requeue, remove stalled",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonY", "some msg Y")
			},
			result: resultRequeue,
			recErr: nil,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonY", "some msg Y"),
			},
		},
		{
			name: "not success result due to arbitrary requeueAfter, remove stalled",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonY", "some msg Y")
			},
			result: ctrl.Result{RequeueAfter: arbitraryInterval},
			recErr: nil,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonY", "some msg Y"),
			},
		},
		{
			name: "not success result and explicit no requeue, keep stalled, add Ready=False",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
			},
			result: ctrl.Result{Requeue: false},
			recErr: nil,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.StalledCondition, "SomeReasonX", "some msg X"),
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonX", "some msg X"),
			},
		},
		{
			name: "stalled and different Ready=False values",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonY", "some msg Y")
			},
			result: resultStalled,
			recErr: nil,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.StalledCondition, "SomeReasonX", "some msg X"),
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonY", "some msg Y"),
			},
		},
		{
			name: "success result with reconciling and ready, remove reconciling, Ready=True",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkReconciling(obj, "SomeReasonX", "some msg X")
			},
			result:            resultSuccess,
			recErr:            nil,
			statusObservedGen: 1,
			wantErr:           false,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.ReadyCondition, meta.SucceededReason, readySuccessMsg),
			},
		},
		{
			name: "success results but not ready, Ready=False, return error",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, meta.FailedReason, "fail-msg")
			},
			result:  resultSuccess,
			recErr:  nil,
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "fail-msg"),
			},
		},
		{
			name:              "success no other conditions, Ready=True",
			beforeFunc:        func(obj conditions.Setter) {},
			result:            resultSuccess,
			recErr:            nil,
			statusObservedGen: 1,
			wantErr:           false,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.ReadyCondition, meta.SucceededReason, readySuccessMsg),
			},
		},
		{
			name: "reconcile annotation",
			beforeFunc: func(obj conditions.Setter) {
				obj.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: "foo"})
				conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, readySuccessMsg)
			},
			result:                     resultSuccess,
			recErr:                     nil,
			statusObservedGen:          1,
			wantErr:                    false,
			wantLastHandledReconcileAt: "foo",
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.ReadyCondition, meta.SucceededReason, readySuccessMsg),
			},
		},
		// NOTE: The following is a situation in which no Ready condition is
		// present in the status after result computation.
		// {
		// 	name:             "no ready condition",
		// 	result:           resultRequeue,
		// 	recErr:           nil,
		// 	assertConditions: []metav1.Condition{},
		// },
		{
			name:                "success with summarize conditions",
			summarizeConditions: []Conditions{summarizeReadyConditions},
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, artifactInStorageCondition, meta.SucceededReason, "stored artifact")
			},
			result:            resultSuccess,
			recErr:            nil,
			statusObservedGen: 1,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.ReadyCondition, meta.SucceededReason, "stored artifact"),
				*conditions.TrueCondition(artifactInStorageCondition, meta.SucceededReason, "stored artifact"),
			},
		},
		{
			name:                "failure with negative polarity conditions summary",
			summarizeConditions: []Conditions{summarizeReadyConditions},
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, fetchFailedCondition, meta.FailedReason, "auth failed")
			},
			result:  resultFailed,
			recErr:  errors.New("secret not found"),
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "auth failed"),
				*conditions.TrueCondition(fetchFailedCondition, meta.FailedReason, "auth failed"),
			},
		},
		{
			name:                "reconciling and positive polarity conditions summary",
			summarizeConditions: []Conditions{summarizeReadyConditions},
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkReconciling(obj, "NewArtifact", "new artifact")
				conditions.MarkTrue(obj, artifactInStorageCondition, meta.SucceededReason, "stored artifact")
			},
			result:            resultSuccess,
			recErr:            nil,
			statusObservedGen: 1,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.ReadyCondition, meta.SucceededReason, "stored artifact"),
				*conditions.TrueCondition(artifactInStorageCondition, meta.SucceededReason, "stored artifact"),
			},
		},
		{
			name:                "stalled with artifact in storage summary",
			summarizeConditions: []Conditions{summarizeReadyConditions},
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "InvalidURL", "invalid URL")
				conditions.MarkTrue(obj, artifactInStorageCondition, meta.SucceededReason, "stored artifact")
			},
			result:            resultStalled,
			recErr:            nil,
			statusObservedGen: 1,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, "InvalidURL", "invalid URL"),
				*conditions.TrueCondition(meta.StalledCondition, "InvalidURL", "invalid URL"),
				*conditions.TrueCondition(artifactInStorageCondition, meta.SucceededReason, "stored artifact"),
			},
		},
		{
			name:                "reconciling, stalled with conditions summary",
			summarizeConditions: []Conditions{summarizeReadyConditions},
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReconcilingCondition, "SomeReasonX", "some msg X")
				conditions.MarkTrue(obj, meta.StalledCondition, "SomeReasonY", "some msg Y")
			},
			result:            resultStalled,
			recErr:            nil,
			statusObservedGen: 1,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonY", "some msg Y"),
				*conditions.TrueCondition(meta.StalledCondition, "SomeReasonY", "some msg Y"),
			},
		},
		{
			name:                "not ready after summarize and result is success, should set error",
			summarizeConditions: []Conditions{summarizeReadyConditions},
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, artifactOutdatedCondition, meta.FailedReason, "outdated")
			},
			result:  resultSuccess,
			recErr:  nil,
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "outdated"),
				*conditions.TrueCondition(artifactOutdatedCondition, meta.FailedReason, "outdated"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			condns := &conditionscheck.Conditions{
				NegativePolarity: []string{
					meta.StalledCondition,
					meta.ReconcilingCondition,
				},
			}
			checker := conditionscheck.NewChecker(fakeclient.NewClientBuilder().Build(), condns)
			checker.DisableFetch = true

			obj := &testdata.Fake{}
			// Set non-zero generation in order to set valid observed
			// generation in status root and conditions.
			obj.ObjectMeta.Generation = 1
			// Set status.observedGeneration for valid kstatus result.
			obj.Status.ObservedGeneration = tt.statusObservedGen

			if tt.beforeFunc != nil {
				tt.beforeFunc(obj)
			}

			rf := NewResultFinalizer(isSuccess, readySuccessMsg, tt.summarizeConditions...)
			gotErr := rf.Finalize(obj, tt.result, tt.recErr)
			g.Expect(gotErr != nil).To(Equal(tt.wantErr))
			g.Expect(obj.Status.Conditions).To(conditions.MatchConditions(tt.assertConditions))
			if tt.wantLastHandledReconcileAt != "" {
				g.Expect(obj.Status.LastHandledReconcileAt).To(Equal(tt.wantLastHandledReconcileAt))
			}
			// kstatus comformance check.
			checker.CheckErr(context.TODO(), obj)
		})
	}
}

// Same as the above test but for SuccessNoRequest type reconciler.
func TestResultFinalizer_successNoRequeue(t *testing.T) {
	readySuccessMsg := "Success"
	resultSuccess := ctrl.Result{}
	resultStalled := ctrl.Result{}

	isSuccess := func(r ctrl.Result, err error) bool {
		if err != nil || r.RequeueAfter != 0 || r.Requeue {
			return false
		}
		return true
	}

	tests := []struct {
		name                       string
		beforeFunc                 func(obj conditions.Setter)
		result                     ctrl.Result
		recErr                     error
		statusObservedGen          int64
		wantErr                    bool
		wantLastHandledReconcileAt string
		assertConditions           []metav1.Condition
	}{
		{
			name: "result with error and stalled",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
			},
			result:  resultStalled,
			recErr:  errors.New("foo failed"),
			wantErr: true,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "foo failed"),
			},
		},
		{
			name: "stalled, Ready=True, overwrite ready",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
			},
			result:  resultStalled,
			wantErr: false,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, "SomeReasonX", "some msg X"),
				*conditions.TrueCondition(meta.StalledCondition, "SomeReasonX", "some msg X"),
			},
		},
		{
			name: "success result",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "some msg X")
			},
			result:            resultSuccess,
			statusObservedGen: 1,
			wantErr:           false,
			assertConditions: []metav1.Condition{
				*conditions.TrueCondition(meta.ReadyCondition, meta.SucceededReason, "some msg X"),
			},
		},
		{
			name: "success result but not ready, Ready=False, no error",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkFalse(obj, meta.ReadyCondition, meta.FailedReason, "fail-msg")
			},
			result:  resultSuccess,
			recErr:  nil,
			wantErr: false,
			assertConditions: []metav1.Condition{
				*conditions.FalseCondition(meta.ReadyCondition, meta.FailedReason, "fail-msg"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			condns := &conditionscheck.Conditions{
				NegativePolarity: []string{
					meta.StalledCondition,
					meta.ReconcilingCondition,
				},
			}
			checker := conditionscheck.NewChecker(fakeclient.NewClientBuilder().Build(), condns)
			checker.DisableFetch = true

			obj := &testdata.Fake{}
			obj.ObjectMeta.Generation = 1
			obj.Status.ObservedGeneration = tt.statusObservedGen

			if tt.beforeFunc != nil {
				tt.beforeFunc(obj)
			}

			rf := NewResultFinalizer(isSuccess, readySuccessMsg)
			gotErr := rf.Finalize(obj, tt.result, tt.recErr)
			g.Expect(gotErr != nil).To(Equal(tt.wantErr))
			g.Expect(obj.Status.Conditions).To(conditions.MatchConditions(tt.assertConditions))
			if tt.wantLastHandledReconcileAt != "" {
				g.Expect(obj.Status.LastHandledReconcileAt).To(Equal(tt.wantLastHandledReconcileAt))
			}
			// kstatus comformance check.
			checker.CheckErr(context.TODO(), obj)
		})
	}
}

func TestAddPatchOptions(t *testing.T) {
	tests := []struct {
		name                         string
		beforeFunc                   func(obj conditions.Setter)
		fieldOwner                   string
		ownedConditions              []string
		wantFieldOwner               string
		wantOwnedConditions          []string
		wantIncludeStatusObservedGen bool
	}{
		{
			name:                         "no conditions, no field owner",
			wantFieldOwner:               "",
			wantOwnedConditions:          nil,
			wantIncludeStatusObservedGen: false,
		},
		{
			name:                "owned conditions and field owner",
			fieldOwner:          "foo-ctrl",
			ownedConditions:     []string{"A", "B"},
			wantFieldOwner:      "foo-ctrl",
			wantOwnedConditions: []string{"A", "B"},
		},
		{
			name: "reconciling=True, Ready=False status conditions",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkReconciling(obj, "SomeReasonX", "some msg X")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonY", "some msg Y")
			},
			wantIncludeStatusObservedGen: false,
		},
		{
			name: "stalled=True, Ready=False status conditions",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonY", "some msg Y")
			},
			wantIncludeStatusObservedGen: true,
		},
		{
			name: "Ready=True, no other status condition",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "success")
			},
			wantIncludeStatusObservedGen: true,
		},
		{
			name: "owned conditions, field owner and Stalled=True, Ready=False",
			beforeFunc: func(obj conditions.Setter) {
				conditions.MarkStalled(obj, "SomeReasonX", "some msg X")
				conditions.MarkFalse(obj, meta.ReadyCondition, "SomeReasonY", "some msg Y")
			},
			fieldOwner:                   "foo-ctrl",
			ownedConditions:              []string{meta.StalledCondition, meta.ReadyCondition},
			wantFieldOwner:               "foo-ctrl",
			wantOwnedConditions:          []string{meta.StalledCondition, meta.ReadyCondition},
			wantIncludeStatusObservedGen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &testdata.Fake{}
			opts := []patch.Option{}

			if tt.beforeFunc != nil {
				tt.beforeFunc(obj)
			}

			opts = AddPatchOptions(obj, opts, tt.ownedConditions, tt.fieldOwner)

			// Apply the options on a patch helper.
			helperOpts := &patch.HelperOptions{}
			for _, opt := range opts {
				opt.ApplyToHelper(helperOpts)
			}
			g.Expect(helperOpts.FieldOwner).To(Equal(tt.wantFieldOwner))
			g.Expect(helperOpts.OwnedConditions).To(Equal(tt.wantOwnedConditions))
			g.Expect(helperOpts.IncludeStatusObservedGeneration).To(Equal(tt.wantIncludeStatusObservedGen))
		})
	}
}

func TestDetermineSuccessType(t *testing.T) {
	testRequeuePeriod := time.Minute

	tests := []struct {
		name      string
		isSuccess IsResultSuccess
		want      SuccessType
	}{
		{
			name: "requeuing reconciler",
			isSuccess: func(r ctrl.Result, err error) bool {
				if err != nil || r.RequeueAfter != testRequeuePeriod || r.Requeue {
					return false
				}
				return true
			},
			want: SuccessWithRequeue,
		},
		{
			name: "no requeue reconciler",
			isSuccess: func(r ctrl.Result, err error) bool {
				if err != nil || r.RequeueAfter != 0 || r.Requeue {
					return false
				}
				return true
			},
			want: SuccessNoRequeue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := determineSuccessType(tt.isSuccess)
			g.Expect(result).To(Equal(tt.want))
		})
	}
}
