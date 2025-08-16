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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
)

func TestGetRegistryFromArtifactRepository(t *testing.T) {
	for _, tt := range []struct {
		name               string
		artifactRepository string
		expectedRegistry   string
	}{
		{
			name:               "dot-less host with port",
			artifactRepository: "localhost:5000",
			expectedRegistry:   "localhost:5000",
		},
		{
			name:               "dot-less host without port",
			artifactRepository: "localhost",
			expectedRegistry:   "localhost",
		},
		{
			name:               "host with port",
			artifactRepository: "registry.io:5000",
			expectedRegistry:   "registry.io:5000",
		},
		{
			name:               "host without port",
			artifactRepository: "registry.io",
			expectedRegistry:   "registry.io",
		},
		{
			name:               "dot-less repo with port",
			artifactRepository: "localhost:5000/repo",
			expectedRegistry:   "localhost:5000",
		},
		{
			name:               "dot-less repo without port",
			artifactRepository: "localhost/repo",
			expectedRegistry:   "index.docker.io",
		},
		{
			name:               "repo with port",
			artifactRepository: "registry.io:5000/repo",
			expectedRegistry:   "registry.io:5000",
		},
		{
			name:               "repo without port",
			artifactRepository: "registry.io/repo",
			expectedRegistry:   "registry.io",
		},
		{
			name:               "tag",
			artifactRepository: "registry.io/repo:tag",
			expectedRegistry:   "registry.io",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			reg, err := auth.GetRegistryFromArtifactRepository(tt.artifactRepository)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(reg).To(Equal(tt.expectedRegistry))
		})
	}
}

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
	saRef := client.ObjectKey{
		Name:      defaultServiceAccount.Name,
		Namespace: defaultServiceAccount.Namespace,
	}

	now := time.Now()

	for _, tt := range []struct {
		name               string
		provider           *mockProvider
		artifactRepository string
		opts               []auth.Option
		disableObjectLevel bool
		defaultSA          string
		expectedCreds      *auth.ArtifactRegistryCredentials
		expectedErr        string
	}{
		{
			name: "registry token from controller access token",
			provider: &mockProvider{
				returnRegistryInput:   "some-registry.io/some/artifact",
				returnControllerToken: &mockToken{token: "mock-default-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAccessToken:        &mockToken{token: "mock-default-token"},
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "registry token from access token from service account",
			provider: &mockProvider{
				returnName:          "mock-provider",
				returnRegistryInput: "some-registry.io/some/artifact",
				returnAccessToken:   &mockToken{token: "mock-access-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAudiences:          []string{"audience1", "audience2"},
				paramServiceAccount:     *defaultServiceAccount,
				paramOIDCTokenClient:    oidcClient,
				paramArtifactRepository: "some-registry.io/some/artifact",
				paramAccessToken:        &mockToken{token: "mock-access-token"},
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "registry token from access token from service account, works with lockdown",
			provider: &mockProvider{
				returnName:          "mock-provider",
				returnRegistryInput: "some-registry.io/some/artifact",
				returnAccessToken:   &mockToken{token: "mock-access-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAudiences:          []string{"audience1", "audience2"},
				paramServiceAccount:     *defaultServiceAccount,
				paramOIDCTokenClient:    oidcClient,
				paramArtifactRepository: "some-registry.io/some/artifact",
				paramAccessToken:        &mockToken{token: "mock-access-token"},
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			defaultSA: saRef.Name,
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "registry token from access token from service account, works with lockdown and feature gate",
			provider: &mockProvider{
				returnName:          "mock-provider",
				returnRegistryInput: "some-registry.io/some/artifact",
				returnAccessToken:   &mockToken{token: "mock-access-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAudiences:          []string{"audience1", "audience2"},
				paramServiceAccount:     *defaultServiceAccount,
				paramOIDCTokenClient:    oidcClient,
				paramArtifactRepository: "some-registry.io/some/artifact",
				paramAccessToken:        &mockToken{token: "mock-access-token"},
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			defaultSA:          saRef.Name,
			disableObjectLevel: true,
			expectedCreds: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
			expectedErr: "ObjectLevelWorkloadIdentity feature gate is not enabled",
		},
		{
			name: "all the options are taken into account in the cache key",
			provider: &mockProvider{
				returnName:              "mock-provider",
				returnIdentity:          "mock-identity",
				returnRegistryInput:     "artifact-cache-key",
				paramServiceAccount:     *defaultServiceAccount,
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			artifactRepository: "some-registry.io/some/artifact",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(2)
					g.Expect(err).NotTo(HaveOccurred())

					const accessTokenKey = "db625bd5a96dc48fcc100659c6db98857d1e0ceec930bbded0fdece14af4307c"
					var token auth.Token = &mockToken{token: "cached-token"}
					cachedToken, ok, err := tokenCache.GetOrSet(ctx, accessTokenKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					const artifactRegistryCredentialsKey = "61fe71ebbf306060d67acbdc2389d5fd816bee40e7685afe2fdc18b7d3bde1d6"
					token = &auth.ArtifactRegistryCredentials{
						Authenticator: authn.FromConfig(authn.AuthConfig{Username: "cached-registry-token"}),
						ExpiresAt:     now.Add(time.Hour),
					}
					cachedToken, ok, err = tokenCache.GetOrSet(ctx, artifactRegistryCredentialsKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

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
		{
			name: "disable object level workload identity",
			provider: &mockProvider{
				paramServiceAccount: *defaultServiceAccount,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			disableObjectLevel: true,
			expectedErr:        "ObjectLevelWorkloadIdentity feature gate is not enabled",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.provider.t = t

			if !tt.disableObjectLevel {
				auth.EnableObjectLevelWorkloadIdentity()
				t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)
			}

			if tt.defaultSA != "" {
				auth.SetDefaultServiceAccount(tt.defaultSA)
				t.Cleanup(func() { auth.SetDefaultServiceAccount("") })
			}

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
