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

package cioidc_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fluxcd/pkg/auth/actionsoidc"
	"github.com/fluxcd/pkg/auth/utils/cioidc"
)

// makeJWT mints an unsigned-but-parseable JWT carrying the given exp claim.
func makeJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": exp.Unix()})
	signed, err := tok.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to mint test JWT: %v", err)
	}
	return signed
}

// tokenServer mints a fresh JWT with the configured TTL on each call, counting
// requests so tests can assert cache behavior. It points the actionsoidc env
// vars at itself.
type tokenServer struct {
	server *httptest.Server
	calls  atomic.Int32
}

func newTokenServer(t *testing.T, ttl time.Duration) *tokenServer {
	t.Helper()
	ts := &tokenServer{}
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.calls.Add(1)
		_, _ = w.Write([]byte(`{"value":"` + makeJWT(t, time.Now().Add(ttl)) + `"}`))
	}))
	t.Cleanup(ts.server.Close)
	t.Setenv(actionsoidc.EnvRequestURL, ts.server.URL)
	t.Setenv(actionsoidc.EnvRequestToken, "request-token")
	return ts
}

// recordingRT records the Authorization header and host of every request it
// sees and replies 200.
type recordingRT struct {
	auths []string
	hosts []string
}

func (r *recordingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	r.auths = append(r.auths, req.Header.Get("Authorization"))
	r.hosts = append(r.hosts, req.URL.Host)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func mustNewTransport(t *testing.T, inner http.RoundTripper, opts ...cioidc.Option) *cioidc.Transport {
	t.Helper()
	tr, err := cioidc.NewTransport(append([]cioidc.Option{cioidc.WithInner(inner)}, opts...)...)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	return tr
}

func get(t *testing.T, tr http.RoundTripper, host string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "https://"+host+"/v2/", nil)
	req.Header.Set("Authorization", "Basic should-be-overwritten")
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
}

func TestNewTransport_Validation(t *testing.T) {
	tests := []struct {
		name    string
		opts    []cioidc.Option
		wantErr string
	}{
		{
			name:    "no hosts",
			opts:    nil,
			wantErr: "at least one host",
		},
		{
			name: "duplicate within tokens",
			opts: []cioidc.Option{
				cioidc.WithHostToken("a.example", "t1"),
				cioidc.WithHostToken("a.example", "t2"),
			},
			wantErr: `host "a.example" is configured more than once`,
		},
		{
			name: "duplicate within audiences",
			opts: []cioidc.Option{
				cioidc.WithHostAudience("a.example", "aud1"),
				cioidc.WithHostAudience("a.example", "aud2"),
			},
			wantErr: `host "a.example" is configured more than once`,
		},
		{
			name: "duplicate across token and audience",
			opts: []cioidc.Option{
				cioidc.WithHostToken("a.example", "t"),
				cioidc.WithHostAudience("a.example", "aud"),
			},
			wantErr: `host "a.example" is configured more than once`,
		},
		{
			name: "valid mix of distinct hosts",
			opts: []cioidc.Option{
				cioidc.WithHostToken("static.example", "t"),
				cioidc.WithHostAudience("mint.example", "aud"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cioidc.NewTransport(tt.opts...)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestTransport_PassesThroughUnconfiguredHosts(t *testing.T) {
	rec := &recordingRT{}
	tr := mustNewTransport(t, rec, cioidc.WithHostToken("configured.example", "tok"))

	get(t, tr, "other.example")

	if len(rec.auths) != 1 || rec.auths[0] != "Basic should-be-overwritten" {
		t.Errorf("Authorization = %v, want untouched Basic header", rec.auths)
	}
}

func TestTransport_StaticTokenStampedAsIs(t *testing.T) {
	rec := &recordingRT{}
	tr := mustNewTransport(t, rec, cioidc.WithHostToken("static.example", "static.jwt.value"))

	get(t, tr, "static.example")
	get(t, tr, "static.example")

	want := []string{"Bearer static.jwt.value", "Bearer static.jwt.value"}
	if len(rec.auths) != 2 || rec.auths[0] != want[0] || rec.auths[1] != want[1] {
		t.Errorf("Authorization = %v, want %v", rec.auths, want)
	}
}

func TestTransport_MintsAndCachesPerHost(t *testing.T) {
	ts := newTokenServer(t, time.Hour)
	rec := &recordingRT{}
	tr := mustNewTransport(t, rec, cioidc.WithHostAudience("mint.example", "aud"))

	for range 5 {
		get(t, tr, "mint.example")
	}

	if ts.calls.Load() != 1 {
		t.Errorf("token endpoint called %d times, want 1 (cached)", ts.calls.Load())
	}
	for _, a := range rec.auths {
		if !strings.HasPrefix(a, "Bearer ") || strings.Contains(a, "Basic") {
			t.Errorf("Authorization = %v, want minted Bearer tokens", rec.auths)
			break
		}
	}
}

func TestTransport_RemintsAfterHalfLife(t *testing.T) {
	// 4s TTL → cached for ~2s (above the 1s near-expiry guard). After 2.5s the
	// next call must remint.
	ts := newTokenServer(t, 4*time.Second)
	rec := &recordingRT{}
	tr := mustNewTransport(t, rec, cioidc.WithHostAudience("mint.example", "aud"))

	get(t, tr, "mint.example")
	if ts.calls.Load() != 1 {
		t.Fatalf("token endpoint called %d times, want 1", ts.calls.Load())
	}

	time.Sleep(2500 * time.Millisecond)

	get(t, tr, "mint.example")
	if ts.calls.Load() != 2 {
		t.Errorf("token endpoint called %d times, want 2 (reminted)", ts.calls.Load())
	}
}

func TestTransport_NearExpiredMintErrors(t *testing.T) {
	newTokenServer(t, 500*time.Millisecond) // half-life 250ms < 1s guard
	rec := &recordingRT{}
	tr := mustNewTransport(t, rec, cioidc.WithHostAudience("mint.example", "aud"))

	req, _ := http.NewRequest(http.MethodGet, "https://mint.example/v2/", nil)
	_, err := tr.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "near expiry") {
		t.Fatalf("expected near-expiry error, got: %v", err)
	}
}

func TestTransport_DoesNotMutateCallerRequest(t *testing.T) {
	rec := &recordingRT{}
	tr := mustNewTransport(t, rec, cioidc.WithHostToken("static.example", "static.jwt"))

	req, _ := http.NewRequest(http.MethodGet, "https://static.example/v2/", nil)
	req.Header.Set("Authorization", "Basic original")
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Basic original" {
		t.Errorf("caller request mutated: Authorization = %q", got)
	}
}

func TestTransport_RoutesPerHost(t *testing.T) {
	newTokenServer(t, time.Hour)
	rec := &recordingRT{}
	tr := mustNewTransport(t, rec,
		cioidc.WithHostToken("static.example", "static-token"),
		cioidc.WithHostAudience("mint.example", "aud"),
	)

	get(t, tr, "static.example")
	get(t, tr, "mint.example")
	get(t, tr, "other.example")

	if rec.auths[0] != "Bearer static-token" {
		t.Errorf("static host auth = %q, want Bearer static-token", rec.auths[0])
	}
	if !strings.HasPrefix(rec.auths[1], "Bearer ") || rec.auths[1] == "Bearer static-token" {
		t.Errorf("mint host auth = %q, want a distinct minted Bearer token", rec.auths[1])
	}
	if rec.auths[2] != "Basic should-be-overwritten" {
		t.Errorf("unconfigured host auth = %q, want untouched", rec.auths[2])
	}
}
