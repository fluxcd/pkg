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

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
)

// Expression represents a parsed CEL expression.
type Expression struct {
	expr string
	prog cel.Program
}

// Option is a function that configures the CEL expression.
type Option func(*options)

type options struct {
	variables  []cel.EnvOption
	compile    bool
	outputType *cel.Type
}

// WithStructVariables declares variables of type google.protobuf.Struct.
func WithStructVariables(vars ...string) Option {
	return func(o *options) {
		for _, v := range vars {
			d := cel.Variable(v, cel.ObjectType("google.protobuf.Struct"))
			o.variables = append(o.variables, d)
		}
	}
}

// WithCompile specifies that the expression should be compiled,
// which provides stricter checks at parse time, before evaluation.
func WithCompile() Option {
	return func(o *options) {
		o.compile = true
	}
}

// WithOutputType specifies the expected output type of the expression.
func WithOutputType(t *cel.Type) Option {
	return func(o *options) {
		o.outputType = t
	}
}

// NewExpression parses the given CEL expression and returns a new Expression.
func NewExpression(expr string, opts ...Option) (*Expression, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if !o.compile && (o.outputType != nil || len(o.variables) > 0) {
		return nil, fmt.Errorf("output type and variables can only be set when compiling the expression")
	}

	envOpts := append([]cel.EnvOption{
		cel.HomogeneousAggregateLiterals(),
		cel.EagerlyValidateDeclarations(true),
		cel.DefaultUTCTimeZone(true),
		cel.CrossTypeNumericComparisons(true),
		cel.OptionalTypes(),
		ext.Strings(),
		ext.Sets(),
		ext.Encoders(),
	}, o.variables...)

	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	parse := env.Parse
	if o.compile {
		parse = env.Compile
	}
	e, issues := parse(expr)
	if issues != nil {
		return nil, fmt.Errorf("failed to parse the CEL expression '%s': %s", expr, issues.String())
	}

	if w, g := o.outputType, e.OutputType(); w != nil && w != g {
		return nil, fmt.Errorf("CEL expression output type mismatch: expected %s, got %s", w, g)
	}

	progOpts := []cel.ProgramOption{
		cel.EvalOptions(cel.OptOptimize),

		// 100 is the kubernetes default:
		// https://github.com/kubernetes/kubernetes/blob/3f26d005571dc5903e7cebae33ada67986bc40f3/staging/src/k8s.io/apiserver/pkg/apis/cel/config.go#L33-L35
		cel.InterruptCheckFrequency(100),
	}

	prog, err := env.Program(e, progOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	return &Expression{
		expr: expr,
		prog: prog,
	}, nil
}

// EvaluateBoolean evaluates the expression with the given data and returns the result as a boolean.
func (e *Expression) EvaluateBoolean(ctx context.Context, data map[string]any) (bool, error) {
	val, _, err := e.prog.ContextEval(ctx, data)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate the CEL expression '%s': %w", e.expr, err)
	}
	result, ok := val.(types.Bool)
	if !ok {
		return false, fmt.Errorf("failed to evaluate CEL expression as boolean: '%s'", e.expr)
	}
	return bool(result), nil
}
