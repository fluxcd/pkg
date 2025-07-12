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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TLSConfigFromSecret creates a TLS configuration from a Kubernetes secret.
//
// The function looks for TLS certificate data in the secret using standard
// field names (tls.crt, tls.key, ca.crt). It also supports legacy field names
// (certFile, keyFile, caFile) as fallbacks, logging warnings when they are used.
//
// Standard field names always take precedence over legacy ones.
func TLSConfigFromSecret(ctx context.Context, secret *corev1.Secret) (*tls.Config, error) {
	logger := log.FromContext(ctx)
	certData, err := getTLSCertificateData(secret, logger)
	if err != nil {
		return nil, enhanceSecretValidationError(err, secret)
	}

	return buildTLSConfig(certData)
}

// ProxyURLFromSecret creates a proxy URL from a Kubernetes secret.
//
// The function expects the secret to contain an "address" field with the
// proxy URL. Optional "username" and "password" fields can be provided
// for proxy authentication.
func ProxyURLFromSecret(ctx context.Context, secret *corev1.Secret) (*url.URL, error) {
	addressData, exists := secret.Data[AddressKey]
	if !exists {
		return nil, &KeyNotFoundError{Key: AddressKey, Secret: secret}
	}

	address := string(addressData)
	if address == "" {
		ref := client.ObjectKeyFromObject(secret)
		return nil, fmt.Errorf("secret '%s': proxy address is empty", ref)
	}

	proxyURL, err := url.Parse(address)
	if err != nil {
		ref := client.ObjectKeyFromObject(secret)
		return nil, fmt.Errorf("secret '%s': failed to parse proxy address '%s': %w", ref, address, err)
	}

	username, hasUsername := secret.Data[UsernameKey]
	password, hasPassword := secret.Data[PasswordKey]

	if hasUsername && hasPassword {
		proxyURL.User = url.UserPassword(string(username), string(password))
	} else if hasUsername {
		proxyURL.User = url.User(string(username))
	}

	return proxyURL, nil
}

// BasicAuthFromSecret retrieves basic authentication credentials from a Kubernetes secret.
//
// The function expects the secret to contain "username" and "password" fields.
// Both fields are required and the function will return an error if either is missing.
func BasicAuthFromSecret(ctx context.Context, secret *corev1.Secret) (string, string, error) {
	usernameData, exists := secret.Data[UsernameKey]
	if !exists {
		return "", "", &KeyNotFoundError{Key: UsernameKey, Secret: secret}
	}

	passwordData, exists := secret.Data[PasswordKey]
	if !exists {
		return "", "", &KeyNotFoundError{Key: PasswordKey, Secret: secret}
	}

	return string(usernameData), string(passwordData), nil
}

func getTLSCertificateData(secret *corev1.Secret, logger logr.Logger) (*tlsCertificateData, error) {
	return newTLSCertificateData(secret, logger)
}

func buildTLSConfig(certData *tlsCertificateData) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if certData.hasCertPair() {
		cert, err := tls.X509KeyPair(certData.cert, certData.key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TLS certificate and key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if certData.hasCA() {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(certData.caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}
