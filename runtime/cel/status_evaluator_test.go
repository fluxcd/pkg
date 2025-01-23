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

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/runtime/cel"
)

func TestNewStatusEvaluator(t *testing.T) {
	for _, tt := range []struct {
		name  string
		exprs kustomize.HealthCheckExpressions
		err   string
	}{
		{
			name: "all expressions are present",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.inProgress",
				Failed:     "data.failed",
			},
		},
		{
			name: "InProgress and Failed are optional",
			exprs: kustomize.HealthCheckExpressions{
				Current: "data.current",
			},
		},
		{
			name: "Current is required",
			err:  "expression Current not specified",
		},
		{
			name: "errors if Current is invalid",
			exprs: kustomize.HealthCheckExpressions{
				Current: "data.",
			},
			err: "failed to parse the expression Current",
		},
		{
			name: "errors if InProgress is invalid",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.",
			},
			err: "failed to parse the expression InProgress",
		},
		{
			name: "errors if Failed is invalid",
			exprs: kustomize.HealthCheckExpressions{
				Current: "data.current",
				Failed:  "data.",
			},
			err: "failed to parse the expression Failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			s, err := cel.NewStatusEvaluator(&tt.exprs)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(s).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(s).NotTo(BeNil())
			}
		})
	}
}

func TestStatusEvaluator_Evaluate(t *testing.T) {
	for _, tt := range []struct {
		name   string
		exprs  kustomize.HealthCheckExpressions
		obj    map[string]any
		result status.Result
		err    string
	}{
		{
			name: "observed generation exists and is different",
			exprs: kustomize.HealthCheckExpressions{
				Current: "true",
			},
			obj: map[string]any{
				"metadata": map[string]any{
					"generation": int64(2),
				},
				"status": map[string]any{
					"observedGeneration": int64(1),
				},
			},
			result: status.Result{Status: status.InProgressStatus},
		},
		{
			name: "if Current returns an error, the error is returned",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.currentt",
				InProgress: "data.inProgress",
				Failed:     "data.failed",
			},
			obj: map[string]any{"data": map[string]any{
				"current":    true,
				"inProgress": false,
				"failed":     false,
			}},
			err: "failed to evaluate the CEL expression 'data.currentt': no such key: currentt",
		},
		{
			name: "if InProgress returns an error, the error is returned",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.inProgresss",
				Failed:     "data.failed",
			},
			obj: map[string]any{"data": map[string]any{
				"current":    true,
				"inProgress": false,
				"failed":     false,
			}},
			err: "failed to evaluate the CEL expression 'data.inProgresss': no such key: inProgresss",
		},
		{
			name: "if Failed returns an error, the error is returned",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.inProgress",
				Failed:     "data.failedd",
			},
			obj: map[string]any{"data": map[string]any{
				"current":    true,
				"inProgress": false,
				"failed":     false,
			}},
			err: "failed to evaluate the CEL expression 'data.failedd': no such key: failedd",
		},
		{
			name: "if all expressions evaluate to false then the object is in progress",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.inProgress",
				Failed:     "data.failed",
			},
			obj: map[string]any{"data": map[string]any{
				"current":    false,
				"inProgress": false,
				"failed":     false,
			}},
			result: status.Result{Status: status.InProgressStatus},
		},
		{
			name: "if all expressions evaluate to true then the object is in progress",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.inProgress",
				Failed:     "data.failed",
			},
			obj: map[string]any{"data": map[string]any{
				"current":    true,
				"inProgress": true,
				"failed":     true,
			}},
			result: status.Result{Status: status.InProgressStatus},
		},
		{
			name: "if both Current and Failed evaluate to true, then the object failed",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.inProgress",
				Failed:     "data.failed",
			},
			obj: map[string]any{"data": map[string]any{
				"current":    true,
				"inProgress": false,
				"failed":     true,
			}},
			result: status.Result{Status: status.FailedStatus},
		},
		{
			name: "if only Current evaluates to true, then the object is current",
			exprs: kustomize.HealthCheckExpressions{
				Current:    "data.current",
				InProgress: "data.inProgress",
				Failed:     "data.failed",
			},
			obj: map[string]any{"data": map[string]any{
				"current":    true,
				"inProgress": false,
				"failed":     false,
			}},
			result: status.Result{Status: status.CurrentStatus},
		},
		{
			name: "object without status with inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				InProgress: "has(status.observedGeneration) && metadata.generation != status.observedGeneration",
				Failed:     "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current:    "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{},
			err: "failed to evaluate the CEL expression 'has(status.observedGeneration) && metadata.generation != status.observedGeneration': no such attribute(s): status.observedGeneration",
		},
		{
			name: "object without status without inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				Failed:  "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current: "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{},
			err: "failed to evaluate the CEL expression 'status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')': no such attribute(s): status.conditions",
		},
		{
			name: "object with status without status.conditions with inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				InProgress: "has(status.observedGeneration) && metadata.generation != status.observedGeneration",
				Failed:     "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current:    "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{"status": map[string]any{}},
			err: "failed to evaluate the CEL expression 'status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')': no such key: conditions",
		},
		{
			name: "object with status without status.conditions without inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				Failed:  "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current: "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{"status": map[string]any{}},
			err: "failed to evaluate the CEL expression 'status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')': no such key: conditions",
		},
		{
			name: "object with status with empty status.conditions with inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				InProgress: "has(status.observedGeneration) && metadata.generation != status.observedGeneration",
				Failed:     "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current:    "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{
				"status": map[string]any{
					"conditions": []any{},
				},
			},
			result: status.Result{Status: status.FailedStatus},
		},
		{
			name: "object with status with empty status.conditions without inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				Failed:  "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current: "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{
				"status": map[string]any{
					"conditions": []any{},
				},
			},
			result: status.Result{Status: status.FailedStatus},
		},
		{
			name: "object with status.observedGeneration with inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				InProgress: "has(status.observedGeneration) && metadata.generation != status.observedGeneration",
				Failed:     "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current:    "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{
				"metadata": map[string]any{
					"generation": int64(2),
				},
				"status": map[string]any{
					"observedGeneration": int64(1),
				},
			},
			result: status.Result{Status: status.InProgressStatus},
		},
		{
			name: "object with status.observedGeneration without inProgress expression",
			exprs: kustomize.HealthCheckExpressions{
				Failed:  "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'False')",
				Current: "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')",
			},
			obj: map[string]any{
				"metadata": map[string]any{
					"generation": int64(2),
				},
				"status": map[string]any{
					"observedGeneration": int64(1),
				},
			},
			result: status.Result{Status: status.InProgressStatus},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			e, err := cel.NewStatusEvaluator(&tt.exprs)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := e.Evaluate(context.Background(), &unstructured.Unstructured{Object: tt.obj})
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*result).To(Equal(tt.result))
			}
		})
	}
}
