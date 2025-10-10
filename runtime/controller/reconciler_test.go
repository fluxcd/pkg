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

package controller_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluxcd/pkg/runtime/controller"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type waitForRequeueReconciler struct {
	reconcileCtx atomic.Pointer[context.Context]
}

func (r *waitForRequeueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.reconcileCtx.Store(&ctx)
	select {
	case <-controller.GetInterruptContext(ctx).Done():
	case <-time.After(time.Second):
		return ctrl.Result{}, fmt.Errorf("timeout after waiting for queue context to be done")
	}
	is, src := controller.IsObjectEnqueued(ctx)
	if !is {
		return ctrl.Result{}, fmt.Errorf("expected object to be marked as enqueued")
	}
	if src == nil ||
		src.Kind != "ConfigMap" ||
		src.Name != "test-cm" ||
		src.Namespace != "default" ||
		src.UID != "12345" ||
		src.ResourceVersion != "54321" {
		return ctrl.Result{}, fmt.Errorf("expected source to match request")
	}
	return ctrl.Result{}, nil
}

func TestReconciler_HooksToObjectEnqueuedEvents(t *testing.T) {
	g := NewWithT(t)

	// Wrap the reconciler.
	r := &waitForRequeueReconciler{}
	reconciler := controller.WrapReconciler(r)

	// Add some garbage to exercise the garbage collection logic.
	garbageCtx, garbageCancel, _ := reconciler.ContextWithCancelOnObjectEnqueued(
		context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{
			Name:      "garbage",
			Namespace: "default",
		}})
	g.Expect(garbageCtx).ToNot(BeNil())
	g.Expect(garbageCancel).ToNot(BeNil())
	garbageCancel() // Cancel immediately to create garbage.

	// Add also a listener that will be kept during garbage collection.
	listenerCtx, listenerCancel, _ := reconciler.ContextWithCancelOnObjectEnqueued(
		context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{
			Name:      "listener",
			Namespace: "default",
		}})
	g.Expect(listenerCtx).ToNot(BeNil())
	g.Expect(listenerCancel).ToNot(BeNil())
	defer listenerCancel() // Cancel at the end of the test to avoid garbage collection.

	// Start object reconciliation in a separate goroutine.
	obj := ctrl.Request{NamespacedName: types.NamespacedName{
		Name:      "test",
		Namespace: "default",
	}}
	errCh := make(chan error, 1)
	go func() {
		_, err := reconciler.Reconcile(context.Background(), obj)
		errCh <- err
		close(errCh)
	}()

	// Give some time for the Reconcile to start and block on the queue context.
	time.Sleep(10 * time.Millisecond)

	// Check that the object is not enqueued yet.
	g.Eventually(func() bool { return r.reconcileCtx.Load() != nil }, time.Second).To(BeTrue())
	is, _ := controller.IsObjectEnqueued(*r.reconcileCtx.Load())
	g.Expect(is).To(BeFalse())

	// Simulate object being enqueued.
	reconciler.EnqueueRequestsFromMapFunc("ConfigMap", func(context.Context, client.Object) []ctrl.Request {
		return []ctrl.Request{obj}
	}).Create(context.Background(), event.CreateEvent{
		Object: &corev1.ConfigMap{
			ObjectMeta: ctrl.ObjectMeta{
				Name:            "test-cm",
				Namespace:       "default",
				UID:             "12345",
				ResourceVersion: "54321",
			},
		},
	}, priorityqueue.New[ctrl.Request]("test"))

	// Wait for the Reconcile to finish and check the result.
	select {
	case err := <-errCh:
		g.Expect(err).ToNot(HaveOccurred())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Reconcile to finish")
	}
}

func TestGetObjectContext(t *testing.T) {
	t.Run("returns the same context when called outside Reconcile", func(t *testing.T) {
		g := NewWithT(t)
		ctx := context.Background()
		g.Expect(controller.GetInterruptContext(ctx)).To(Equal(ctx))
	})
}

func TestIsObjectEnqueued(t *testing.T) {
	t.Run("returns false when called outside Reconcile", func(t *testing.T) {
		g := NewWithT(t)
		ctx := context.Background()
		g.Expect(controller.IsObjectEnqueued(ctx)).To(BeFalse())
	})
}
