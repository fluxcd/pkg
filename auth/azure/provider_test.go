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

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
)

func TestProvider_NewDefaultToken_Options(t *testing.T) {
	g := NewWithT(t)

	impl := &mockImplementation{
		t:           t,
		argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argScopes:   []string{"scope1", "scope2"},
	}

	opts := []auth.Option{
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
		auth.WithScopes("scope1", "scope2"),
	}

	provider := azure.Provider{Implementation: impl}
	token, err := provider.NewDefaultToken(context.Background(), opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeNil())
}

func TestProvider_NewTokenForServiceAccount_Options(t *testing.T) {
	g := NewWithT(t)

	impl := &mockImplementation{
		t:            t,
		argTenantID:  "tenant-id",
		argClientID:  "client-id",
		argOIDCToken: "oidc-token",
		argProxyURL:  &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argScopes:    []string{"scope1", "scope2"},
	}

	oidcToken := "oidc-token"
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"azure.workload.identity/tenant-id": "tenant-id",
				"azure.workload.identity/client-id": "client-id",
			},
		},
	}
	opts := []auth.Option{
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
		auth.WithScopes("scope1", "scope2"),
	}

	provider := azure.Provider{Implementation: impl}
	token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, serviceAccount, opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeNil())
}

func TestProvider_NewArtifactRegistryToken_Options(t *testing.T) {
	g := NewWithT(t)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	g.Expect(err).NotTo(HaveOccurred())
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(privateKey)
	g.Expect(err).NotTo(HaveOccurred())

	impl := &mockImplementation{
		t:           t,
		argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
		returnResp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"refresh_token":"%s"}`, refreshToken))),
		},
	}

	artifactRepository := "acr-repo"
	accessToken := &azure.Token{}
	opts := []auth.Option{
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
	}

	provider := azure.Provider{Implementation: impl}
	token, err := provider.NewArtifactRegistryToken(context.Background(), artifactRepository, accessToken, opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeNil())
}
