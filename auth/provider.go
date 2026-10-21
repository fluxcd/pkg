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
	"context"
)

// Provider contains the logic to retrieve security credentials
// for accessing resources in a cloud provider.
type Provider interface {
	// GetName returns the name of the cloud provider.
	GetName() string

	// NewControllerToken returns a credential that can be used to authenticate
	// with the cloud provider retrieved from the default source, i.e. from
	// the environment of the controller pod, e.g. files mounted in the pod,
	// environment variables, local metadata services, etc.
	NewControllerToken(ctx context.Context, opts ...Option) (Credential, error)

	// GetAudiences returns the audiences the OIDC tokens issued by the
	// credential provider should have. These are usually strings that represent
	// the cloud provider's STS service, or some entity in the provider for
	// which the OIDC tokens are targeted to. The provider decides whether any
	// caller-provided credential audiences are used.
	GetAudiences(ctx context.Context, opts ...Option) ([]string, error)

	// GetIdentity returns the identity which the ServiceAccount wants to
	// impersonate, as configured in the options.
	GetIdentity(opts ...Option) (string, error)

	// NewAccessCredentialForIdentity takes the identity credential and
	// returns a credential that can be used to authenticate with the cloud
	// provider. The implementation should exchange this credential for a
	// cloud provider access credential through the provider's STS service.
	NewAccessCredentialForIdentity(ctx context.Context, identity Credential, opts ...Option) (Credential, error)
}
