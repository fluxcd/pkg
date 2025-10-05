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

package controller

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler represents a controller-runtime ctrl.Request reconciler
// with additional functionality.
type Reconciler interface {
	reconcile.TypedReconciler[ctrl.Request]

	// NewQueue implements controller.Options.NewQueue.
	NewQueue(controllerName string,
		rateLimiter workqueue.TypedRateLimiter[ctrl.Request],
	) workqueue.TypedRateLimitingInterface[ctrl.Request]
}

// reconcilerWrapper wraps a Reconciler to enhance it with additional functionality.
type reconcilerWrapper struct {
	reconcile.TypedReconciler[ctrl.Request]
	queueFactory
}

// Ensure reconcilerWrapper implements controller.Options.NewQueue.
var _ = controller.Options{NewQueue: (&reconcilerWrapper{}).NewQueue}

// queueContextContextKey is the type used as a context
// key for storing a context that will be cancelled when
// the object associated with a call to Reconcile() is
// requeued.
type queueContextContextKey struct{}

// WrapReconciler wraps a reconcile.Reconciler to add the ability to
// get a context that will be cancelled when the object associated
// with a call to Reconcile() is requeued.
func WrapReconciler(r reconcile.TypedReconciler[ctrl.Request]) Reconciler {
	return &reconcilerWrapper{TypedReconciler: r}
}

func (r *reconcilerWrapper) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	queueCtx, cancel, err := r.contextWithCancelOnObjectEnqueued(ctx, req)
	if err != nil {
		// An error is only returned here if the queue was not created yet,
		// which should never happen as it's impossible for Reconcile to be
		// called before the queue is created. If this happens then we have
		// a very serious bug in the code, in which case it's better to
		// fail catastrophically and return a terminal error.
		// Modifying r.contextWithCancelOnObjectEnqueued to skip the nil
		// check would cause a panic here instead, which is arguably worse
		// than returning a terminal error.
		return ctrl.Result{}, reconcile.TerminalError(err)
	}
	defer cancel()

	ctx = context.WithValue(ctx, queueContextContextKey{}, queueCtx)
	return r.TypedReconciler.Reconcile(ctx, req)
}

// GetQueueContext returns a context that will be cancelled when
// the object associated with a call to Reconcile() is requeued,
// or the provided context if not called from within a Reconcile()
// call.
func GetQueueContext(ctx context.Context) context.Context {
	if v := ctx.Value(queueContextContextKey{}); v != nil {
		return v.(context.Context)
	}
	return ctx
}

// IsObjectEnqueued returns true if the context belongs to a
// Reconcile() call and the object associated with that call
// has been requeued since the reconciliation started.
func IsObjectEnqueued(ctx context.Context) bool {
	queueCtx := GetQueueContext(ctx)
	return ctx != queueCtx && ctx.Err() == nil && queueCtx.Err() != nil
}
