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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth/actionsoidc"
	"github.com/fluxcd/pkg/auth/kubernetes"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/spiffe/brokerapi"
)

// NewAccessCredential returns an access credential for accessing resources in
// the given cloud provider.
func NewAccessCredential(ctx context.Context, provider Provider, opts ...Option) (Credential, error) {

	var o Options
	o.Apply(opts...)

	// Initialize access credential fetcher for the controller.
	newAccessCredential := func() (Credential, error) {
		credential, err := provider.NewControllerToken(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider access credential for the controller: %w", err)
		}
		return credential, nil
	}

	// Update access credential fetcher for a ServiceAccount if specified.
	var serviceAccount *kubernetes.ServiceAccountReference
	var providerIdentity string
	var audiences []string
	if o.performCredentialExchange {
		var err error
		serviceAccount, audiences, providerIdentity, err =
			getServiceAccountAndProviderInfo(ctx, provider, opts...)
		if err != nil {
			return nil, err
		}

		// Update the function to create an access credential using the service account.
		newAccessCredential = func() (Credential, error) {
			identity, err := o.newIdentityCredential(ctx, audiences)
			if err != nil {
				return nil, err
			}

			// Exchange the identity credential for a provider access credential.
			credential, err := provider.NewAccessCredentialForIdentity(ctx, identity, opts...)
			if err != nil {
				if serviceAccount != nil {
					return nil, fmt.Errorf("failed to create provider access credential for service account '%s': %w",
						serviceAccount, err)
				}
				return nil, fmt.Errorf("failed to create provider access credential: %w", err)
			}

			return credential, nil
		}
	}

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newAccessCredential()
	}

	// Build cache key.
	cacheKey := buildAccessCredentialCacheKey(provider, audiences, providerIdentity, serviceAccount, opts...)

	// Get credential from cache.
	credential, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newAccessCredential()
	}, cache.WithInvolvedObject(o.involvedObject()))
	if err != nil {
		return nil, err
	}

	return credential, nil
}

// newIdentityCredential returns the identity credential used to obtain a
// provider access credential for the object. Kubernetes issues a ServiceAccount
// token, SPIFFE fetches a JWT-SVID either from the SPIFFE Broker API or from
// the SPIFFE Workload API, and Actions OIDC fetches an OIDC ID token from the
// Github/Forgejo Actions token endpoint.
func (o *Options) newIdentityCredential(ctx context.Context, audiences []string) (Credential, error) {
	credProvider := o.credentialProvider()
	if credProvider == cryptov1.CredentialProviderSPIFFE && o.credentialType() != cryptov1.CredentialTypeJWT {
		return nil, fmt.Errorf("credential type '%s' for the %s credential provider is not implemented yet",
			o.credentialType(), credProvider)
	}
	switch {
	case credProvider == cryptov1.CredentialProviderKubernetes:
		sa := o.ServiceAccountRef()
		token, err := kubernetes.CreateJWT(ctx, o.Client, sa, audiences, o.CredentialExpiration)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes token for service account '%s': %w", sa, err)
		}
		return newJWTCredential(cryptov1.CredentialProviderKubernetes, token)

	case credProvider == cryptov1.CredentialProviderSPIFFE && o.SPIFFEBrokerAPI != nil:
		return o.spiffeBrokerAPICredential(ctx, audiences)

	case credProvider == cryptov1.CredentialProviderSPIFFE:
		return o.spiffeWorkloadAPICredential(ctx, audiences)

	case credProvider == cryptov1.CredentialProviderActionsOIDC:
		token, err := actionsoidc.FetchToken(ctx, audiences)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Actions OIDC ID token: %w", err)
		}
		return newJWTCredential(cryptov1.CredentialProviderActionsOIDC, token)

	default:
		return nil, fmt.Errorf("credential provider '%s' is not supported", credProvider)
	}
}

// spiffeBrokerAPICredential fetches a JWT-SVID for the entitled object from the
// SPIFFE Broker API.
func (o *Options) spiffeBrokerAPICredential(ctx context.Context, audiences []string) (Credential, error) {
	if o.ObjectReference.Kind == "" {
		return nil, errors.New("object reference is required for the SPIFFE credential provider with the SPIFFE Broker API")
	}

	svid, err := o.SPIFFEBrokerAPI.FetchJWTSVID(ctx, brokerapi.KubernetesObjectReference{
		Group:     o.ObjectReference.APIGroup,
		Kind:      o.ObjectReference.Kind,
		Namespace: o.ObjectReference.Namespace,
		Name:      o.ObjectReference.Name,
		UID:       o.ObjectReference.UID,
	}, audiences)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SPIFFE JWT-SVID for object '%s/%s %s/%s': %w",
			o.ObjectReference.APIGroup, o.ObjectReference.Kind,
			o.ObjectReference.Namespace, o.ObjectReference.Name, err)
	}
	return newJWTCredential(cryptov1.CredentialProviderSPIFFE, svid.Marshal())
}

// spiffeWorkloadAPICredential fetches a JWT-SVID for the current workload from
// the SPIFFE Workload API.
func (o *Options) spiffeWorkloadAPICredential(ctx context.Context, audiences []string) (Credential, error) {
	var params jwtsvid.Params
	if len(audiences) > 0 {
		params.Audience = audiences[0]
		params.ExtraAudiences = audiences[1:]
	}
	svid, err := workloadapi.FetchJWTSVID(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SPIFFE JWT-SVID from the Workload API: %w", err)
	}
	return newJWTCredential(cryptov1.CredentialProviderSPIFFE, svid.Marshal())
}

// newJWTCredential returns a JWTCredential for the given provider and
// serialized JWT, extracting the expiration time from the token.
func newJWTCredential(provider, token string) (*JWTCredential, error) {
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}
	expiration, err := parsed.Claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get JWT expiration: %w", err)
	}
	var expiresAt time.Time
	if expiration != nil {
		expiresAt = expiration.Time
	}
	return &JWTCredential{
		Provider:  provider,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func getServiceAccountAndProviderInfo(ctx context.Context, provider Provider,
	opts ...Option) (*kubernetes.ServiceAccountReference, []string, string, error) {
	var o Options
	o.Apply(opts...)

	sa := o.ServiceAccountRef()

	// Get provider audience. The provider decides whether any configured
	// credential audiences are used.
	audiences, err := provider.GetAudiences(ctx, opts...)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get provider audience: %w", err)
	}

	// Get provider identity.
	providerIdentity, err := provider.GetIdentity(opts...)
	if err != nil {
		if sa != nil {
			return nil, nil, "", fmt.Errorf("failed to get provider identity from service account '%s': %w",
				sa, err)
		}
		return nil, nil, "", fmt.Errorf("failed to get provider identity: %w", err)
	}

	return sa, audiences, providerIdentity, nil
}

func buildAccessCredentialCacheKey(provider Provider, audiences []string, providerIdentity string,
	serviceAccount *kubernetes.ServiceAccountReference, opts ...Option) string {

	var o Options
	o.Apply(opts...)

	var parts []string

	parts = append(parts, fmt.Sprintf("provider=%s", provider.GetName()))

	if o.CredentialProvider != "" {
		parts = append(parts, fmt.Sprintf("credentialProvider=%s", o.CredentialProvider))
		parts = append(parts, fmt.Sprintf("credentialType=%s", o.credentialType()))
	}

	if len(audiences) == 0 {
		audiences = o.CredentialAudiences
	}
	if len(audiences) > 0 {
		parts = append(parts, fmt.Sprintf("audiences=%s", strings.Join(audiences, ",")))
	}

	if o.performCredentialExchange {
		parts = append(parts, fmt.Sprintf("providerIdentity=%s", providerIdentity))
		credProvider := o.credentialProvider()
		switch {
		case credProvider == cryptov1.CredentialProviderKubernetes:
			parts = append(parts,
				fmt.Sprintf("serviceAccountName=%s", serviceAccount.Name),
				fmt.Sprintf("serviceAccountNamespace=%s", serviceAccount.Namespace),
			)
		case credProvider == cryptov1.CredentialProviderSPIFFE && o.SPIFFEBrokerAPI != nil:
			// Even if the object shares its SPIFFE ID with others in pratice,
			// Flux has to assume it doesn't, so the full object reference must
			// be included in the cache key for SPIFFE. This means different
			// objects will never share cached credentials when using SPIFFE
			// (but the same object may still reuse its cached credentials).
			parts = append(parts,
				fmt.Sprintf("objectGroup=%s", o.ObjectReference.APIGroup),
				fmt.Sprintf("objectKind=%s", o.ObjectReference.Kind),
				fmt.Sprintf("objectNamespace=%s", o.ObjectReference.Namespace),
				fmt.Sprintf("objectName=%s", o.ObjectReference.Name),
				fmt.Sprintf("objectUID=%s", o.ObjectReference.UID),
			)
		case credProvider == cryptov1.CredentialProviderSPIFFE:
			// The Workload API issues the JWT-SVID for the current workload,
			// not for an object, so there is nothing to add to the cache key.
		case credProvider == cryptov1.CredentialProviderActionsOIDC:
			// Actions OIDC does not allow choosing a credential identity like
			// Kubernetes TokenRequest API and SPIFFE Broker API, so there's
			// nothing to add to the cache key.
		}
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
