/*
Copyright 2026 The Flux authors

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

package dockerhub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
)

const (
	// ProviderName is the name of the Docker Hub authentication provider.
	ProviderName = "dockerhub"

	defaultAudience = "https://identity.docker.com"
	stageAudience   = "https://identity-stage.docker.com"

	tokenExchangeGrant = "urn:ietf:params:oauth:grant-type:token-exchange"
	subjectTokenType   = "urn:ietf:params:oauth:token-type:id_token"

	defaultExpiration = time.Hour
)

// registryAudiences maps the supported Docker Hub registry hosts to the
// audience of the OIDC token exchanged for an access token.
var registryAudiences = map[string]string{
	"docker.io":                  defaultAudience,
	"index.docker.io":            defaultAudience,
	"registry-1.docker.io":       defaultAudience,
	"registry-1-stage.docker.io": stageAudience,
	"dhi.io":                     defaultAudience,
}

// Provider implements auth.ArtifactRegistryCredentialsProvider for Docker Hub
// OIDC connections. It exchanges an OIDC identity for a short-lived Docker Hub
// access token; it does not support controller, Git, or REST config credentials.
type Provider struct{ Implementation }

var _ auth.ArtifactRegistryCredentialsProvider = Provider{}

// GetName implements auth.Provider.
func (Provider) GetName() string {
	return ProviderName
}

// NewControllerToken implements auth.Provider. Docker Hub OIDC has no ambient
// controller identity: credentials must be obtained by exchanging a workload
// identity.
func (Provider) NewControllerToken(context.Context, ...auth.Option) (auth.Credential, error) {
	return nil, errors.New("dockerhub does not support controller credentials; a credential exchange is required")
}

// GetAudiences implements auth.Provider.
func (Provider) GetAudiences(_ context.Context, opts ...auth.Option) ([]string, error) {
	var o auth.Options
	o.Apply(opts...)
	if len(o.CredentialAudiences) == 0 {
		return []string{defaultAudience}, nil
	}
	if len(o.CredentialAudiences) > 1 {
		return nil, errors.New("dockerhub accepts a single audience")
	}
	switch o.CredentialAudiences[0] {
	case defaultAudience, stageAudience:
		return []string{o.CredentialAudiences[0]}, nil
	default:
		return nil, fmt.Errorf("invalid dockerhub audience '%s'", o.CredentialAudiences[0])
	}
}

// GetIdentity implements auth.Provider. The connection ID scopes cached
// credentials to the Docker Hub OIDC connection that issued them.
func (Provider) GetIdentity(opts ...auth.Option) (string, error) {
	var o auth.Options
	o.Apply(opts...)
	if strings.TrimSpace(o.DockerHubOIDCConnectionID) == "" {
		return "", errors.New("dockerhub oidc connection id is required")
	}
	return o.DockerHubOIDCConnectionID, nil
}

// NewAccessCredentialForIdentity implements auth.Provider. It exchanges the
// supplied JWT identity for a Docker Hub access token.
func (p Provider) NewAccessCredentialForIdentity(ctx context.Context, identity auth.Credential,
	opts ...auth.Option) (auth.Credential, error) {

	var o auth.Options
	o.Apply(opts...)

	jwtCredential, ok := identity.(*auth.JWTCredential)
	if !ok {
		return nil, fmt.Errorf("expected *auth.JWTCredential, got %T", identity)
	}
	connectionID := strings.TrimSpace(o.DockerHubOIDCConnectionID)
	if connectionID == "" {
		return nil, errors.New("dockerhub oidc connection id is required")
	}

	audience, err := audienceFromOptions(o.CredentialAudiences)
	if err != nil {
		return nil, err
	}
	expiration := o.CredentialExpiration
	if expiration <= 0 {
		expiration = defaultExpiration
	}

	form := url.Values{}
	form.Set("grant_type", tokenExchangeGrant)
	form.Set("subject_token_type", subjectTokenType)
	form.Set("subject_token", jwtCredential.Token)
	form.Set("connection_id", connectionID)
	form.Set("expires_in", strconv.FormatInt(int64(expiration/time.Second), 10))

	resp, err := p.impl().ExchangeToken(ctx, o.GetHTTPClient(), audience+"/oauth/token", form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read dockerhub token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apiError(resp.StatusCode, body)
	}

	token, err := parseTokenResponse(body)
	if err != nil {
		return nil, err
	}

	lifetime := expiration
	if token.ExpiresIn > 0 {
		lifetime = time.Duration(token.ExpiresIn) * time.Second
	}
	return &AccessToken{
		Token:     token.AccessToken,
		ExpiresAt: time.Now().Add(lifetime),
	}, nil
}

// GetAccessCredentialOptionsForArtifactRepository implements
// auth.ArtifactRegistryCredentialsProvider. It pins the OIDC audience to the
// identity endpoint matching the registry host.
func (Provider) GetAccessCredentialOptionsForArtifactRepository(artifactRepository string,
	_ ...auth.Option) ([]auth.Option, error) {

	audience, err := audienceForArtifactRepository(artifactRepository)
	if err != nil {
		return nil, err
	}
	return []auth.Option{auth.WithCredentialAudiences(audience)}, nil
}

// ParseArtifactRepository implements auth.ArtifactRegistryCredentialsProvider.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	registry, err := cryptov1.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}
	if _, ok := registryAudiences[registry]; !ok {
		return "", fmt.Errorf("invalid dockerhub registry '%s'", registry)
	}
	return registry, nil
}

// NewArtifactRegistryCredentials implements
// auth.ArtifactRegistryCredentialsProvider.
func (Provider) NewArtifactRegistryCredentials(_ context.Context, _ string,
	accessCredential auth.Credential, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	var o auth.Options
	o.Apply(opts...)

	credential, ok := accessCredential.(*AccessToken)
	if !ok {
		return nil, fmt.Errorf("expected *dockerhub.AccessToken, got %T", accessCredential)
	}
	if strings.TrimSpace(o.CredentialUsername) == "" {
		return nil, errors.New("credential username is required for dockerhub")
	}

	return &auth.ArtifactRegistryCredentials{
		Authenticator: authn.FromConfig(authn.AuthConfig{
			Username: o.CredentialUsername,
			Password: credential.Token,
		}),
		ExpiresAt: credential.ExpiresAt,
	}, nil
}

// audienceForArtifactRepository returns the Docker Hub identity endpoint for
// the artifact repository's registry host.
func audienceForArtifactRepository(artifactRepository string) (string, error) {
	registry, err := cryptov1.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}
	audience, ok := registryAudiences[registry]
	if !ok {
		return "", fmt.Errorf("invalid dockerhub registry '%s'", registry)
	}
	return audience, nil
}

// audienceFromOptions returns the single configured Docker Hub audience, or the
// default audience when none is set.
func audienceFromOptions(audiences []string) (string, error) {
	if len(audiences) == 0 {
		return defaultAudience, nil
	}
	if len(audiences) > 1 {
		return "", errors.New("dockerhub accepts a single audience")
	}
	switch audiences[0] {
	case defaultAudience, stageAudience:
		return audiences[0], nil
	default:
		return "", fmt.Errorf("invalid dockerhub audience '%s'", audiences[0])
	}
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
