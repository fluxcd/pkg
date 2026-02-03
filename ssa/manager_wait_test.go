/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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

package ssa

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	kstatusreaders "github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"

	"github.com/fluxcd/pkg/ssa/utils"
)

func TestWaitForSet(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("wait")
	objects, err := readManifest("testdata/test5.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "infra", "default")

	_, crd := getFirstObject(objects, "CustomResourceDefinition", "clustertests.testing.fluxcd.io")
	_, cr := getFirstObject(objects, "ClusterTest", id)

	t.Run("waits for CRD and CR", func(t *testing.T) {
		cs, err := manager.Apply(ctx, crd, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		if err := manager.WaitForSet([]object.ObjMetadata{cs.ObjMetadata}, DefaultWaitOptions()); err != nil {
			t.Errorf("wait failed for CRD: %v", err)
		}

		changeSet, err := manager.ApplyAll(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		if err := manager.WaitForSet(changeSet.ToObjMetadataSet(), WaitOptions{Interval: time.Second, Timeout: 3 * time.Second}); err == nil {
			t.Error("wanted wait error due to observedGeneration < generation")
		}

		clusterCR := &unstructured.Unstructured{}
		clusterCR.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "testing.fluxcd.io",
			Kind:    "ClusterTest",
			Version: "v1",
		})
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cr), clusterCR); err != nil {
			t.Fatal(err)
		}

		var observedGeneration int64 = 1
		clusterCR.SetManagedFields(nil)
		err = unstructured.SetNestedField(clusterCR.Object, observedGeneration, "status", "observedGeneration")
		if err != nil {
			t.Fatal(err)
		}

		opts := &client.SubResourcePatchOptions{
			PatchOptions: client.PatchOptions{
				FieldManager: manager.owner.Field,
			},
		}

		if err := manager.client.Status().Patch(ctx, clusterCR, client.Apply, opts); err != nil {
			t.Fatal(err)
		}

		if err := manager.WaitForSet(changeSet.ToObjMetadataSet(), DefaultWaitOptions()); err != nil {
			t.Errorf("wait error: %v", err)
		}
	})
}

func TestWaitForSet_failFast(t *testing.T) {
	timeout := 5 * time.Second
	interval := 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	id := generateName("failfast")
	objects, err := readManifest("testdata/test10.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "infra", "default")
	_, pvc := getFirstObject(objects, "PersistentVolumeClaim", id)
	_, deploy := getFirstObject(objects, "Deployment", id)

	deployObjMeta, _ := object.RuntimeToObjMeta(deploy)

	cs, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
	if err != nil {
		t.Fatal(err)
	}

	var clusterDeploy = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id,
			Namespace: id,
		},
	}
	err = manager.client.Get(ctx, client.ObjectKeyFromObject(deploy), clusterDeploy)
	if err != nil {
		t.Fatal(err)
	}

	// Set Progressing Condition to false and reason to ProgressDeadlineExceeded.
	// This tells kstatus that the deployment has stalled.
	cond := appsv1.DeploymentCondition{
		Type:               appsv1.DeploymentProgressing,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Time{},
		Reason:             "ProgressDeadlineExceeded",
		Message:            "timeout progressing",
	}
	clusterDeploy.Status = appsv1.DeploymentStatus{
		ObservedGeneration:  clusterDeploy.Generation,
		Replicas:            *clusterDeploy.Spec.Replicas,
		UpdatedReplicas:     *clusterDeploy.Spec.Replicas,
		UnavailableReplicas: *clusterDeploy.Spec.Replicas,
		Conditions:          []appsv1.DeploymentCondition{cond},
	}
	err = manager.client.Status().Update(ctx, clusterDeploy)
	if err != nil {
		t.Fatal(err)
	}

	// Set PVC phase to Pending.
	// This tells kstatus that the PVC is in progress.
	clusterPvc := &unstructured.Unstructured{}
	clusterPvc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "PersistentVolumeClaim",
		Version: "v1",
	})
	if err := manager.client.Get(ctx, client.ObjectKeyFromObject(pvc), clusterPvc); err != nil {
		t.Fatal(err)
	}

	if err := unstructured.SetNestedField(clusterPvc.Object, "Pending", "status", "phase"); err != nil {
		t.Fatal(err)
	}

	opts := &client.SubResourcePatchOptions{
		PatchOptions: client.PatchOptions{
			FieldManager: manager.owner.Field,
		},
	}

	clusterPvc.SetManagedFields(nil)
	if err := manager.client.Status().Patch(ctx, clusterPvc, client.Apply, opts); err != nil {
		t.Fatal(err)
	}

	t.Run("timeout when fail fast is disabled", func(t *testing.T) {
		err = manager.WaitForSet(cs.ToObjMetadataSet(), WaitOptions{
			Interval: interval,
			Timeout:  timeout,
			FailFast: false,
		})

		deployFailedMsg := fmt.Sprintf("%s status: '%s'", utils.FmtObjMetadata(deployObjMeta), status.FailedStatus)

		if err == nil || !strings.Contains(err.Error(), "timeout waiting for") {
			t.Fatal("expected WaitForSet to timeout waiting for deployment")
		}

		if !strings.Contains(err.Error(), deployFailedMsg) {
			t.Fatal("expected error to contain status of failed deployment", err.Error())
		}

		if !strings.Contains(err.Error(), "InProgress") {
			t.Fatal("expected error to contain InProgress deployment", err.Error())
		}
	})

	t.Run("fail early even if there are still progressing resources", func(t *testing.T) {
		err = manager.WaitForSet(cs.ToObjMetadataSet(), WaitOptions{
			Interval: interval,
			Timeout:  timeout,
			FailFast: true,
		})

		deployFailedMsg := fmt.Sprintf("%s status: '%s'", utils.FmtObjMetadata(deployObjMeta), status.FailedStatus)

		if err == nil || !strings.Contains(err.Error(), "failed early") {
			t.Fatal("expected to fail early due to stalled deployment", err.Error())
		}

		if !strings.Contains(err.Error(), deployFailedMsg) {
			t.Fatal("expected error to contain status of failed deployment", err.Error())
		}

		if strings.Contains(err.Error(), "InProgress") {
			t.Fatal("expected error to not contain InProgress resources", err.Error())
		}
	})
}

func TestWaitForSet_terminalState(t *testing.T) {
	timeout := 5 * time.Second
	interval := 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	id := generateName("terminal")
	objects, err := readManifest("testdata/test10.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "infra", "default")
	_, pvc := getFirstObject(objects, "PersistentVolumeClaim", id)
	_, deploy := getFirstObject(objects, "Deployment", id)

	deployObjMeta, _ := object.RuntimeToObjMeta(deploy)

	cs, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
	if err != nil {
		t.Fatal(err)
	}

	var clusterDeploy = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id,
			Namespace: id,
		},
	}
	err = manager.client.Get(ctx, client.ObjectKeyFromObject(deploy), clusterDeploy)
	if err != nil {
		t.Fatal(err)
	}

	// Set Progressing Condition to false and reason to ProgressDeadlineExceeded.
	// This tells kstatus that the deployment has stalled.
	cond := appsv1.DeploymentCondition{
		Type:               appsv1.DeploymentProgressing,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Time{},
		Reason:             "ProgressDeadlineExceeded",
		Message:            "timeout progressing",
	}
	clusterDeploy.Status = appsv1.DeploymentStatus{
		ObservedGeneration:  clusterDeploy.Generation,
		Replicas:            *clusterDeploy.Spec.Replicas,
		UpdatedReplicas:     *clusterDeploy.Spec.Replicas,
		UnavailableReplicas: *clusterDeploy.Spec.Replicas,
		Conditions:          []appsv1.DeploymentCondition{cond},
	}
	err = manager.client.Status().Update(ctx, clusterDeploy)
	if err != nil {
		t.Fatal(err)
	}

	// Set PVC phase to Bound.
	// This tells kstatus that the PVC is Current (terminal).
	clusterPvc := &unstructured.Unstructured{}
	clusterPvc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "PersistentVolumeClaim",
		Version: "v1",
	})
	if err := manager.client.Get(ctx, client.ObjectKeyFromObject(pvc), clusterPvc); err != nil {
		t.Fatal(err)
	}

	if err := unstructured.SetNestedField(clusterPvc.Object, "Bound", "status", "phase"); err != nil {
		t.Fatal(err)
	}

	opts := &client.SubResourcePatchOptions{
		PatchOptions: client.PatchOptions{
			FieldManager: manager.owner.Field,
		},
	}

	clusterPvc.SetManagedFields(nil)
	if err := manager.client.Status().Patch(ctx, clusterPvc, client.Apply, opts); err != nil {
		t.Fatal(err)
	}

	t.Run("fail early when all resources are in terminal states", func(t *testing.T) {
		// With FailFast disabled but all resources in terminal states (Failed + Current),
		// WaitForSet should exit early instead of waiting for the full timeout.
		err = manager.WaitForSet(cs.ToObjMetadataSet(), WaitOptions{
			Interval: interval,
			Timeout:  timeout,
			FailFast: false,
		})

		deployFailedMsg := fmt.Sprintf("%s status: '%s'", utils.FmtObjMetadata(deployObjMeta), status.FailedStatus)

		if err == nil || !strings.Contains(err.Error(), "failed early") {
			t.Fatal("expected to fail early due to terminal state", err)
		}

		if !strings.Contains(err.Error(), deployFailedMsg) {
			t.Fatal("expected error to contain status of failed deployment", err.Error())
		}

		// InProgress resources should NOT appear since PVC is Bound (Current).
		if strings.Contains(err.Error(), "InProgress") {
			t.Fatal("expected error to not contain InProgress resources", err.Error())
		}
	})
}

func TestWaitForSet_RateLimiterError(t *testing.T) {
	g := NewWithT(t)

	id := generateName("ratelimit")

	// Create real resources on the cluster: 4 ConfigMaps + 1 PVC.
	objects, err := readManifest("testdata/test14.yaml", id)
	g.Expect(err).NotTo(HaveOccurred())

	manager.SetOwnerLabels(objects, "infra", "default")
	cs, err := manager.ApplyAllStaged(context.Background(), objects, DefaultApplyOptions())
	g.Expect(err).NotTo(HaveOccurred())

	// Use a custom status reader that returns the rate limiter error
	// after initial polls. This exercises the real cli-utils error handling
	// code path (errResourceToResourceStatus / errIdentifierToResourceStatus)
	// that PR https://github.com/fluxcd/cli-utils/pull/18 fixes.
	//
	// On old cli-utils (before PR #18): the rate limiter error is NOT recognized
	// as cancellation, so it becomes a ResourceStatus with Unknown status for ALL
	// resources that encounter it. The timeout error message would list every
	// single resource with "Unknown: rate: Wait(n=1) would exceed context deadline".
	//
	// On new cli-utils (with PR #18): the rate limiter error IS recognized as
	// cancellation via isRateLimiterContextDeadlineExceeded(), so polling stops
	// cleanly and the error is propagated directly without dumping all resources.
	var callCount int
	manager.poller = polling.NewStatusPoller(manager.client, restMapper, polling.Options{
		CustomStatusReaders: []engine.StatusReader{
			kstatusreaders.NewGenericStatusReader(restMapper,
				func(u *unstructured.Unstructured) (*status.Result, error) {
					callCount++
					// After a few poll cycles, return the rate limiter error
					// for every resource. This is the exact error string that
					// old kstatus failed to handle properly.
					if callCount > 10 {
						return nil, fmt.Errorf("rate: Wait(n=1) would exceed context deadline")
					}
					// Use the built-in status computation for initial polls.
					return status.Compute(u)
				},
			),
		},
	})
	defer func() {
		manager.poller = poller
	}()

	err = manager.WaitForSet(cs.ToObjMetadataSet(), WaitOptions{
		Interval: 100 * time.Millisecond,
		Timeout:  2 * time.Second,
		FailFast: false,
	})

	g.Expect(err).To(HaveOccurred())
	errMsg := err.Error()
	t.Logf("error message: %s", errMsg)

	// With the new cli-utils (PR #18 fix), the rate limiter error should NOT
	// produce a huge error message with all resources listed as Unknown.
	for i := 1; i <= 4; i++ {
		cmName := fmt.Sprintf("%s-cm%d", id, i)
		g.Expect(errMsg).NotTo(ContainSubstring(cmName),
			"error should not contain ConfigMap %s — rate limiter errors should not pollute the error message with all resources", cmName)
	}
}

func TestWaitForSet_ErrorOnReaderError(t *testing.T) {
	g := NewWithT(t)

	err := manager.client.Create(context.Background(), &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
			"data": map[string]any{
				"foo": "bar",
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	manager.poller = polling.NewStatusPoller(manager.client, restMapper, polling.Options{
		CustomStatusReaders: []engine.StatusReader{
			kstatusreaders.NewGenericStatusReader(restMapper,
				func(*unstructured.Unstructured) (*status.Result, error) {
					return nil, fmt.Errorf("error reading status")
				},
			),
		},
	})

	// Restore the original poller otherwise all other tests will fail
	defer func() {
		manager.poller = poller
	}()

	set := []object.ObjMetadata{{
		Name:      "test",
		Namespace: "default",
		GroupKind: schema.GroupKind{Group: "", Kind: "ConfigMap"},
	}}
	err = manager.WaitForSet(set, WaitOptions{
		Interval: 40 * time.Millisecond,
		Timeout:  100 * time.Millisecond,
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("timeout waiting for: [ConfigMap/default/test status: 'Unknown': error reading status]"))
}

func TestWaitWithContext_Cancellation(t *testing.T) {
	g := NewWithT(t)

	id := generateName("cancellation")
	objects, err := readManifest("testdata/test2.yaml", id)
	g.Expect(err).NotTo(HaveOccurred())

	// Apply objects to the cluster which will never reach Ready state
	manager.SetOwnerLabels(objects, "app1", "cancellation")
	changeSet, err := manager.ApplyAllStaged(context.Background(), objects, ApplyOptions{
		WaitInterval: 500 * time.Millisecond,
		WaitTimeout:  5 * time.Second,
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create a context that we can cancel for the wait operation
	ctx, cancel := context.WithCancel(context.Background())

	// Configure wait options with a longer timeout to ensure we can cancel before it times out
	waitOpts := WaitOptions{
		Interval: 500 * time.Millisecond,
		Timeout:  5 * time.Second,
		FailFast: true,
	}

	// Channel to capture the error from WaitForSetWithContext
	errChan := make(chan error, 1)

	// Start WaitForSetWithContext in a goroutine
	go func() {
		errChan <- manager.WaitForSetWithContext(ctx, changeSet.ToObjMetadataSet(), waitOpts)
	}()

	// Wait for one second to ensure WaitForSetWithContext has started
	time.Sleep(time.Second)

	// Cancel the context to trigger early exit
	cancel()

	// Wait for the goroutine to finish and verify it returned due to context cancellation
	select {
	case waitErr := <-errChan:
		g.Expect(waitErr).To(HaveOccurred(), "Expected an error due to context cancellation")
		g.Expect(waitErr).To(Equal(context.Canceled))
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSetWithContext did not return within expected time after cancellation")
	}
}

func TestWaitForSet_JobWithTTL(t *testing.T) {
	g := NewWithT(t)

	id := generateName("job-ttl")

	// Create a Job with ttlSecondsAfterFinished set
	job := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]any{
				"name":      id,
				"namespace": "default",
			},
			"spec": map[string]any{
				"ttlSecondsAfterFinished": int64(0),
				"template": map[string]any{
					"spec": map[string]any{
						"restartPolicy": "Never",
						"containers": []any{
							map[string]any{
								"name":    "test",
								"image":   "busybox",
								"command": []any{"echo", "hello"},
							},
						},
					},
				},
			},
		},
	}

	// Apply the Job
	_, err := manager.ApplyAll(context.Background(), []*unstructured.Unstructured{job}, DefaultApplyOptions())
	g.Expect(err).NotTo(HaveOccurred())

	jobObjMeta := object.UnstructuredToObjMetadata(job)

	// Use a custom status reader that returns NotFoundStatus for the Job
	// to simulate the TTL controller deleting it after completion
	manager.poller = polling.NewStatusPoller(manager.client, restMapper, polling.Options{
		CustomStatusReaders: []engine.StatusReader{
			kstatusreaders.NewGenericStatusReader(restMapper,
				func(u *unstructured.Unstructured) (*status.Result, error) {
					if u.GetKind() == "Job" && u.GetName() == id {
						return &status.Result{
							Status:  status.NotFoundStatus,
							Message: "Resource not found",
						}, nil
					}
					return status.Compute(u)
				},
			),
		},
	})
	defer func() {
		manager.poller = poller
	}()

	t.Run("NotFound Job with TTL is treated as success", func(t *testing.T) {
		start := time.Now()
		err = manager.WaitForSet([]object.ObjMetadata{jobObjMeta}, WaitOptions{
			Interval:    100 * time.Millisecond,
			Timeout:     5 * time.Second,
			JobsWithTTL: object.ObjMetadataSet{jobObjMeta},
		})
		elapsed := time.Since(start)

		g.Expect(err).NotTo(HaveOccurred(), "NotFound status for Job with TTL should be treated as success")
		g.Expect(elapsed).To(BeNumerically("<", 2*time.Second), "should return early, not wait for full timeout")
	})

	t.Run("NotFound Job without TTL option is treated as error", func(t *testing.T) {
		err = manager.WaitForSet([]object.ObjMetadata{jobObjMeta}, WaitOptions{
			Interval: 100 * time.Millisecond,
			Timeout:  2 * time.Second,
			// JobsWithTTL not set
		})
		g.Expect(err).To(HaveOccurred(), "NotFound status for Job without TTL option should be an error")
		g.Expect(err.Error()).To(ContainSubstring("NotFound"))
	})
}

func TestWaitForSetTermination(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	waitOpts := WaitOptions{
		Interval: 40 * time.Millisecond,
		Timeout:  100 * time.Millisecond,
	}

	id := generateName("wait-block")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, namespace := getFirstObject(objects, "Namespace", id)
	meta := map[string]string{
		"fluxcd.io/prune": "Disabled",
	}
	namespace.SetAnnotations(meta)
	manager.SetOwnerLabels(objects, "test", id)

	_, err = manager.Apply(ctx, namespace, DefaultApplyOptions())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("applies objects", func(t *testing.T) {
		gt := NewWithT(t)
		_, err = manager.ApplyAll(ctx, objects, DefaultApplyOptions())
		gt.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("timeout waiting for termination", func(t *testing.T) {
		gt := NewWithT(t)

		cs := NewChangeSet()
		cs.Add(ChangeSetEntry{
			ObjMetadata:  object.UnstructuredToObjMetadata(namespace),
			GroupVersion: namespace.GroupVersionKind().Version,
			Subject:      utils.FmtUnstructured(namespace),
			Action:       DeletedAction,
		})

		err = manager.WaitForSetTermination(cs, waitOpts)
		gt.Expect(err).To(HaveOccurred())
		gt.Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Namespace/%s termination timeout", id)))
	})

	t.Run("delete and wait excluding ignored objects", func(t *testing.T) {
		gt := NewWithT(t)

		delOpts := DefaultDeleteOptions()
		delOpts.Exclusions = meta
		cs, err := manager.DeleteAll(ctx, objects, delOpts)
		gt.Expect(err).NotTo(HaveOccurred())

		err = manager.WaitForSetTermination(cs, waitOpts)
		gt.Expect(err).NotTo(HaveOccurred())
	})
}
