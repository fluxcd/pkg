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
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/clusterreader"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/runtime/cel"
)

func TestStatusReader_Supports(t *testing.T) {
	for _, tt := range []struct {
		name        string
		supportedGK schema.GroupKind
		gk          schema.GroupKind
		result      bool
	}{
		{
			name: "supported",
			supportedGK: schema.GroupKind{
				Group: "test",
				Kind:  "Test",
			},
			gk: schema.GroupKind{
				Group: "test",
				Kind:  "Test",
			},
			result: true,
		},
		{
			name: "unsupported",
			supportedGK: schema.GroupKind{
				Group: "test",
				Kind:  "Test",
			},
			gk: schema.GroupKind{
				Group: "test",
				Kind:  "Unsupported",
			},
			result: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			se, err := cel.NewStatusEvaluator(&kustomize.HealthCheckExpressions{
				Current: "something",
			})
			g.Expect(err).NotTo(HaveOccurred())

			sr := cel.NewStatusReader(context.Background(), nil, tt.supportedGK, se)

			result := sr.Supports(tt.gk)
			g.Expect(result).To(Equal(tt.result))
		})
	}
}

func TestStatusReader_ReadStatus(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mapper := testEnv.GetRESTMapper()
	clusterReader := &clusterreader.DirectClusterReader{Reader: testEnv.GetClient()}

	gk := schema.GroupKind{
		Group: "",
		Kind:  "ConfigMap",
	}

	ns, err := testEnv.CreateNamespace(ctx, "test-namespace")
	g.Expect(err).NotTo(HaveOccurred())
	objNamespace := ns.Name

	const objName = "test-configmap"
	err = testEnv.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objName,
			Namespace: objNamespace,
		},
		Data: map[string]string{
			"current":    "true",
			"inProgress": "true",
			"failed":     "true",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	for _, tt := range []struct {
		name   string
		exprs  kustomize.HealthCheckExpressions
		status status.Status
	}{
		{
			name: "current",
			exprs: kustomize.HealthCheckExpressions{
				Current: "data.current == 'true'",
			},
			status: status.CurrentStatus,
		},
		{
			name: "in progress",
			exprs: kustomize.HealthCheckExpressions{
				InProgress: "data.inProgress == 'true'",
				Current:    "data.current == 'true'",
			},
			status: status.InProgressStatus,
		},
		{
			name: "failed",
			exprs: kustomize.HealthCheckExpressions{
				Failed:  "data.failed == 'true'",
				Current: "data.current == 'true'",
			},
			status: status.FailedStatus,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			se, err := cel.NewStatusEvaluator(&tt.exprs)
			g.Expect(err).NotTo(HaveOccurred())

			sr := cel.NewStatusReader(context.Background(), mapper, gk, se)

			result, err := sr.ReadStatus(ctx, clusterReader, object.ObjMetadata{
				Name:      objName,
				Namespace: objNamespace,
				GroupKind: gk,
			})

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Status).To(Equal(tt.status))
		})
	}
}

func TestStatusReader_ReadStatusForObject(t *testing.T) {
	gk := schema.GroupKind{
		Group: "",
		Kind:  "ConfigMap",
	}

	for _, tt := range []struct {
		name   string
		exprs  kustomize.HealthCheckExpressions
		status status.Status
	}{
		{
			name: "current",
			exprs: kustomize.HealthCheckExpressions{
				Current: "data.current",
			},
			status: status.CurrentStatus,
		},
		{
			name: "in progress",
			exprs: kustomize.HealthCheckExpressions{
				InProgress: "data.inProgress",
				Current:    "data.current",
			},
			status: status.InProgressStatus,
		},
		{
			name: "failed",
			exprs: kustomize.HealthCheckExpressions{
				Failed:  "data.failed",
				Current: "data.current",
			},
			status: status.FailedStatus,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			se, err := cel.NewStatusEvaluator(&tt.exprs)
			g.Expect(err).NotTo(HaveOccurred())

			sr := cel.NewStatusReader(context.Background(), nil, gk, se)

			result, err := sr.ReadStatusForObject(context.Background(), nil, &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"data": map[string]any{
						"current":    true,
						"inProgress": true,
						"failed":     true,
					},
				},
			})

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Status).To(Equal(tt.status))
		})
	}
}
