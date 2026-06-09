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

package client

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	rc "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/meta"
)

func TestCanImpersonate(t *testing.T) {
	g := NewWithT(t)
	ns, err := testEnv.CreateNamespace(ctx, "can-impersonate")
	g.Expect(err).NotTo(HaveOccurred())
	testNamespace := ns.Name

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-sa",
			Namespace: testNamespace,
		},
	}
	g.Expect(testEnv.CreateAndWait(ctx, sa)).To(Succeed())

	tests := []struct {
		name                    string
		defaultServiceAccount   string
		serviceAccountName      string
		serviceAccountNamespace string
		want                    bool
	}{
		{
			name: "no service account configured",
			want: true,
		},
		{
			name:                    "default service account exists",
			defaultServiceAccount:   "existing-sa",
			serviceAccountNamespace: testNamespace,
			want:                    true,
		},
		{
			name:                    "default service account does not exist",
			defaultServiceAccount:   "missing-sa",
			serviceAccountNamespace: testNamespace,
			want:                    false,
		},
		{
			name:                    "specific service account exists",
			serviceAccountName:      "existing-sa",
			serviceAccountNamespace: testNamespace,
			want:                    true,
		},
		{
			name:                    "specific service account does not exist",
			serviceAccountName:      "missing-sa",
			serviceAccountNamespace: testNamespace,
			want:                    false,
		},
		{
			name:                    "specific overrides default and exists",
			defaultServiceAccount:   "default-sa",
			serviceAccountName:      "existing-sa",
			serviceAccountNamespace: testNamespace,
			want:                    true,
		},
		{
			name:                    "specific overrides default and does not exist",
			defaultServiceAccount:   "existing-sa",
			serviceAccountName:      "missing-sa",
			serviceAccountNamespace: testNamespace,
			want:                    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			imp := NewImpersonator(testEnv.Client,
				WithServiceAccount(tt.defaultServiceAccount, tt.serviceAccountName, tt.serviceAccountNamespace),
			)
			got := imp.CanImpersonate(ctx)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestSetImpersonationConfig(t *testing.T) {
	tests := []struct {
		name                    string
		defaultServiceAccount   string
		serviceAccountName      string
		serviceAccountNamespace string
		wantUsername            string
	}{
		{
			name:         "no service account configured",
			wantUsername: "",
		},
		{
			name:                    "default service account only",
			defaultServiceAccount:   "default",
			serviceAccountNamespace: "test-ns",
			wantUsername:            "system:serviceaccount:test-ns:default",
		},
		{
			name:                    "specific service account only",
			serviceAccountName:      "custom-sa",
			serviceAccountNamespace: "test-ns",
			wantUsername:            "system:serviceaccount:test-ns:custom-sa",
		},
		{
			name:                    "specific overrides default",
			defaultServiceAccount:   "default",
			serviceAccountName:      "custom-sa",
			serviceAccountNamespace: "test-ns",
			wantUsername:            "system:serviceaccount:test-ns:custom-sa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			imp := &Impersonator{
				defaultServiceAccount:   tt.defaultServiceAccount,
				serviceAccountName:      tt.serviceAccountName,
				serviceAccountNamespace: tt.serviceAccountNamespace,
			}
			restConfig := &rest.Config{}
			imp.setImpersonationConfig(restConfig)
			g.Expect(restConfig.Impersonate.UserName).To(Equal(tt.wantUsername))
		})
	}
}

func TestGetRESTConfigFromSecret(t *testing.T) {
	g := NewWithT(t)
	ns, err := testEnv.CreateNamespace(ctx, "get-restconfig")
	g.Expect(err).NotTo(HaveOccurred())
	testNamespace := ns.Name

	tests := []struct {
		name            string
		secretRef       *meta.KubeConfigReference
		secret          *corev1.Secret
		wantErrContains string
	}{
		{
			name: "reads from default 'value' key",
			secretRef: &meta.KubeConfigReference{
				SecretRef: &meta.SecretKeyReference{Name: "kc-value"},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kc-value",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{"value": kubeConfig},
			},
		},
		{
			name: "reads from 'value.yaml' key when 'value' is absent",
			secretRef: &meta.KubeConfigReference{
				SecretRef: &meta.SecretKeyReference{Name: "kc-valueyaml"},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kc-valueyaml",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{"value.yaml": kubeConfig},
			},
		},
		{
			name: "reads from user-defined key",
			secretRef: &meta.KubeConfigReference{
				SecretRef: &meta.SecretKeyReference{
					Name: "kc-custom",
					Key:  "custom-key",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kc-custom",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{"custom-key": kubeConfig},
			},
		},
		{
			name: "error when user-defined key is missing from secret",
			secretRef: &meta.KubeConfigReference{
				SecretRef: &meta.SecretKeyReference{
					Name: "kc-missingkey",
					Key:  "missing-key",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kc-missingkey",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{"value": kubeConfig},
			},
			wantErrContains: "does not contain a 'missing-key' key",
		},
		{
			name: "error when secret is not found",
			secretRef: &meta.KubeConfigReference{
				SecretRef: &meta.SecretKeyReference{Name: "missing-secret"},
			},
			wantErrContains: "unable to read KubeConfig secret",
		},
		{
			name: "error when no default key present in secret",
			secretRef: &meta.KubeConfigReference{
				SecretRef: &meta.SecretKeyReference{Name: "kc-nodefault"},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kc-nodefault",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{"unrelated": kubeConfig},
			},
			wantErrContains: "does not contain a 'value' key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if tt.secret != nil {
				g.Expect(testEnv.CreateAndWait(ctx, tt.secret)).To(Succeed())
			}

			imp := &Impersonator{
				client:              testEnv.Client,
				kubeConfigRef:       tt.secretRef,
				kubeConfigNamespace: testNamespace,
			}

			cfg, err := imp.getRESTConfigFromSecret(context.Background())
			if tt.wantErrContains != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErrContains))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cfg.Host).NotTo(BeEmpty())
		})
	}
}

func TestGetClient_KubeConfigFromSecret(t *testing.T) {
	g := NewWithT(t)
	ns, err := testEnv.CreateNamespace(ctx, "getclient-kc")
	g.Expect(err).NotTo(HaveOccurred())
	testNamespace := ns.Name

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"value.yaml": kubeConfig,
		},
	}
	g.Expect(testEnv.CreateAndWait(ctx, secret)).To(Succeed())

	t.Run("returns a working client from kubeconfig secret", func(t *testing.T) {
		g := NewWithT(t)

		imp := NewImpersonator(testEnv.Client,
			WithKubeConfig(
				&meta.KubeConfigReference{
					SecretRef: &meta.SecretKeyReference{Name: "kubeconfig"},
				},
				KubeConfigOptions{},
				testNamespace,
				nil,
			),
		)

		client, poller, err := imp.GetClient(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(client).NotTo(BeNil())
		g.Expect(poller).NotTo(BeNil())

		// Verify the client can talk to the API server.
		nsList := &corev1.NamespaceList{}
		g.Expect(client.List(ctx, nsList)).To(Succeed())
		g.Expect(nsList.Items).NotTo(BeEmpty())
	})

	t.Run("returns a working client with impersonation", func(t *testing.T) {
		g := NewWithT(t)

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-sa",
				Namespace: testNamespace,
			},
		}
		g.Expect(testEnv.CreateAndWait(ctx, sa)).To(Succeed())

		imp := NewImpersonator(testEnv.Client,
			WithKubeConfig(
				&meta.KubeConfigReference{
					SecretRef: &meta.SecretKeyReference{Name: "kubeconfig"},
				},
				KubeConfigOptions{},
				testNamespace,
				nil,
			),
			WithServiceAccount("", "test-sa", testNamespace),
		)

		client, poller, err := imp.GetClient(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(client).NotTo(BeNil())
		g.Expect(poller).NotTo(BeNil())
	})
}

func TestGetClient_InvalidKubeConfigReference(t *testing.T) {
	g := NewWithT(t)

	imp := NewImpersonator(testEnv.Client,
		WithKubeConfig(&meta.KubeConfigReference{}, KubeConfigOptions{}, "test-ns", nil),
	)

	_, _, err := imp.GetClient(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("neither .spec.kubeConfig.provider nor .spec.kubeConfig.secretRef is set"))
}

func TestGetClient_ProviderError(t *testing.T) {
	g := NewWithT(t)

	providerCalled := false
	provider := func(_ context.Context, _ meta.KubeConfigReference, _ string, _ rc.Client) (*rest.Config, error) {
		providerCalled = true
		return nil, fmt.Errorf("provider error for testing")
	}

	imp := NewImpersonator(testEnv.Client,
		WithKubeConfig(
			&meta.KubeConfigReference{ConfigMapRef: &meta.LocalObjectReference{Name: "test"}},
			KubeConfigOptions{}, "test-ns", provider,
		),
	)

	_, _, err := imp.GetClient(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(providerCalled).To(BeTrue(), "provider function should have been called")
	g.Expect(err.Error()).To(ContainSubstring("provider error for testing"))
}

func TestGetClient_ProviderSuccess(t *testing.T) {
	g := NewWithT(t)

	provider := func(_ context.Context, _ meta.KubeConfigReference, _ string, _ rc.Client) (*rest.Config, error) {
		return rest.CopyConfig(testEnv.Config), nil
	}

	imp := NewImpersonator(testEnv.Client,
		WithKubeConfig(
			&meta.KubeConfigReference{ConfigMapRef: &meta.LocalObjectReference{Name: "test"}},
			KubeConfigOptions{},
			"default",
			provider,
		),
	)

	client, poller, err := imp.GetClient(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
	g.Expect(poller).NotTo(BeNil())

	// Verify the client can talk to the API server.
	nsList := &corev1.NamespaceList{}
	g.Expect(client.List(ctx, nsList)).To(Succeed())
	g.Expect(nsList.Items).NotTo(BeEmpty())
}

func TestNewImpersonator(t *testing.T) {
	t.Run("applies service account options", func(t *testing.T) {
		g := NewWithT(t)
		imp := NewImpersonator(testEnv.Client,
			WithServiceAccount("default-sa", "custom-sa", "test-ns"),
		)
		g.Expect(imp.defaultServiceAccount).To(Equal("default-sa"))
		g.Expect(imp.serviceAccountName).To(Equal("custom-sa"))
		g.Expect(imp.serviceAccountNamespace).To(Equal("test-ns"))
	})

	t.Run("applies kubeconfig options", func(t *testing.T) {
		g := NewWithT(t)
		ref := &meta.KubeConfigReference{
			SecretRef: &meta.SecretKeyReference{Name: "test"},
		}
		opts := KubeConfigOptions{UserAgent: "test-agent"}
		imp := NewImpersonator(testEnv.Client,
			WithKubeConfig(ref, opts, "config-ns", nil),
		)
		g.Expect(imp.kubeConfigRef).To(Equal(ref))
		g.Expect(imp.kubeConfigOpts).To(Equal(opts))
		g.Expect(imp.kubeConfigNamespace).To(Equal("config-ns"))
	})
}
