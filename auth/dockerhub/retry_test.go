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

package dockerhub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestImplementation_ExchangeTokenRetriesRateLimit(t *testing.T) {
	g := NewWithT(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := implementation{}.ExchangeToken(context.Background(), server.Client(),
		server.URL, url.Values{"grant_type": {"test"}})
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
	g.Expect(calls.Load()).To(Equal(int32(2)))
}

func TestRetryAfter(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{value: "", ok: false},
		{value: "0", want: 0, ok: true},
		{value: " 2 ", want: 2 * time.Second, ok: true},
		{value: "-1", want: 0, ok: true},
		{value: "nope", ok: false},
	} {
		t.Run(tt.value, func(t *testing.T) {
			g := NewWithT(t)
			got, ok := retryAfter(tt.value)
			g.Expect(ok).To(Equal(tt.ok))
			g.Expect(got).To(Equal(tt.want))
		})
	}
}
