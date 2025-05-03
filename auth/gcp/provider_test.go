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
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2/google/externalaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/gcp"
)

func TestProvider_NewDefaultToken_Options(t *testing.T) {
	g := NewWithT(t)

	impl := &mockImplementation{
		t:           t,
		argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
	}

	opts := []auth.Option{
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
	}

	provider := gcp.Provider{Implementation: impl}
	token, err := provider.NewDefaultToken(context.Background(), opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeNil())
}

func TestProvider_NewTokenForServiceAccount_Options(t *testing.T) {
	g := NewWithT(t)

	// Start GKE metadata server.
	lis, err := net.Listen("tcp", ":0")
	g.Expect(err).NotTo(HaveOccurred())
	gkeMetadataServer := &http.Server{
		Addr: lis.Addr().String(),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/computeMetadata/v1/project/project-id":
				fmt.Fprintf(w, "%s", "project-id")
			case "/computeMetadata/v1/instance/attributes/cluster-location":
				fmt.Fprintf(w, "%s", "cluster-location")
			case "/computeMetadata/v1/instance/attributes/cluster-name":
				fmt.Fprintf(w, "%s", "cluster-name")
			}
		}),
	}
	go func() {
		err := gkeMetadataServer.Serve(lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			g.Expect(err).NotTo(HaveOccurred())
		}
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := gkeMetadataServer.Shutdown(ctx)
		g.Expect(err).NotTo(HaveOccurred())
	})
	gceMetadataHost := strings.TrimPrefix(lis.Addr().String(), "http://")
	t.Setenv("GCE_METADATA_HOST", gceMetadataHost)

	for _, tt := range []struct {
		name          string
		conf          externalaccount.Config
		saAnnotations map[string]string
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
			saAnnotations: map[string]string{
				"iam.gke.io/gcp-service-account": "test-sa@project-id.iam.gserviceaccount.com",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:           t,
				argConfig:   tt.conf,
				argProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com"},
			}

			oidcToken := "oidc-token"
			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "test-ns",
					Annotations: tt.saAnnotations,
				},
			}
			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint("https://sts.example.com"),
			}

			provider := gcp.Provider{Implementation: impl}
			token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, serviceAccount, opts...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(token).NotTo(BeNil())
		})
	}
}
