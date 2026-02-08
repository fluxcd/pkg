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
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google/externalaccount"
	"google.golang.org/api/container/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/gcp"
)

func TestProvider_NewControllerToken(t *testing.T) {
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

func TestProvider_NewTokenForOIDCToken(t *testing.T) {
	for _, tt := range []struct {
		name             string
		conf             externalaccount.Config
		exchangeAudience string
		identity         *gcp.Identity
	}{
		{
			name: "direct access - GKE",
			conf: externalaccount.Config{
				Audience:         "identitynamespace:project-id.svc.id.goog:https://container.googleapis.com/v1/projects/project-id/locations/cluster-location/clusters/cluster-name",
				SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:         "https://sts.googleapis.com/v1/token",
				TokenInfoURL:     "https://sts.googleapis.com/v1/introspect",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.StaticTokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			exchangeAudience: "identitynamespace:project-id.svc.id.goog:https://container.googleapis.com/v1/projects/project-id/locations/cluster-location/clusters/cluster-name",
			identity:         &gcp.Identity{},
		},
		{
			name: "impersonation - GKE",
			conf: externalaccount.Config{
				Audience:                       "identitynamespace:project-id.svc.id.goog:https://container.googleapis.com/v1/projects/project-id/locations/cluster-location/clusters/cluster-name",
				SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:                       "https://sts.googleapis.com/v1/token",
				ServiceAccountImpersonationURL: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/test-sa@project-id.iam.gserviceaccount.com:generateAccessToken",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.StaticTokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			exchangeAudience: "identitynamespace:project-id.svc.id.goog:https://container.googleapis.com/v1/projects/project-id/locations/cluster-location/clusters/cluster-name",
			identity:         &gcp.Identity{GCPServiceAccount: "test-sa@project-id.iam.gserviceaccount.com"},
		},
		{
			name: "direct access - federation",
			conf: externalaccount.Config{
				Audience:         "//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
				SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:         "https://sts.googleapis.com/v1/token",
				TokenInfoURL:     "https://sts.googleapis.com/v1/introspect",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.StaticTokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			exchangeAudience: "//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			identity:         &gcp.Identity{},
		},
		{
			name: "impersonation - federation",
			conf: externalaccount.Config{
				Audience:                       "//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
				SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:                       "https://sts.googleapis.com/v1/token",
				ServiceAccountImpersonationURL: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/test-sa@project-id.iam.gserviceaccount.com:generateAccessToken",
				Scopes: []string{
					"https://www.googleapis.com/auth/cloud-platform",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				SubjectTokenSupplier: gcp.StaticTokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			exchangeAudience: "//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			identity:         &gcp.Identity{GCPServiceAccount: "test-sa@project-id.iam.gserviceaccount.com"},
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
			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint("https://sts.example.com"),
			}

			provider := gcp.Provider{Implementation: impl}
			token, err := provider.NewTokenForOIDCToken(context.Background(), oidcToken, tt.exchangeAudience, tt.identity, opts...)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(token).To(Equal(&gcp.Token{oauth2.Token{AccessToken: "access-token"}}))
		})
	}
}

// TestProvider_GetAudience_NoAudience must run before TestProvider_GetAudience
// because gkeMetadata is a package-level singleton that caches results. Once loaded,
// it cannot be reset from the external test package.
func TestProvider_GetAudience_NoAudience(t *testing.T) {
	g := NewWithT(t)
	// Start a failing mock so we don't hang trying to reach the real metadata service.
	startFailingGKEMetadataServer(t)
	// No audiences, no SA, no GKE metadata → ErrNoAudienceForOIDCImpersonation
	_, _, err := gcp.Provider{}.GetAudiences(context.Background())
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, auth.ErrNoAudienceForOIDCImpersonation)).To(BeTrue())
}

func TestProvider_GetAudience(t *testing.T) {
	startGKEMetadataServer(t)

	for _, tt := range []struct {
		name             string
		annotations      map[string]string
		expectedOIDC     string
		expectedExchange string
	}{
		{
			name: "federation",
			annotations: map[string]string{
				"gcp.auth.fluxcd.io/workload-identity-provider": "projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			},
			expectedOIDC:     "//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			expectedExchange: "//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
		},
		{
			name:             "gke",
			expectedOIDC:     "project-id.svc.id.goog",
			expectedExchange: "identitynamespace:project-id.svc.id.goog:https://container.googleapis.com/v1/projects/project-id/locations/cluster-location/clusters/cluster-name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			oidcAud, exchangeAud, err := gcp.Provider{}.GetAudiences(context.Background(), auth.WithServiceAccount(serviceAccount))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(oidcAud).To(Equal(tt.expectedOIDC))
			g.Expect(exchangeAud).To(Equal(tt.expectedExchange))
		})
	}
}

func TestProvider_GetAudience_ExplicitAudiences(t *testing.T) {
	t.Run("valid workload identity provider", func(t *testing.T) {
		g := NewWithT(t)
		oidcAud, exchangeAud, err := gcp.Provider{}.GetAudiences(context.Background(),
			auth.WithAudiences("projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(oidcAud).To(Equal("//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider"))
		g.Expect(exchangeAud).To(Equal(oidcAud))
	})

	t.Run("invalid workload identity provider", func(t *testing.T) {
		g := NewWithT(t)
		_, _, err := gcp.Provider{}.GetAudiences(context.Background(),
			auth.WithAudiences("invalid-provider"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid GCP workload identity provider"))
	})
}

func TestProvider_GetIdentity(t *testing.T) {
	for _, tt := range []struct {
		name        string
		annotations map[string]string
		expected    *gcp.Identity
		err         string
	}{
		{
			name: "impersonation",
			annotations: map[string]string{
				"iam.gke.io/gcp-service-account": "test-sa@project-id.iam.gserviceaccount.com",
			},
			expected: &gcp.Identity{GCPServiceAccount: "test-sa@project-id.iam.gserviceaccount.com"},
		},
		{
			name:     "direct access",
			expected: &gcp.Identity{},
		},
		{
			name: "invalid email",
			annotations: map[string]string{
				"iam.gke.io/gcp-service-account": "foobar",
			},
			err: "invalid iam.gke.io/gcp-service-account annotation",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			identity, err := gcp.Provider{}.GetIdentity(auth.WithServiceAccount(serviceAccount))
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(identity).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(identity).To(Equal(tt.expected))
			}
		})
	}
}

func TestProvider_GetIdentity_WithExplicitIdentity(t *testing.T) {
	t.Run("valid explicit identity with email", func(t *testing.T) {
		g := NewWithT(t)
		identity, err := gcp.Provider{}.GetIdentity(
			auth.WithIdentityForOIDCImpersonation(&gcp.Identity{GCPServiceAccount: "sa@project.iam.gserviceaccount.com"}))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(identity).To(Equal(&gcp.Identity{GCPServiceAccount: "sa@project.iam.gserviceaccount.com"}))
	})

	t.Run("valid explicit identity without email (direct access)", func(t *testing.T) {
		g := NewWithT(t)
		identity, err := gcp.Provider{}.GetIdentity(
			auth.WithIdentityForOIDCImpersonation(&gcp.Identity{}))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(identity).To(Equal(&gcp.Identity{}))
	})

	t.Run("invalid explicit identity", func(t *testing.T) {
		g := NewWithT(t)
		identity, err := gcp.Provider{}.GetIdentity(
			auth.WithIdentityForOIDCImpersonation(&gcp.Identity{GCPServiceAccount: "invalid"}))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid GCP service account email"))
		g.Expect(identity).To(BeNil())
	})

	t.Run("wrong identity type", func(t *testing.T) {
		g := NewWithT(t)
		type wrongIdentity struct{ auth.Identity }
		identity, err := gcp.Provider{}.GetIdentity(
			auth.WithIdentityForOIDCImpersonation(&wrongIdentity{}))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid identity type"))
		g.Expect(identity).To(BeNil())
	})

	t.Run("no SA and no identity", func(t *testing.T) {
		g := NewWithT(t)
		identity, err := gcp.Provider{}.GetIdentity()
		g.Expect(err).To(MatchError(auth.ErrNoIdentityForOIDCImpersonation))
		g.Expect(identity).To(BeNil())
	})
}

func TestIdentity_Validate(t *testing.T) {
	g := NewWithT(t)

	g.Expect((&gcp.Identity{GCPServiceAccount: "sa@project.iam.gserviceaccount.com"}).Validate()).To(Succeed())
	g.Expect((&gcp.Identity{GCPServiceAccount: "invalid"}).Validate()).To(HaveOccurred())
	g.Expect((&gcp.Identity{GCPServiceAccount: ""}).Validate()).To(HaveOccurred())
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	g := NewWithT(t)

	exp := time.Now()

	provider := gcp.Provider{
		Implementation: &mockImplementation{
			t:           t,
			argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
			returnToken: &oauth2.Token{
				AccessToken: "access-token",
				Expiry:      exp,
			},
		},
	}

	creds, err := auth.GetArtifactRegistryCredentials(context.Background(), provider, "gcr.io",
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(creds).To(Equal(&auth.ArtifactRegistryCredentials{
		Authenticator: &authn.Basic{
			Username: "oauth2accesstoken",
			Password: "access-token",
		},
		ExpiresAt: exp,
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

func TestProvider_NewRESTConfig(t *testing.T) {
	for _, tt := range []struct {
		name           string
		cluster        string
		clusterAddress string
		caData         string
		masterAuth     *container.MasterAuth
		endpoint       string
		err            string
	}{
		{
			name:    "valid GKE cluster",
			cluster: "projects/test-project/locations/us-central1/clusters/test-cluster",
			masterAuth: &container.MasterAuth{
				ClusterCaCertificate: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t", // base64 encoded "-----BEGIN CERTIFICATE-----"
			},
			endpoint: "https://203.0.113.10",
		},
		{
			name:           "valid GKE cluster with address match",
			cluster:        "projects/test-project/locations/us-central1/clusters/test-cluster",
			clusterAddress: "https://203.0.113.10:443",
			masterAuth: &container.MasterAuth{
				ClusterCaCertificate: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t",
			},
			endpoint: "https://203.0.113.10",
		},
		{
			name:     "valid GKE cluster with CA",
			cluster:  "projects/test-project/locations/us-central1/clusters/test-cluster",
			caData:   "-----BEGIN CERTIFICATE-----",
			endpoint: "https://203.0.113.10",
		},
		{
			name:           "CA and address only",
			clusterAddress: "https://203.0.113.10",
			caData:         "-----BEGIN CERTIFICATE-----",
			endpoint:       "https://203.0.113.10",
		},
		{
			name:           "valid GKE cluster with address override",
			cluster:        "projects/test-project/locations/us-central1/clusters/test-cluster",
			clusterAddress: "https://198.51.100.10:443",
			masterAuth: &container.MasterAuth{
				ClusterCaCertificate: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t",
			},
			endpoint: "https://203.0.113.10",
		},
		{
			name:    "invalid cluster ID",
			cluster: "invalid-cluster-id",
			err:     "invalid GKE cluster ID: 'invalid-cluster-id'. must match ^projects/[^/]{1,200}/locations/[^/]{1,200}/clusters/[^/]{1,200}$",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tokenExpiry := time.Now().Add(1 * time.Hour)
			impl := &mockImplementation{
				t:                t,
				expectGKEAPICall: tt.clusterAddress == "" || tt.caData == "",
				argCluster:       tt.cluster,
				argProxyURL:      &url.URL{Scheme: "http", Host: "proxy.example.com"},
				returnToken: &oauth2.Token{
					AccessToken: "access-token",
					Expiry:      tokenExpiry,
				},
				returnCluster: &container.Cluster{
					Endpoint:   tt.endpoint,
					MasterAuth: tt.masterAuth,
				},
			}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
			}

			if tt.cluster != "" {
				opts = append(opts, auth.WithClusterResource(tt.cluster))
			}

			if tt.clusterAddress != "" {
				opts = append(opts, auth.WithClusterAddress(tt.clusterAddress))
			}

			if tt.caData != "" {
				opts = append(opts, auth.WithCAData(tt.caData))
			}

			provider := gcp.Provider{Implementation: impl}
			restConfig, err := auth.GetRESTConfig(context.Background(), provider, opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(restConfig).NotTo(BeNil())
				expectedHost := tt.endpoint
				if tt.clusterAddress != "" {
					expectedHost = tt.clusterAddress
				}
				g.Expect(restConfig.Host).To(Equal(expectedHost))
				g.Expect(restConfig.BearerToken).To(Equal("access-token"))
				g.Expect(restConfig.CAData).To(Equal([]byte("-----BEGIN CERTIFICATE-----")))
				g.Expect(restConfig.ExpiresAt).To(Equal(tokenExpiry))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(restConfig).To(BeNil())
			}
		})
	}
}

func TestProvider_GetAccessTokenOptionsForCluster(t *testing.T) {
	g := NewWithT(t)

	t.Run("with cluster resource", func(t *testing.T) {
		opts, err := gcp.Provider{}.GetAccessTokenOptionsForCluster(
			auth.WithClusterResource("projects/test-project/locations/us-central1/clusters/test-cluster"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).To(HaveLen(1))
		g.Expect(opts[0]).To(HaveLen(0)) // Empty slice - no options needed for GCP
	})

	t.Run("without cluster resource", func(t *testing.T) {
		opts, err := gcp.Provider{}.GetAccessTokenOptionsForCluster()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).To(HaveLen(1))
		g.Expect(opts[0]).To(HaveLen(0)) // Empty slice - no options needed for GCP
	})
}

func TestProvider_GetImpersonationAnnotationKey(t *testing.T) {
	g := NewWithT(t)
	g.Expect(gcp.Provider{}.GetImpersonationAnnotationKey()).To(Equal("impersonate"))
}

func TestProvider_NewIdentity(t *testing.T) {
	g := NewWithT(t)
	identity := gcp.Provider{}.NewIdentity()
	g.Expect(identity).To(Equal(&gcp.Identity{}))
}

func TestIdentity_UnmarshalJSON(t *testing.T) {
	for _, tt := range []struct {
		name     string
		json     string
		expected *gcp.Identity
		err      string
	}{
		{
			name:     "valid GCP service account",
			json:     `{"gcpServiceAccount":"test-sa@project-id.iam.gserviceaccount.com"}`,
			expected: &gcp.Identity{GCPServiceAccount: "test-sa@project-id.iam.gserviceaccount.com"},
		},
		{
			name: "invalid GCP service account",
			json: `{"gcpServiceAccount":"foobar"}`,
			err:  "invalid GCP service account email",
		},
		{
			name: "empty GCP service account",
			json: `{"gcpServiceAccount":""}`,
			err:  "invalid GCP service account email",
		},
		{
			name: "invalid JSON",
			json: `{invalid`,
			err:  "invalid character",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			identity := &gcp.Identity{}
			err := json.Unmarshal([]byte(tt.json), identity)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(identity).To(Equal(tt.expected))
			}
		})
	}
}

func TestProvider_NewTokenForNativeToken(t *testing.T) {
	for _, tt := range []struct {
		name              string
		gcpServiceAccount string
		impersonationErr  error
		err               string
	}{
		{
			name:              "valid",
			gcpServiceAccount: "target-sa@project-id.iam.gserviceaccount.com",
		},
		{
			name:              "impersonation error",
			gcpServiceAccount: "target-sa@project-id.iam.gserviceaccount.com",
			impersonationErr:  errors.New("impersonation failed"),
			err:               "failed to create impersonated token source: impersonation failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tokenExpiry := time.Now().Add(1 * time.Hour)
			impl := &mockImplementation{
				t:                          t,
				expectImpersonationAPICall: true,
				argProxyURL:                &url.URL{Scheme: "http", Host: "proxy.example.com"},
				argImpersonateTarget:       tt.gcpServiceAccount,
				returnToken: &oauth2.Token{
					AccessToken: "initial-token",
					Expiry:      tokenExpiry,
				},
				returnImpersonatedToken: &oauth2.Token{
					AccessToken: "impersonated-token",
					Expiry:      tokenExpiry,
				},
				returnImpersonationErr: tt.impersonationErr,
			}

			// Create the identity via NewIdentity + json.Unmarshal.
			identity := gcp.Provider{}.NewIdentity()
			err := json.Unmarshal([]byte(`{"gcpServiceAccount":"`+tt.gcpServiceAccount+`"}`), identity)
			g.Expect(err).NotTo(HaveOccurred())

			// Create a mock initial token.
			initialToken := &gcp.Token{Token: oauth2.Token{
				AccessToken: "initial-token",
				Expiry:      tokenExpiry,
			}}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
			}

			provider := gcp.Provider{Implementation: impl}
			token, err := provider.NewTokenForNativeToken(context.Background(), initialToken, identity, opts...)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(token).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&gcp.Token{Token: oauth2.Token{
					AccessToken: "impersonated-token",
					Expiry:      tokenExpiry,
				}}))
			}
		})
	}
}
