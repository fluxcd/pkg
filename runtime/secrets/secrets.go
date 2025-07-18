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

package secrets

import (
	"crypto/tls"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// KeyTLSCert is the standard key for TLS certificate data in secrets.
	KeyTLSCert = corev1.TLSCertKey
	// KeyTLSPrivateKey is the standard key for TLS private key data in secrets.
	KeyTLSPrivateKey = corev1.TLSPrivateKeyKey
	// KeyCACert is the standard key for CA certificate data in secrets.
	KeyCACert = "ca.crt"

	// LegacyKeyTLSCert is the legacy key for TLS certificate data in secrets.
	LegacyKeyTLSCert = "certFile"
	// LegacyKeyTLSPrivateKey is the legacy key for TLS private key data in secrets.
	LegacyKeyTLSPrivateKey = "keyFile"
	// LegacyKeyCACert is the legacy key for CA certificate data in secrets.
	LegacyKeyCACert = "caFile"

	// KeyUsername is the key for username data in basic auth secrets.
	KeyUsername = "username"
	// KeyPassword is the key for password data in basic auth secrets.
	KeyPassword = "password"

	// KeyAddress is the key for proxy address data in proxy secrets.
	KeyAddress = "address"

	// KeyBearerToken is the key for bearer token data in secrets.
	KeyBearerToken = "bearerToken"
	// KeyToken is the key for generic API token data in secrets.
	KeyToken = "token"

	// KeyGitHubAppID is the key for GitHub App ID data in secrets.
	KeyGitHubAppID = "githubAppID"
	// KeyGitHubAppInstallationID is the key for GitHub App installation ID data in secrets.
	KeyGitHubAppInstallationID = "githubAppInstallationID"
	// KeyGitHubAppPrivateKey is the key for GitHub App private key data in secrets.
	KeyGitHubAppPrivateKey = "githubAppPrivateKey"
	// KeyGitHubAppBaseURL is the key for GitHub App base URL data in secrets.
	KeyGitHubAppBaseURL = "githubAppBaseURL"

	// KeySSHPrivateKey is the key for SSH private key data in secrets.
	KeySSHPrivateKey = "identity"
	// KeySSHPublicKey is the key for SSH public key data in secrets.
	KeySSHPublicKey = "identity.pub"
	// KeySSHKnownHosts is the key for SSH known hosts data in secrets.
	KeySSHKnownHosts = "known_hosts"
)

// AuthMethods holds all available authentication methods detected from a secret.
type AuthMethods struct {
	Basic  *BasicAuth
	Bearer BearerAuth
	SSH    *SSHAuth
	TLS    *tls.Config
}

// HasBasicAuth returns true if basic authentication is available.
func (am *AuthMethods) HasBasicAuth() bool {
	return am.Basic != nil
}

// HasBearerAuth returns true if bearer token authentication is available.
func (am *AuthMethods) HasBearerAuth() bool {
	return am.Bearer != ""
}

// HasSSH returns true if SSH authentication is available.
func (am *AuthMethods) HasSSH() bool {
	return am.SSH != nil
}

// HasTLS returns true if TLS configuration is available.
func (am *AuthMethods) HasTLS() bool {
	return am.TLS != nil
}

// tlsCertificateData holds TLS certificate, key, and optional CA data
type tlsCertificateData struct {
	cert   []byte
	key    []byte
	caCert []byte
}

// newTLSCertificateData creates tlsCertificateData from a Kubernetes secret.
func newTLSCertificateData(secret *corev1.Secret, logger logr.Logger) (*tlsCertificateData, error) {
	data := &tlsCertificateData{
		cert:   getSecretData(secret, KeyTLSCert, LegacyKeyTLSCert, logger),
		key:    getSecretData(secret, KeyTLSPrivateKey, LegacyKeyTLSPrivateKey, logger),
		caCert: getSecretData(secret, KeyCACert, LegacyKeyCACert, logger),
	}

	if err := data.validate(); err != nil {
		return nil, err
	}

	return data, nil
}

func (t *tlsCertificateData) validate() error {
	hasCert := len(t.cert) > 0
	hasKey := len(t.key) > 0
	hasCA := len(t.caCert) > 0

	if hasCert != hasKey {
		if hasCert {
			return &TLSValidationError{Type: ErrMissingPrivateKey}
		}
		return &TLSValidationError{Type: ErrMissingCertificate}
	}

	if !hasCert && !hasCA {
		return &TLSValidationError{Type: ErrNoCertificatePairOrCA}
	}

	return nil
}

func (t *tlsCertificateData) hasCertPair() bool {
	return len(t.cert) > 0 && len(t.key) > 0
}

func (t *tlsCertificateData) hasCA() bool {
	return len(t.caCert) > 0
}

// validateCertificatePairIfPresent validates the certificate and key pair if both are present.
func (t *tlsCertificateData) validateCertificatePairIfPresent() error {
	if t.hasCertPair() {
		if _, err := tls.X509KeyPair(t.cert, t.key); err != nil {
			return fmt.Errorf("invalid TLS certificate and key pair: %w", err)
		}
	}
	return nil
}

// toSecret creates a Kubernetes secret from the certificate data.
func (t *tlsCertificateData) toSecret(name, namespace string) *corev1.Secret {
	secretData := make(map[string]string)
	var secretType corev1.SecretType

	if t.hasCertPair() {
		secretData[KeyTLSCert] = string(t.cert)
		secretData[KeyTLSPrivateKey] = string(t.key)
		secretType = corev1.SecretTypeTLS
	} else {
		secretType = corev1.SecretTypeOpaque
	}

	if t.hasCA() {
		secretData[KeyCACert] = string(t.caCert)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       secretType,
		StringData: secretData,
	}
	secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
	return secret
}

// BasicAuth holds basic authentication credentials.
type BasicAuth struct {
	Username string
	Password string
}

// BearerAuth holds bearer token authentication credentials.
type BearerAuth string

// SSHAuth holds SSH authentication credentials.
type SSHAuth struct {
	PrivateKey []byte
	PublicKey  []byte
	KnownHosts string
	Password   string
}

// GitHubAppAuth holds GitHub App authentication credentials.
type GitHubAppAuth struct {
	AppID          string
	InstallationID string
	PrivateKey     []byte
	BaseURL        string
}

// getSecretData retrieves data from secret with fallback support for legacy keys.
func getSecretData(secret *corev1.Secret, key, fallbackKey string, logger logr.Logger) []byte {
	if data, exists := secret.Data[key]; exists {
		return data
	}

	// Always support legacy fields for consistency across Flux APIs
	if data, exists := secret.Data[fallbackKey]; exists {
		logger.Error(nil, "using legacy key in secret data",
			"secret", client.ObjectKeyFromObject(secret),
			"key", fallbackKey,
			"preferred", key)
		return data
	}

	return nil
}
