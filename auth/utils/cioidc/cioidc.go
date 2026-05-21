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

// Package cioidc provides an http.RoundTripper that authenticates outbound
// requests on a per-host basis with a JWT obtained from a CI/CD platform's OIDC
// integration.
//
// Each configured host gets its token one of two ways:
//   - WithHostAudience mints an OIDC ID token for the given audience from the
//     GitHub/Forgejo Actions token endpoint (see the actionsoidc package),
//     caching it for the first 50% of its lifetime and reminting on demand.
//   - WithHostToken sends a static JWT as-is, e.g. a GitLab CI id_token injected
//     into the job environment.
//
// Requests to hosts that were not configured are forwarded unchanged, so a
// request to a registry the JWT is not meant for keeps its existing
// authentication.
package cioidc

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fluxcd/pkg/auth/actionsoidc"
)

type hostValue struct {
	host  string
	value string
}

type options struct {
	inner     http.RoundTripper
	tokens    []hostValue
	audiences []hostValue
}

// Option configures a Transport.
type Option func(*options)

// WithInner sets the underlying RoundTripper that requests are forwarded to.
// Defaults to http.DefaultTransport.
func WithInner(rt http.RoundTripper) Option {
	return func(o *options) { o.inner = rt }
}

// WithHostToken configures host to be authenticated with the given static JWT,
// sent as-is (e.g. a GitLab CI id_token).
func WithHostToken(host, token string) Option {
	return func(o *options) { o.tokens = append(o.tokens, hostValue{host, token}) }
}

// WithHostAudience configures host to be authenticated with an OIDC ID token
// minted for the given audience from the GitHub/Forgejo Actions token endpoint,
// cached for the first 50% of its lifetime and reminted on demand.
func WithHostAudience(host, audience string) Option {
	return func(o *options) { o.audiences = append(o.audiences, hostValue{host, audience}) }
}

type cacheEntry struct {
	token string
	// exp is when the cached token must be reminted. A zero value means it
	// never expires (a static token configured with WithHostToken).
	exp time.Time
}

// Transport is an http.RoundTripper that stamps Authorization: Bearer <jwt> on
// requests whose URL host was configured with WithHostToken or WithHostAudience.
// Any existing Authorization header on a configured host is overwritten;
// requests to other hosts pass through untouched.
type Transport struct {
	inner http.RoundTripper
	// audiences maps a host to the audience minted for it; the factory used on
	// a cache miss.
	audiences map[string]string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewTransport returns a Transport configured by opts. At least one host must be
// configured. It returns an error if the same host is configured more than once,
// whether via WithHostToken, WithHostAudience, or a mix of the two.
func NewTransport(opts ...Option) (*Transport, error) {
	o := &options{inner: http.DefaultTransport}
	for _, opt := range opts {
		opt(o)
	}

	t := &Transport{
		inner:     o.inner,
		audiences: make(map[string]string, len(o.audiences)),
		cache:     make(map[string]cacheEntry, len(o.tokens)),
	}

	seen := make(map[string]bool, len(o.tokens)+len(o.audiences))
	claim := func(host string) error {
		if seen[host] {
			return fmt.Errorf("host %q is configured more than once", host)
		}
		seen[host] = true
		return nil
	}

	for _, hv := range o.tokens {
		if err := claim(hv.host); err != nil {
			return nil, err
		}
		// Seed the cache with a static token that never expires.
		t.cache[hv.host] = cacheEntry{token: hv.value}
	}
	for _, hv := range o.audiences {
		if err := claim(hv.host); err != nil {
			return nil, err
		}
		t.audiences[hv.host] = hv.value
	}

	if len(seen) == 0 {
		return nil, fmt.Errorf("at least one host must be configured with WithHostToken or WithHostAudience")
	}

	return t, nil
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, ok, err := t.tokenForHost(req.Context(), req.URL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain CI OIDC token for host %q: %w", req.URL.Host, err)
	}
	if !ok {
		// Host not configured: forward unchanged, preserving existing auth.
		return t.inner.RoundTrip(req)
	}
	// Clone so the Authorization edit is scoped to this request and does not
	// mutate the caller's request.
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Authorization", "Bearer "+token)
	return t.inner.RoundTrip(cloned)
}

// tokenForHost returns the bearer token for host, minting and caching it on a
// miss. The boolean is false when host was not configured.
func (t *Transport) tokenForHost(ctx context.Context, host string) (string, bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if e, ok := t.cache[host]; ok && (e.exp.IsZero() || now.Before(e.exp)) {
		return e.token, true, nil
	}

	audience, ok := t.audiences[host]
	if !ok {
		// Not configured (static hosts are always served from the cache above).
		return "", false, nil
	}

	token, exp, err := actionsoidc.FetchToken(ctx, audience)
	if err != nil {
		return "", false, err
	}
	// Cache for the first 50% of the remaining lifetime, reminting afterwards.
	half := exp.Sub(now) / 2
	if half < time.Second {
		return "", false, fmt.Errorf("minted token already near expiry (exp=%s)", exp)
	}
	t.cache[host] = cacheEntry{token: token, exp: now.Add(half)}
	return token, true, nil
}
