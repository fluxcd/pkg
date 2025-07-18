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
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fluxcd/pkg/runtime/secrets"
)

func TestAuthMethodsFromSecret(t *testing.T) {
	validCACert, _, _ := generateTestCertificates(t)

	tests := []struct {
		name       string
		secretData map[string][]byte
		wantBasic  bool
		wantBearer bool
		wantToken  bool
		wantSSH    bool
		wantTLS    bool
		wantErr    error
	}{
		{
			name:       "empty secret",
			secretData: map[string][]byte{},
			wantBasic:  false,
			wantBearer: false,
			wantToken:  false,
			wantSSH:    false,
			wantTLS:    false,
		},
		{
			name: "basic auth only",
			secretData: map[string][]byte{
				secrets.KeyUsername: []byte("testuser"),
				secrets.KeyPassword: []byte("testpass"),
			},
			wantBasic:  true,
			wantBearer: false,
			wantToken:  false,
			wantSSH:    false,
			wantTLS:    false,
		},
		{
			name: "bearer token only",
			secretData: map[string][]byte{
				secrets.KeyBearerToken: []byte("token123"),
			},
			wantBasic:  false,
			wantBearer: true,
			wantToken:  false,
			wantSSH:    false,
			wantTLS:    false,
		},
		{
			name: "SSH auth only",
			secretData: map[string][]byte{
				secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
				secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
			},
			wantBasic:  false,
			wantBearer: false,
			wantToken:  false,
			wantSSH:    true,
			wantTLS:    false,
		},
		{
			name: "TLS only",
			secretData: map[string][]byte{
				secrets.KeyCACert: validCACert,
			},
			wantBasic:  false,
			wantBearer: false,
			wantToken:  false,
			wantSSH:    false,
			wantTLS:    true,
		},
		{
			name: "basic auth + TLS",
			secretData: map[string][]byte{
				secrets.KeyUsername: []byte("testuser"),
				secrets.KeyPassword: []byte("testpass"),
				secrets.KeyCACert:   validCACert,
			},
			wantBasic:  true,
			wantBearer: false,
			wantToken:  false,
			wantSSH:    false,
			wantTLS:    true,
		},
		{
			name: "bearer token + TLS",
			secretData: map[string][]byte{
				secrets.KeyBearerToken: []byte("token123"),
				secrets.KeyCACert:      validCACert,
			},
			wantBasic:  false,
			wantBearer: true,
			wantToken:  false,
			wantSSH:    false,
			wantTLS:    true,
		},
		{
			name: "token only",
			secretData: map[string][]byte{
				secrets.KeyToken: []byte("api-token-123"),
			},
			wantBasic:  false,
			wantBearer: false,
			wantToken:  true,
			wantSSH:    false,
			wantTLS:    false,
		},
		{
			name: "bearer token + basic auth",
			secretData: map[string][]byte{
				secrets.KeyUsername:    []byte("testuser"),
				secrets.KeyPassword:    []byte("testpass"),
				secrets.KeyBearerToken: []byte("token123"),
			},
			wantBasic:  true,
			wantBearer: true,
			wantToken:  false,
			wantSSH:    false,
			wantTLS:    false,
		},
		{
			name: "all authentication methods",
			secretData: map[string][]byte{
				secrets.KeyUsername:      []byte("testuser"),
				secrets.KeyPassword:      []byte("testpass"),
				secrets.KeyBearerToken:   []byte("token123"),
				secrets.KeyToken:         []byte("api-token-123"),
				secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
				secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
				secrets.KeyCACert:        validCACert,
			},
			wantBasic:  true,
			wantBearer: true,
			wantToken:  true,
			wantSSH:    true,
			wantTLS:    true,
		},
		{
			name: "malformed SSH auth with valid TLS",
			secretData: map[string][]byte{
				secrets.KeySSHPrivateKey: []byte(sshPrivateKey), // known_hosts missing
				secrets.KeyCACert:        validCACert,
			},
			wantBasic:  false,
			wantBearer: false,
			wantSSH:    false, // SSH auth should fail
			wantTLS:    true,  // TLS should succeed
		},
		{
			name: "malformed basic auth - username only",
			secretData: map[string][]byte{
				secrets.KeyUsername: []byte("testuser"), // password missing
			},
			wantErr: fmt.Errorf("secret 'test-namespace/test-secret': malformed basic auth - has 'username' but missing 'password'"),
		},
		{
			name: "malformed basic auth - password only",
			secretData: map[string][]byte{
				secrets.KeyPassword: []byte("testpass"), // username missing
			},
			wantErr: fmt.Errorf("secret 'test-namespace/test-secret': malformed basic auth - has 'password' but missing 'username'"),
		},
		{
			name: "malformed basic auth with valid bearer token",
			secretData: map[string][]byte{
				secrets.KeyUsername:    []byte("testuser"), // password missing
				secrets.KeyBearerToken: []byte("token123"), // this should be ignored due to error
			},
			wantErr: fmt.Errorf("secret 'test-namespace/test-secret': malformed basic auth - has 'username' but missing 'password'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.TODO()

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
				},
				Data: tt.secretData,
			}

			result, err := secrets.AuthMethodsFromSecret(ctx, secret)

			if tt.wantErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(result).To(BeNil())
				g.Expect(err.Error()).To(Equal(tt.wantErr.Error()))
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).ToNot(BeNil())

			g.Expect(result.HasBasicAuth()).To(Equal(tt.wantBasic))
			g.Expect(result.HasBearerAuth()).To(Equal(tt.wantBearer))
			g.Expect(result.HasTokenAuth()).To(Equal(tt.wantToken))
			g.Expect(result.HasSSH()).To(Equal(tt.wantSSH))
			g.Expect(result.HasTLS()).To(Equal(tt.wantTLS))

			if tt.wantBasic {
				g.Expect(result.Basic).ToNot(BeNil())
				g.Expect(result.Basic.Username).To(Equal("testuser"))
				g.Expect(result.Basic.Password).To(Equal("testpass"))
			}

			if tt.wantBearer {
				g.Expect(result.Bearer).ToNot(BeEmpty())
				g.Expect(string(result.Bearer)).To(Equal("token123"))
			}

			if tt.wantToken {
				g.Expect(result.Token).ToNot(BeEmpty())
				g.Expect(string(result.Token)).To(Equal("api-token-123"))
			}

			if tt.wantSSH {
				g.Expect(result.SSH).ToNot(BeNil())
				g.Expect(result.SSH.PrivateKey).ToNot(BeEmpty())
				g.Expect(result.SSH.KnownHosts).ToNot(BeEmpty())
			}

			if tt.wantTLS {
				g.Expect(result.TLS).ToNot(BeNil())
				g.Expect(result.TLS.RootCAs).ToNot(BeNil())
			}

		})
	}
}

func TestTLSConfigFromSecret(t *testing.T) {
	t.Parallel()

	caCert, tlsCert, tlsKey := generateTestCertificates(t)

	tests := []struct {
		name           string
		secret         *corev1.Secret
		errMsg         string
		expectedFields map[string]string // legacy key -> preferred key mapping
	}{
		{
			name: "valid TLS secret with standard fields",
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
			name: "valid TLS secret without CA",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSCert:       tlsCert,
					secrets.KeyTLSPrivateKey: tlsKey,
				}),
			),
		},
		{
			name: "valid TLS secret with legacy fields",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.LegacyKeyTLSCert:       tlsCert,
					secrets.LegacyKeyTLSPrivateKey: tlsKey,
					secrets.LegacyKeyCACert:        caCert,
				}),
			),
			expectedFields: map[string]string{
				"certFile": "tls.crt",
				"keyFile":  "tls.key",
				"caFile":   "ca.crt",
			},
		},
		{
			name: "only legacy CA field used",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSCert:       tlsCert,
					secrets.KeyTLSPrivateKey: tlsKey,
					secrets.LegacyKeyCACert:  caCert,
				}),
			),
			expectedFields: map[string]string{
				"caFile": "ca.crt",
			},
		},
		{
			name: "standard fields take precedence over legacy",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSCert:             tlsCert,
					secrets.KeyTLSPrivateKey:       tlsKey,
					secrets.KeyCACert:              caCert,
					secrets.LegacyKeyTLSCert:       []byte("ignored"),
					secrets.LegacyKeyTLSPrivateKey: []byte("ignored"),
					secrets.LegacyKeyCACert:        []byte("ignored"),
				}),
			),
		},
		{
			name: "invalid certificate data",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSCert:       []byte("invalid-cert-data"),
					secrets.KeyTLSPrivateKey: []byte("invalid-key-data"),
				}),
			),
			errMsg: "failed to parse TLS certificate and key",
		},
		{
			name: "invalid CA certificate",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSCert:       tlsCert,
					secrets.KeyTLSPrivateKey: tlsKey,
					secrets.KeyCACert:        []byte("invalid-ca-data"),
				}),
			),
			errMsg: "failed to parse CA certificate",
		},
		{
			name: "CA certificate only",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyCACert: caCert,
				}),
			),
		},
		{
			name: "certificate without key",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSCert: tlsCert,
				}),
			),
			errMsg: "secret 'default/tls-secret' contains 'tls.crt' but missing 'tls.key'",
		},
		{
			name: "key without certificate",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{
					secrets.KeyTLSPrivateKey: tlsKey,
				}),
			),
			errMsg: "secret 'default/tls-secret' contains 'tls.key' but missing 'tls.crt'",
		},
		{
			name: "no certificates at all",
			secret: testSecret(
				withName("tls-secret"),
				withData(map[string][]byte{}),
			),
			errMsg: "secret 'default/tls-secret' must contain either 'ca.crt' or both 'tls.crt' and 'tls.key'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()
			var observedLogs *observer.ObservedLogs

			if tt.expectedFields != nil {
				// Use observer logger for tests that expect logging
				observedZapCore, logs := observer.New(zap.InfoLevel)
				zapLogger := zap.New(observedZapCore)
				logger := zapr.NewLogger(zapLogger)
				ctx = log.IntoContext(ctx, logger)
				observedLogs = logs
			} else {
				// Use discard logger for tests that don't expect logging
				ctx = log.IntoContext(ctx, logr.Discard())
			}

			tlsConfig, err := secrets.TLSConfigFromSecret(ctx, tt.secret)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(tlsConfig).ToNot(BeNil())

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

				// Verify logging behavior if expected
				if tt.expectedFields != nil {
					logs := observedLogs.All()
					g.Expect(logs).To(HaveLen(len(tt.expectedFields)))

					loggedFields := make(map[string]string)
					for _, logEntry := range logs {
						g.Expect(logEntry.Message).To(Equal("using legacy key in secret data"))

						var field, preferred string
						for _, contextField := range logEntry.Context {
							switch contextField.Key {
							case "key":
								field = contextField.String
							case "preferred":
								preferred = contextField.String
							}
						}

						g.Expect(field).ToNot(BeEmpty(), "key should be present in log")
						g.Expect(preferred).ToNot(BeEmpty(), "preferred should be present in log")
						loggedFields[field] = preferred
					}

					g.Expect(loggedFields).To(Equal(tt.expectedFields))
				}
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
			name: "proxy with username only",
			secret: testSecret(
				withName("proxy-secret"),
				withData(map[string][]byte{
					secrets.KeyAddress:  []byte("http://proxy.example.com:8080"),
					secrets.KeyUsername: []byte("user"),
				}),
			),
			wantURL: "http://user@proxy.example.com:8080",
		},
		{
			name: "proxy without authentication",
			secret: testSecret(
				withName("proxy-secret"),
				withData(map[string][]byte{
					secrets.KeyAddress: []byte("http://proxy.example.com:8080"),
				}),
			),
			wantURL: "http://proxy.example.com:8080",
		},
		{
			name: "https proxy",
			secret: testSecret(
				withName("proxy-secret"),
				withData(map[string][]byte{
					secrets.KeyAddress: []byte("https://secure-proxy.example.com:8443"),
				}),
			),
			wantURL: "https://secure-proxy.example.com:8443",
		},
		{
			name: "missing address key",
			secret: testSecret(
				withName("proxy-secret"),
				withData(map[string][]byte{
					secrets.KeyUsername: []byte("user"),
					secrets.KeyPassword: []byte("pass"),
				}),
			),
			errMsg: `secret 'default/proxy-secret': key 'address' not found`,
		},
		{
			name: "empty address",
			secret: testSecret(
				withName("proxy-secret"),
				withData(map[string][]byte{
					secrets.KeyAddress: []byte(""),
				}),
			),
			errMsg: "secret 'default/proxy-secret': proxy address is empty",
		},
		{
			name: "invalid URL",
			secret: testSecret(
				withName("proxy-secret"),
				withData(map[string][]byte{
					secrets.KeyAddress: []byte("://invalid-url"),
				}),
			),
			errMsg: "secret 'default/proxy-secret': failed to parse proxy address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			proxyURL, err := secrets.ProxyURLFromSecret(ctx, tt.secret)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(proxyURL.String()).To(Equal(tt.wantURL))
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
			secret: testSecret(
				withName("auth-secret"),
				withData(map[string][]byte{
					secrets.KeyUsername: []byte("user"),
					secrets.KeyPassword: []byte("pass"),
				}),
			),
			wantUsername: "user",
			wantPassword: "pass",
		},
		{
			name: "empty username and password",
			secret: testSecret(
				withName("auth-secret"),
				withData(map[string][]byte{
					secrets.KeyUsername: []byte(""),
					secrets.KeyPassword: []byte(""),
				}),
			),
			wantUsername: "",
			wantPassword: "",
		},
		{
			name: "special characters in credentials",
			secret: testSecret(
				withName("auth-secret"),
				withData(map[string][]byte{
					secrets.KeyUsername: []byte("user@domain.com"),
					secrets.KeyPassword: []byte("p@ssw0rd!@#$%"),
				}),
			),
			wantUsername: "user@domain.com",
			wantPassword: "p@ssw0rd!@#$%",
		},
		{
			name: "missing username key",
			secret: testSecret(
				withName("auth-secret"),
				withData(map[string][]byte{
					secrets.KeyPassword: []byte("pass"),
				}),
			),
			errMsg: `secret 'default/auth-secret': malformed basic auth - has 'password' but missing 'username'`,
		},
		{
			name: "missing password key",
			secret: testSecret(
				withName("auth-secret"),
				withData(map[string][]byte{
					secrets.KeyUsername: []byte("user"),
				}),
			),
			errMsg: `secret 'default/auth-secret': malformed basic auth - has 'username' but missing 'password'`,
		},
		{
			name: "completely empty secret",
			secret: testSecret(
				withName("empty-secret"),
				withData(map[string][]byte{}),
			),
			errMsg: `secret 'default/empty-secret': key 'username' not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			basicAuth, err := secrets.BasicAuthFromSecret(ctx, tt.secret)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(basicAuth.Username).To(Equal(tt.wantUsername))
				g.Expect(basicAuth.Password).To(Equal(tt.wantPassword))
			}
		})
	}
}

func TestBearerAuthFromSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		secret    *corev1.Secret
		wantToken string
		errMsg    string
	}{
		{
			name: "valid bearer token",
			secret: testSecret(
				withName("bearer-secret"),
				withData(map[string][]byte{
					secrets.KeyBearerToken: []byte("token123"),
				}),
			),
			wantToken: "token123",
		},
		{
			name: "empty bearer token",
			secret: testSecret(
				withName("bearer-secret"),
				withData(map[string][]byte{
					secrets.KeyBearerToken: []byte(""),
				}),
			),
			wantToken: "",
		},
		{
			name: "special characters in token",
			secret: testSecret(
				withName("bearer-secret"),
				withData(map[string][]byte{
					secrets.KeyBearerToken: []byte("ghp_1234567890abcdef"),
				}),
			),
			wantToken: "ghp_1234567890abcdef",
		},
		{
			name: "missing bearer token key",
			secret: testSecret(
				withName("bearer-secret"),
				withData(map[string][]byte{}),
			),
			errMsg: `secret 'default/bearer-secret': key 'bearerToken' not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			bearerAuth, err := secrets.BearerAuthFromSecret(ctx, tt.secret)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(bearerAuth)).To(Equal(tt.wantToken))
			}
		})
	}
}

func TestTokenAuthFromSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		secret    *corev1.Secret
		wantToken string
		errMsg    string
	}{
		{
			name: "valid token",
			secret: testSecret(
				withName("token-secret"),
				withData(map[string][]byte{
					secrets.KeyToken: []byte("api-token-123"),
				}),
			),
			wantToken: "api-token-123",
		},
		{
			name: "empty token",
			secret: testSecret(
				withName("token-secret"),
				withData(map[string][]byte{
					secrets.KeyToken: []byte(""),
				}),
			),
			wantToken: "",
		},
		{
			name: "missing token key",
			secret: testSecret(
				withName("token-secret"),
				withData(map[string][]byte{}),
			),
			errMsg: `secret 'default/token-secret': key 'token' not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			tokenAuth, err := secrets.TokenAuthFromSecret(ctx, tt.secret)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(tokenAuth)).To(Equal(tt.wantToken))
			}
		})
	}
}

func TestSSHAuthFromSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		secret         *corev1.Secret
		wantPrivateKey []byte
		wantKnownHosts string
		wantPublicKey  []byte
		wantPassword   string
		errMsg         string
	}{
		{
			name: "valid SSH auth with all fields",
			secret: testSecret(
				withName("ssh-secret"),
				withData(map[string][]byte{
					secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
					secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
					secrets.KeySSHPublicKey:  []byte(sshPublicKey),
					secrets.KeyPassword:      []byte("passphrase"),
				}),
			),
			wantPrivateKey: []byte(sshPrivateKey),
			wantKnownHosts: sshKnownHosts,
			wantPublicKey:  []byte(sshPublicKey),
			wantPassword:   "passphrase",
		},
		{
			name: "SSH auth with required fields only",
			secret: testSecret(
				withName("ssh-secret"),
				withData(map[string][]byte{
					secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
					secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
				}),
			),
			wantPrivateKey: []byte(sshPrivateKey),
			wantKnownHosts: sshKnownHosts,
		},
		{
			name: "missing private key",
			secret: testSecret(
				withName("ssh-secret"),
				withData(map[string][]byte{
					secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
				}),
			),
			errMsg: `secret 'default/ssh-secret': key 'identity' not found`,
		},
		{
			name: "missing known hosts",
			secret: testSecret(
				withName("ssh-secret"),
				withData(map[string][]byte{
					secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
				}),
			),
			errMsg: `secret 'default/ssh-secret': key 'known_hosts' not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			sshAuth, err := secrets.SSHAuthFromSecret(ctx, tt.secret)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(sshAuth.PrivateKey).To(Equal(tt.wantPrivateKey))
				g.Expect(sshAuth.KnownHosts).To(Equal(tt.wantKnownHosts))

				if tt.wantPublicKey != nil {
					g.Expect(sshAuth.PublicKey).To(Equal(tt.wantPublicKey))
				} else {
					g.Expect(sshAuth.PublicKey).To(BeNil())
				}

				if tt.wantPassword != "" {
					g.Expect(sshAuth.Password).To(Equal(tt.wantPassword))
				} else {
					g.Expect(sshAuth.Password).To(Equal(""))
				}
			}
		})
	}
}
