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

	corev1 "k8s.io/api/core/v1"
)

// ServiceProvider contains the logic to retrieve security credentials
// for accessing resources in a service, e.g. a cloud provider.
type ServiceProvider interface {
	// GetName returns the name of the service provider.
	GetName() string

	// NewAmbientCredential returns a token that can be used to authenticate
	// with the service provider retrieved from the ambient environment,
	// e.g. from the environment of the process, files mounted in the pod,
	// environment variables, local metadata services, etc.
	NewAmbientCredential(ctx context.Context, opts ...Option) (Credential, error)

	// GetJWTAudiences returns the audiences the OIDC tokens issued representing
	// ServiceAccounts should have. These are usually strings that represent
	// the service provider's STS service, or some entity in the provider for
	// which the OIDC tokens are targeted to.
	GetJWTAudiences(ctx context.Context, serviceAccount corev1.ServiceAccount) ([]string, error)

	// GetJWTIdentity takes a ServiceAccount and returns the identity which the
	// ServiceAccount wants to impersonate, by looking at annotations.
	GetJWTIdentity(serviceAccount corev1.ServiceAccount) (string, error)

	// NewCredentialForMaterial takes a CredentialMaterial and returns a token that can
	// be used to authenticate with the service provider. Implementations may
	// exchange the credential for a service-specific token (e.g. a cloud
	// provider access token), or return it as-is when no exchange is needed.
	NewCredentialForMaterial(ctx context.Context, credential CredentialMaterial, opts ...Option) (Credential, error)
}
