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

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// QueueEventSource holds enough tracking information about the
// source object that triggered a queue event and implements the
// error interface.
type QueueEventSource struct {
	Kind            string    `json:"kind"`
	Name            string    `json:"name"`
	Namespace       string    `json:"namespace"`
	UID             types.UID `json:"uid"`
	ResourceVersion string    `json:"resourceVersion"`
}

// Ensure QueueEventSource implements the error interface.
var _ error = &QueueEventSource{}

// Error returns a string representation of the object represented by QueueEventSource.
func (q *QueueEventSource) Error() string {
	return fmt.Sprintf("%s/%s/%s", q.Kind, q.Namespace, q.Name)
}

// Is returns true if the target error is a QueueEventSource object.
func (*QueueEventSource) Is(target error) bool {
	_, ok := target.(*QueueEventSource)
	return ok
}

// queueEventType represents the type of event that occurred in the queue.
type queueEventType int

const (
	// queueEventObjectEnqueued indicates that an object was enqueued.
	queueEventObjectEnqueued queueEventType = iota
)

// queueEventPayload is the payload delivered to listeners
// when a queue event occurs.
type queueEventPayload struct {
	source QueueEventSource
}

// queueHooks implements mechanisms for hooking to queue events.
type queueHooks struct {
	lis map[queueEvent][]*queueListener
	mu  sync.Mutex
}

// queueEvent represents an event related to the queue.
type queueEvent struct {
	queueEventType
	ctrl.Request
}

// queueListener represents a listener for a queue event.
type queueListener struct {
	ctx     context.Context
	cancel  context.CancelFunc
	payload chan<- *queueEventPayload
}

func newQueueHooks() *queueHooks {
	return &queueHooks{
		lis: make(map[queueEvent][]*queueListener),
	}
}

func (q *queueHooks) dispatch(event queueEvent, payload queueEventPayload) {
	q.mu.Lock()
	listeners := q.lis[event]
	delete(q.lis, event)
	q.collectGarbage()
	q.mu.Unlock()

	for _, l := range listeners {
		l.payload <- &payload
		l.cancel()
	}
}

func (q *queueHooks) registerListener(ctx context.Context, event queueEvent) (
	context.Context, context.CancelFunc, <-chan *queueEventPayload,
) {
	ctx, cancel := context.WithCancel(ctx)
	payload := make(chan *queueEventPayload, 1)

	q.mu.Lock()
	q.collectGarbage()
	q.lis[event] = append(q.lis[event], &queueListener{ctx, cancel, payload})
	q.mu.Unlock()

	return ctx, cancel, payload
}

func (q *queueHooks) collectGarbage() {
	for key, listeners := range q.lis {
		var alive []*queueListener
		for _, l := range listeners {
			if l.ctx.Err() == nil {
				alive = append(alive, l)
			}
		}
		if len(alive) > 0 {
			q.lis[key] = alive
		} else {
			delete(q.lis, key)
		}
	}
}
