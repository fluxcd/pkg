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

package serviceaccounttoken_test

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/serviceaccounttoken"
	"github.com/fluxcd/pkg/auth/utils"
)

func TestProvider_NewControllerToken(t *testing.T) {
	t.Run("no client", func(t *testing.T) {
		g := NewWithT(t)
		token, err := serviceaccounttoken.Provider{}.NewControllerToken(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("client is required to create a controller token"))
		g.Expect(token).To(BeNil())
	})

	t.Run("with audiences", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		t.Cleanup(cancel)

		_, envClient, oidcClient := newTestEnv(t, ctx)

		// Create service account.
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controller",
				Namespace: "default",
			},
		}
		err := envClient.Create(ctx, serviceAccount)
		g.Expect(err).NotTo(HaveOccurred())

		// Create token.
		m := &mockImplementation{
			t: t,
			b: []byte("eyJhbGciOiJSUzI1NiIsImtpZCI6IkU2cUVmaVJ0QUY2OWhoNThZWU1QUmhPc1F1b1N5XzJuT1ZfRWF3TVRETlkifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiXSwiZXhwIjoxNzUyMjkwMDE1LCJpYXQiOjE3NTIyODY0MTUsImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwianRpIjoiMzEwMTgxZGItZDc3MC00MGE5LTg5MDEtN2M1NTQzOTBjZDhjIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJkZWZhdWx0Iiwic2VydmljZWFjY291bnQiOnsibmFtZSI6ImNvbnRyb2xsZXIiLCJ1aWQiOiJjMTUzNWEyNi01NDY5LTRmYzAtOGRiMi1kZWFhMGRlNDRmZjUifX0sIm5iZiI6MTc1MjI4NjQxNSwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6Y29udHJvbGxlciJ9.k-jt09bIwrGUNbSATEwaHHaaoym7NjcdStXcM0RYXZbL_PXCwP-TZPgBb2FzCq6V79E_q-NtZrY3RyvyAynUezXr6IPVkGne201uvOAjaibLvDxLzvbA5jWlZ0bHuLCfOxlC7GYSWjsglyH_ufulb6vxoMhY0rmiQzBbDHfB3EWM79-udcqLrxBsGgxjDnW4BXMIgSpuvipNA1GaMkpQb5AaY7Ns4zd0FftOimQmmvnwz8oDrGrCf2kmw91r0sAovva5B2BoJKlZwYGwO93zwTwK1qOMPLN2QHCUNBEY4K-QQlgz0oMUYR-YRpPJr7akjTQ6hm9zrTD90Tm0Jbqw7g\n"),
		}
		token, err := auth.GetAccessToken(ctx, serviceaccounttoken.Provider{m},
			auth.WithClient(envClient),
			auth.WithAudiences("audience1", "audience2"))
		g.Expect(err).NotTo(HaveOccurred())
		genericToken := token.(*serviceaccounttoken.Token)
		g.Expect(genericToken).NotTo(BeNil())

		// Validate token.
		jwtToken, _, err := jwt.NewParser().ParseUnverified(genericToken.Token, jwt.MapClaims{})
		g.Expect(err).NotTo(HaveOccurred())
		sub, err := jwtToken.Claims.GetSubject()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sub).To(Equal("system:serviceaccount:default:controller"))
		iss, err := jwtToken.Claims.GetIssuer()
		g.Expect(err).NotTo(HaveOccurred())
		ctx = oidc.ClientContext(ctx, oidcClient)
		jwks := oidc.NewRemoteKeySet(ctx, iss+"openid/v1/jwks")
		for _, aud := range []string{"audience1", "audience2"} {
			_, err = oidc.NewVerifier(iss, jwks, &oidc.Config{
				ClientID:             aud,
				SupportedSigningAlgs: []string{jwtToken.Method.Alg()},
			}).Verify(ctx, genericToken.Token)
			g.Expect(err).NotTo(HaveOccurred())
		}
		g.Expect(time.Until(genericToken.ExpiresAt)).To(BeNumerically("~", time.Hour, 10*time.Second))
	})
}

func TestProvider_NewTokenForServiceAccount(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	_, envClient, oidcClient := newTestEnv(t, ctx)

	auth.EnableObjectLevelWorkloadIdentity()
	t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)

	// Create service account.
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	}
	err := envClient.Create(ctx, serviceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Create token.
	token, err := auth.GetAccessToken(ctx, serviceaccounttoken.Provider{},
		auth.WithClient(envClient),
		auth.WithServiceAccountName(serviceAccount.Name),
		auth.WithServiceAccountNamespace(serviceAccount.Namespace),
		auth.WithAudiences("audience1", "audience2"))
	g.Expect(err).NotTo(HaveOccurred())
	genericToken := token.(*serviceaccounttoken.Token)
	g.Expect(genericToken).NotTo(BeNil())

	// Validate token.
	jwtToken, _, err := jwt.NewParser().ParseUnverified(genericToken.Token, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())
	sub, err := jwtToken.Claims.GetSubject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub).To(Equal("system:serviceaccount:default:tenant"))
	iss, err := jwtToken.Claims.GetIssuer()
	g.Expect(err).NotTo(HaveOccurred())
	ctx = oidc.ClientContext(ctx, oidcClient)
	jwks := oidc.NewRemoteKeySet(ctx, iss+"openid/v1/jwks")
	for _, aud := range []string{"audience1", "audience2"} {
		_, err = oidc.NewVerifier(iss, jwks, &oidc.Config{
			ClientID:             aud,
			SupportedSigningAlgs: []string{jwtToken.Method.Alg()},
		}).Verify(ctx, genericToken.Token)
		g.Expect(err).NotTo(HaveOccurred())
	}
	g.Expect(time.Until(genericToken.ExpiresAt)).To(BeNumerically("~", time.Hour, 10*time.Second))
}

func TestProvider_GetName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(serviceaccounttoken.Provider{}.GetName()).To(Equal(serviceaccounttoken.CredentialName))
}

func TestProvider_GetIdentity(t *testing.T) {
	g := NewWithT(t)
	id, err := serviceaccounttoken.Provider{}.GetIdentity(corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal("system:serviceaccount:default:tenant"))
}

func TestProvider_NewRESTConfig(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	_, envClient, oidcClient := newTestEnv(t, ctx)

	// Create service account.
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller",
			Namespace: "default",
		},
	}
	err := envClient.Create(ctx, serviceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Mock implementation.
	m := &mockImplementation{
		t: t,
		b: []byte("eyJhbGciOiJSUzI1NiIsImtpZCI6IkU2cUVmaVJ0QUY2OWhoNThZWU1QUmhPc1F1b1N5XzJuT1ZfRWF3TVRETlkifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiXSwiZXhwIjoxNzUyMjkwMDE1LCJpYXQiOjE3NTIyODY0MTUsImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwianRpIjoiMzEwMTgxZGItZDc3MC00MGE5LTg5MDEtN2M1NTQzOTBjZDhjIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJkZWZhdWx0Iiwic2VydmljZWFjY291bnQiOnsibmFtZSI6ImNvbnRyb2xsZXIiLCJ1aWQiOiJjMTUzNWEyNi01NDY5LTRmYzAtOGRiMi1kZWFhMGRlNDRmZjUifX0sIm5iZiI6MTc1MjI4NjQxNSwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6Y29udHJvbGxlciJ9.k-jt09bIwrGUNbSATEwaHHaaoym7NjcdStXcM0RYXZbL_PXCwP-TZPgBb2FzCq6V79E_q-NtZrY3RyvyAynUezXr6IPVkGne201uvOAjaibLvDxLzvbA5jWlZ0bHuLCfOxlC7GYSWjsglyH_ufulb6vxoMhY0rmiQzBbDHfB3EWM79-udcqLrxBsGgxjDnW4BXMIgSpuvipNA1GaMkpQb5AaY7Ns4zd0FftOimQmmvnwz8oDrGrCf2kmw91r0sAovva5B2BoJKlZwYGwO93zwTwK1qOMPLN2QHCUNBEY4K-QQlgz0oMUYR-YRpPJr7akjTQ6hm9zrTD90Tm0Jbqw7g\n"),
	}

	for _, tt := range []struct {
		name           string
		audiences      []string
		clusterAddress string
		err            string
	}{
		{
			name: "address is required",
			err:  "cluster address is required to create a REST config",
		},
		{
			name:           "with audiences",
			clusterAddress: "https://example.com",
			audiences:      []string{"audience1", "audience2"},
		},
		{
			name:           "without audiences",
			clusterAddress: "https://example.com",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := []auth.Option{
				auth.WithClient(envClient),
				auth.WithCAData("----- BEGIN CERTIFICATE-----"),
			}

			if len(tt.audiences) > 0 {
				opts = append(opts, auth.WithAudiences(tt.audiences...))
			}

			if tt.clusterAddress != "" {
				opts = append(opts, auth.WithClusterAddress(tt.clusterAddress))
			}

			conf, err := auth.GetRESTConfig(ctx, serviceaccounttoken.Provider{m}, opts...)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(conf).To(BeNil())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(conf).NotTo(BeNil())

			// Validate REST config.
			g.Expect(conf.Host).To(Equal("https://example.com:443"))
			g.Expect(conf.CAData).To(Equal([]byte("----- BEGIN CERTIFICATE-----")))
			g.Expect(time.Until(conf.ExpiresAt)).To(BeNumerically("~", time.Hour, 10*time.Second))

			// Validate token.
			jwtToken, _, err := jwt.NewParser().ParseUnverified(conf.BearerToken, jwt.MapClaims{})
			g.Expect(err).NotTo(HaveOccurred())
			sub, err := jwtToken.Claims.GetSubject()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(sub).To(Equal("system:serviceaccount:default:controller"))
			iss, err := jwtToken.Claims.GetIssuer()
			g.Expect(err).NotTo(HaveOccurred())
			ctx = oidc.ClientContext(ctx, oidcClient)
			jwks := oidc.NewRemoteKeySet(ctx, iss+"openid/v1/jwks")
			expectedAudiences := []string{"audience1", "audience2"}
			if len(tt.audiences) == 0 {
				expectedAudiences = []string{"https://example.com"}
			}
			for _, aud := range expectedAudiences {
				_, err = oidc.NewVerifier(iss, jwks, &oidc.Config{
					ClientID:             aud,
					SupportedSigningAlgs: []string{jwtToken.Method.Alg()},
				}).Verify(ctx, conf.BearerToken)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestProvider_NewRESTConfig_EndToEnd(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	envConfig, envClient, _ := newTestEnv(t, ctx)

	auth.EnableObjectLevelWorkloadIdentity()
	t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)

	// Create service account.
	const (
		namespace = "default"
		saName    = "tenant"
		cmName    = "kubeconfig"
	)
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).NotTo(HaveOccurred())

	// Create kubeconfig configmap.
	kubeconfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: namespace,
		},
		Data: map[string]string{
			meta.KubeConfigKeyProvider:           serviceaccounttoken.ProviderName,
			meta.KubeConfigKeyAddress:            envConfig.Host,
			meta.KubeConfigKeyCACert:             string(envConfig.CAData),
			meta.KubeConfigKeyServiceAccountName: saName,
		},
	}
	g.Expect(envClient.Create(ctx, kubeconfig)).NotTo(HaveOccurred())

	// Create the authenticated client.
	fetcher := utils.GetRESTConfigFetcher(
		auth.WithClient(envClient),
		auth.WithClusterAddress(envConfig.Host),
		auth.WithCAData(string(envConfig.CAData)))
	conf, err := fetcher(ctx, meta.KubeConfigReference{
		ConfigMapRef: &meta.LocalObjectReference{
			Name: cmName,
		},
	}, namespace, envClient)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conf).NotTo(BeNil())
	client, err := kubernetes.NewForConfig(conf)
	g.Expect(err).NotTo(HaveOccurred())

	// Test a permission that an authenticated ServiceAccount should have.
	version, err := client.Discovery().ServerVersion()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(version).NotTo(BeNil())

	// Test a permission that an authenticated ServiceAccount without any RBAC should NOT have.
	_, err = client.CoreV1().Namespaces().Get(ctx, "default", metav1.GetOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(`forbidden: User "system:serviceaccount:default:tenant" cannot get resource "namespaces"`))
}

func TestProvider_GetAccessTokenOptionsForCluster(t *testing.T) {
	t.Run("without audiences", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := serviceaccounttoken.Provider{}.GetAccessTokenOptionsForCluster(
			auth.WithClusterAddress("https://example.com"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).To(HaveLen(1))
		g.Expect(opts[0]).To(HaveLen(1))
		var o auth.Options
		o.Apply(opts[0]...)
		g.Expect(o.Audiences).To(ConsistOf("https://example.com"))
	})

	t.Run("with audiences", func(t *testing.T) {
		g := NewWithT(t)
		opts, err := serviceaccounttoken.Provider{}.GetAccessTokenOptionsForCluster(
			auth.WithClusterAddress("https://example.com"),
			auth.WithAudiences("audience1", "audience2"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).To(HaveLen(1))
		g.Expect(opts[0]).To(HaveLen(1))
		var o auth.Options
		o.Apply(opts[0]...)
		g.Expect(o.Audiences).To(ConsistOf("audience1", "audience2"))
	})
}

func TestProvider_GetAccessTokenOptionsForArtifactRepository(t *testing.T) {
	g := NewWithT(t)
	opts, err := serviceaccounttoken.Provider{}.GetAccessTokenOptionsForArtifactRepository("any-registry.io")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts).To(BeNil())
}

func TestProvider_ParseArtifactRepository(t *testing.T) {
	p := serviceaccounttoken.Provider{}
	for _, repo := range []string{
		"any-registry.io",
		"ghcr.io/owner/repo",
		"docker.io/library/nginx",
	} {
		t.Run(repo, func(t *testing.T) {
			g := NewWithT(t)
			parsed, err := p.ParseArtifactRepository(repo)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(parsed).To(Equal(serviceaccounttoken.CredentialName))
		})
	}
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	g := NewWithT(t)

	expiresAt := time.Now().Add(time.Hour)
	token := &serviceaccounttoken.Token{
		Token:     "test-token",
		ExpiresAt: expiresAt,
	}

	creds, err := serviceaccounttoken.Provider{}.NewArtifactRegistryCredentials(
		context.Background(), "any-registry.io", token)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(creds).NotTo(BeNil())
	g.Expect(creds.ExpiresAt).To(Equal(expiresAt))

	// Verify the authenticator returns the correct authorization header.
	authConfig, err := creds.Authenticator.Authorization()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(authConfig.RegistryToken).To(Equal("test-token"))
}
