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

package secrets_test

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fluxcd/pkg/runtime/secrets"
)

func TestTLSConfigFromSecretRef(t *testing.T) {
	t.Parallel()

	caCert, tlsCert, tlsKey := generateTestCertificates(t)

	tests := []struct {
		name               string
		secretRef          types.NamespacedName
		secret             *corev1.Secret // Secret to add to fake client (nil = not added)
		targetURL          string
		expectedServerName string
		errMsg             string
	}{
		{
			name:      "integration test - basic TLS secret functionality",
			secretRef: types.NamespacedName{Name: "tls-secret", Namespace: testNS},
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSCert:       tlsCert,
					secrets.KeyTLSPrivateKey: tlsKey,
					secrets.KeyCACert:        caCert,
				}),
			),
		},
		{
			name:      "secret not found",
			secretRef: types.NamespacedName{Name: "missing-secret", Namespace: testNS},
			errMsg:    "secret 'default/missing-secret' not found",
		},
		{
			name:      "TLS secret with parameters",
			secretRef: types.NamespacedName{Name: "tls-secret", Namespace: testNS},
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyCACert: caCert,
				}),
			),
			targetURL:          "https://example.com",
			expectedServerName: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()
			// Use discard logger for integration tests
			ctx = log.IntoContext(ctx, logr.Discard())

			var objects []client.Object
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}
			c := fakeClient(objects...)

			tlsConfig, err := secrets.TLSConfigFromSecretRef(ctx, c, tt.secretRef, tt.targetURL)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig).ToNot(BeNil())

				g.Expect(tlsConfig.ServerName).To(Equal(tt.expectedServerName))
				// InsecureSkipVerify must always be false per Flux security policy.
				// The insecure parameter was removed to prevent bypassing certificate validation.
				g.Expect(tlsConfig.InsecureSkipVerify).To(BeFalse())

				hasCert := len(tt.secret.Data[secrets.KeyTLSCert]) > 0 || len(tt.secret.Data[secrets.LegacyKeyTLSCert]) > 0
				hasKey := len(tt.secret.Data[secrets.KeyTLSPrivateKey]) > 0 || len(tt.secret.Data[secrets.LegacyKeyTLSPrivateKey]) > 0
				hasCertPair := hasCert && hasKey

				if hasCertPair {
					g.Expect(tlsConfig.Certificates).To(HaveLen(1))
					expectedCert, err := tls.X509KeyPair(tlsCert, tlsKey)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(tlsConfig.Certificates[0]).To(Equal(expectedCert))
				} else {
					g.Expect(tlsConfig.Certificates).To(BeEmpty())
				}

				hasCA := len(tt.secret.Data[secrets.KeyCACert]) > 0 || len(tt.secret.Data[secrets.LegacyKeyCACert]) > 0
				if hasCA {
					g.Expect(tlsConfig.RootCAs).ToNot(BeNil())
				}

			}
		})
	}
}

func TestProxyURLFromSecretRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		secretRef types.NamespacedName
		secret    *corev1.Secret // Secret to add to fake client (nil = not added)
		wantURL   string
		errMsg    string
	}{
		{
			name:      "integration test - basic proxy functionality",
			secretRef: types.NamespacedName{Name: "proxy-secret", Namespace: testNS},
			secret: testSecret(
				withName("proxy-secret"),
				withData(map[string][]byte{
					secrets.KeyAddress:  []byte("http://proxy.example.com:8080"),
					secrets.KeyUsername: []byte("user"),
					secrets.KeyPassword: []byte("pass"),
				}),
			),
			wantURL: "http://user:pass@proxy.example.com:8080",
		},
		{
			name:      "secret not found",
			secretRef: types.NamespacedName{Name: "missing-secret", Namespace: testNS},
			errMsg:    "secret 'default/missing-secret' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			var objects []client.Object
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}
			c := fakeClient(objects...)

			proxyURL, err := secrets.ProxyURLFromSecretRef(ctx, c, tt.secretRef)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(proxyURL.String()).To(Equal(tt.wantURL))
			}
		})
	}
}

func TestPullSecretsFromServiceAccountRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		saRef          types.NamespacedName
		serviceAccount *corev1.ServiceAccount
		secrets        []*corev1.Secret
		wantSecrets    []string
		errMsg         string
	}{
		{
			name:  "service account with multiple pull secrets",
			saRef: types.NamespacedName{Name: "test-sa", Namespace: testNS},
			serviceAccount: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: testNS,
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "registry-secret-1"},
					{Name: "registry-secret-2"},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-secret-1",
						Namespace: testNS,
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry1.com":{"auth":"dGVzdA=="}}}`),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-secret-2",
						Namespace: testNS,
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry2.com":{"auth":"dGVzdA=="}}}`),
					},
				},
			},
			wantSecrets: []string{"registry-secret-1", "registry-secret-2"},
		},
		{
			name:  "service account with single pull secret",
			saRef: types.NamespacedName{Name: "test-sa", Namespace: testNS},
			serviceAccount: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: testNS,
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "registry-secret"},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "registry-secret",
						Namespace: testNS,
					},
					Type: corev1.SecretTypeDockerConfigJson,
				},
			},
			wantSecrets: []string{"registry-secret"},
		},
		{
			name:  "service account with no pull secrets",
			saRef: types.NamespacedName{Name: "test-sa", Namespace: testNS},
			serviceAccount: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: testNS,
				},
				ImagePullSecrets: []corev1.LocalObjectReference{},
			},
			secrets:     []*corev1.Secret{},
			wantSecrets: []string{},
		},
		{
			name:    "service account not found",
			saRef:   types.NamespacedName{Name: "missing-sa", Namespace: testNS},
			secrets: []*corev1.Secret{},
			errMsg:  "serviceaccount 'default/missing-sa' not found",
		},
		{
			name:  "referenced secret not found",
			saRef: types.NamespacedName{Name: "test-sa", Namespace: testNS},
			serviceAccount: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: testNS,
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "missing-secret"},
				},
			},
			secrets: []*corev1.Secret{},
			errMsg:  "failed to get image pull secret for serviceaccount 'default/test-sa': secret 'default/missing-secret' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			objects := make([]client.Object, 0, 1+len(tt.secrets))
			if tt.serviceAccount != nil {
				objects = append(objects, tt.serviceAccount)
			}
			for _, secret := range tt.secrets {
				objects = append(objects, secret)
			}

			c := fakeClient(objects...)

			secretList, err := secrets.PullSecretsFromServiceAccountRef(ctx, c, tt.saRef)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secretList).To(HaveLen(len(tt.wantSecrets)))

				secretNames := make([]string, len(secretList))
				for i, secret := range secretList {
					secretNames[i] = secret.Name
				}
				g.Expect(secretNames).To(ConsistOf(tt.wantSecrets))
			}
		})
	}
}
