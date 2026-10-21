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

package generic_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/generic"
)

func TestProvider_NewControllerToken(t *testing.T) {
	g := NewWithT(t)
	token, err := generic.Provider{}.NewControllerToken(context.Background())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("client is required to create a controller token"))
	g.Expect(token).To(BeNil())
}

func TestProvider_NewAccessCredentialForIdentity(t *testing.T) {
	signToken := func(t *testing.T, claims jwt.MapClaims) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte("test-secret"))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return signed
	}

	expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)
	validToken := signToken(t, jwt.MapClaims{"exp": expiresAt.Unix()})

	t.Run("kubernetes identity", func(t *testing.T) {
		g := NewWithT(t)

		identity := &auth.JWTCredential{
			Provider:  cryptov1.CredentialProviderKubernetes,
			Token:     validToken,
			ExpiresAt: expiresAt,
		}
		credential, err := generic.Provider{}.NewAccessCredentialForIdentity(context.Background(), identity)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(credential).To(Equal(identity))
	})

	t.Run("spiffe identity disables caching", func(t *testing.T) {
		g := NewWithT(t)

		identity := &auth.JWTCredential{
			Provider:  cryptov1.CredentialProviderSPIFFE,
			Token:     validToken,
			ExpiresAt: expiresAt,
		}
		credential, err := generic.Provider{}.NewAccessCredentialForIdentity(context.Background(), identity)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(credential).NotTo(BeIdenticalTo(identity))
		g.Expect(credential).To(Equal(&auth.JWTCredential{
			Provider: cryptov1.CredentialProviderSPIFFE,
			Token:    validToken,
		}))
		g.Expect(identity.ExpiresAt).To(Equal(expiresAt))
	})

	t.Run("invalid identity type", func(t *testing.T) {
		g := NewWithT(t)

		credential, err := generic.Provider{}.NewAccessCredentialForIdentity(
			context.Background(), &auth.RESTConfig{})
		g.Expect(err).To(MatchError(ContainSubstring("expected *auth.JWTCredential")))
		g.Expect(credential).To(BeNil())
	})
}

func TestProvider_GetAudiences(t *testing.T) {
	g := NewWithT(t)
	audiences, err := generic.Provider{}.GetAudiences(context.Background(),
		auth.WithCredentialAudiences("audience1", "audience2"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(audiences).To(ConsistOf("audience1", "audience2"))
}

func TestProvider_GetAccessCredentialOptionsForArtifactRepository(t *testing.T) {
	t.Run("registry host is the default audience", func(t *testing.T) {
		g := NewWithT(t)

		opts, err := generic.Provider{}.GetAccessCredentialOptionsForArtifactRepository(
			"my-zot-registry.com/owner/repo")
		g.Expect(err).NotTo(HaveOccurred())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.CredentialAudiences).To(ConsistOf("my-zot-registry.com"))
	})

	t.Run("explicit audiences are not overridden", func(t *testing.T) {
		g := NewWithT(t)

		opts, err := generic.Provider{}.GetAccessCredentialOptionsForArtifactRepository(
			"my-zot-registry.com/owner/repo",
			auth.WithCredentialAudiences("audience1", "audience2"))
		g.Expect(err).NotTo(HaveOccurred())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.CredentialAudiences).To(ConsistOf("audience1", "audience2"))
	})

	t.Run("invalid artifact repository", func(t *testing.T) {
		g := NewWithT(t)

		opts, err := generic.Provider{}.GetAccessCredentialOptionsForArtifactRepository(
			"registry.io/OWNER/repo")
		g.Expect(err).To(MatchError(ContainSubstring("failed to parse artifact repository")))
		g.Expect(opts).To(BeNil())
	})
}

func TestProvider_ParseArtifactRepository(t *testing.T) {
	p := generic.Provider{}
	for _, repo := range []string{
		"any-registry.io",
		"ghcr.io/owner/repo",
		"docker.io/library/nginx",
	} {
		t.Run(repo, func(t *testing.T) {
			g := NewWithT(t)
			parsed, err := p.ParseArtifactRepository(repo)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(parsed).To(Equal(generic.ProviderName))
		})
	}
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	credential := &auth.JWTCredential{
		Token:     "test-token",
		ExpiresAt: expiresAt,
	}

	t.Run("bearer token", func(t *testing.T) {
		g := NewWithT(t)

		creds, err := generic.Provider{}.NewArtifactRegistryCredentials(
			context.Background(), "any-registry.io", credential)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(creds).To(Equal(&auth.ArtifactRegistryCredentials{
			Authenticator: &authn.Bearer{Token: "test-token"},
			ExpiresAt:     expiresAt,
		}))
	})

	t.Run("username and password", func(t *testing.T) {
		g := NewWithT(t)

		creds, err := generic.Provider{}.NewArtifactRegistryCredentials(
			context.Background(), "any-registry.io", credential,
			auth.WithCredentialUsername("user"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(creds).To(Equal(&auth.ArtifactRegistryCredentials{
			Authenticator: authn.FromConfig(authn.AuthConfig{
				Username: "user",
				Password: "test-token",
			}),
			ExpiresAt: expiresAt,
		}))
	})
}

func TestProvider_GetIdentity(t *testing.T) {
	g := NewWithT(t)
	id, err := generic.Provider{}.GetIdentity(
		auth.WithServiceAccountName("tenant"),
		auth.WithObject("", "ServiceAccount", &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal("system:serviceaccount:default:tenant"))
}

func TestProvider_NewRESTConfig(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	accessCredential := &auth.JWTCredential{
		Token:     "token",
		ExpiresAt: expiresAt,
	}

	for _, tt := range []struct {
		name           string
		clusterAddress string
		caData         string
		err            string
	}{
		{
			name: "address is required",
			err:  "cluster address is required to create a REST config",
		},
		{
			name:           "with CA data",
			clusterAddress: "https://example.com",
			caData:         "----- BEGIN CERTIFICATE-----",
		},
		{
			name:           "without CA data",
			clusterAddress: "https://example.com",
		},
		{
			name:           "invalid address",
			clusterAddress: "http://example.com",
			err:            "must use https scheme",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := []auth.Option{}
			if tt.clusterAddress != "" {
				opts = append(opts, auth.WithClusterAddress(tt.clusterAddress))
			}
			if tt.caData != "" {
				opts = append(opts, auth.WithCAData(tt.caData))
			}

			conf, err := generic.Provider{}.NewRESTConfig(context.Background(),
				[]auth.Credential{accessCredential}, opts...)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(conf).To(BeNil())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(conf).NotTo(BeNil())

			// Validate REST config.
			g.Expect(conf.Host).To(Equal("https://example.com:443"))
			g.Expect(conf.CAData).To(Equal([]byte(tt.caData)))
			g.Expect(conf.BearerToken).To(Equal("token"))
			g.Expect(conf.ExpiresAt).To(Equal(expiresAt))
		})
	}
}

func TestProvider_GetAccessCredentialOptionsForCluster(t *testing.T) {
	t.Run("without audiences", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := generic.Provider{}.GetAccessCredentialOptionsForCluster(
			auth.WithClusterAddress("https://example.com"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).To(HaveLen(1))
		g.Expect(opts[0]).To(HaveLen(1))
		var o auth.Options
		o.Apply(opts[0]...)
		g.Expect(o.CredentialAudiences).To(ConsistOf("https://example.com"))
	})

	t.Run("with audiences", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := generic.Provider{}.GetAccessCredentialOptionsForCluster(
			auth.WithClusterAddress("https://example.com"),
			auth.WithCredentialAudiences("audience1", "audience2"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).To(HaveLen(1))
		g.Expect(opts[0]).To(HaveLen(1))
		var o auth.Options
		o.Apply(opts[0]...)
		g.Expect(o.CredentialAudiences).To(ConsistOf("audience1", "audience2"))
	})
}
