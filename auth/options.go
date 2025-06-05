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

package auth

import (
	"net/http"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/cache"
)

// Option is a functional option for the auth package.
type Option func(*Options)

// Options contains options for configuring the behavior of the provider methods.
// Not all providers/methods support all options.
type Options struct {
	Client             client.Client
	Cache              *cache.TokenCache
	ServiceAccount     *client.ObjectKey
	InvolvedObject     cache.InvolvedObject
	Scopes             []string
	ArtifactRepository string
	STSRegion          string
	STSEndpoint        string
	ProxyURL           *url.URL
	AllowShellOut      bool
}

// WithServiceAccount sets the ServiceAccount reference for the token
// and a controller-runtime client to fetch the ServiceAccount and
// create an OIDC token for it in the Kubernetes API.
func WithServiceAccount(saRef client.ObjectKey, client client.Client) Option {
	return func(o *Options) {
		o.ServiceAccount = &saRef
		o.Client = client
	}
}

// WithCache sets the token cache and the involved object for recording events.
func WithCache(cache cache.TokenCache, involvedObject cache.InvolvedObject) Option {
	return func(o *Options) {
		o.Cache = &cache
		o.InvolvedObject = involvedObject
	}
}

// WithScopes sets the scopes for the token.
func WithScopes(scopes ...string) Option {
	return func(o *Options) {
		o.Scopes = scopes
	}
}

// WithArtifactRepository sets the artifact repository the token will be used for.
// In most cases artifact registry credentials require an additional
// token exchange at the end. This option allows the library to implement
// this exchange and cache the final token.
func WithArtifactRepository(artifactRepository string) Option {
	return func(o *Options) {
		o.ArtifactRepository = artifactRepository
	}
}

// WithSTSRegion sets the region for the STS service (some cloud providers
// require a region, e.g. AWS).
func WithSTSRegion(stsRegion string) Option {
	return func(o *Options) {
		o.STSRegion = stsRegion
	}
}

// WithSTSEndpoint sets the endpoint for the STS service.
func WithSTSEndpoint(stsEndpoint string) Option {
	return func(o *Options) {
		o.STSEndpoint = stsEndpoint
	}
}

// WithProxyURL sets a *url.URL for an HTTP/S proxy for acquiring the token.
func WithProxyURL(proxyURL url.URL) Option {
	return func(o *Options) {
		o.ProxyURL = &proxyURL
	}
}

// WithAllowShellOut allows the provider to shell out to binaries.
func WithAllowShellOut() Option {
	return func(o *Options) {
		o.AllowShellOut = true
	}
}

// Apply applies the given slice of Option(s) to the Options struct.
func (o *Options) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// GetHTTPClient returns a *http.Client with the configured proxy URL
// or nil if no proxy URL is set.
func (o *Options) GetHTTPClient() *http.Client {
	if o.ProxyURL == nil {
		return nil
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(o.ProxyURL)
	return &http.Client{Transport: transport}
}
