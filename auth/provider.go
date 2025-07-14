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

	// GetAudiences returns the audiences the OIDC tokens issued representing
	// ServiceAccounts should have. These are usually strings that represent
	// the cloud provider's STS service, or some entity in the provider for
	// which the OIDC tokens are targeted to.
	GetAudiences(ctx context.Context, serviceAccount corev1.ServiceAccount) ([]string, error)

	// GetIdentity takes a ServiceAccount and returns the identity which the
	// ServiceAccount wants to impersonate, by looking at annotations.
	GetIdentity(serviceAccount corev1.ServiceAccount) (string, error)

	// NewToken takes a ServiceAccount and its OIDC token and returns a token
	// that can be used to authenticate with the cloud provider. The OIDC token is
	// the JWT token that was issued for the ServiceAccount by the Kubernetes API.
	// The implementation should exchange this token for a cloud provider access
	// token through the provider's STS service.
	NewTokenForServiceAccount(ctx context.Context, oidcToken string,
		serviceAccount corev1.ServiceAccount, opts ...Option) (Token, error)
}
