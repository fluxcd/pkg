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

package auth_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/kubernetes"
	"github.com/fluxcd/pkg/cache"
)

func TestGetArtifactRegistryCredentials(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	kubeClient, oidcClient := newTestEnv(t, ctx)

	// Create a default service account.
	defaultServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}
	err := kubeClient.Create(ctx, defaultServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	now := time.Now()

	for _, tt := range []struct {
		name               string
		provider           *mockProvider
		artifactRepository string
		opts               []auth.Option
		expectedCreds      *auth.ArtifactRegistryCredentials
		expectedErr        string
	}{
		{
			name: "registry credential from controller access credential",
			provider: &mockProvider{
				returnRegistryInput:        "some-registry.io/some/artifact",
				returnControllerCredential: &mockCredential{token: "mock-default-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAccessCredential:   &mockCredential{token: "mock-default-token"},
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "registry credential from access credential from service account",
			provider: &mockProvider{
				returnName:             "mock-provider",
				returnRegistryInput:    "some-registry.io/some/artifact",
				returnAccessCredential: &mockCredential{token: "mock-access-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAudiences:          []string{"audience1", "audience2"},
				paramServiceAccount:     &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient:    oidcClient,
				paramArtifactRepository: "some-registry.io/some/artifact",
				paramAccessCredential:   &mockCredential{token: "mock-access-token"},
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "registry credential from access credential from explicit service account",
			provider: &mockProvider{
				returnName:             "mock-provider",
				returnRegistryInput:    "some-registry.io/some/artifact",
				returnAccessCredential: &mockCredential{token: "mock-access-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAudiences:          []string{"audience1", "audience2"},
				paramServiceAccount:     &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient:    oidcClient,
				paramArtifactRepository: "some-registry.io/some/artifact",
				paramAccessCredential:   &mockCredential{token: "mock-access-token"},
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "all the options are taken into account in the cache key",
			provider: &mockProvider{
				returnName:              "mock-provider",
				returnIdentity:          "mock-identity",
				returnRegistryInput:     "artifact-cache-key",
				paramServiceAccount:     &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(2)
					g.Expect(err).NotTo(HaveOccurred())

					const accessCredentialKey = "de4d54f024651d5ead7cd72d94e704d205a66e1c24ede96a2999c7e0ce9f2485"
					var credential auth.Credential = &mockCredential{token: "cached-token"}
					cachedCredential, ok, err := tokenCache.GetOrSet(ctx, accessCredentialKey, func(ctx context.Context) (cache.Token, error) {
						return credential, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedCredential).To(Equal(credential))

					const artifactRegistryCredentialsKey = "817fc36ca56514ef10c40796327e77922036090a9e2d51cc55fb9e1c2c70f31f"
					credential = &auth.ArtifactRegistryCredentials{
						Authenticator: authn.FromConfig(authn.AuthConfig{Username: "cached-registry-token"}),
						ExpiresAt:     now.Add(time.Hour),
					}
					cachedCredential, ok, err = tokenCache.GetOrSet(ctx, artifactRegistryCredentialsKey, func(ctx context.Context) (cache.Token, error) {
						return credential, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedCredential).To(Equal(credential))

					o.Cache = tokenCache
				},
			},
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "cached-registry-token"}),
				ExpiresAt:     now.Add(time.Hour),
			},
		},
		{
			name: "error parsing artifact repository",
			provider: &mockProvider{
				paramArtifactRepository: "some-registry.io/some/artifact",
				returnRegistryErr:       "mock error",
			},
			artifactRepository: "some-registry.io/some/artifact",
			expectedErr:        "mock error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.provider.t = t

			creds, err := auth.GetArtifactRegistryCredentials(ctx, tt.provider, tt.artifactRepository, tt.opts...)

			if tt.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
				g.Expect(creds).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(creds).To(Equal(tt.expectedCreds))
			}
		})
	}
}

// TestGetArtifactRegistryCredentials_CredentialUsernameCacheKey is a regression
// test for a cache collision: the generic provider authenticates with HTTP Basic
// when a credential username is set and with a bearer token otherwise, so a
// changed username must never return registry credentials cached for another.
func TestGetArtifactRegistryCredentials_CredentialUsernameCacheKey(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Four entries are needed: an access credential and registry credentials
	// for both the username and the no-username cases.
	tokenCache, err := cache.NewTokenCache(10)
	g.Expect(err).NotTo(HaveOccurred())

	provider := &mockProvider{
		t:                          t,
		returnName:                 "mock-provider",
		returnRegistryInput:        "some-registry.io/some/artifact",
		returnControllerCredential: &mockCredential{token: "mock-default-token"},
		paramAccessCredential:      &mockCredential{token: "mock-default-token"},
		paramArtifactRepository:    "some-registry.io/some/artifact",
	}

	// The non-username options exist only to satisfy the mock provider's checks.
	opts := func(extra ...auth.Option) []auth.Option {
		return append([]auth.Option{
			auth.WithCredentialAudiences("audience1", "audience2"),
			auth.WithScopes("scope1", "scope2"),
			auth.WithAWSRegion("us-east-1"),
			auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
			auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
			auth.WithCAData("ca-data"),
			auth.WithCache(*tokenCache),
		}, extra...)
	}
	basic := &auth.ArtifactRegistryCredentials{
		Authenticator: authn.FromConfig(authn.AuthConfig{Username: "some-user", Password: "token"}),
		ExpiresAt:     time.Now().Add(time.Hour),
	}
	bearer := &auth.ArtifactRegistryCredentials{
		Authenticator: &authn.Bearer{Token: "token"},
		ExpiresAt:     time.Now().Add(time.Hour),
	}

	// Mint and cache credentials for a username.
	provider.returnRegistryToken = basic
	got, err := auth.GetArtifactRegistryCredentials(ctx, provider, "some-registry.io/some/artifact",
		opts(auth.WithCredentialUsername("some-user"))...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(basic))

	// Removing the username must NOT reuse the credentials cached for it.
	provider.returnRegistryToken = bearer
	got, err = auth.GetArtifactRegistryCredentials(ctx, provider, "some-registry.io/some/artifact",
		opts()...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(bearer))

	// The username still returns its own cached credentials.
	provider.returnRegistryToken = nil
	got, err = auth.GetArtifactRegistryCredentials(ctx, provider, "some-registry.io/some/artifact",
		opts(auth.WithCredentialUsername("some-user"))...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(basic))
}
