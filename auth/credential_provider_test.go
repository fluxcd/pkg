/*
Copyright 2026 The Flux authors

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
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/crypto"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/generic"
	"github.com/fluxcd/pkg/auth/kubernetes"
)

func TestGetAccessToken_WithCredentialProvider(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	envClient, _ := newTestEnv(t, ctx)

	auth.EnableObjectLevelWorkloadIdentity()
	t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	credential := &crypto.Credential{
		Provider:  crypto.CredentialProviderKubernetes,
		Type:      crypto.CredentialTypeJWT,
		Audiences: []string{"audience"},
	}

	token, err := auth.GetCredential(ctx, generic.Provider{},
		auth.WithClient(envClient),
		auth.WithServiceAccountName(serviceAccount.Name),
		auth.WithServiceAccountNamespace(serviceAccount.Namespace),
		auth.WithCredential(credential),
		auth.WithCredentialProvider(kubernetes.Provider{}))
	g.Expect(err).NotTo(HaveOccurred())

	genericCredential, ok := token.(*generic.Credential)
	g.Expect(ok).To(BeTrue())

	jwtToken, _, err := jwt.NewParser().ParseUnverified(genericCredential.Token, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())
	sub, err := jwtToken.Claims.GetSubject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub).To(Equal("system:serviceaccount:default:tenant"))
	aud, err := jwtToken.Claims.GetAudience()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aud).To(ConsistOf("audience"))
}

func TestGetArtifactRegistryCredentials_WithCredentialProvider(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	envClient, _ := newTestEnv(t, ctx)

	auth.EnableObjectLevelWorkloadIdentity()
	t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	credential := &crypto.Credential{
		Provider:  crypto.CredentialProviderKubernetes,
		Type:      crypto.CredentialTypeJWT,
		Audiences: []string{"audience"},
	}

	creds, err := auth.GetArtifactRegistryCredentials(ctx, generic.Provider{},
		"registry.example.com/repo",
		auth.WithClient(envClient),
		auth.WithServiceAccountName(serviceAccount.Name),
		auth.WithServiceAccountNamespace(serviceAccount.Namespace),
		auth.WithCredential(credential),
		auth.WithCredentialProvider(kubernetes.Provider{}))
	g.Expect(err).NotTo(HaveOccurred())

	authConfig, err := creds.Authorization()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(authConfig.RegistryToken).NotTo(BeEmpty())

	jwtToken, _, err := jwt.NewParser().ParseUnverified(authConfig.RegistryToken, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())
	sub, err := jwtToken.Claims.GetSubject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub).To(Equal("system:serviceaccount:default:tenant"))
}
