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

package dependency

import (
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	celtypes "github.com/google/cel-go/common/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/cel"
	"github.com/fluxcd/pkg/runtime/conditions"
)

const (
	selfName = "self"
	depName  = "dep"
)

// CheckOption configures CheckDependencies behavior.
type CheckOption func(*checkOptions)

type checkOptions struct {
	additiveCEL bool
}

// WithAdditiveCEL configures CheckDependencies to run both the CEL
// expression evaluation and the built-in kstatus readiness check.
func WithAdditiveCEL() CheckOption {
	return func(o *checkOptions) {
		o.additiveCEL = true
	}
}

// BuildDependencyExpressions parses the ReadyExpr of each dependency
// declared by obj and returns a slice aligned to obj.GetDependsOn().
// Entries without a ReadyExpr are nil. Each expression has access to
// self (the parent) and dep (the dependency) as struct variables.
//
// An invalid expression is returned as a reconcile.TerminalError, as it
// cannot recover without a change of spec. Callers should detect it with
// errors.Is and mark the object as stalled instead of retrying.
func BuildDependencyExpressions(obj Dependent) ([]*cel.Expression, error) {
	exprs := make([]*cel.Expression, len(obj.GetDependsOn()))
	for i, dep := range obj.GetDependsOn() {
		if dep.ReadyExpr == "" {
			continue
		}
		expr, err := cel.NewExpression(dep.ReadyExpr,
			cel.WithCompile(),
			cel.WithOutputType(celtypes.BoolType),
			cel.WithStructVariables(selfName, depName),
		)
		if err != nil {
			return nil, reconcile.TerminalError(fmt.Errorf("failed to parse expression for dependency %s: %w", dep, err))
		}
		exprs[i] = expr
	}
	return exprs, nil
}

// CheckDependencies verifies every dependency declared by obj exists and is ready.
//
// A dependency with a non-empty ReadyExpr is evaluated using that CEL
// expression. By default, the ReadyExpr replaces the kstatus check; use
// WithAdditiveCEL to run both. A dependency with an empty ReadyExpr
// falls back to the kstatus check, plus a same-kind Ready-condition
// check because kstatus.Compute() tolerates missing conditions.
//
// An invalid ReadyExpr is returned as a reconcile.TerminalError; all
// other errors are transient and callers should retry.
//
// The given client should preferably be an uncached reader (e.g. the
// manager's APIReader), so that dependency status is observed without
// cache lag and no informers are started for arbitrary dependency kinds.
//
// Controller-specific semantics (e.g. source-revision equality) are not
// handled here; callers should check them after this returns nil.
func CheckDependencies(
	ctx context.Context,
	c ctrlclient.Reader,
	obj *unstructured.Unstructured,
	opts ...CheckOption,
) error {
	var o checkOptions
	for _, opt := range opts {
		opt(&o)
	}

	deps, err := getDependsOn(obj)
	if err != nil {
		return err
	}

	unstructuredObj := &unstructuredDependent{obj, deps}
	exprs, err := BuildDependencyExpressions(unstructuredObj)
	if err != nil {
		return err
	}

	for i, dep := range deps {
		dep = ApplyDependencyDefaults(unstructuredObj, dep)
		depObj, err := FetchDependency(ctx, c, dep)
		if err != nil {
			return err
		}

		if dep.ReadyExpr != "" {
			if err := EvaluateCEL(ctx, obj, depObj, exprs[i]); err != nil {
				return err
			}
			if !o.additiveCEL {
				continue
			}
		}

		stat, err := status.Compute(depObj)
		if err != nil {
			return fmt.Errorf("dependency %s is not ready: %w", dep, err)
		}
		if stat.Status != status.CurrentStatus {
			return fmt.Errorf("dependency %s is not ready: status %s", dep, stat.Status)
		}

		// kstatus.Compute() tolerates missing conditions, so verify the Ready
		// condition explicitly for same-Kind dependencies.
		if dep.APIVersion != obj.GetAPIVersion() || dep.Kind != obj.GetKind() {
			continue
		}
		if !conditions.IsTrue(conditions.UnstructuredGetter(depObj), meta.ReadyCondition) {
			return fmt.Errorf("dependency %s is not ready", dep)
		}
	}

	return nil
}

// getDependsOn extracts spec.dependsOn from an unstructured object.
func getDependsOn(obj *unstructured.Unstructured) ([]meta.DependencyReference, error) {
	rawSpec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("failed to read spec: %w", err)
	}
	if !found {
		return nil, nil
	}

	var spec struct {
		DependsOn []meta.DependencyReference `json:"dependsOn"`
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawSpec, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}
	return spec.DependsOn, nil
}

// ApplyDependencyDefaults applies defaults to dep: Kind defaults to the
// parent's, then APIVersion and Namespace default to the dependent's
// when Kind matches.
func ApplyDependencyDefaults(obj Dependent, dep meta.DependencyReference) meta.DependencyReference {
	if dep.Kind == "" {
		dep.Kind = obj.GetKind()
	}
	if dep.Kind != obj.GetKind() {
		return dep
	}
	if dep.APIVersion == "" {
		dep.APIVersion = obj.GetAPIVersion()
	}
	if dep.Namespace == "" {
		dep.Namespace = obj.GetNamespace()
	}
	return dep
}

// FetchDependency retrieves the dependency object from the cluster.
func FetchDependency(
	ctx context.Context,
	c ctrlclient.Reader,
	dep meta.DependencyReference,
) (*unstructured.Unstructured, error) {
	depObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": dep.APIVersion,
			"kind":       dep.Kind,
			"metadata": map[string]any{
				"name":      dep.Name,
				"namespace": dep.Namespace,
			},
		},
	}

	if err := c.Get(ctx, ctrlclient.ObjectKeyFromObject(depObj), depObj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("dependency %s not found: %w", dep, err)
		}
		return nil, fmt.Errorf("failed to get dependency %s: %w", dep, err)
	}
	return depObj, nil
}

// EvaluateCEL runs the dep's ReadyExpr with the parent and dependency
// objects as struct variables, and returns nil if it evaluates to true.
func EvaluateCEL(
	ctx context.Context,
	obj *unstructured.Unstructured,
	dep *unstructured.Unstructured,
	expr *cel.Expression,
) error {
	depID := fmt.Sprintf("%s/%s/%s/%s",
		dep.GetAPIVersion(), dep.GetKind(), dep.GetNamespace(), dep.GetName())
	vars := map[string]any{
		selfName: obj.UnstructuredContent(),
		depName:  dep.UnstructuredContent(),
	}

	ready, err := expr.EvaluateBoolean(ctx, vars)
	if err != nil {
		return fmt.Errorf("failed to evaluate dependency %s: %w", depID, err)
	}
	if !ready {
		return fmt.Errorf("dependency %s is not ready according to readyExpr eval", depID)
	}
	return nil
}
