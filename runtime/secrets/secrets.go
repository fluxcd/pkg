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
	// TLSCertKey is the standard key for TLS certificate data in secrets.
	TLSCertKey = corev1.TLSCertKey
	// TLSPrivateKeyKey is the standard key for TLS private key data in secrets.
	TLSPrivateKeyKey = corev1.TLSPrivateKeyKey
	// CACertKey is the standard key for CA certificate data in secrets.
	CACertKey = "ca.crt"

	// LegacyTLSCertFileKey is the legacy key for TLS certificate data in secrets.
	LegacyTLSCertFileKey = "certFile"
	// LegacyTLSPrivateKeyKey is the legacy key for TLS private key data in secrets.
	LegacyTLSPrivateKeyKey = "keyFile"
	// LegacyCACertKey is the legacy key for CA certificate data in secrets.
	LegacyCACertKey = "caFile"

	// UsernameKey is the key for username data in basic auth secrets.
	UsernameKey = "username"
	// PasswordKey is the key for password data in basic auth secrets.
	PasswordKey = "password"

	// AddressKey is the key for proxy address data in proxy secrets.
	AddressKey = "address"

	// BearerTokenKey is the key for bearer token data in secrets.
	BearerTokenKey = "bearerToken"
	// TokenKey is the key for generic API token data in secrets.
	TokenKey = "token"

	// GitHubAppIDKey is the key for GitHub App ID data in secrets.
	GitHubAppIDKey = "githubAppID"
	// GitHubAppInstallationIDKey is the key for GitHub App installation ID data in secrets.
	GitHubAppInstallationIDKey = "githubAppInstallationID"
	// GitHubAppPrivateKey is the key for GitHub App private key data in secrets.
	GitHubAppPrivateKey = "githubAppPrivateKey"
	// GitHubAppBaseUrlKey is the key for GitHub App base URL data in secrets.
	GitHubAppBaseUrlKey = "githubAppBaseURL"
)

// tlsCertificateData holds TLS certificate, key, and optional CA data
type tlsCertificateData struct {
	cert   []byte
	key    []byte
	caCert []byte
}

// newTLSCertificateData creates tlsCertificateData from a Kubernetes secret.
func newTLSCertificateData(secret *corev1.Secret, logger logr.Logger) (*tlsCertificateData, error) {
	data := &tlsCertificateData{
		cert:   getSecretData(secret, TLSCertKey, LegacyTLSCertFileKey, logger),
		key:    getSecretData(secret, TLSPrivateKeyKey, LegacyTLSPrivateKeyKey, logger),
		caCert: getSecretData(secret, CACertKey, LegacyCACertKey, logger),
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
		secretData[TLSCertKey] = string(t.cert)
		secretData[TLSPrivateKeyKey] = string(t.key)
		secretType = corev1.SecretTypeTLS
	} else {
		secretType = corev1.SecretTypeOpaque
	}

	if t.hasCA() {
		secretData[CACertKey] = string(t.caCert)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       secretType,
		StringData: secretData,
	}
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
