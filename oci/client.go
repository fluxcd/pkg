/*
Copyright 2022 The Flux authors

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
	"net/http"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// Client holds the options for accessing remote OCI registries.
type Client struct {
	options []remote.Option
}

// NewClient returns an OCI client configured with the given remote options.
func NewClient(opts []remote.Option) *Client {
	options := []remote.Option{
		remote.WithUserAgent(UserAgent),
	}
	options = append(options, opts...)

	return &Client{options: options}
}

// DefaultOptions returns an empty list of client options.
func DefaultOptions() []remote.Option {
	return []remote.Option{}
}

// GetOptions returns the list of remote.Option used by this Client.
func (c *Client) GetOptions() []remote.Option {
	return c.options
}

// optionsWithContext returns the remote options for the given context.
func (c *Client) optionsWithContext(ctx context.Context) []remote.Option {
	options := []remote.Option{
		remote.WithContext(ctx),
	}
	return append(options, c.options...)
}

// WithRetryBackOff returns a function for setting the given backoff on
// remote.Option.
func WithRetryBackOff(backoff remote.Backoff) remote.Option {
	return remote.WithRetryBackoff(backoff)
}

// WithTransport returns a remote.Option that sets the HTTP transport.
func WithTransport(t http.RoundTripper) remote.Option {
	return remote.WithTransport(t)
}

// defaultRetryTransport wraps an http.RoundTripper with retry logic
// suitable for use with the remote package.
func defaultRetryTransport(inner http.RoundTripper) http.RoundTripper {
	return transport.NewRetry(inner,
		transport.WithRetryPredicate(defaultRetryPredicate),
		transport.WithRetryStatusCodes(retryableStatusCodes...),
	)
}
