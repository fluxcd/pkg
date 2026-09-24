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

package events

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
)

// TestSink is an in-process HTTP server that captures the eventv1.Event
// payloads a Recorder posts to its webhook address. It is the notification
// endpoint side of the recorder: while the Kubernetes Event sink can be
// inspected via the apiserver, the webhook payload can only be observed by
// receiving the HTTP POST.
//
// It is intended for use both by this module's tests and by controllers that
// embed the Recorder and want to assert on the events they emit. Point a
// Recorder at TestSink.URL(), emit events, then inspect Events().
//
// TestSink is safe for concurrent use.
type TestSink struct {
	server *httptest.Server

	mu     sync.Mutex
	events []eventv1.Event

	// statusCode, when non-zero, is returned for every request. Use it to
	// simulate notification-controller responses such as 429 (duplicate)
	// or 500 (retry). Defaults to 200.
	statusCode int
}

// NewTestSink starts a TestSink and returns it. Call Close when done, typically
// via t.Cleanup(sink.Close).
func NewTestSink() *TestSink {
	s := &TestSink{}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// WithStatusCode sets the HTTP status code the sink returns for every request.
// It returns the sink to allow chaining. A zero value keeps the default 200.
func (s *TestSink) WithStatusCode(code int) *TestSink {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCode = code
	return s
}

func (s *TestSink) handle(w http.ResponseWriter, r *http.Request) {
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
func (s *TestSink) URL() string {
	return s.server.URL
}

// Events returns a copy of the events received so far, in arrival order.
func (s *TestSink) Events() []eventv1.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]eventv1.Event, len(s.events))
	copy(out, s.events)
	return out
}

// Len returns the number of events received so far.
func (s *TestSink) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

// Reset discards all captured events.
func (s *TestSink) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = nil
}

// Close shuts down the underlying HTTP server.
func (s *TestSink) Close() {
	s.server.Close()
}
