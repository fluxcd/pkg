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
	"google.golang.org/api/container/v1"

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

func TestProvider_NewTokenForServiceAccount(t *testing.T) {
	startGKEMetadataServer(t)

	for _, tt := range []struct {
		name  string
		conf  externalaccount.Config
		gcpSA string
		wip   string
		err   string
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
				SubjectTokenSupplier: gcp.StaticTokenSupplier("oidc-token"),
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
				SubjectTokenSupplier: gcp.StaticTokenSupplier("oidc-token"),
				UniverseDomain:       "googleapis.com",
			},
			gcpSA: "test-sa@project-id.iam.gserviceaccount.com",
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
			wip: "projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
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
			gcpSA: "test-sa@project-id.iam.gserviceaccount.com",
			wip:   "projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
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
			}
			if tt.wip != "" {
				opts = append(opts, auth.WithGCPWorkloadIdentityProvider(tt.wip))
			}
			if tt.gcpSA != "" {
				opts = append(opts, auth.WithGCPServiceAccount(tt.gcpSA))
			}

			provider := gcp.Provider{Implementation: impl}
			token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, opts...)

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
		name     string
		wip      string
		expected string
	}{
		{
			name:     "federation",
			wip:      "projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
			expected: "//iam.googleapis.com/projects/1234567890/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
		},
		{
			name:     "gke",
			expected: "project-id.svc.id.goog",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var opts []auth.Option
			if tt.wip != "" {
				opts = append(opts, auth.WithGCPWorkloadIdentityProvider(tt.wip))
			}

			aud, err := gcp.Provider{}.GetAudiences(context.Background(), opts...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(aud).To(Equal([]string{tt.expected}))
		})
	}
}

func TestProvider_GetIdentity(t *testing.T) {
	for _, tt := range []struct {
		name     string
		gcpSA    string
		expected string
	}{
		{
			name:     "impersonation",
			gcpSA:    "test-sa@project-id.iam.gserviceaccount.com",
			expected: "test-sa@project-id.iam.gserviceaccount.com",
		},
		{
			name:     "direct access",
			expected: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var opts []auth.Option
			if tt.gcpSA != "" {
				opts = append(opts, auth.WithGCPServiceAccount(tt.gcpSA))
			}

			identity, err := gcp.Provider{}.GetIdentity(opts...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(identity).To(Equal(tt.expected))
		})
	}
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
		{
			artifactRepository: "docker.s3nsregistry.fr/s3ns/dsna-shared-registry-0/shared-registry",
			expectValid:        true,
		},
		{
			artifactRepository: "docker.s3nsregistry.fr",
			expectValid:        true,
		},
		{
			artifactRepository: "u-france-east1-docker.s3nsregistry.fr",
			expectValid:        false,
		},
		{
			artifactRepository: "-docker.s3nsregistry.fr",
			expectValid:        false,
		},
		{
			artifactRepository: "a.docker.s3nsregistry.fr",
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
