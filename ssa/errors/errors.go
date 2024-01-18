/*
Copyright 2023 The Flux authors

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

package errors

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fluxcd/pkg/ssa/utils"
)

// DryRunErr is an error that occurs during a server-side dry-run apply.
type DryRunErr struct {
	underlyingErr  error
	involvedObject *unstructured.Unstructured
}

// NewDryRunErr returns a new DryRunErr.
func NewDryRunErr(err error, involvedObject *unstructured.Unstructured) *DryRunErr {
	return &DryRunErr{
		underlyingErr:  err,
		involvedObject: involvedObject,
	}
}

// InvolvedObject returns the involved object.
func (e *DryRunErr) InvolvedObject() *unstructured.Unstructured {
	return e.involvedObject
}

// Error returns the error message.
func (e *DryRunErr) Error() string {
	if e.involvedObject == nil {
		return e.underlyingErr.Error()
	}

	if apierrors.IsNotFound(e.Unwrap()) {
		if e.involvedObject.GetNamespace() == "" {
			return fmt.Sprintf("%s namespace not specified: %s", utils.FmtUnstructured(e.involvedObject), e.Unwrap().Error())
		}
		return fmt.Sprintf("%s not found: %s", utils.FmtUnstructured(e.involvedObject), e.Unwrap().Error())
	}

	reason := string(apierrors.ReasonForError(e.Unwrap()))

	// Detect managed field conflict.
	if status, ok := apierrors.StatusCause(e.Unwrap(), metav1.CauseTypeFieldManagerConflict); ok {
		reason = string(status.Type)
	}

	if reason != "" {
		reason = fmt.Sprintf(" (%s)", reason)
	}

	return fmt.Sprintf("%s dry-run failed%s: %s", utils.FmtUnstructured(e.involvedObject), reason, e.underlyingErr.Error())
}

// Unwrap returns the underlying error.
func (e *DryRunErr) Unwrap() error {
	return e.underlyingErr
}
