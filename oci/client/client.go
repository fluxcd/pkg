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

package client

import (
	"context"

	"github.com/google/go-containerregistry/pkg/crane"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/fluxcd/pkg/oci"
)

// Client holds the options for accessing remote OCI registries.
type Client struct {
	options []crane.Option
}

// NewClient returns an OCI client configured with the given crane options.
func NewClient(opts []crane.Option) *Client {
	options := []crane.Option{
		crane.WithUserAgent(oci.UserAgent),
	}
	options = append(options, opts...)

	return &Client{options: options}
}

// NewLocalClient returns an OCI client configured with the Docker keychain helpers.
func NewLocalClient() *Client {
	options := []crane.Option{
		crane.WithUserAgent(oci.UserAgent),
		crane.WithPlatform(&gcrv1.Platform{
			Architecture: "flux",
			OS:           "flux",
			OSVersion:    "v2",
		}),
	}
	return &Client{options: options}
}

// optionsWithContext returns the crane options for the given context.
func (c *Client) optionsWithContext(ctx context.Context) []crane.Option {
	options := []crane.Option{
		crane.WithContext(ctx),
	}
	return append(options, c.options...)
}
