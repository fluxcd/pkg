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

package oci

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func TestWithRetryTransportInsecureTLS(t *testing.T) {
	registry := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	registry.Config.ErrorLog = log.New(io.Discard, "", 0)
	registry.StartTLS()
	t.Cleanup(registry.Close)

	registryHost := strings.TrimPrefix(registry.URL, "https://")
	ref, err := name.ParseReference(registryHost + "/test:latest")
	if err != nil {
		t.Fatalf("failed to parse registry reference: %v", err)
	}

	backoff := remote.Backoff{
		Duration: time.Millisecond,
		Factor:   1,
		Steps:    1,
	}
	scopes := []string{ref.Context().Scope(transport.PushScope)}

	tests := []struct {
		name      string
		insecure  bool
		wantError bool
	}{
		{
			name:      "certificate verification enabled",
			wantError: true,
		},
		{
			name:     "certificate verification disabled",
			insecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := WithRetryTransport(ctx, ref, authn.Anonymous, backoff, scopes, tt.insecure)
			if tt.wantError && err == nil {
				t.Fatal("expected TLS certificate verification to fail")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected insecure TLS connection to succeed: %v", err)
			}
		})
	}
}
