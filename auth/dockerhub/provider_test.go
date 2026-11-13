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

package dockerhub_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/dockerhub"
)

func TestProvider_GetName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(dockerhub.Provider{}.GetName()).To(Equal(dockerhub.ProviderName))
	g.Expect(dockerhub.ProviderName).To(Equal("dockerhub"))
}

func TestProvider_NewControllerToken(t *testing.T) {
	g := NewWithT(t)
	credential, err := dockerhub.Provider{}.NewControllerToken(context.Background())
	g.Expect(err).To(MatchError(ContainSubstring("does not support controller credentials")))
	g.Expect(credential).To(BeNil())
}

func TestProvider_GetAudiences(t *testing.T) {
	for _, tt := range []struct {
		name      string
		audiences []string
		want      []string
		wantErr   string
	}{
		{
			name: "default",
			want: []string{defaultAudience},
		},
		{
			name:      "default audience",
			audiences: []string{defaultAudience},
			want:      []string{defaultAudience},
		},
		{
			name:      "stage audience",
			audiences: []string{stageAudience},
			want:      []string{stageAudience},
		},
		{
			name:      "multiple audiences",
			audiences: []string{defaultAudience, stageAudience},
			wantErr:   "accepts a single audience",
		},
		{
			name:      "invalid audience",
			audiences: []string{"https://example.com"},
			wantErr:   "invalid dockerhub audience",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			audiences, err := dockerhub.Provider{}.GetAudiences(context.Background(),
				auth.WithCredentialAudiences(tt.audiences...))
			if tt.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(audiences).To(Equal(tt.want))
		})
	}
}

func TestProvider_GetIdentity(t *testing.T) {
	g := NewWithT(t)
	identity, err := dockerhub.Provider{}.GetIdentity(
		auth.WithDockerHubOIDCConnectionID("00000000-0000-0000-0000-000000000000"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(identity).To(Equal("00000000-0000-0000-0000-000000000000"))

	_, err = dockerhub.Provider{}.GetIdentity()
	g.Expect(err).To(MatchError(ContainSubstring("connection id is required")))
}

func TestProvider_ParseArtifactRepository(t *testing.T) {
	for _, tt := range []struct {
		name    string
		repo    string
		want    string
		wantErr string
	}{
		{name: "docker.io", repo: "docker.io/library/nginx", want: "index.docker.io"},
		{name: "index.docker.io", repo: "index.docker.io/library/nginx", want: "index.docker.io"},
		{name: "registry-1", repo: "registry-1.docker.io/library/nginx", want: "registry-1.docker.io"},
		{name: "staging", repo: "registry-1-stage.docker.io/library/nginx", want: "registry-1-stage.docker.io"},
		{name: "dhi", repo: "dhi.io/library/nginx", want: "dhi.io"},
		{name: "unsupported", repo: "ghcr.io/fluxcd/source-controller", wantErr: "invalid dockerhub registry"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, err := dockerhub.Provider{}.ParseArtifactRepository(tt.repo)
			if tt.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestProvider_GetAccessCredentialOptionsForArtifactRepository(t *testing.T) {
	for _, tt := range []struct {
		name    string
		repo    string
		want    string
		wantErr string
	}{
		{name: "docker.io", repo: "docker.io/library/nginx", want: defaultAudience},
		{name: "dhi", repo: "dhi.io/library/nginx", want: defaultAudience},
		{name: "staging", repo: "registry-1-stage.docker.io/library/nginx", want: stageAudience},
		{name: "unsupported", repo: "ghcr.io/fluxcd/source-controller", wantErr: "invalid dockerhub registry"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			opts, err := dockerhub.Provider{}.GetAccessCredentialOptionsForArtifactRepository(tt.repo)
			if tt.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			var o auth.Options
			o.Apply(opts...)
			g.Expect(o.CredentialAudiences).To(Equal([]string{tt.want}))
		})
	}
}

func TestProvider_NewAccessCredentialForIdentity(t *testing.T) {
	connectionID := "00000000-0000-0000-0000-000000000000"
	identity := &auth.JWTCredential{Token: "github-oidc-token"}

	t.Run("exchanges identity", func(t *testing.T) {
		g := NewWithT(t)
		impl := &mockImplementation{resp: response(http.StatusOK,
			`{"access_token":"dockerhub-token","expires_in":120}`)}
		provider := dockerhub.Provider{Implementation: impl}

		credential, err := provider.NewAccessCredentialForIdentity(context.Background(), identity,
			auth.WithDockerHubOIDCConnectionID(connectionID))
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(impl.endpoint).To(Equal(defaultAudience + "/oauth/token"))
		g.Expect(impl.form.Get("grant_type")).To(Equal("urn:ietf:params:oauth:grant-type:token-exchange"))
		g.Expect(impl.form.Get("subject_token_type")).To(Equal("urn:ietf:params:oauth:token-type:id_token"))
		g.Expect(impl.form.Get("subject_token")).To(Equal("github-oidc-token"))
		g.Expect(impl.form.Get("connection_id")).To(Equal(connectionID))
		g.Expect(impl.form.Get("expires_in")).To(Equal("3600"))

		token := credential.(*dockerhub.AccessToken)
		g.Expect(token.Token).To(Equal("dockerhub-token"))
		g.Expect(token.GetDuration()).To(BeNumerically("~", 120*time.Second, 5*time.Second))
	})

	t.Run("staging audience", func(t *testing.T) {
		g := NewWithT(t)
		impl := &mockImplementation{resp: response(http.StatusOK, `{"access_token":"dockerhub-token"}`)}
		provider := dockerhub.Provider{Implementation: impl}

		_, err := provider.NewAccessCredentialForIdentity(context.Background(), identity,
			auth.WithDockerHubOIDCConnectionID(connectionID),
			auth.WithCredentialAudiences(stageAudience))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(impl.endpoint).To(Equal(stageAudience + "/oauth/token"))
	})

	t.Run("custom expiration", func(t *testing.T) {
		g := NewWithT(t)
		impl := &mockImplementation{resp: response(http.StatusOK, `{"access_token":"dockerhub-token"}`)}
		provider := dockerhub.Provider{Implementation: impl}

		credential, err := provider.NewAccessCredentialForIdentity(context.Background(), identity,
			auth.WithDockerHubOIDCConnectionID(connectionID),
			auth.WithCredentialExpiration(2*time.Hour))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(impl.form.Get("expires_in")).To(Equal("7200"))
		g.Expect(credential.GetDuration()).To(BeNumerically("~", 2*time.Hour, 5*time.Second))
	})

	t.Run("missing connection id", func(t *testing.T) {
		g := NewWithT(t)
		_, err := dockerhub.Provider{}.NewAccessCredentialForIdentity(context.Background(), identity)
		g.Expect(err).To(MatchError(ContainSubstring("connection id is required")))
	})

	t.Run("invalid identity type", func(t *testing.T) {
		g := NewWithT(t)
		_, err := dockerhub.Provider{}.NewAccessCredentialForIdentity(context.Background(),
			&auth.RESTConfig{}, auth.WithDockerHubOIDCConnectionID(connectionID))
		g.Expect(err).To(MatchError(ContainSubstring("expected *auth.JWTCredential")))
	})

	t.Run("exchange error", func(t *testing.T) {
		g := NewWithT(t)
		impl := &mockImplementation{err: errors.New("boom")}
		_, err := (dockerhub.Provider{Implementation: impl}).NewAccessCredentialForIdentity(
			context.Background(), identity, auth.WithDockerHubOIDCConnectionID(connectionID))
		g.Expect(err).To(MatchError(ContainSubstring("boom")))
	})

	t.Run("api error", func(t *testing.T) {
		g := NewWithT(t)
		impl := &mockImplementation{resp: response(http.StatusUnauthorized, `{"error":"denied"}`)}
		_, err := (dockerhub.Provider{Implementation: impl}).NewAccessCredentialForIdentity(
			context.Background(), identity, auth.WithDockerHubOIDCConnectionID(connectionID))
		g.Expect(err).To(MatchError(ContainSubstring("bad status code 401")))
	})

	t.Run("missing access token", func(t *testing.T) {
		g := NewWithT(t)
		impl := &mockImplementation{resp: response(http.StatusOK, `{}`)}
		_, err := (dockerhub.Provider{Implementation: impl}).NewAccessCredentialForIdentity(
			context.Background(), identity, auth.WithDockerHubOIDCConnectionID(connectionID))
		g.Expect(err).To(MatchError(ContainSubstring("did not contain an access token")))
	})
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	credential := &dockerhub.AccessToken{Token: "dockerhub-token", ExpiresAt: expiresAt}

	t.Run("username and password", func(t *testing.T) {
		g := NewWithT(t)
		creds, err := dockerhub.Provider{}.NewArtifactRegistryCredentials(
			context.Background(), "index.docker.io", credential, auth.WithCredentialUsername("my-org"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(creds).To(Equal(&auth.ArtifactRegistryCredentials{
			Authenticator: authn.FromConfig(authn.AuthConfig{
				Username: "my-org",
				Password: "dockerhub-token",
			}),
			ExpiresAt: expiresAt,
		}))
	})

	t.Run("missing username", func(t *testing.T) {
		g := NewWithT(t)
		_, err := dockerhub.Provider{}.NewArtifactRegistryCredentials(
			context.Background(), "index.docker.io", credential)
		g.Expect(err).To(MatchError(ContainSubstring("credential username is required")))
	})

	t.Run("invalid credential type", func(t *testing.T) {
		g := NewWithT(t)
		_, err := dockerhub.Provider{}.NewArtifactRegistryCredentials(
			context.Background(), "index.docker.io", &auth.JWTCredential{},
			auth.WithCredentialUsername("my-org"))
		g.Expect(err).To(MatchError(ContainSubstring("expected *dockerhub.AccessToken")))
	})
}
