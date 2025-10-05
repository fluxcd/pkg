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
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluxcd/pkg/runtime/controller"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type noopReconciler struct {
	reconciled atomic.Bool
}

func (r *noopReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name == "test-configmap" && req.Namespace == "default" {
		r.reconciled.Store(true)
	}
	return ctrl.Result{}, nil
}

func TestControllerBuilder(t *testing.T) {
	g := NewWithT(t)

	// Create test environment.
	testEnv := &envtest.Environment{}
	conf, err := testEnv.Start()
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { testEnv.Stop() })
	kubeClient, err := client.New(conf, client.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(kubeClient).NotTo(BeNil())

	// Create manager.
	mgr, err := ctrl.NewManager(conf, ctrl.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(mgr).NotTo(BeNil())

	// Create and setup controller.
	nr := &noopReconciler{}
	r := controller.WrapReconciler(nr)
	err = controller.NewControllerManagedBy(mgr, r).
		For(&corev1.ConfigMap{}, predicate.ResourceVersionChangedPredicate{}).
		Complete(r)
	g.Expect(err).NotTo(HaveOccurred())

	// Start manager.
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		errCh <- mgr.Start(ctx)
		close(errCh)
	}()

	// Create a ConfigMap and expect the reconciler to be called.
	g.Expect(nr.reconciled.Load()).To(BeFalse())
	g.Expect(kubeClient.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "default",
		},
	})).To(Succeed())
	g.Eventually(func() bool { return nr.reconciled.Load() }, time.Second).To(BeTrue())

	// Stop the manager.
	cancel()
	select {
	case err := <-errCh:
		g.Expect(err).NotTo(HaveOccurred())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for manager to stop")
	}
}
