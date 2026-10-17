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
	"strings"

	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/cache"
)

// GetAccessToken returns an access token for accessing resources in the given cloud provider.
func GetAccessToken(ctx context.Context, provider Provider, opts ...Option) (Token, error) {

	var o Options
	o.Apply(opts...)

	// Initialize access token fetcher for controller.
	newAccessToken := func() (Token, error) {
		token, err := provider.NewControllerToken(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider access token for the controller: %w", err)
		}
		return token, nil
	}

	// Update access token fetcher for a service account if specified.
	var serviceAccount *corev1.ServiceAccount
	var providerIdentity string
	var audiences []string
	if o.ShouldGetServiceAccountToken() {
		var err error
		serviceAccount, audiences, providerIdentity, err =
			getServiceAccountAndProviderInfo(ctx, provider, opts...)
		if err != nil {
			return nil, err
		}

		// Update the function to create an access token using the service account.
		newAccessToken = func() (Token, error) {
			// Issue Kubernetes OIDC token for the service account.
			tokenReq := &authnv1.TokenRequest{
				Spec: authnv1.TokenRequestSpec{
					Audiences: audiences,
				},
			}
			if err := o.Client.SubResource("token").Create(ctx, serviceAccount, tokenReq); err != nil {
				return nil, fmt.Errorf("failed to create kubernetes token for service account '%s/%s': %w",
					serviceAccount.Namespace, serviceAccount.Name, err)
			}
			oidcToken := tokenReq.Status.Token

			// Exchange the Kubernetes OIDC token for a provider access token.
			token, err := provider.NewTokenForServiceAccount(ctx, oidcToken, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create provider access token for service account '%s/%s': %w",
					serviceAccount.Namespace, serviceAccount.Name, err)
			}

			return token, nil
		}
	}

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newAccessToken()
	}

	// Build cache key.
	cacheKey := buildAccessTokenCacheKey(provider, audiences, providerIdentity, serviceAccount, opts...)

	// Get token from cache.
	io := o.InvolvedObject()
	token, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newAccessToken()
	}, cache.WithInvolvedObject(io.Kind, io.Name, io.Namespace, io.Operation))
	if err != nil {
		return nil, err
	}

	return token, nil
}

func getServiceAccountAndProviderInfo(ctx context.Context, provider Provider,
	opts ...Option) (*corev1.ServiceAccount, []string, string, error) {

	var o Options
	o.Apply(opts...)

	serviceAccount := o.GetServiceAccount()

	// Get provider audience.
	audiences := o.CredentialAudiences
	if len(audiences) == 0 {
		var err error
		audiences, err = provider.GetAudiences(ctx, opts...)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to get provider audience: %w", err)
		}
	}

	// Get provider identity.
	providerIdentity, err := provider.GetIdentity(opts...)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get provider identity from service account '%s/%s': %w",
			serviceAccount.Namespace, serviceAccount.Name, err)
	}

	return &serviceAccount, audiences, providerIdentity, nil
}

func buildAccessTokenCacheKey(provider Provider, audiences []string, providerIdentity string,
	serviceAccount *corev1.ServiceAccount, opts ...Option) string {

	var o Options
	o.Apply(opts...)

	var parts []string

	parts = append(parts, fmt.Sprintf("provider=%s", provider.GetName()))

	if len(audiences) == 0 {
		audiences = o.CredentialAudiences
	}
	if len(audiences) > 0 {
		parts = append(parts, fmt.Sprintf("audiences=%s", strings.Join(audiences, ",")))
	}

	if serviceAccount != nil {
		parts = append(parts, fmt.Sprintf("providerIdentity=%s", providerIdentity))
		parts = append(parts, fmt.Sprintf("serviceAccountName=%s", serviceAccount.Name))
		parts = append(parts, fmt.Sprintf("serviceAccountNamespace=%s", serviceAccount.Namespace))
	}

	if len(o.Scopes) > 0 {
		parts = append(parts, fmt.Sprintf("scopes=%s", strings.Join(o.Scopes, ",")))
	}

	if o.AWSRegion != "" {
		parts = append(parts, fmt.Sprintf("stsRegion=%s", o.AWSRegion))
	}

	if o.AWSSTSEndpoint != "" {
		parts = append(parts, fmt.Sprintf("stsEndpoint=%s", o.AWSSTSEndpoint))
	}

	if o.ProxyURL != nil {
		parts = append(parts, fmt.Sprintf("proxyURL=%s", o.ProxyURL))
	}

	if o.CAData != "" {
		parts = append(parts, fmt.Sprintf("caData=%s", o.CAData))
	}

	return buildCacheKey(parts...)
}
