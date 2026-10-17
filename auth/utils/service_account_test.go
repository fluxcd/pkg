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

package utils_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	crypto "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	authutils "github.com/fluxcd/pkg/auth/utils"
)

func TestServiceAccountOptions(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	newSA := func(name, namespace string, annotations map[string]string) *corev1.ServiceAccount {
		return &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   namespace,
				Annotations: annotations,
			},
		}
	}

	awsAnnotations := map[string]string{
		crypto.AnnotationCredentialProvider: crypto.CredentialProviderKubernetes,
		crypto.AnnotationCredentialType:     crypto.CredentialTypeJWT,
		crypto.AnnotationEKSRoleARN:         "arn:aws:iam::123456789012:role/role",
	}

	t.Run("no ServiceAccount returns nil options", func(t *testing.T) {
		g := NewWithT(t)
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		opts, err := authutils.ServiceAccountOptions(context.Background(), c,
			client.ObjectKey{Namespace: "default"}, "aws")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).To(BeNil())
	})

	t.Run("resolves explicit ServiceAccount and provider options", func(t *testing.T) {
		g := NewWithT(t)
		c := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(newSA("tenant", "default", awsAnnotations)).Build()

		opts, err := authutils.ServiceAccountOptions(context.Background(), c,
			client.ObjectKey{Name: "tenant", Namespace: "default"}, "aws",
			authutils.WithObjectLevelWorkloadIdentity(true))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(opts).NotTo(BeEmpty())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.ServiceAccount).NotTo(BeNil())
		g.Expect(o.ServiceAccount.Name).To(Equal("tenant"))
		g.Expect(o.AWSRoleARN).To(Equal("arn:aws:iam::123456789012:role/role"))
	})

	t.Run("falls back to the default ServiceAccount", func(t *testing.T) {
		g := NewWithT(t)
		c := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(newSA("default-sa", "flux-system", awsAnnotations)).Build()

		opts, err := authutils.ServiceAccountOptions(context.Background(), c,
			client.ObjectKey{Namespace: "flux-system"}, "aws",
			authutils.WithDefaultServiceAccount("default-sa"),
			authutils.WithObjectLevelWorkloadIdentity(true))
		g.Expect(err).NotTo(HaveOccurred())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.ServiceAccount).NotTo(BeNil())
		g.Expect(o.ServiceAccount.Name).To(Equal("default-sa"))
	})

	t.Run("errors when object-level workload identity is disabled", func(t *testing.T) {
		g := NewWithT(t)
		c := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(newSA("tenant", "default", awsAnnotations)).Build()

		opts, err := authutils.ServiceAccountOptions(context.Background(), c,
			client.ObjectKey{Name: "tenant", Namespace: "default"}, "aws")
		g.Expect(err).To(MatchError(auth.ErrObjectLevelWorkloadIdentityNotEnabled))
		g.Expect(opts).To(BeNil())
	})

	t.Run("errors when the ServiceAccount does not exist", func(t *testing.T) {
		g := NewWithT(t)
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		opts, err := authutils.ServiceAccountOptions(context.Background(), c,
			client.ObjectKey{Name: "missing", Namespace: "default"}, "aws",
			authutils.WithObjectLevelWorkloadIdentity(true))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get service account 'default/missing'"))
		g.Expect(opts).To(BeNil())
	})

	t.Run("passes crypto options", func(t *testing.T) {
		g := NewWithT(t)
		sa := newSA("tenant", "default", map[string]string{
			crypto.AnnotationCredentialProvider:          crypto.CredentialProviderKubernetes,
			crypto.AnnotationCredentialType:              crypto.CredentialTypeJWT,
			crypto.AnnotationGCPWorkloadIdentityProvider: "projects/1/locations/global/workloadIdentityPools/p/providers/x",
			crypto.AnnotationGCPServiceAccount:           "test-sa@test-project.iam.gserviceaccount.com",
		})
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sa).Build()

		opts, err := authutils.ServiceAccountOptions(context.Background(), c,
			client.ObjectKey{Name: "tenant", Namespace: "default"}, "gcp",
			authutils.WithObjectLevelWorkloadIdentity(true),
			authutils.WithCryptoOptions(crypto.WithOCI(true)))
		g.Expect(err).NotTo(HaveOccurred())

		var o auth.Options
		o.Apply(opts...)
		g.Expect(o.GCPServiceAccount).To(Equal("test-sa@test-project.iam.gserviceaccount.com"))
	})

	t.Run("passes crypto options to validation", func(t *testing.T) {
		g := NewWithT(t)
		c := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(newSA("tenant", "default", awsAnnotations)).Build()

		opts, err := authutils.ServiceAccountOptions(context.Background(), c,
			client.ObjectKey{Name: "tenant", Namespace: "default"}, "aws",
			authutils.WithObjectLevelWorkloadIdentity(true),
			authutils.WithCryptoOptions(crypto.WithKubeConfigCluster("nope")))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid kubeconfig ConfigMap data key cluster"))
		g.Expect(opts).To(BeNil())
	})
}
