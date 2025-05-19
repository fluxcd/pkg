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

package gcp_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google/externalaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/gcp"
)

func TestNewControllerToken(t *testing.T) {
	g := NewWithT(t)

	impl := &mockImplementation{
		t:           t,
		argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
		returnToken: &oauth2.Token{AccessToken: "access-token"},
	}

	opts := []auth.Option{
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
	}

	provider := gcp.Provider{Implementation: impl}
	token, err := provider.NewControllerToken(context.Background(), opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&gcp.Token{oauth2.Token{AccessToken: "access-token"}}))
}

func TestProvider_NewTokenForServiceAccount(t *testing.T) {
	startGKEMetadataServer(t)

	for _, tt := range []struct {
		name        string
		conf        externalaccount.Config
		annotations map[string]string
		err         string
	}{
		{
			name: "direct access",
			conf: externalaccount.Config{
				Audience:         "identitynamespace:project-id.svc.id.goog:https://container.googleapis.com/v1/projects/project-id/locations/cluster-location/clusters/cluster-name",
				SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:         "https://sts.googleapis.com/v1/token",
				TokenInfoURL:     "https://sts.googleapis.com/v1/introspect",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.TokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
		},
		{
			name: "impersonation",
			conf: externalaccount.Config{
				Audience:                       "identitynamespace:project-id.svc.id.goog:https://container.googleapis.com/v1/projects/project-id/locations/cluster-location/clusters/cluster-name",
				SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:                       "https://sts.googleapis.com/v1/token",
				ServiceAccountImpersonationURL: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/test-sa@project-id.iam.gserviceaccount.com:generateAccessToken",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.TokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			annotations: map[string]string{
				"iam.gke.io/gcp-service-account": "test-sa@project-id.iam.gserviceaccount.com",
			},
		},
		{
			name: "direct access - federation",
			conf: externalaccount.Config{
				Audience:         "https://iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
				SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:         "https://sts.googleapis.com/v1/token",
				TokenInfoURL:     "https://sts.googleapis.com/v1/introspect",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.TokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			annotations: map[string]string{
				"gcp.auth.fluxcd.io/workload-identity-provider": "projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			},
		},
		{
			name: "impersonation - federation",
			conf: externalaccount.Config{
				Audience:                       "https://iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
				SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:                       "https://sts.googleapis.com/v1/token",
				ServiceAccountImpersonationURL: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/test-sa@project-id.iam.gserviceaccount.com:generateAccessToken",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.TokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			annotations: map[string]string{
				"iam.gke.io/gcp-service-account":                "test-sa@project-id.iam.gserviceaccount.com",
				"gcp.auth.fluxcd.io/workload-identity-provider": "projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			},
		},
		{
			name: "invalid sa email",
			annotations: map[string]string{
				"iam.gke.io/gcp-service-account": "foobar",
			},
			err: `invalid iam.gke.io/gcp-service-account annotation: 'foobar'. must match ^[a-zA-Z0-9-]{1,100}@[a-zA-Z0-9-]{1,100}\.iam\.gserviceaccount\.com$`,
		},
		{
			name: "invalid workload identity provider",
			annotations: map[string]string{
				"gcp.auth.fluxcd.io/workload-identity-provider": "foobar",
			},
			err: `invalid gcp.auth.fluxcd.io/workload-identity-provider annotation: 'foobar'. must match ^projects/\d{1,30}/locations/global/workloadIdentityPools/[^/]{1,100}/providers/[^/]{1,100}$`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:           t,
				argConfig:   tt.conf,
				argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
				returnToken: &oauth2.Token{AccessToken: "access-token"},
			}

			oidcToken := "oidc-token"
			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "test-ns",
					Annotations: tt.annotations,
				},
			}
			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint("https://sts.example.com"),
			}

			provider := gcp.Provider{Implementation: impl}
			token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, serviceAccount, opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&gcp.Token{oauth2.Token{AccessToken: "access-token"}}))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(token).To(BeNil())
			}
		})
	}
}

func TestProvider_GetAudience(t *testing.T) {
	startGKEMetadataServer(t)

	for _, tt := range []struct {
		name        string
		annotations map[string]string
		expected    string
	}{
		{
			name: "federation",
			annotations: map[string]string{
				"gcp.auth.fluxcd.io/workload-identity-provider": "projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			},
			expected: "https://iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
		},
		{
			name:     "gke",
			expected: "project-id.svc.id.goog",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			aud, err := gcp.Provider{}.GetAudience(context.Background(), serviceAccount)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(aud).To(Equal(tt.expected))
		})
	}
}

func TestProvider_GetIdentity(t *testing.T) {
	for _, tt := range []struct {
		name        string
		annotations map[string]string
		expected    string
	}{
		{
			name: "impersonation",
			annotations: map[string]string{
				"iam.gke.io/gcp-service-account": "test-sa@project-id.iam.gserviceaccount.com",
			},
			expected: "test-sa@project-id.iam.gserviceaccount.com",
		},
		{
			name:     "direct access",
			expected: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			identity, err := gcp.Provider{}.GetIdentity(serviceAccount)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(identity).To(Equal(tt.expected))
		})
	}
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	g := NewWithT(t)

	exp := time.Now()

	accessToken := &gcp.Token{oauth2.Token{
		AccessToken: "access-token",
		Expiry:      exp,
	}}

	creds, err := gcp.Provider{}.NewArtifactRegistryCredentials(context.Background(), "", accessToken)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(creds).NotTo(BeNil())
	g.Expect(creds.ExpiresAt).To(Equal(exp))
	g.Expect(creds.Authenticator).NotTo(BeNil())
	authConf, err := creds.Authenticator.Authorization()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(authConf).To(Equal(&authn.AuthConfig{
		Username: "oauth2accesstoken",
		Password: "access-token",
	}))
}

func TestProvider_ParseArtifactRegistry(t *testing.T) {
	for _, tt := range []struct {
		artifactRepository string
		expectValid        bool
	}{
		{
			artifactRepository: "gcr.io",
			expectValid:        true,
		},
		{
			artifactRepository: ".gcr.io",
			expectValid:        false,
		},
		{
			artifactRepository: "a.gcr.io",
			expectValid:        true,
		},
		{
			artifactRepository: "-docker.pkg.dev",
			expectValid:        false,
		},
		{
			artifactRepository: "a-docker.pkg.dev",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com",
			expectValid:        false,
		},
	} {
		t.Run(tt.artifactRepository, func(t *testing.T) {
			g := NewWithT(t)

			cacheKey, err := gcp.Provider{}.ParseArtifactRepository(tt.artifactRepository)

			if tt.expectValid {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cacheKey).To(Equal("gcp"))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(cacheKey).To(BeEmpty())
			}
		})
	}
}
