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
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/runtime/cel"
)

func TestPollerWithCustomHealthChecks(t *testing.T) {
	g := NewWithT(t)

	result, err := cel.PollerWithCustomHealthChecks(context.Background(), []kustomize.CustomHealthCheck{
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
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(result).To(HaveLen(1))

	ctor := result[0]
	g.Expect(ctor).NotTo(BeNil())

	r := ctor(nil)

	supports := r.Supports(schema.GroupKind{
		Group: "",
		Kind:  "ConfigMap",
	})
	g.Expect(supports).To(BeTrue())

	doesNotSupport := r.Supports(schema.GroupKind{
		Group: "",
		Kind:  "Pod",
	})
	g.Expect(doesNotSupport).To(BeFalse())
}

func TestPollerWithCustomHealthChecksError(t *testing.T) {
	g := NewWithT(t)

	result, err := cel.PollerWithCustomHealthChecks(context.Background(), []kustomize.CustomHealthCheck{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		HealthCheckExpressions: kustomize.HealthCheckExpressions{
			Current: "something.",
		},
	}})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to create custom status evaluator for healthchecks[0]"))
	g.Expect(result).To(BeEmpty())
}

func TestStatusPoller_CustomResourceLifeCycle(t *testing.T) {
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

	ctors, err := cel.PollerWithCustomHealthChecks(context.Background(), healthchecks)
	g.Expect(err).NotTo(HaveOccurred())

	opts := polling.Options{}
	for _, ctor := range ctors {
		opts.CustomStatusReaders = append(opts.CustomStatusReaders, ctor(mapper))
	}
	poller := polling.NewStatusPoller(testEnv.GetClient(), mapper, opts)
	events := poller.Poll(ctx, identifiers, polling.PollOptions{
		PollInterval: 100 * time.Millisecond,
	})

	// No status at first. Our InProgress expression returns an error, the
	// status should be Unknown.
	event := waitForEvent(t, ctx, events)
	g.Expect(event.Resource.Status).To(Equal(status.UnknownStatus))

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
	event = waitForEvent(t, ctx, events)
	g.Expect(event.Resource.Status).To(Equal(status.InProgressStatus))

	// Controller adds Synced=True, the status should be Current.
	u.Object["status"] = map[string]any{
		"observedGeneration": u.GetGeneration(),
		"conditions":         []map[string]any{{"type": "Synced", "status": "True"}},
	}
	err = testEnv.Status().Update(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	event = waitForEvent(t, ctx, events)
	g.Expect(event.Resource.Status).To(Equal(status.CurrentStatus))

	// Controller adds Synced=False, the status should be Failed.
	u.Object["status"] = map[string]any{
		"observedGeneration": u.GetGeneration(),
		"conditions":         []map[string]any{{"type": "Synced", "status": "False"}},
	}
	err = testEnv.Status().Update(ctx, u)
	g.Expect(err).NotTo(HaveOccurred())
	event = waitForEvent(t, ctx, events)
	g.Expect(event.Resource.Status).To(Equal(status.FailedStatus))
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
