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

	"sigs.k8s.io/controller-runtime/pkg/client"

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
	var saInfo *serviceAccountInfo
	if o.ShouldGetServiceAccount() {
		// Check the feature gate for object-level workload identity.
		if !IsObjectLevelWorkloadIdentityEnabled() {
			return nil, ErrObjectLevelWorkloadIdentityNotEnabled
		}

		// Fetch service account details.
		var err error
		saInfo, err = getServiceAccountInfo(ctx, provider, o.Client, opts...)
		if err != nil {
			return nil, err
		}

		// Update the function to create an access token using the service account.
		if saInfo.useServiceAccount {
			newAccessToken = func() (Token, error) {
				// Issue Kubernetes OIDC token for the service account.
				saKey := client.ObjectKeyFromObject(saInfo.obj)
				oidcToken, err := CreateServiceAccountToken(ctx, o.Client, saKey, saInfo.audiences...)
				if err != nil {
					return nil, fmt.Errorf("failed to create kubernetes token for service account '%s/%s': %w",
						saInfo.obj.Namespace, saInfo.obj.Name, err)
				}

				// Exchange the Kubernetes OIDC token for a provider access token.
				token, err := provider.NewTokenForServiceAccount(ctx, oidcToken, *saInfo.obj, opts...)
				if err != nil {
					return nil, fmt.Errorf("failed to create provider access token for service account '%s/%s': %w",
						saInfo.obj.Namespace, saInfo.obj.Name, err)
				}

				return token, nil
			}
		}
	}

	// Update access token fetcher for impersonation if supported by the provider.
	if saInfo != nil && saInfo.providerIdentityForImpersonation != nil {
		newNonImpersonatedToken := newAccessToken
		newAccessToken = func() (Token, error) {
			token, err := newNonImpersonatedToken()
			if err != nil {
				return nil, err
			}
			p := provider.(ProviderWithImpersonation)
			return p.NewTokenForIdentity(ctx, token, saInfo.providerIdentityForImpersonation, opts...)
		}
	}

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newAccessToken()
	}

	// Build cache key.
	cacheKey := buildAccessTokenCacheKey(provider, saInfo, opts...)

	// Build involved object details.
	kind := o.InvolvedObject.Kind
	name := o.InvolvedObject.Name
	namespace := o.InvolvedObject.Namespace
	operation := o.InvolvedObject.Operation

	// Get token from cache.
	token, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newAccessToken()
	}, cache.WithInvolvedObject(kind, name, namespace, operation))
	if err != nil {
		return nil, err
	}

	return token, nil
}

func buildAccessTokenCacheKey(provider Provider, saInfo *serviceAccountInfo, opts ...Option) string {

	var o Options
	o.Apply(opts...)

	var parts []string

	parts = append(parts, fmt.Sprintf("provider=%s", provider.GetName()))

	if saInfo != nil {
		if saInfo.useServiceAccount {
			parts = append(parts, fmt.Sprintf("serviceAccountName=%s", saInfo.obj.Name))
			parts = append(parts, fmt.Sprintf("serviceAccountNamespace=%s", saInfo.obj.Namespace))
			parts = append(parts, fmt.Sprintf("serviceAccountTokenAudiences=%s", strings.Join(saInfo.audiences, ",")))
			parts = append(parts, fmt.Sprintf("providerIdentity=%s", saInfo.providerIdentity))
		}
		if saInfo.providerIdentityForImpersonation != nil {
			parts = append(parts, fmt.Sprintf("providerIdentityForImpersonation=%s", saInfo.providerIdentityForImpersonation))
		}
	}

	if len(o.Scopes) > 0 {
		parts = append(parts, fmt.Sprintf("scopes=%s", strings.Join(o.Scopes, ",")))
	}

	if o.STSRegion != "" {
		parts = append(parts, fmt.Sprintf("stsRegion=%s", o.STSRegion))
	}

	if o.STSEndpoint != "" {
		parts = append(parts, fmt.Sprintf("stsEndpoint=%s", o.STSEndpoint))
	}

	if o.ProxyURL != nil {
		parts = append(parts, fmt.Sprintf("proxyURL=%s", o.ProxyURL))
	}

	if o.CAData != "" {
		parts = append(parts, fmt.Sprintf("caData=%s", o.CAData))
	}

	return buildCacheKey(parts...)
}
