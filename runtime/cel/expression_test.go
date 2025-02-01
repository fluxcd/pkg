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

package cel_test

import (
	"context"
	"testing"

	celgo "github.com/google/cel-go/cel"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/runtime/cel"
)

func TestNewExpression(t *testing.T) {
	for _, tt := range []struct {
		name string
		expr string
		opts []cel.Option
		err  string
	}{
		{
			name: "valid expression",
			expr: "foo",
		},
		{
			name: "invalid expression",
			expr: "foo.",
			err:  "failed to parse the CEL expression 'foo.': ERROR: <input>:1:5: Syntax error: no viable alternative at input '.'",
		},
		{
			name: "compilation detects undeclared references",
			expr: "foo",
			opts: []cel.Option{cel.WithCompile()},
			err:  "failed to parse the CEL expression 'foo': ERROR: <input>:1:1: undeclared reference to 'foo'",
		},
		{
			name: "compilation detects type errors",
			expr: "foo == 'bar'",
			opts: []cel.Option{cel.WithCompile(), cel.WithStructVariables("foo")},
			err:  "failed to parse the CEL expression 'foo == 'bar'': ERROR: <input>:1:5: found no matching overload for '_==_' applied to '(map(string, dyn), string)'",
		},
		{
			name: "can't check output type without compiling",
			expr: "foo",
			opts: []cel.Option{cel.WithOutputType(celgo.BoolType)},
			err:  "output type and variables can only be set when compiling the expression",
		},
		{
			name: "can't declare variables without compiling",
			expr: "foo",
			opts: []cel.Option{cel.WithStructVariables("foo")},
			err:  "output type and variables can only be set when compiling the expression",
		},
		{
			name: "compilation checks output type",
			expr: "'foo'",
			opts: []cel.Option{cel.WithCompile(), cel.WithOutputType(celgo.BoolType)},
			err:  "CEL expression output type mismatch: expected bool, got string",
		},
		{
			name: "compilation checking output type can't predict type of struct field",
			expr: "foo.bar.baz",
			opts: []cel.Option{cel.WithCompile(), cel.WithStructVariables("foo"), cel.WithOutputType(celgo.BoolType)},
			err:  "CEL expression output type mismatch: expected bool, got dyn",
		},
		{
			name: "compilation checking output type can't predict type of struct field, but if it's a boolean it can be compared to a boolean literal",
			expr: "foo.bar.baz == true",
			opts: []cel.Option{cel.WithCompile(), cel.WithStructVariables("foo"), cel.WithOutputType(celgo.BoolType)},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			e, err := cel.NewExpression(tt.expr, tt.opts...)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(e).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(e).NotTo(BeNil())
			}
		})
	}
}

func TestExpression_EvaluateBoolean(t *testing.T) {
	for _, tt := range []struct {
		name   string
		expr   string
		opts   []cel.Option
		data   map[string]any
		result bool
		err    string
	}{
		{
			name: "inexistent field",
			expr: "foo",
			data: map[string]any{},
			err:  "failed to evaluate the CEL expression 'foo': no such attribute(s): foo",
		},
		{
			name:   "boolean field true",
			expr:   "foo",
			data:   map[string]any{"foo": true},
			result: true,
		},
		{
			name:   "boolean field false",
			expr:   "foo",
			data:   map[string]any{"foo": false},
			result: false,
		},
		{
			name:   "nested boolean field true",
			expr:   "foo.bar",
			data:   map[string]any{"foo": map[string]any{"bar": true}},
			result: true,
		},
		{
			name:   "nested boolean field false",
			expr:   "foo.bar",
			data:   map[string]any{"foo": map[string]any{"bar": false}},
			result: false,
		},
		{
			name:   "boolean literal true",
			expr:   "true",
			data:   map[string]any{},
			result: true,
		},
		{
			name:   "boolean literal false",
			expr:   "false",
			data:   map[string]any{},
			result: false,
		},
		{
			name: "non-boolean literal",
			expr: "'some-value'",
			data: map[string]any{},
			err:  "failed to evaluate CEL expression ''some-value'' as bool: types.String",
		},
		{
			name: "non-boolean field",
			expr: "foo",
			data: map[string]any{"foo": "some-value"},
			err:  "failed to evaluate CEL expression 'foo' as bool: types.String",
		},
		{
			name: "nested non-boolean field",
			expr: "foo.bar",
			data: map[string]any{"foo": map[string]any{"bar": "some-value"}},
			err:  "failed to evaluate CEL expression 'foo.bar' as bool: types.String",
		},
		{
			name:   "complex expression evaluating true",
			expr:   "foo && bar",
			data:   map[string]any{"foo": true, "bar": true},
			result: true,
		},
		{
			name:   "complex expression evaluating false",
			expr:   "foo && bar",
			data:   map[string]any{"foo": true, "bar": false},
			result: false,
		},
		{
			name:   "compiled expression returning true",
			expr:   "foo.bar",
			opts:   []cel.Option{cel.WithCompile(), cel.WithStructVariables("foo")},
			data:   map[string]any{"foo": map[string]any{"bar": true}},
			result: true,
		},
		{
			name:   "compiled expression returning false",
			expr:   "foo.bar",
			opts:   []cel.Option{cel.WithCompile(), cel.WithStructVariables("foo")},
			data:   map[string]any{"foo": map[string]any{"bar": false}},
			result: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			e, err := cel.NewExpression(tt.expr, tt.opts...)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := e.EvaluateBoolean(context.Background(), tt.data)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tt.result))
			}
		})
	}
}

func TestExpression_EvaluateBytes(t *testing.T) {
	for _, tt := range []struct {
		name   string
		expr   string
		opts   []cel.Option
		data   map[string]any
		result string
		err    string
	}{
		{
			name: "YAML values from Secret",
			expr: "data['values.yaml']",
			data: map[string]any{
				"data": map[string]any{
					"values.yaml": []byte("foo:\n  bar: baz"),
				},
			},
			result: "foo:\n  bar: baz",
		},
		{
			name: "YAML values from ConfigMap",
			expr: "data['values.yaml']",
			data: map[string]any{
				"data": map[string]any{
					"values.yaml": "foo:\n  bar: baz",
				},
			},
			result: "foo:\n  bar: baz",
		},
		{
			name: "marshaled YAML values from Deployment (function style)",
			expr: "yaml(spec)",
			data: map[string]any{
				"spec": map[string]any{
					"replicas": 3,
				},
			},
			result: "replicas: 3\n",
		},
		{
			name: "marshaled YAML values from Deployment (method style)",
			expr: "spec.yaml()",
			data: map[string]any{
				"spec": map[string]any{
					"replicas": 3,
				},
			},
			result: "replicas: 3\n",
		},
		{
			name: "crafted YAML values from Deployment",
			expr: "'replicas: ' + string(spec.replicas)",
			data: map[string]any{
				"spec": map[string]any{
					"replicas": 3,
				},
			},
			result: "replicas: 3",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			e, err := cel.NewExpression(tt.expr, tt.opts...)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := e.EvaluateBytes(context.Background(), tt.data)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(result)).To(Equal(tt.result))
			}
		})
	}
}
