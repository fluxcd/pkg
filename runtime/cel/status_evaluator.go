/*
Copyright 2025 The Flux authors

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

package cel

import (
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fluxcd/pkg/apis/kustomize"
)

// StatusEvaluator evaluates the health status of a custom resource object.
type StatusEvaluator struct {
	current    *Expression
	failed     *Expression
	inProgress *Expression
}

// NewStatusEvaluator returns a new StatusEvaluator.
func NewStatusEvaluator(exprs *kustomize.HealthCheckExpressions) (*StatusEvaluator, error) {
	// we can't use the options WithCompile and WithStructVariables here
	// because not all CRDs follow the standard five top-level fields:
	//   apiVersion, kind, metadata, spec and status
	// and because we can't use WithCompile, we also can't use
	//   WithOutputType(celgo.BoolType)

	if exprs.Current == "" {
		return nil, fmt.Errorf("expression Current not specified")
	}
	current, err := NewExpression(exprs.Current)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the expression Current: %w", err)
	}

	var failed *Expression
	if exprs.Failed != "" {
		failed, err = NewExpression(exprs.Failed)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the expression Failed: %w", err)
		}
	}

	var inProgress *Expression
	if exprs.InProgress != "" {
		inProgress, err = NewExpression(exprs.InProgress)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the expression InProgress: %w", err)
		}
	}

	return &StatusEvaluator{
		current:    current,
		failed:     failed,
		inProgress: inProgress,
	}, nil
}

// Evaluate evaluates the health status of a custom resource object
// according to the rules defined in RFC 0009:
//
// First we check if the object has the field status.observedGeneration. If it does,
// and the value is different from metadata.generation, we return the status InProgress.
//
// Then we evaluate the healthcheck expressions in the following order:
// - InProgress: if true, return status InProgress
// - Failed: if true, return status Failed
// - Current: if true, return status Current
//
// If none of the expressions are true, we return status InProgress.
func (s *StatusEvaluator) Evaluate(ctx context.Context, u *unstructured.Unstructured) (*status.Result, error) {
	unsObj := u.UnstructuredContent()

	// Check if the object has the field status.observedGeneration
	// and if it differs from metadata.generation, in which case we
	// return status InProgress.
	observedGeneration, ok, err := unstructured.NestedInt64(unsObj, "status", "observedGeneration")
	if err != nil {
		return nil, err
	}
	if ok {
		generation, ok, err := unstructured.NestedInt64(unsObj, "metadata", "generation")
		if err != nil {
			return nil, err
		}
		if ok && observedGeneration != generation {
			return &status.Result{Status: status.InProgressStatus}, nil
		}
	}

	// Evaluate the healthcheck expressions.
	for _, e := range []struct {
		expr   *Expression
		status status.Status
	}{
		// This order is defined in RFC 0009.
		{expr: s.inProgress, status: status.InProgressStatus},
		{expr: s.failed, status: status.FailedStatus},
		{expr: s.current, status: status.CurrentStatus},
	} {
		if e.expr == nil {
			continue
		}
		result, err := e.expr.EvaluateBoolean(ctx, unsObj)
		if err != nil {
			return nil, err
		}
		if result {
			return &status.Result{Status: e.status}, nil
		}
	}

	return &status.Result{Status: status.InProgressStatus}, nil
}
