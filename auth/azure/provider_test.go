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

package azure_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
)

func TestProvider_NewControllerToken(t *testing.T) {
	g := NewWithT(t)

	impl := &mockImplementation{
		t:           t,
		argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argScopes:   []string{"scope1", "scope2"},
		returnToken: "access-token",
	}

	opts := []auth.Option{
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
		auth.WithScopes("scope1", "scope2"),
	}

	provider := azure.Provider{Implementation: impl}
	token, err := provider.NewControllerToken(context.Background(), opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&azure.Token{AccessToken: azcore.AccessToken{Token: "access-token"}}))
}

func TestProvider_NewTokenForServiceAccount(t *testing.T) {
	impl := &mockImplementation{
		t:            t,
		argTenantID:  "tenant-id",
		argClientID:  "client-id",
		argOIDCToken: "oidc-token",
		argProxyURL:  &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argScopes:    []string{"scope1", "scope2"},
		returnToken:  "access-token",
	}

	for _, tt := range []struct {
		name        string
		annotations map[string]string
		err         string
	}{
		{
			name: "valid",
			annotations: map[string]string{
				"azure.workload.identity/tenant-id": "tenant-id",
				"azure.workload.identity/client-id": "client-id",
			},
		},
		{
			name: "tenant id missing",
			annotations: map[string]string{
				"azure.workload.identity/client-id": "client-id",
			},
			err: "azure tenant ID is not set in the service account annotation azure.workload.identity/tenant-id",
		},
		{
			name: "client id missing",
			annotations: map[string]string{
				"azure.workload.identity/tenant-id": "tenant-id",
			},
			err: "azure client ID is not set in the service account annotation azure.workload.identity/client-id",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			oidcToken := "oidc-token"
			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithScopes("scope1", "scope2"),
			}

			provider := azure.Provider{Implementation: impl}
			token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, serviceAccount, opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&azure.Token{AccessToken: azcore.AccessToken{Token: "access-token"}}))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(token).To(BeNil())
			}
		})
	}
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	g := NewWithT(t)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	g.Expect(err).NotTo(HaveOccurred())
	exp := time.Now().Add(time.Hour).Unix()
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"exp": exp,
	}).SignedString(privateKey)
	g.Expect(err).NotTo(HaveOccurred())

	for _, tt := range []struct {
		registry      string
		expectedScope string
	}{
		{
			registry:      "foo.azurecr.io",
			expectedScope: "https://management.azure.com/.default",
		},
		{
			registry:      "foo.azurecr.cn",
			expectedScope: "https://management.chinacloudapi.cn/.default",
		},
		{
			registry:      "foo.azurecr.us",
			expectedScope: "https://management.usgovcloudapi.net/.default",
		},
	} {
		t.Run(tt.registry, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:           t,
				argURL:      fmt.Sprintf("https://%s/oauth2/exchange", tt.registry),
				argBody:     fmt.Sprintf("access_token=access-token&grant_type=access_token&service=%s", tt.registry),
				argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
				argScopes:   []string{tt.expectedScope},
				returnToken: "access-token",
				returnResp: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"refresh_token":"%s"}`, refreshToken))),
				},
			}
			provider := azure.Provider{Implementation: impl}

			artifactRepository := fmt.Sprintf("%s/repo", tt.registry)
			opts := []auth.Option{
				auth.WithArtifactRepository(artifactRepository),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
			}

			registryURL, err := provider.ParseArtifactRepository(artifactRepository)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(registryURL).To(Equal(fmt.Sprintf("https://%s", tt.registry)))

			accessToken, err := provider.NewControllerToken(context.Background(), opts...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(accessToken).To(Equal(&azure.Token{AccessToken: azcore.AccessToken{Token: "access-token"}}))

			token, err := provider.NewArtifactRegistryCredentials(context.Background(), registryURL, accessToken, opts...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(token).To(Equal(&auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{
					Username: "00000000-0000-0000-0000-000000000000",
					Password: refreshToken,
				}),
				ExpiresAt: time.Unix(exp, 0),
			}))
		})
	}
}

func TestProvider_ParseArtifactRegistry(t *testing.T) {
	for _, tt := range []struct {
		artifactRepository  string
		expectedRegistryURL string
		expectValid         bool
	}{
		{
			artifactRepository:  "foo.azurecr.io",
			expectedRegistryURL: "https://foo.azurecr.io",
			expectValid:         true,
		},
		{
			artifactRepository:  "foo.azurecr.cn",
			expectedRegistryURL: "https://foo.azurecr.cn",
			expectValid:         true,
		},
		{
			artifactRepository:  "foo.azurecr.de",
			expectedRegistryURL: "https://foo.azurecr.de",
			expectValid:         true,
		},
		{
			artifactRepository:  "foo.azurecr.us",
			expectedRegistryURL: "https://foo.azurecr.us",
			expectValid:         true,
		},
		{
			artifactRepository: "foo.azurecr.com",
			expectValid:        false,
		},
		{
			artifactRepository: ".azurecr.io",
			expectValid:        false,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com",
			expectValid:        false,
		},
	} {
		t.Run(tt.artifactRepository, func(t *testing.T) {
			g := NewWithT(t)

			registryURL, err := azure.Provider{}.ParseArtifactRepository(tt.artifactRepository)

			if tt.expectValid {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(registryURL).To(Equal(tt.expectedRegistryURL))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(registryURL).To(BeEmpty())
			}
		})
	}
}
