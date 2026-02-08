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
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// Identity represents a cloud provider identity that can be impersonated.
type Identity fmt.Stringer

// Provider contains the logic to retrieve security credentials
// for accessing resources in a cloud provider.
type Provider interface {
	// GetName returns the name of the cloud provider.
	GetName() string

	// NewControllerToken returns a token that can be used to authenticate
	// with the cloud provider retrieved from the default source, i.e. from
	// the environment of the controller pod, e.g. files mounted in the pod,
	// environment variables, local metadata services, etc.
	NewControllerToken(ctx context.Context, opts ...Option) (Token, error)
}

// ProviderWithOIDCImpersonation is an optional interface that providers can
// implement if they support impersonation from OIDC tokens.
type ProviderWithOIDCImpersonation interface {
	Provider

	// GetAudiences returns the audiences for OIDC exchange. The first
	// audience should be added to the OIDC token itself as the "aud"
	// claim, and the second is the exchangeAudience input for the
	// NewTokenForOIDCToken method.
	GetAudiences(ctx context.Context, serviceAccount corev1.ServiceAccount) (string, string, error)

	// GetIdentity takes a ServiceAccount and returns the identity which the
	// ServiceAccount wants to impersonate, by looking at annotations.
	GetIdentity(serviceAccount corev1.ServiceAccount) (Identity, error)

	// NewTokenForOIDCToken takes an OIDC token and target identity and
	// returns a native provider token that can be used to authenticate
	// with the cloud provider APIs representing the given identity.
	// This may or may not require an exchange step and depends entirely
	// on the provider. Providers that do not require an exchange step
	// normally ignore the target identity and just return the OIDC token
	// itself wrapped in the provider's token type.
	NewTokenForOIDCToken(ctx context.Context, oidcToken, exchangeAudience string,
		targetIdentity Identity, opts ...Option) (Token, error)
}

// ProviderWithImpersonation is an optional interface that providers can
// implement if they support impersonation from native tokens.
type ProviderWithImpersonation interface {
	Provider

	// GetImpersonationAnnotationKey returns the annotation key without API group
	// that should be used to specify an identity in a Kubernetes ServiceAccount.
	GetImpersonationAnnotationKey() string

	// NewIdentity returns an empty identity struct that can be used to unmarshal
	// the identity description from YAML. Currently used only for impersonation.
	NewIdentity() Identity

	// NewTokenForNativeToken takes an initial provider token and identity and
	// returns another provider token that can be used to authenticate with
	// the cloud provider representing the given identity.
	NewTokenForNativeToken(ctx context.Context, nativeToken Token,
		targetIdentity Identity, opts ...Option) (Token, error)
}
