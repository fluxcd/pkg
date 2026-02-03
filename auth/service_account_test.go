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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
)

func TestCreate(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	envClient, _ := newTestEnv(t, ctx)

	// Create a service account.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-create-sa",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, sa)).NotTo(HaveOccurred())

	t.Run("success with audiences", func(t *testing.T) {
		g := NewWithT(t)

		token, err := auth.CreateServiceAccountToken(ctx, envClient, client.ObjectKey{
			Name:      "test-create-sa",
			Namespace: "default",
		}, "audience1", "audience2")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(token).NotTo(BeEmpty())

		// Verify the token is a valid JWT with the correct subject and audiences.
		jwtToken, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
		g.Expect(err).NotTo(HaveOccurred())
		sub, err := jwtToken.Claims.GetSubject()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sub).To(Equal("system:serviceaccount:default:test-create-sa"))
		aud, err := jwtToken.Claims.GetAudience()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aud).To(ConsistOf("audience1", "audience2"))
	})

	t.Run("success with nil audiences", func(t *testing.T) {
		g := NewWithT(t)

		token, err := auth.CreateServiceAccountToken(ctx, envClient, client.ObjectKey{
			Name:      "test-create-sa",
			Namespace: "default",
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(token).NotTo(BeEmpty())
	})

	t.Run("non-existent service account", func(t *testing.T) {
		g := NewWithT(t)

		_, err := auth.CreateServiceAccountToken(ctx, envClient, client.ObjectKey{
			Name:      "nonexistent-sa",
			Namespace: "default",
		}, "audience1")
		g.Expect(err).To(HaveOccurred())
	})
}
