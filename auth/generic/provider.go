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

package generic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the generic authentication provider.
const ProviderName = "generic"

// usernameAnnotation is the annotation key used to indicate that a JWT
// credential should be used as the password in a username/password pair.
const usernameAnnotation = "auth.fluxcd.io/username"

// Provider implements the auth.Provider interface for generic authentication.
type Provider struct{ Implementation }

// GetName implements auth.RESTConfigProvider.
func (p Provider) GetName() string {
	return ProviderName
}

// NewAmbientCredential implements auth.RESTConfigProvider.
func (p Provider) NewAmbientCredential(ctx context.Context, opts ...auth.Option) (auth.Credential, error) {

	var o auth.Options
	o.Apply(opts...)

	if o.Client == nil {
		return nil, errors.New("client is required to create a controller token")
	}

	// Like all providers, this one should fetch controller-level credentials
	// from the environment. In this case, this means opening the well-known
	// Kubernetes service account token file and parsing it to figure out
	// the controller's identity.
	const tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	b, err := p.impl().ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account token file %s: %w", tokenFile, err)
	}

	// Get controller service account from token subject.
	tok, _, err := jwt.NewParser().ParseUnverified(string(b), jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account token: %w", err)
	}
	sub, err := tok.Claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("failed to get subject from service account token: %w", err)
	}
	parts := strings.Split(sub, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid subject format in service account token: %s", sub)
	}
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      parts[3],
			Namespace: parts[2],
		},
	}

	// Create token.
	tokenReq := &authnv1.TokenRequest{
		Spec: authnv1.TokenRequestSpec{
			Audiences: o.Audiences,
		},
	}
	if err := o.Client.SubResource("token").Create(ctx, &serviceAccount, tokenReq); err != nil {
		return nil, fmt.Errorf("failed to create kubernetes token for controller service account '%s': %w",
			client.ObjectKeyFromObject(&serviceAccount), err)
	}
	rawToken := tokenReq.Status.Token

	exp, err := getExpirationFromJWT(rawToken)
	if err != nil {
		return nil, err
	}

	return &Credential{
		Token:     rawToken,
		ExpiresAt: *exp,
	}, nil
}

// GetJWTAudiences implements auth.RESTConfigProvider.
func (Provider) GetJWTAudiences(context.Context, corev1.ServiceAccount) ([]string, error) {
	// Use TokenRequest default audiences.
	return nil, nil
}

// GetJWTIdentity implements auth.RESTConfigProvider.
func (Provider) GetJWTIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	return fmt.Sprintf("system:serviceaccount:%s:%s", serviceAccount.Namespace, serviceAccount.Name), nil
}

// NewCredentialForMaterial implements auth.RESTConfigProvider.
func (Provider) NewCredentialForMaterial(_ context.Context, credential auth.CredentialMaterial,
	_ ...auth.Option) (auth.Credential, error) {

	jwt, ok := credential.(*auth.JWT)
	if !ok {
		return nil, fmt.Errorf("unsupported credential type %T", credential)
	}

	return &Credential{
		Token:     jwt.Token,
		ExpiresAt: jwt.ExpiresAt,
	}, nil
}

// GetAccessTokenOptionsForArtifactRepository implements auth.ArtifactRegistryCredentialsProvider.
func (Provider) GetAccessTokenOptionsForArtifactRepository(string) ([]auth.Option, error) {
	// No special options are needed to get an access token for an artifact registry.
	return nil, nil
}

// ParseArtifactRepository implements auth.ArtifactRegistryCredentialsProvider.
func (p Provider) ParseArtifactRepository(string) (string, error) {
	// The artifact repository is irrelevant for issuing the credential,
	// just return the provider name for inclusion in the cache key.
	return p.GetName(), nil
}

// NewArtifactRegistryCredentials implements auth.ArtifactRegistryCredentialsProvider.
func (Provider) NewArtifactRegistryCredentials(_ context.Context, _ string,
	accessToken auth.Credential, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	credential, ok := accessToken.(*Credential)
	if !ok {
		return nil, fmt.Errorf("unsupported access token type %T", accessToken)
	}

	var o auth.Options
	o.Apply(opts...)

	// Present the credential as the password of a username/password pair when
	// a username is configured, otherwise as a bearer token.
	if o.Credential != nil {
		if username := o.Credential.Annotations[usernameAnnotation]; username != "" {
			return &auth.ArtifactRegistryCredentials{
				Authenticator: &authn.Basic{
					Username: username,
					Password: credential.Token,
				},
				ExpiresAt: credential.ExpiresAt,
			}, nil
		}
	}
	return &auth.ArtifactRegistryCredentials{
		Authenticator: &authn.Bearer{Token: credential.Token},
		ExpiresAt:     credential.ExpiresAt,
	}, nil
}

// GetAccessTokenOptionsForCluster implements auth.RESTConfigProvider.
func (Provider) GetAccessTokenOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {

	var o auth.Options
	o.Apply(opts...)

	audiences := o.Audiences
	if len(audiences) == 0 {
		// Use cluster address as the default audience.
		audiences = []string{o.ClusterAddress}
	}

	return [][]auth.Option{{auth.WithAudiences(audiences...)}}, nil
}

// NewRESTConfig implements auth.RESTConfigProvider.
func (Provider) NewRESTConfig(ctx context.Context, accessTokens []auth.Credential,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	credential := accessTokens[0].(*Credential)

	var o auth.Options
	o.Apply(opts...)

	// Parse the cluster address.
	host := o.ClusterAddress
	if host == "" {
		return nil, errors.New("cluster address is required to create a REST config")
	}
	var err error
	host, err = auth.ParseClusterAddress(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster address %s: %w", o.ClusterAddress, err)
	}

	// Get CA if provided.
	var caData []byte
	if o.CAData != "" {
		caData = []byte(o.CAData)
	}

	return &auth.RESTConfig{
		Host:        host,
		CAData:      caData,
		BearerToken: credential.Token,
		ExpiresAt:   credential.ExpiresAt,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}

func getExpirationFromJWT(rawToken string) (*time.Time, error) {
	tok, _, err := jwt.NewParser().ParseUnverified(rawToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account token: %w", err)
	}
	exp, err := tok.Claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration time from service account token: %w", err)
	}
	return &exp.Time, nil
}
