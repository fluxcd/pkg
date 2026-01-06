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
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconcilerWrapper wraps a Reconciler to enhance
// it with additional functionality.
type reconcilerWrapper struct {
	reconcile.TypedReconciler[ctrl.Request]
	hooks *queueHooks
}

// interruptContextKey is the type used as a context
// key for storing a context that will be canceled when
// the object associated with a call to Reconcile() is
// requeued.
type interruptContextKey struct{}

// interruptHandle holds a context that will be canceled
// when the object associated with a call to Reconcile() is
// requeued, and the channel used to deliver the payload
// when that happens.
type interruptHandle struct {
	ctx       context.Context
	payloadCh <-chan *queueEventPayload
	payload   *queueEventPayload
	mu        sync.Mutex
}

func (h *interruptHandle) getPayload() *queueEventPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case h.payload = <-h.payloadCh:
	default:
	}
	return h.payload
}

// WrapReconciler wraps a reconcile.TypedReconciler[ctrl.Request] to
// enhance it with additional functionality.
func WrapReconciler(wrapped reconcile.TypedReconciler[ctrl.Request]) *reconcilerWrapper {
	return &reconcilerWrapper{
		TypedReconciler: wrapped,
		hooks:           newQueueHooks(),
	}
}

// Reconcile implements reconcile.TypedReconciler[ctrl.Request].
func (r *reconcilerWrapper) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	interruptCtx, cancel, payloadCh := r.ContextWithCancelOnObjectEnqueued(ctx, req)
	defer cancel()

	h := &interruptHandle{
		ctx:       interruptCtx,
		payloadCh: payloadCh,
	}

	ctx = context.WithValue(ctx, interruptContextKey{}, h)
	return r.TypedReconciler.Reconcile(ctx, req)
}

// ContextWithCancelOnObjectEnqueued returns a context that will be canceled
// when the specified object is requeued. There's no need to call this method
// directly, as the Reconcile() method already does it for the object being
// reconciled. This method is exposed so that it can be used in tests.
func (r *reconcilerWrapper) ContextWithCancelOnObjectEnqueued(parent context.Context,
	obj ctrl.Request) (context.Context, context.CancelFunc, <-chan *queueEventPayload) {
	return r.hooks.registerListener(parent, queueEvent{queueEventObjectEnqueued, obj})
}

// EnqueueRequestsFromMapFunc wraps a handler.MapFunc to fire off
// hooks for the enqueued objects.
func (r *reconcilerWrapper) EnqueueRequestsFromMapFunc(
	objKind string, fn handler.MapFunc) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		reqs := fn(ctx, obj)
		payload := queueEventPayload{source: QueueEventSource{
			Kind:            objKind,
			Name:            obj.GetName(),
			Namespace:       obj.GetNamespace(),
			UID:             obj.GetUID(),
			ResourceVersion: obj.GetResourceVersion(),
		}}
		for _, req := range reqs {
			r.hooks.dispatch(queueEvent{queueEventObjectEnqueued, req}, payload)
		}
		return reqs
	})
}

// GetInterruptContext returns a context that will be canceled when
// the object associated with a call to Reconcile() is requeued, or
// the input context itself if this input context is not associated
// with a call to Reconcile().
func GetInterruptContext(ctx context.Context) context.Context {
	if h := getInterruptHandle(ctx); h != nil {
		return h.ctx
	}
	return ctx
}

func getInterruptHandle(ctx context.Context) *interruptHandle {
	if v := ctx.Value(interruptContextKey{}); v != nil {
		return v.(*interruptHandle)
	}
	return nil
}

// IsObjectEnqueued returns true if the context belongs to a
// Reconcile() call and the object associated with that call
// has been requeued since the reconciliation started.
// If true, it returns also the watched object that caused
// the requeue, otherwise it returns nil.
func IsObjectEnqueued(ctx context.Context) (bool, *QueueEventSource) {
	h := getInterruptHandle(ctx)
	if h == nil {
		return false, nil
	}

	p := h.getPayload()
	if p == nil {
		return false, nil
	}

	return true, &p.source
}
