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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fluxcd/pkg/runtime/secrets"
)

var (
	fixedNow = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	testNS   = "default"
)

func testSecret(name, namespace string, fops ...func(*corev1.Secret)) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, fop := range fops {
		fop(s)
	}
	return s
}

func fakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

func generateTestCertificates(t *testing.T) (caCert, serverCert, serverKey []byte) {
	t.Helper()

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate CA private key: %v", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test CA"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             fixedNow,
		NotAfter:              fixedNow.Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create CA certificate: %v", err)
	}

	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate server private key: %v", err)
	}

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization:  []string{"Test Server"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:     []string{"localhost"},
		NotBefore:    fixedNow,
		NotAfter:     fixedNow.Add(365 * 24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create server certificate: %v", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertDER,
	})

	serverCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertDER,
	})

	serverKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey),
	})

	return caCertPEM, serverCertPEM, serverKeyPEM
}

func TestTLSConfigFromSecret(t *testing.T) {
	t.Parallel()

	caCert, tlsCert, tlsKey := generateTestCertificates(t)

	tests := []struct {
		name    string
		secret  *corev1.Secret
		options []secrets.Option
		errMsg  string
	}{
		{
			name: "valid TLS secret with standard fields",
			secret: testSecret("tls-secret", testNS, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					secrets.TLSCertKey: tlsCert,
					secrets.TLSKeyKey:  tlsKey,
					secrets.CACertKey:  caCert,
				}
			}),
		},
		{
			name: "valid TLS secret without CA",
			secret: testSecret("tls-secret", testNS, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					secrets.TLSCertKey: tlsCert,
					secrets.TLSKeyKey:  tlsKey,
				}
			}),
		},
		{
			name: "valid TLS secret with deprecated fields",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.TLSCertFileKey: tlsCert,
					secrets.TLSKeyFileKey:  tlsKey,
					secrets.CACertFileKey:  caCert,
				},
			},
			options: []secrets.Option{secrets.WithDeprecatedFieldSupport()},
		},
		{
			name: "standard fields take precedence over deprecated",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.TLSCertKey:     tlsCert,
					secrets.TLSKeyKey:      tlsKey,
					secrets.CACertKey:      caCert,
					secrets.TLSCertFileKey: []byte("ignored"),
					secrets.TLSKeyFileKey:  []byte("ignored"),
					secrets.CACertFileKey:  []byte("ignored"),
				},
			},
			options: []secrets.Option{secrets.WithDeprecatedFieldSupport()},
		},
		{
			name: "secret not found",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "different-secret",
					Namespace: testNS,
				},
			},
			errMsg: "secret default/tls-secret not found",
		},
		{
			name: "missing certificate",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.TLSKeyKey: tlsKey,
				},
			},
			errMsg: "failed to get TLS certificate",
		},
		{
			name: "missing private key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.TLSCertKey: tlsCert,
				},
			},
			errMsg: "failed to get TLS private key",
		},
		{
			name: "deprecated fields without option",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.TLSCertFileKey: tlsCert,
					secrets.TLSKeyFileKey:  tlsKey,
				},
			},
			errMsg: `key "tls.crt" not found in secret`,
		},
		{
			name: "invalid certificate data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.TLSCertKey: []byte("invalid-cert-data"),
					secrets.TLSKeyKey:  []byte("invalid-key-data"),
				},
			},
			errMsg: "failed to parse TLS certificate and key",
		},
		{
			name: "invalid CA certificate",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.TLSCertKey: tlsCert,
					secrets.TLSKeyKey:  tlsKey,
					secrets.CACertKey:  []byte("invalid-ca-data"),
				},
			},
			errMsg: "failed to parse CA certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			client := fakeClient(tt.secret)

			tlsConfig, err := secrets.TLSConfigFromSecret(ctx, client, "tls-secret", testNS, tt.options...)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig).ToNot(BeNil())
				g.Expect(tlsConfig.Certificates).To(HaveLen(1))

				expectedCert, err := tls.X509KeyPair(tlsCert, tlsKey)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig.Certificates[0]).To(Equal(expectedCert))
			}
		})
	}
}

func TestProxyURLFromSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantURL string
		errMsg  string
	}{
		{
			name: "proxy with authentication",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.ProxyAddressKey: []byte("http://proxy.example.com:8080"),
					secrets.UsernameKey:     []byte("user"),
					secrets.PasswordKey:     []byte("pass"),
				},
			},
			wantURL: "http://user:pass@proxy.example.com:8080",
		},
		{
			name: "proxy with username only",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.ProxyAddressKey: []byte("http://proxy.example.com:8080"),
					secrets.UsernameKey:     []byte("user"),
				},
			},
			wantURL: "http://user@proxy.example.com:8080",
		},
		{
			name: "proxy without authentication",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.ProxyAddressKey: []byte("http://proxy.example.com:8080"),
				},
			},
			wantURL: "http://proxy.example.com:8080",
		},
		{
			name: "https proxy",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.ProxyAddressKey: []byte("https://secure-proxy.example.com:8443"),
				},
			},
			wantURL: "https://secure-proxy.example.com:8443",
		},
		{
			name: "secret not found",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "different-secret",
					Namespace: testNS,
				},
			},
			errMsg: "secret default/proxy-secret not found",
		},
		{
			name: "missing address key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.UsernameKey: []byte("user"),
					secrets.PasswordKey: []byte("pass"),
				},
			},
			errMsg: `key "address" not found in secret`,
		},
		{
			name: "empty address",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.ProxyAddressKey: []byte(""),
				},
			},
			errMsg: "proxy address is empty",
		},
		{
			name: "invalid URL",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.ProxyAddressKey: []byte("://invalid-url"),
				},
			},
			errMsg: "failed to parse proxy address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			client := fakeClient(tt.secret)

			proxyURL, err := secrets.ProxyURLFromSecret(ctx, client, "proxy-secret", testNS)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(proxyURL.String()).To(Equal(tt.wantURL))
			}
		})
	}
}

func TestPullSecretsFromServiceAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serviceAccount *corev1.ServiceAccount
		secrets        []*corev1.Secret
		wantSecrets    []string
		errMsg         string
	}{
		{
			name: "service account with multiple pull secrets",
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
			name: "service account with single pull secret",
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
			name: "service account with no pull secrets",
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
			name: "service account not found",
			serviceAccount: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "different-sa",
					Namespace: testNS,
				},
			},
			secrets: []*corev1.Secret{},
			errMsg:  "serviceaccount default/test-sa not found",
		},
		{
			name: "referenced secret not found",
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
			errMsg:  "secret default/missing-secret not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			objects := make([]client.Object, 0, 1+len(tt.secrets))
			objects = append(objects, tt.serviceAccount)
			for _, secret := range tt.secrets {
				objects = append(objects, secret)
			}

			client := fakeClient(objects...)

			secretList, err := secrets.PullSecretsFromServiceAccount(ctx, client, "test-sa", testNS)

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

func TestBasicAuthFromSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		secret       *corev1.Secret
		wantUsername string
		wantPassword string
		errMsg       string
	}{
		{
			name: "valid basic auth",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.UsernameKey: []byte("user"),
					secrets.PasswordKey: []byte("pass"),
				},
			},
			wantUsername: "user",
			wantPassword: "pass",
		},
		{
			name: "empty username and password",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.UsernameKey: []byte(""),
					secrets.PasswordKey: []byte(""),
				},
			},
			wantUsername: "",
			wantPassword: "",
		},
		{
			name: "special characters in credentials",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.UsernameKey: []byte("user@domain.com"),
					secrets.PasswordKey: []byte("p@ssw0rd!@#$%"),
				},
			},
			wantUsername: "user@domain.com",
			wantPassword: "p@ssw0rd!@#$%",
		},
		{
			name: "secret not found",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "different-secret",
					Namespace: testNS,
				},
			},
			errMsg: "secret default/auth-secret not found",
		},
		{
			name: "missing username key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.PasswordKey: []byte("pass"),
				},
			},
			errMsg: `key "username" not found in secret`,
		},
		{
			name: "missing password key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth-secret",
					Namespace: testNS,
				},
				Data: map[string][]byte{
					secrets.UsernameKey: []byte("user"),
				},
			},
			errMsg: `key "password" not found in secret`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			client := fakeClient(tt.secret)

			username, password, err := secrets.BasicAuthFromSecret(ctx, client, "auth-secret", testNS)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(username).To(Equal(tt.wantUsername))
				g.Expect(password).To(Equal(tt.wantPassword))
			}
		})
	}
}
