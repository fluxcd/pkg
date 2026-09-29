/*
Copyright 2026 The Flux authors

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

// Package eventstest provides test helpers for asserting on the events a
// Recorder posts to its notification webhook.
//
// Sink is an in-process HTTP server that captures the eventv1.Event payloads a
// Recorder posts to its webhook address. It is the notification endpoint side
// of the recorder: while the Kubernetes Event sink can be inspected via the
// apiserver, the webhook payload can only be observed by receiving the HTTP
// POST.
//
// It is intended for use both by the events module's tests and by controllers
// that embed the Recorder and want to assert on the events they emit. Point a
// Recorder at Sink.URL(), emit events, then inspect Events or EventsFor.
package eventstest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

// Sink is an in-process HTTP server that captures the eventv1.Event payloads a
// Recorder posts to its webhook address.
//
// Sink is safe for concurrent use.
type Sink struct {
	server *httptest.Server

	mu     sync.Mutex
	events []eventv1.Event

	// statusCode, when non-zero, is returned for every request. Use it to
	// simulate notification-controller responses such as 429 (duplicate)
	// or 500 (retry). Defaults to 200.
	statusCode int
}

// NewSink starts a Sink and registers its shutdown with t.Cleanup. The caller
// does not need to call Close explicitly.
func NewSink(t testing.TB) *Sink {
	t.Helper()
	s := &Sink{}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

func (s *Sink) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var ev eventv1.Event
	if err := json.Unmarshal(body, &ev); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.events = append(s.events, ev)
	code := s.statusCode
	s.mu.Unlock()

	if code != 0 {
		w.WriteHeader(code)
	}
}

// URL returns the address to pass as the Recorder webhook.
func (s *Sink) URL() string {
	return s.server.URL
}

// Events returns a snapshot copy of the events received so far, in arrival
// order.
func (s *Sink) Events() []eventv1.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]eventv1.Event, len(s.events))
	copy(out, s.events)
	return out
}

// EventsFor returns a snapshot copy of the received events whose InvolvedObject
// matches the given kind, namespace and name, in arrival order.
func (s *Sink) EventsFor(kind, namespace, name string) []eventv1.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []eventv1.Event
	for _, ev := range s.events {
		if ev.InvolvedObject.Kind == kind &&
			ev.InvolvedObject.Namespace == namespace &&
			ev.InvolvedObject.Name == name {
			out = append(out, ev)
		}
	}
	return out
}

// Reset discards all captured events.
func (s *Sink) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = nil
}

// SetStatusCode sets the HTTP status code the sink returns for every request.
// It can be called mid-test to change the response. A zero value keeps the
// default 200.
func (s *Sink) SetStatusCode(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCode = code
}

// Close shuts down the underlying HTTP server. NewSink already registers this
// via t.Cleanup; call it directly only when finer control is needed.
func (s *Sink) Close() {
	s.server.Close()
}
