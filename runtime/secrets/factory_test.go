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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/runtime/secrets"
)

func TestMakeTLSSecret(t *testing.T) {
	t.Parallel()

	caCert, tlsCert, tlsKey := generateTestCertificates(t)

	tests := []struct {
		name         string
		secretName   string
		namespace    string
		options      []secrets.TLSSecretOption
		expectedData map[string][]byte
		expectedType corev1.SecretType
		errMsg       string
	}{
		{
			name:       "complete TLS secret",
			secretName: "tls-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCertKeyPair(tlsCert, tlsKey), secrets.WithCAData(caCert)},
			expectedData: map[string][]byte{
				secrets.TLSCertKey:       tlsCert,
				secrets.TLSPrivateKeyKey: tlsKey,
				secrets.CACertKey:        caCert,
			},
			expectedType: corev1.SecretTypeTLS,
		},
		{
			name:       "TLS secret without CA",
			secretName: "tls-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCertKeyPair(tlsCert, tlsKey)},
			expectedData: map[string][]byte{
				secrets.TLSCertKey:       tlsCert,
				secrets.TLSPrivateKeyKey: tlsKey,
			},
			expectedType: corev1.SecretTypeTLS,
		},
		{
			name:       "CA only secret",
			secretName: "ca-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCAData(caCert)},
			expectedData: map[string][]byte{
				secrets.CACertKey: caCert,
			},
			expectedType: corev1.SecretTypeOpaque,
		},
		{
			name:       "invalid certificate and key pair",
			secretName: "tls-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCertKeyPair([]byte("invalid-cert"), []byte("invalid-key"))},
			errMsg:     "invalid TLS certificate and key pair",
		},
		{
			name:       "no options provided",
			secretName: "empty-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{},
			errMsg:     "no CA certificate or client certificate pair found",
		},
		{
			name:       "cert without key",
			secretName: "invalid-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCertKeyPair(tlsCert, nil)},
			errMsg:     "found certificate but missing private key",
		},
		{
			name:       "key without cert",
			secretName: "invalid-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCertKeyPair(nil, tlsKey)},
			errMsg:     "found private key but missing certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeTLSSecret(tt.secretName, tt.namespace, tt.options...)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(tt.expectedType))

				expectedStringData := make(map[string]string)
				for key, value := range tt.expectedData {
					expectedStringData[key] = string(value)
				}
				g.Expect(secret.StringData).To(Equal(expectedStringData))
			}
		})
	}
}

func TestMakeBasicAuthSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretName string
		namespace  string
		username   string
		password   string
		errMsg     string
	}{
		{
			name:       "basic auth secret",
			secretName: "auth-secret",
			namespace:  testNS,
			username:   "user",
			password:   "pass",
		},
		{
			name:       "empty username",
			secretName: "auth-secret",
			namespace:  testNS,
			username:   "",
			password:   "pass",
			errMsg:     "username is required",
		},
		{
			name:       "empty password",
			secretName: "auth-secret",
			namespace:  testNS,
			username:   "user",
			password:   "",
			errMsg:     "password is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeBasicAuthSecret(tt.secretName, tt.namespace, tt.username, tt.password)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeBasicAuth))
				g.Expect(secret.StringData[secrets.UsernameKey]).To(Equal(tt.username))
				g.Expect(secret.StringData[secrets.PasswordKey]).To(Equal(tt.password))
			}
		})
	}
}

func TestMakeProxySecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		secretName   string
		namespace    string
		address      string
		username     string
		password     string
		expectedData map[string][]byte
		errMsg       string
	}{
		{
			name:       "proxy with authentication",
			secretName: "proxy-secret",
			namespace:  testNS,
			address:    "http://proxy.example.com:8080",
			username:   "user",
			password:   "pass",
			expectedData: map[string][]byte{
				secrets.AddressKey:  []byte("http://proxy.example.com:8080"),
				secrets.UsernameKey: []byte("user"),
				secrets.PasswordKey: []byte("pass"),
			},
		},
		{
			name:       "proxy without authentication",
			secretName: "proxy-secret",
			namespace:  testNS,
			address:    "http://proxy.example.com:8080",
			username:   "",
			password:   "",
			expectedData: map[string][]byte{
				secrets.AddressKey: []byte("http://proxy.example.com:8080"),
			},
		},
		{
			name:       "proxy with username only",
			secretName: "proxy-secret",
			namespace:  testNS,
			address:    "http://proxy.example.com:8080",
			username:   "user",
			password:   "",
			expectedData: map[string][]byte{
				secrets.AddressKey:  []byte("http://proxy.example.com:8080"),
				secrets.UsernameKey: []byte("user"),
			},
		},
		{
			name:       "empty address",
			secretName: "proxy-secret",
			namespace:  testNS,
			address:    "",
			errMsg:     "address is required",
		},
		{
			name:       "invalid proxy address",
			secretName: "proxy-secret",
			namespace:  testNS,
			address:    "://invalid-url",
			errMsg:     "invalid proxy address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeProxySecret(tt.secretName, tt.namespace, tt.address, tt.username, tt.password)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))

				expectedStringData := make(map[string]string)
				for key, value := range tt.expectedData {
					expectedStringData[key] = string(value)
				}
				g.Expect(secret.StringData).To(Equal(expectedStringData))
			}
		})
	}
}

func TestMakeBearerTokenSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretName string
		namespace  string
		token      string
		errMsg     string
	}{
		{
			name:       "bearer token secret",
			secretName: "token-secret",
			namespace:  testNS,
			token:      "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
		},
		{
			name:       "empty token",
			secretName: "token-secret",
			namespace:  testNS,
			token:      "",
			errMsg:     "token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeBearerTokenSecret(tt.secretName, tt.namespace, tt.token)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
				g.Expect(secret.StringData[secrets.BearerTokenKey]).To(Equal(tt.token))
			}
		})
	}
}

func TestMakeTokenSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretName string
		namespace  string
		token      string
		errMsg     string
	}{
		{
			name:       "token secret",
			secretName: "api-token-secret",
			namespace:  testNS,
			token:      "xoxb-1234567890-1234567890-abcdef",
		},
		{
			name:       "github token",
			secretName: "github-token",
			namespace:  testNS,
			token:      "ghp_abcdefghijklmnopqrstuvwxyz1234567890",
		},
		{
			name:       "telegram bot token",
			secretName: "telegram-token",
			namespace:  testNS,
			token:      "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijk",
		},
		{
			name:       "token with special characters",
			secretName: "special-token",
			namespace:  testNS,
			token:      "abc.def-ghi_jkl+mno=pqr/stu",
		},
		{
			name:       "empty token",
			secretName: "empty-token",
			namespace:  testNS,
			token:      "",
			errMsg:     "token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeTokenSecret(tt.secretName, tt.namespace, tt.token)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
				g.Expect(secret.StringData[secrets.TokenKey]).To(Equal(tt.token))
			}
		})
	}
}

func TestMakeRegistrySecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretName string
		namespace  string
		server     string
		username   string
		password   string
		wantJSON   string
		errMsg     string
	}{
		{
			name:       "registry secret",
			secretName: "registry-secret",
			namespace:  testNS,
			server:     "registry.example.com",
			username:   "user",
			password:   "pass",
			wantJSON: `{
  "auths": {
    "registry.example.com": {
      "username": "user",
      "password": "pass",
      "auth": "dXNlcjpwYXNz"
    }
  }
}`,
		},
		{
			name:       "docker hub registry",
			secretName: "dockerhub-secret",
			namespace:  testNS,
			server:     "docker.io",
			username:   "dockeruser",
			password:   "dockerpass",
			wantJSON: `{
  "auths": {
    "docker.io": {
      "username": "dockeruser",
      "password": "dockerpass",
      "auth": "ZG9ja2VydXNlcjpkb2NrZXJwYXNz"
    }
  }
}`,
		},
		{
			name:       "registry with special characters in credentials",
			secretName: "special-secret",
			namespace:  testNS,
			server:     "special.registry.com",
			username:   `user"with"quotes`,
			password:   `pass"word\with"special'chars`,
			wantJSON: `{
  "auths": {
    "special.registry.com": {
      "username": "user\"with\"quotes",
      "password": "pass\"word\\with\"special'chars",
      "auth": "dXNlciJ3aXRoInF1b3RlczpwYXNzIndvcmRcd2l0aCJzcGVjaWFsJ2NoYXJz"
    }
  }
}`,
		},
		{
			name:       "empty server",
			secretName: "registry-secret",
			namespace:  testNS,
			server:     "",
			username:   "user",
			password:   "pass",
			errMsg:     "server is required",
		},
		{
			name:       "empty username",
			secretName: "registry-secret",
			namespace:  testNS,
			server:     "registry.example.com",
			username:   "",
			password:   "pass",
			errMsg:     "username is required",
		},
		{
			name:       "empty password",
			secretName: "registry-secret",
			namespace:  testNS,
			server:     "registry.example.com",
			username:   "user",
			password:   "",
			errMsg:     "password is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeRegistrySecret(tt.secretName, tt.namespace, tt.server, tt.username, tt.password)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
				g.Expect(secret.StringData[corev1.DockerConfigJsonKey]).To(Equal(tt.wantJSON))
			}
		})
	}
}
