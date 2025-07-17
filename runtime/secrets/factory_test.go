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
				secrets.KeyTLSCert:       tlsCert,
				secrets.KeyTLSPrivateKey: tlsKey,
				secrets.KeyCACert:        caCert,
			},
			expectedType: corev1.SecretTypeTLS,
		},
		{
			name:       "TLS secret without CA",
			secretName: "tls-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCertKeyPair(tlsCert, tlsKey)},
			expectedData: map[string][]byte{
				secrets.KeyTLSCert:       tlsCert,
				secrets.KeyTLSPrivateKey: tlsKey,
			},
			expectedType: corev1.SecretTypeTLS,
		},
		{
			name:       "CA only secret",
			secretName: "ca-secret",
			namespace:  testNS,
			options:    []secrets.TLSSecretOption{secrets.WithCAData(caCert)},
			expectedData: map[string][]byte{
				secrets.KeyCACert: caCert,
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
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
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
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeBasicAuth))
				g.Expect(secret.StringData[secrets.KeyUsername]).To(Equal(tt.username))
				g.Expect(secret.StringData[secrets.KeyPassword]).To(Equal(tt.password))
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
				secrets.KeyAddress:  []byte("http://proxy.example.com:8080"),
				secrets.KeyUsername: []byte("user"),
				secrets.KeyPassword: []byte("pass"),
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
				secrets.KeyAddress: []byte("http://proxy.example.com:8080"),
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
				secrets.KeyAddress:  []byte("http://proxy.example.com:8080"),
				secrets.KeyUsername: []byte("user"),
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
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
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
			errMsg:     "bearerToken is required",
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
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
				g.Expect(secret.StringData[secrets.KeyBearerToken]).To(Equal(tt.token))
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
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
				g.Expect(secret.StringData[secrets.KeyToken]).To(Equal(tt.token))
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
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
				g.Expect(secret.StringData[corev1.DockerConfigJsonKey]).To(Equal(tt.wantJSON))
			}
		})
	}
}

func TestMakeGitHubAppSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		secretName     string
		namespace      string
		appID          string
		installationID string
		privateKey     string
		baseURL        string
		expectedData   map[string][]byte
		errMsg         string
	}{
		{
			name:           "github app secret with base URL",
			secretName:     "github-app-secret",
			namespace:      testNS,
			appID:          "123456",
			installationID: "7891011",
			privateKey:     githubAppPrivateKey,
			baseURL:        "https://github.enterprise.com",
			expectedData: map[string][]byte{
				secrets.KeyGitHubAppID:             []byte("123456"),
				secrets.KeyGitHubAppInstallationID: []byte("7891011"),
				secrets.KeyGitHubAppPrivateKey:     []byte(githubAppPrivateKey),
				secrets.KeyGitHubAppBaseURL:        []byte("https://github.enterprise.com"),
			},
		},
		{
			name:           "github app secret without base URL",
			secretName:     "github-app-secret",
			namespace:      testNS,
			appID:          "123456",
			installationID: "7891011",
			privateKey:     githubAppPrivateKey,
			baseURL:        "",
			expectedData: map[string][]byte{
				secrets.KeyGitHubAppID:             []byte("123456"),
				secrets.KeyGitHubAppInstallationID: []byte("7891011"),
				secrets.KeyGitHubAppPrivateKey:     []byte(githubAppPrivateKey),
			},
		},
		{
			name:           "empty app ID",
			secretName:     "github-app-secret",
			namespace:      testNS,
			appID:          "",
			installationID: "7891011",
			privateKey:     githubAppPrivateKey,
			errMsg:         "githubAppID is required",
		},
		{
			name:           "empty installation ID",
			secretName:     "github-app-secret",
			namespace:      testNS,
			appID:          "123456",
			installationID: "",
			privateKey:     githubAppPrivateKey,
			errMsg:         "githubAppInstallationID is required",
		},
		{
			name:           "empty private key",
			secretName:     "github-app-secret",
			namespace:      testNS,
			appID:          "123456",
			installationID: "7891011",
			privateKey:     "",
			errMsg:         "githubAppPrivateKey is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeGitHubAppSecret(tt.secretName, tt.namespace, tt.appID, tt.installationID, tt.privateKey, tt.baseURL)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
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

func TestMakeSSHSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		secretName   string
		namespace    string
		privateKey   string
		publicKey    string
		knownHosts   string
		password     string
		expectedData map[string][]byte
		errMsg       string
	}{
		{
			name:       "ssh secret with all fields",
			secretName: "ssh-secret",
			namespace:  testNS,
			privateKey: sshPrivateKey,
			publicKey:  sshPublicKey,
			knownHosts: sshKnownHosts,
			password:   "passphrase123",
			expectedData: map[string][]byte{
				secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
				secrets.KeySSHPublicKey:  []byte(sshPublicKey),
				secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
				secrets.KeyPassword:      []byte("passphrase123"),
			},
		},
		{
			name:       "ssh secret without optional fields",
			secretName: "ssh-secret",
			namespace:  testNS,
			privateKey: sshPrivateKey,
			publicKey:  "",
			knownHosts: sshKnownHosts,
			password:   "",
			expectedData: map[string][]byte{
				secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
				secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
			},
		},
		{
			name:       "ssh secret with publicKey only",
			secretName: "ssh-secret",
			namespace:  testNS,
			privateKey: sshPrivateKey,
			publicKey:  sshPublicKey,
			knownHosts: sshKnownHosts,
			password:   "",
			expectedData: map[string][]byte{
				secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
				secrets.KeySSHPublicKey:  []byte(sshPublicKey),
				secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
			},
		},
		{
			name:       "ssh secret with password only",
			secretName: "ssh-secret",
			namespace:  testNS,
			privateKey: sshPrivateKey,
			publicKey:  "",
			knownHosts: sshKnownHosts,
			password:   "secret-passphrase",
			expectedData: map[string][]byte{
				secrets.KeySSHPrivateKey: []byte(sshPrivateKey),
				secrets.KeySSHKnownHosts: []byte(sshKnownHosts),
				secrets.KeyPassword:      []byte("secret-passphrase"),
			},
		},
		{
			name:       "empty private key",
			secretName: "ssh-secret",
			namespace:  testNS,
			privateKey: "",
			publicKey:  sshPublicKey,
			knownHosts: sshKnownHosts,
			password:   "",
			errMsg:     "identity is required",
		},
		{
			name:       "empty known hosts",
			secretName: "ssh-secret",
			namespace:  testNS,
			privateKey: sshPrivateKey,
			publicKey:  sshPublicKey,
			knownHosts: "",
			password:   "",
			errMsg:     "known_hosts is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeSSHSecret(tt.secretName, tt.namespace, tt.privateKey, tt.publicKey, tt.knownHosts, tt.password)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
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

func TestMakeSOPSSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		secretName string
		namespace  string
		ageKeys    []string
		gpgKeys    []string
		errMsg     string
	}{
		{
			name:       "sops secret with age keys only",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    []string{"age1abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			gpgKeys:    nil,
		},
		{
			name:       "sops secret with gpg keys only",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    nil,
			gpgKeys:    []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFa..."},
		},
		{
			name:       "sops secret with both age and gpg keys",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    []string{"age1abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			gpgKeys:    []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFa..."},
		},
		{
			name:       "sops secret with multiple age keys",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    []string{"age1abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", "age1xyz9876543210xyz9876543210xyz9876543210xyz9876543210xyz9876543210"},
			gpgKeys:    nil,
		},
		{
			name:       "sops secret with multiple gpg keys",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    nil,
			gpgKeys:    []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFa...", "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v2\n\nmQENBFb..."},
		},
		{
			name:       "no keys provided",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    nil,
			gpgKeys:    nil,
			errMsg:     "at least one key must be provided",
		},
		{
			name:       "empty age key",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    []string{""},
			gpgKeys:    nil,
			errMsg:     "Age key cannot be empty",
		},
		{
			name:       "empty gpg key",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    nil,
			gpgKeys:    []string{""},
			errMsg:     "GPG key cannot be empty",
		},
		{
			name:       "empty age key in mixed keys",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    []string{"age1abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", ""},
			gpgKeys:    []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFa..."},
			errMsg:     "Age key cannot be empty",
		},
		{
			name:       "empty gpg key in mixed keys",
			secretName: "sops-secret",
			namespace:  testNS,
			ageKeys:    []string{"age1abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			gpgKeys:    []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFa...", ""},
			errMsg:     "GPG key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			secret, err := secrets.MakeSOPSSecret(tt.secretName, tt.namespace, tt.ageKeys, tt.gpgKeys)

			if tt.errMsg != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errMsg)))
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(secret).ToNot(BeNil())
				g.Expect(secret.Kind).To(Equal("Secret"))
				g.Expect(secret.APIVersion).To(Equal("v1"))
				g.Expect(secret.Name).To(Equal(tt.secretName))
				g.Expect(secret.Namespace).To(Equal(tt.namespace))
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))

				expectedKeyCount := len(tt.ageKeys) + len(tt.gpgKeys)
				g.Expect(secret.StringData).To(HaveLen(expectedKeyCount))

				for _, ageKey := range tt.ageKeys {
					found := false
					for key, value := range secret.StringData {
						if value == ageKey && key != "" {
							g.Expect(key).To(HaveSuffix(".agekey"))
							found = true
							break
						}
					}
					g.Expect(found).To(BeTrue(), "Age key not found in secret data")
				}

				for _, gpgKey := range tt.gpgKeys {
					found := false
					for key, value := range secret.StringData {
						if value == gpgKey && key != "" {
							g.Expect(key).To(HaveSuffix(".asc"))
							found = true
							break
						}
					}
					g.Expect(found).To(BeTrue(), "GPG key not found in secret data")
				}
			}
		})
	}
}
