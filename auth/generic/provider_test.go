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
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func TestProvider_NewTokenForServiceAccount(t *testing.T) {
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

	t.Run("valid token", func(t *testing.T) {
		g := NewWithT(t)

		token, err := generic.Provider{}.NewTokenForServiceAccount(context.Background(), validToken)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(token).To(Equal(&generic.Token{
			Token:     validToken,
			ExpiresAt: expiresAt,
		}))
	})

	t.Run("invalid token", func(t *testing.T) {
		g := NewWithT(t)

		token, err := generic.Provider{}.NewTokenForServiceAccount(context.Background(), "not-a-token")
		g.Expect(err).To(MatchError(ContainSubstring("failed to parse service account token")))
		g.Expect(token).To(BeNil())
	})
}

func TestProvider_GetAudiences(t *testing.T) {
	g := NewWithT(t)
	audiences, err := generic.Provider{}.GetAudiences(context.Background(),
		auth.WithCredentialAudiences("audience1", "audience2"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(audiences).To(ConsistOf("audience1", "audience2"))
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
	accessToken := &generic.Token{
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
				[]auth.Token{accessToken}, opts...)

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

func TestProvider_GetAccessTokenOptionsForCluster(t *testing.T) {
	t.Run("without audiences", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := generic.Provider{}.GetAccessTokenOptionsForCluster(
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
		opts, err := generic.Provider{}.GetAccessTokenOptionsForCluster(
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
