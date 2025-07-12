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
	"github.com/go-logr/zapr"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fluxcd/pkg/runtime/secrets"
)

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
			errMsg: `secret 'default/auth-secret': key 'username' not found`,
		},
		{
			name: "missing password key",
			secret: testSecret(
				withName("auth-secret"),
				withData(map[string][]byte{
					secrets.KeyUsername: []byte("user"),
				}),
			),
			errMsg: `secret 'default/auth-secret': key 'password' not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ctx := context.Background()

			username, password, err := secrets.BasicAuthFromSecret(ctx, tt.secret)

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
