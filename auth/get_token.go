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
	"crypto/sha256"
	"fmt"
	"strings"

	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/cache"
)

// GetToken returns an access token for accessing resources in the given cloud provider.
func GetToken(ctx context.Context, provider Provider, opts ...Option) (Token, error) {

	var o Options
	o.Apply(opts...)

	// Initialize access token fetcher for controller.
	newAccessToken := func() (Token, error) {
		token, err := provider.NewControllerToken(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider access token for the controlller: %w", err)
		}
		return token, nil
	}

	// Update access token fetcher for a service account if specified.
	var providerIdentity string
	var serviceAccountP *corev1.ServiceAccount
	if o.ServiceAccount != nil {
		// Check the feature gate for object-level workload identity.
		if !IsObjectLevelWorkloadIdentityEnabled() {
			return nil, ErrObjectLevelWorkloadIdentityNotEnabled
		}

		// Get service account and prepare a function to create a token for it.
		var serviceAccount corev1.ServiceAccount
		if err := o.Client.Get(ctx, *o.ServiceAccount, &serviceAccount); err != nil {
			return nil, fmt.Errorf("failed to get service account '%s/%s': %w",
				o.ServiceAccount.Namespace, o.ServiceAccount.Name, err)
		}
		serviceAccountP = &serviceAccount

		// Get provider audience.
		var err error
		providerAudience, err := provider.GetAudience(ctx, serviceAccount)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider audience: %w", err)
		}

		// Get provider identity.
		providerIdentity, err = provider.GetIdentity(serviceAccount)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider identity from service account '%s/%s' annotations: %w",
				serviceAccount.Namespace, serviceAccount.Name, err)
		}

		// Update access token fetcher.
		newAccessToken = func() (Token, error) {
			identityToken, err := newServiceAccountToken(ctx, o.Client, serviceAccount, providerAudience)
			if err != nil {
				return nil, fmt.Errorf("failed to create kubernetes token for service account '%s/%s': %w",
					serviceAccount.Namespace, serviceAccount.Name, err)
			}

			token, err := provider.NewTokenForServiceAccount(ctx, identityToken, serviceAccount, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create provider access token for service account '%s/%s': %w",
					serviceAccount.Namespace, serviceAccount.Name, err)
			}

			return token, nil
		}
	}

	// Initialize token fetcher with access token fetcher.
	newToken := newAccessToken

	// Update token fetcher to registry token fetcher if artifact repository is specified.
	var artifactRepositoryCacheKey string
	if o.ArtifactRepository != "" {
		// Parse artifact repository.
		registryInput, err := provider.ParseArtifactRepository(o.ArtifactRepository)
		if err != nil {
			return nil, fmt.Errorf("failed to parse artifact repository '%s': %w",
				o.ArtifactRepository, err)
		}

		// Set artifact repository cache key.
		artifactRepositoryCacheKey = registryInput

		// Update token fetcher.
		newToken = func() (Token, error) {
			accessToken, err := newAccessToken()
			if err != nil {
				return nil, err
			}

			token, err := provider.NewArtifactRegistryCredentials(ctx, registryInput, accessToken, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create artifact registry credentials: %w", err)
			}

			return token, nil
		}
	}

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newToken()
	}

	// Build cache key.
	cacheKey := buildCacheKey(provider, providerIdentity, artifactRepositoryCacheKey, serviceAccountP, opts...)

	// Get involved object details.
	kind := o.InvolvedObject.Kind
	name := o.InvolvedObject.Name
	namespace := o.InvolvedObject.Namespace
	operation := o.InvolvedObject.Operation

	// Get token from cache.
	token, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newToken()
	}, cache.WithInvolvedObject(kind, name, namespace, operation))
	if err != nil {
		return nil, err
	}

	return token, nil
}

func newServiceAccountToken(ctx context.Context, client client.Client,
	serviceAccount corev1.ServiceAccount, providerAudience string) (string, error) {
	tokenReq := &authnv1.TokenRequest{
		Spec: authnv1.TokenRequestSpec{
			Audiences: []string{providerAudience},
		},
	}
	if err := client.SubResource("token").Create(ctx, &serviceAccount, tokenReq); err != nil {
		return "", err
	}
	return tokenReq.Status.Token, nil
}

func buildCacheKey(provider Provider, providerIdentity, artifactRepositoryKey string,
	serviceAccount *corev1.ServiceAccount, opts ...Option) string {

	var o Options
	o.Apply(opts...)

	var keyParts []string

	keyParts = append(keyParts, fmt.Sprintf("provider=%s", provider.GetName()))

	if serviceAccount != nil {
		keyParts = append(keyParts, fmt.Sprintf("providerIdentity=%s", providerIdentity))
		keyParts = append(keyParts, fmt.Sprintf("serviceAccountName=%s", serviceAccount.Name))
		keyParts = append(keyParts, fmt.Sprintf("serviceAccountNamespace=%s", serviceAccount.Namespace))
	}

	if len(o.Scopes) > 0 {
		keyParts = append(keyParts, fmt.Sprintf("scopes=%s", strings.Join(o.Scopes, ",")))
	}

	if o.ArtifactRepository != "" {
		keyParts = append(keyParts, fmt.Sprintf("artifactRepositoryKey=%s", artifactRepositoryKey))
	}

	if o.STSRegion != "" {
		keyParts = append(keyParts, fmt.Sprintf("stsRegion=%s", o.STSRegion))
	}

	if o.STSEndpoint != "" {
		keyParts = append(keyParts, fmt.Sprintf("stsEndpoint=%s", o.STSEndpoint))
	}

	if o.ProxyURL != nil {
		keyParts = append(keyParts, fmt.Sprintf("proxyURL=%s", o.ProxyURL.String()))
	}

	s := strings.Join(keyParts, ",")
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
}
