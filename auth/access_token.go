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
	"slices"
	"strings"

	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/crypto"
	"github.com/fluxcd/pkg/cache"
)

// GetCredential returns a credential for accessing resources in the given
// service provider.
func GetCredential(ctx context.Context, provider ServiceProvider, opts ...Option) (Credential, error) {

	var o Options
	o.Apply(opts...)

	// Effective audiences, preferring the credential configuration.
	effectiveAudiences := o.Audiences
	if len(effectiveAudiences) == 0 && o.Credential != nil {
		effectiveAudiences = o.Credential.Audiences
	}

	// newCredential obtains a credential from the configured credential
	// provider, if any.
	newCredential := func(audiences []string) (CredentialMaterial, error) {
		if o.CredentialProvider == nil {
			return nil, nil
		}
		sourceOpts := opts
		if len(audiences) > 0 {
			sourceOpts = append(slices.Clone(opts), WithAudiences(audiences...))
		}
		credentialType := crypto.CredentialTypeJWT
		if o.Credential != nil {
			credentialType = o.Credential.Type
		}
		switch credentialType {
		case crypto.CredentialTypeJWT:
			p, ok := o.CredentialProvider.(JWTProvider)
			if !ok {
				return nil, fmt.Errorf("credential provider '%s' does not support JWT credentials",
					o.CredentialProvider.GetName())
			}
			return p.NewJWT(ctx, sourceOpts...)
		case crypto.CredentialTypeX509:
			p, ok := o.CredentialProvider.(X509Provider)
			if !ok {
				return nil, fmt.Errorf("credential provider '%s' does not support X.509 credentials",
					o.CredentialProvider.GetName())
			}
			return p.NewX509(ctx, sourceOpts...)
		default:
			return nil, fmt.Errorf("unsupported credential type '%s'", credentialType)
		}
	}

	// Initialize access token fetcher for the ambient environment.
	newAccessToken := func() (Credential, error) {
		if o.CredentialProvider != nil {
			cred, err := newCredential(effectiveAudiences)
			if err != nil {
				return nil, err
			}
			return provider.NewCredentialForMaterial(ctx, cred, opts...)
		}
		credential, err := provider.NewAmbientCredential(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider access token for the ambient environment: %w", err)
		}
		return credential, nil
	}

	// Update access token fetcher for a service account if specified.
	var serviceAccount *corev1.ServiceAccount
	var providerIdentity string
	var audiences []string
	if o.ShouldGetServiceAccount() {
		// Fetch service account details.
		var err error
		saRef := client.ObjectKey{
			Name:      o.ServiceAccountName,
			Namespace: o.ServiceAccountNamespace,
		}
		serviceAccount, audiences, providerIdentity, err =
			getServiceAccountAndProviderInfo(ctx, provider, o.Client, saRef, opts...)
		if err != nil {
			return nil, err
		}

		// Make the ServiceAccount available to the credential and service
		// providers.
		opts = append(opts, WithServiceAccount(*serviceAccount))

		// Update the function to create an access token using the service account.
		newAccessToken = func() (Credential, error) {
			// Check the feature gate for object-level workload identity.
			if !IsObjectLevelWorkloadIdentityEnabled() {
				return nil, ErrObjectLevelWorkloadIdentityNotEnabled
			}

			// Obtain the credential from the credential provider, or issue a
			// Kubernetes token directly if no credential provider is configured.
			var cred CredentialMaterial
			if o.CredentialProvider != nil {
				var err error
				cred, err = newCredential(audiences)
				if err != nil {
					return nil, err
				}
			} else {
				tokenReq := &authnv1.TokenRequest{
					Spec: authnv1.TokenRequestSpec{
						Audiences: audiences,
					},
				}
				if err := o.Client.SubResource("token").Create(ctx, serviceAccount, tokenReq); err != nil {
					return nil, fmt.Errorf("failed to create kubernetes token for service account '%s/%s': %w",
						serviceAccount.Namespace, serviceAccount.Name, err)
				}
				cred = &JWT{
					Token:     tokenReq.Status.Token,
					ExpiresAt: tokenReq.Status.ExpirationTimestamp.Time,
				}
			}

			// Exchange the credential for a service provider access token.
			credential, err := provider.NewCredentialForMaterial(ctx, cred, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create provider access token for service account '%s/%s': %w",
					serviceAccount.Namespace, serviceAccount.Name, err)
			}

			return credential, nil
		}
	}

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newAccessToken()
	}

	// Build cache key.
	cacheKey := buildAccessTokenCacheKey(provider, audiences, providerIdentity, serviceAccount, opts...)

	// Build involved object details.
	kind := o.InvolvedObject.Kind
	name := o.InvolvedObject.Name
	namespace := o.InvolvedObject.Namespace
	operation := o.InvolvedObject.Operation

	// Get token from cache.
	credential, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Credential, error) {
		return newAccessToken()
	}, cache.WithInvolvedObject(kind, name, namespace, operation))
	if err != nil {
		return nil, err
	}

	return credential, nil
}

func getServiceAccountAndProviderInfo(ctx context.Context, provider ServiceProvider, client client.Client,
	key client.ObjectKey, opts ...Option) (*corev1.ServiceAccount, []string, string, error) {

	var o Options
	o.Apply(opts...)

	var setDefaultSA bool

	// Apply multi-tenancy lockdown: use default service account when .serviceAccountName
	// is not explicitly specified in the object. This results in Object-Level Workload Identity.
	if key.Name == "" && o.DefaultServiceAccount != "" {
		key.Name = o.DefaultServiceAccount
		setDefaultSA = true
	}

	// Get service account.
	var serviceAccount corev1.ServiceAccount
	if err := client.Get(ctx, key, &serviceAccount); err != nil {
		if errors.IsNotFound(err) && setDefaultSA {
			return nil, nil, "", fmt.Errorf("failed to get service account '%s': %w",
				key, ErrDefaultServiceAccountNotFound)
		}
		return nil, nil, "", fmt.Errorf("failed to get service account '%s': %w",
			key, err)
	}

	// Get provider audience.
	audiences := o.Audiences
	if len(audiences) == 0 && o.Credential != nil {
		audiences = o.Credential.Audiences
	}
	if len(audiences) == 0 {
		var err error
		audiences, err = provider.GetJWTAudiences(ctx, serviceAccount)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to get provider audience: %w", err)
		}
	}

	// Get provider identity.
	providerIdentity, err := provider.GetJWTIdentity(serviceAccount)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get provider identity from service account '%s/%s' annotations: %w",
			key.Namespace, key.Name, err)
	}

	return &serviceAccount, audiences, providerIdentity, nil
}

func buildAccessTokenCacheKey(provider ServiceProvider, audiences []string, providerIdentity string,
	serviceAccount *corev1.ServiceAccount, opts ...Option) string {

	var o Options
	o.Apply(opts...)

	var parts []string

	parts = append(parts, fmt.Sprintf("provider=%s", provider.GetName()))

	if o.CredentialProvider != nil {
		parts = append(parts, fmt.Sprintf("credentialProvider=%s", o.CredentialProvider.GetName()))
	}

	if o.Credential != nil {
		parts = append(parts, fmt.Sprintf("credentialType=%s", o.Credential.Type))
		if o.Credential.ExpirationSeconds != nil {
			parts = append(parts, fmt.Sprintf("credentialExpirationSeconds=%d", *o.Credential.ExpirationSeconds))
		}
	}

	if len(audiences) == 0 {
		audiences = o.Audiences
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
