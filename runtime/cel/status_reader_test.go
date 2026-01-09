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

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/clusterreader"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

			ctor, err := cel.NewStatusReader([]kustomize.CustomHealthCheck{{
				APIVersion: tt.supportedGK.Group + "/v1",
				Kind:       tt.supportedGK.Kind,
				HealthCheckExpressions: kustomize.HealthCheckExpressions{
					Current: "something",
				},
			}})
			g.Expect(err).NotTo(HaveOccurred())

			sr := ctor(nil)

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

			ctor, err := cel.NewStatusReader([]kustomize.CustomHealthCheck{{
				APIVersion:             "v1",
				Kind:                   "ConfigMap",
				HealthCheckExpressions: tt.exprs,
			}})
			g.Expect(err).NotTo(HaveOccurred())

			sr := ctor(mapper)

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

			ctor, err := cel.NewStatusReader([]kustomize.CustomHealthCheck{{
				APIVersion:             "v1",
				Kind:                   "ConfigMap",
				HealthCheckExpressions: tt.exprs,
			}})
			g.Expect(err).NotTo(HaveOccurred())

			sr := ctor(nil)

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

func TestNewStatusReader_DuplicateGroupKindError(t *testing.T) {
	g := NewWithT(t)

	result, err := cel.NewStatusReader([]kustomize.CustomHealthCheck{
		{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			HealthCheckExpressions: kustomize.HealthCheckExpressions{
				Current: "something",
			},
		},
		{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			HealthCheckExpressions: kustomize.HealthCheckExpressions{
				Current: "something",
			},
		},
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("duplicate custom health check for GroupKind"))
	g.Expect(err.Error()).To(ContainSubstring("healthchecks[1]"))
	g.Expect(result).To(BeNil())
}

func TestNewStatusReader_CELCompileError(t *testing.T) {
	g := NewWithT(t)

	result, err := cel.NewStatusReader([]kustomize.CustomHealthCheck{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		HealthCheckExpressions: kustomize.HealthCheckExpressions{
			Current: "something.",
		},
	}})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to create custom status evaluator for healthchecks[0]"))
	g.Expect(result).To(BeNil())
}

func TestStatusReader_CustomResourceLifeCycle(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test-namespace")
	g.Expect(err).NotTo(HaveOccurred())
	objNamespace := ns.Name

	err = testEnv.Create(ctx, &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "bitnami.com/v1alpha1",
			"kind":       "SealedSecret",
			"metadata": map[string]any{
				"name":      "test-sealedsecret",
				"namespace": objNamespace,
			},
			"spec": map[string]any{
				"encryptedData": map[string]any{
					"foo": "c2VjcmV0",
				},
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	healthchecks := []kustomize.CustomHealthCheck{{
		APIVersion: "bitnami.com/v1alpha1",
		Kind:       "SealedSecret",
		HealthCheckExpressions: kustomize.HealthCheckExpressions{
			InProgress: "has(status.observedGeneration) && status.observedGeneration != metadata.generation",
			Failed:     "status.conditions.filter(e, e.type == 'Synced').all(e, e.status == 'False')",
			Current:    "status.conditions.filter(e, e.type == 'Synced').all(e, e.status == 'True')",
		},
	}}

	identifiers := object.ObjMetadataSet{{
		Name:      "test-sealedsecret",
		Namespace: objNamespace,
		GroupKind: schema.GroupKind{
			Group: "bitnami.com",
			Kind:  "SealedSecret",
		},
	}}

	mapper := testEnv.GetRESTMapper()

	ctor, err := cel.NewStatusReader(healthchecks)
	g.Expect(err).NotTo(HaveOccurred())

	opts := polling.Options{
		CustomStatusReaders: []engine.StatusReader{ctor(mapper)},
	}
	poller := polling.NewStatusPoller(testEnv.GetClient(), mapper, opts)
	events := poller.Poll(ctx, identifiers, polling.PollOptions{
		PollInterval: 100 * time.Millisecond,
	})

	// No status at first. Our InProgress expression returns an error, the
	// status should be Unknown.
	ev := waitForEvent(t, ctx, events)
	g.Expect(ev.Resource.Status).To(Equal(status.UnknownStatus))

	// Controller adds status.observedGeneration, the status should be InProgress.
	u := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "bitnami.com/v1alpha1",
			"kind":       "SealedSecret",
		},
	}
	err = testEnv.Get(ctx, client.ObjectKey{Name: "test-sealedsecret", Namespace: objNamespace}, u)
	g.Expect(err).NotTo(HaveOccurred())
	u.Object["status"] = map[string]any{
		"observedGeneration": u.GetGeneration() - 1,
	}
	err = testEnv.Status().Update(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	ev = waitForEvent(t, ctx, events)
	g.Expect(ev.Resource.Status).To(Equal(status.InProgressStatus))

	// Controller adds Synced=True, the status should be Current.
	u.Object["status"] = map[string]any{
		"observedGeneration": u.GetGeneration(),
		"conditions":         []map[string]any{{"type": "Synced", "status": "True"}},
	}
	err = testEnv.Status().Update(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	ev = waitForEvent(t, ctx, events)
	g.Expect(ev.Resource.Status).To(Equal(status.CurrentStatus))

	// Controller adds Synced=False, the status should be Failed.
	u.Object["status"] = map[string]any{
		"observedGeneration": u.GetGeneration(),
		"conditions":         []map[string]any{{"type": "Synced", "status": "False"}},
	}
	err = testEnv.Status().Update(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	ev = waitForEvent(t, ctx, events)
	g.Expect(ev.Resource.Status).To(Equal(status.FailedStatus))
}

func waitForEvent(t *testing.T, ctx context.Context, events <-chan event.Event) *event.Event {
	t.Helper()

	select {
	case e := <-events:
		return &e
	case <-ctx.Done():
		t.Errorf("timed out waiting for event")
		t.FailNow()
		return nil
	}
}
