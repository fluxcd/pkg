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

// Package cijwt provides an http.RoundTripper that authenticates outbound
// requests on a per-host basis with a JWT, sourcing the token from a CI/CD
// platform's OIDC integration or signing it locally.
//
// Each configured host gets its token one of four ways:
//   - WithHostAudience mints an OIDC ID token for the given audience from the
//     GitHub/Forgejo Actions token endpoint (see the actionsoidc package),
//     caching it for the first 50% of its lifetime and reminting on demand.
//   - WithHostToken sends a static JWT as-is, e.g. a GitLab CI id_token injected
//     into the job environment.
//   - WithHostTokenFile reads the JWT from a file for every request, so a token
//     rotated by an external process is picked up without restarting.
//   - WithHostJWK signs a fresh, short-lived JWT with a private key from a JWK,
//     issuing a new token for every request rather than caching it.
//
// Requests to hosts that were not configured are forwarded unchanged, so a
// request to a registry the JWT is not meant for keeps its existing
// authentication.
package cijwt

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fluxcd/pkg/auth/actionsoidc"
	"github.com/fluxcd/pkg/auth/jwt"
)

// jwkTokenTTL is the lifetime of the JWTs signed for WithHostJWK hosts. They are
// minted fresh for every request, so the window only needs to cover a single
// request's round trip plus clock skew between the issuer and the verifier.
const jwkTokenTTL = 60 * time.Second

type hostValue struct {
	host  string
	value string
}

type hostJWK struct {
	host string
	jwk  string
	iss  string
	aud  string
	sub  string
}

type options struct {
	inner      http.RoundTripper
	tokens     []hostValue
	tokenFiles []hostValue
	audiences  []hostValue
	jwks       []hostJWK
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

// WithHostTokenFile configures host to be authenticated with a static JWT read
// from path. The file is read on every request, with leading and trailing
// whitespace trimmed, so a token rotated by an external process (e.g. a
// projected service account token) is picked up without restarting. An
// unreadable or empty file errors the request.
func WithHostTokenFile(host, path string) Option {
	return func(o *options) { o.tokenFiles = append(o.tokenFiles, hostValue{host, path}) }
}

// WithHostAudience configures host to be authenticated with an OIDC ID token
// minted for the given audience from the GitHub/Forgejo Actions token endpoint,
// cached for the first 50% of its lifetime and reminted on demand.
func WithHostAudience(host, audience string) Option {
	return func(o *options) { o.audiences = append(o.audiences, hostValue{host, audience}) }
}

// WithHostJWK configures host to be authenticated with a JWT signed locally
// using a private key parsed from jwk (a single JSON Web Key holding an Ed25519
// or ECDSA private key; the signing algorithm is derived from the key type, see
// the jwt package). Each request gets a freshly signed, 60-second-lived token
// carrying iss, aud, and sub as given and the signing key's id in the "kid"
// header. Unlike WithHostAudience, the token is never cached.
func WithHostJWK(host, jwk, iss, aud, sub string) Option {
	return func(o *options) { o.jwks = append(o.jwks, hostJWK{host, jwk, iss, aud, sub}) }
}

type cacheEntry struct {
	token string
	// exp is when the cached token must be reminted. A zero value means it
	// never expires (a static token configured with WithHostToken).
	exp time.Time
}

type jwkConfig struct {
	key *jwt.SigningKey
	iss string
	aud string
	sub string
}

// Transport is an http.RoundTripper that stamps Authorization: Bearer <jwt> on
// requests whose URL host was configured with WithHostToken, WithHostTokenFile,
// WithHostAudience, or WithHostJWK. Any existing Authorization header on a
// configured host is overwritten; requests to other hosts pass through
// untouched.
type Transport struct {
	inner http.RoundTripper
	// audiences maps a host to the audience minted for it; the factory used on
	// a cache miss.
	audiences map[string]string
	// jwk maps a host to the signing config used to mint a fresh token for
	// every request. It is read-only after construction.
	jwk map[string]jwkConfig
	// tokenFiles maps a host to a file path read on every request. It is
	// read-only after construction.
	tokenFiles map[string]string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewTransport returns a Transport configured by opts. At least one host must be
// configured. It returns an error if the same host is configured more than once,
// whether via WithHostToken, WithHostTokenFile, WithHostAudience, WithHostJWK,
// or a mix of them, or if a WithHostJWK key fails to parse.
func NewTransport(opts ...Option) (*Transport, error) {
	o := &options{inner: http.DefaultTransport}
	for _, opt := range opts {
		opt(o)
	}

	t := &Transport{
		inner:      o.inner,
		audiences:  make(map[string]string, len(o.audiences)),
		jwk:        make(map[string]jwkConfig, len(o.jwks)),
		tokenFiles: make(map[string]string, len(o.tokenFiles)),
		cache:      make(map[string]cacheEntry, len(o.tokens)),
	}

	seen := make(map[string]bool, len(o.tokens)+len(o.tokenFiles)+len(o.audiences)+len(o.jwks))
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
	for _, hv := range o.tokenFiles {
		if err := claim(hv.host); err != nil {
			return nil, err
		}
		t.tokenFiles[hv.host] = hv.value
	}
	for _, hv := range o.audiences {
		if err := claim(hv.host); err != nil {
			return nil, err
		}
		t.audiences[hv.host] = hv.value
	}
	for _, hj := range o.jwks {
		if err := claim(hj.host); err != nil {
			return nil, err
		}
		key, err := jwt.ParseJWK(hj.jwk)
		if err != nil {
			return nil, fmt.Errorf("host %q: %w", hj.host, err)
		}
		t.jwk[hj.host] = jwkConfig{key: key, iss: hj.iss, aud: hj.aud, sub: hj.sub}
	}

	if len(seen) == 0 {
		return nil, fmt.Errorf("at least one host must be configured with WithHostToken, WithHostTokenFile, WithHostAudience, or WithHostJWK")
	}

	return t, nil
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, ok, err := t.tokenForHost(req.Context(), req.URL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain CI JWT for host %q: %w", req.URL.Host, err)
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

// tokenForHost returns the bearer token for host. WithHostJWK and
// WithHostTokenFile hosts get a fresh token on every call; WithHostAudience
// hosts are minted and cached on a miss. The boolean is false when host was
// not configured.
func (t *Transport) tokenForHost(ctx context.Context, host string) (string, bool, error) {
	// JWK and token-file hosts produce a fresh token per request and never
	// touch the cache, so they need no locking (both maps are read-only after
	// construction).
	if cfg, ok := t.jwk[host]; ok {
		token, err := cfg.key.Issue(cfg.iss, cfg.sub, cfg.aud, jwkTokenTTL)
		if err != nil {
			return "", false, err
		}
		return token, true, nil
	}
	if path, ok := t.tokenFiles[host]; ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("read token file: %w", err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", false, fmt.Errorf("token file %q is empty", path)
		}
		return token, true, nil
	}

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
