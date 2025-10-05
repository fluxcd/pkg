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
	"fmt"
	"sync"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// queueFactory creates notifyingQueues and tracks the latest created queue.
type queueFactory struct {
	q  *notifyingQueue
	mu sync.Mutex
}

// Ensure queueFactory implements controller.Options.NewQueue.
var _ = controller.Options{NewQueue: (&queueFactory{}).NewQueue}

// NewQueue creates a queue equivalent to the default queue created
// by controller-runtime, but also injects an abstraction to hook
// listeners to the Add method.
func (f *queueFactory) NewQueue(controllerName string,
	rateLimiter workqueue.TypedRateLimiter[ctrl.Request],
) workqueue.TypedRateLimitingInterface[ctrl.Request] {

	nq := newNotifyingQueue(workqueue.NewTypedWithConfig(workqueue.TypedQueueConfig[ctrl.Request]{
		Name: controllerName,
	}))

	dq := workqueue.NewTypedDelayingQueueWithConfig(workqueue.TypedDelayingQueueConfig[ctrl.Request]{
		Name:  controllerName,
		Queue: nq,
	})

	rlq := workqueue.NewTypedRateLimitingQueueWithConfig(rateLimiter, workqueue.TypedRateLimitingQueueConfig[ctrl.Request]{
		Name:          controllerName,
		DelayingQueue: dq,
	})

	// Track the created queue.
	f.mu.Lock()
	f.q = nq
	f.mu.Unlock()

	return rlq
}

// contextWithCancelOnObjectEnqueued gets the current notifyingQueue and calls its
// contextWithCancelOnObjectEnqueued method.
func (f *queueFactory) contextWithCancelOnObjectEnqueued(parent context.Context,
	obj ctrl.Request) (context.Context, context.CancelFunc, error) {

	f.mu.Lock()
	q := f.q
	f.mu.Unlock()

	if q == nil {
		return nil, nil, fmt.Errorf("queue has not been created yet")
	}

	ctx, cancel := q.contextWithCancelOnObjectEnqueued(parent, obj)
	return ctx, cancel, nil
}

// notifyingQueue wraps a workqueue and allows listeners to be notified
// when an item is added to the queue. It can be used to cancel ongoing
// operations when a new reconciliation request is made for the same object.
// The listeners are registered in the form of a context and its cancel
// function, they are notified by calling the cancel function. Therefore,
// listeners are notified only once.
type notifyingQueue struct {
	workqueue.TypedInterface[ctrl.Request]

	lis map[ctrl.Request][]queueListener
	mu  sync.Mutex
}

type queueListener struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newNotifyingQueue(q workqueue.TypedInterface[ctrl.Request]) *notifyingQueue {
	return &notifyingQueue{
		TypedInterface: q,
		lis:            make(map[ctrl.Request][]queueListener),
	}
}

func (n *notifyingQueue) contextWithCancelOnObjectEnqueued(parent context.Context,
	obj ctrl.Request) (context.Context, context.CancelFunc) {

	ctx, cancel := context.WithCancel(parent)

	n.mu.Lock()
	n.collectGarbage()
	n.lis[obj] = append(n.lis[obj], queueListener{ctx, cancel})
	n.mu.Unlock()

	return ctx, cancel
}

func (n *notifyingQueue) Add(item ctrl.Request) {
	n.mu.Lock()
	n.TypedInterface.Add(item)
	listeners := n.lis[item]
	delete(n.lis, item)
	n.collectGarbage()
	n.mu.Unlock()

	for _, l := range listeners {
		l.cancel()
	}
}

func (n *notifyingQueue) collectGarbage() {
	for key, listeners := range n.lis {
		var alive []queueListener
		for _, l := range listeners {
			if l.ctx.Err() == nil {
				alive = append(alive, l)
			}
		}
		if len(alive) == 0 {
			delete(n.lis, key)
		} else {
			n.lis[key] = alive
		}
	}
}
