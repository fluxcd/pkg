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
	"errors"
	"fmt"
	"net/url"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TLSConfigOption is a functional option for configuring TLS behavior.
type TLSConfigOption func(*tlsConfig)

// WithSystemCertPool enables the use of system certificate pool in addition to user-provided CA certificates.
func WithSystemCertPool() TLSConfigOption {
	return func(c *tlsConfig) {
		c.useSystemCertPool = true
	}
}

// AuthMethodsFromSecret extracts all available authentication methods from a Kubernetes secret.
//
// The function attempts to parse all supported authentication methods from the secret data.
// It does not fail if a particular authentication method is not present, but will return
// an error if the secret contains malformed authentication data.
//
// Supported authentication methods:
//   - Basic authentication (username/password)
//   - Bearer token authentication
//   - Token authentication
//   - SSH authentication (private key, known hosts)
//   - GitHub App authentication (app ID, installation ID, private key)
//   - TLS client certificates
//
// Multiple authentication methods can be present in a single secret and will be extracted
// simultaneously, enabling use cases like Basic Auth + TLS or Bearer Token + TLS.
//
// Options can be provided to configure TLS extraction behavior.
func AuthMethodsFromSecret(ctx context.Context, secret *corev1.Secret, opts ...AuthMethodsOption) (*AuthMethods, error) {
	config := &authMethodsConfig{}
	for _, opt := range opts {
		opt(config)
	}

	var methods AuthMethods

	if err := trySetAuth(ctx, secret, &methods.Basic, BasicAuthFromSecret); err != nil {
		return nil, err
	}

	if err := trySetAuth(ctx, secret, &methods.Bearer, BearerAuthFromSecret); err != nil {
		return nil, err
	}

	if err := trySetAuth(ctx, secret, &methods.Token, TokenAuthFromSecret); err != nil {
		return nil, err
	}

	if err := trySetAuth(ctx, secret, &methods.SSH, SSHAuthFromSecret); err != nil {
		return nil, err
	}

	if err := trySetAuth(ctx, secret, &methods.GitHubAppData, GitHubAppDataFromSecret); err != nil {
		return nil, err
	}

	if err := trySetAuth(ctx, secret, &methods.TLS, func(ctx context.Context, secret *corev1.Secret) (*tls.Config, error) {
		return TLSConfigFromSecret(ctx, secret, config.tlsConfigOpts...)
	}); err != nil {
		return nil, err
	}

	return &methods, nil
}

// TLSConfigFromSecret creates a TLS configuration from a Kubernetes secret.
//
// The function looks for TLS certificate data in the secret using standard
// field names (tls.crt, tls.key, ca.crt). It also supports legacy field names
// (certFile, keyFile, caFile) as fallbacks, logging warnings when they are used.
//
// Standard field names always take precedence over legacy ones.
//
// Optional TLSConfigOption parameters can be used to configure CA certificate handling:
//   - WithSystemCertPool(): Include system certificates in addition to user-provided CA
func TLSConfigFromSecret(ctx context.Context, secret *corev1.Secret, opts ...TLSConfigOption) (*tls.Config, error) {
	config := &tlsConfig{}
	for _, opt := range opts {
		opt(config)
	}

	logger := log.FromContext(ctx)
	certData, err := getTLSCertificateData(secret, logger)
	if err != nil {
		var tlsErr *TLSValidationError
		if errors.As(err, &tlsErr) {
			return nil, &SecretTLSValidationError{
				TLSValidationError: tlsErr,
				Secret:             secret,
			}
		}
		return nil, err
	}

	return buildTLSConfig(certData, config)
}

// ProxyURLFromSecret creates a proxy URL from a Kubernetes secret.
//
// The function expects the secret to contain an "address" field with the
// proxy URL. Optional "username" and "password" fields can be provided
// for proxy authentication.
func ProxyURLFromSecret(ctx context.Context, secret *corev1.Secret) (*url.URL, error) {
	ref := client.ObjectKeyFromObject(secret)

	addressData, exists := secret.Data[KeyAddress]
	if !exists {
		return nil, &KeyNotFoundError{Key: KeyAddress, Secret: secret}
	}

	address := string(addressData)
	if address == "" {
		return nil, fmt.Errorf("secret '%s': proxy address is empty", ref)
	}

	// Validate length before parsing to avoid parsing large invalid URLs.
	// The 2048 character limit matches the validation used in notification-controller's
	// spec.address field.
	if len(address) > 2048 {
		return nil, fmt.Errorf("secret '%s': proxy URL exceeds maximum length of 2048 characters", ref)
	}

	proxyURL, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("secret '%s': failed to parse proxy address '%s': %w", ref, address, err)
	}

	if proxyURL.Scheme != "http" && proxyURL.Scheme != "https" {
		return nil, fmt.Errorf("secret '%s': proxy URL must use http or https scheme, got '%s'", ref, proxyURL.Scheme)
	}

	username, hasUsername := secret.Data[KeyUsername]
	password, hasPassword := secret.Data[KeyPassword]

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
// Partial presence (username without password, or password without username) is treated
// as malformed and will return a BasicAuthMalformedError.
func BasicAuthFromSecret(ctx context.Context, secret *corev1.Secret) (*BasicAuth, error) {
	_, hasUsername := secret.Data[KeyUsername]
	_, hasPassword := secret.Data[KeyPassword]

	// Complete absence - return KeyNotFoundError (will be ignored by trySetAuth)
	if !hasUsername && !hasPassword {
		return nil, &KeyNotFoundError{Key: KeyUsername, Secret: secret}
	}

	// Partial presence - return BasicAuthMalformedError (will be propagated by trySetAuth)
	if hasUsername && !hasPassword {
		return nil, &BasicAuthMalformedError{
			Present: KeyUsername,
			Missing: KeyPassword,
			Secret:  secret,
		}
	}
	if !hasUsername && hasPassword {
		return nil, &BasicAuthMalformedError{
			Present: KeyPassword,
			Missing: KeyUsername,
			Secret:  secret,
		}
	}

	// Complete presence - normal processing
	return &BasicAuth{
		Username: string(secret.Data[KeyUsername]),
		Password: string(secret.Data[KeyPassword]),
	}, nil
}

// BearerAuthFromSecret retrieves bearer token authentication credentials from a Kubernetes secret.
//
// The function expects the secret to contain "bearerToken" field.
// The field is required and the function will return an error if it is missing.
func BearerAuthFromSecret(ctx context.Context, secret *corev1.Secret) (BearerAuth, error) {
	tokenData, exists := secret.Data[KeyBearerToken]
	if !exists {
		return "", &KeyNotFoundError{Key: KeyBearerToken, Secret: secret}
	}

	return BearerAuth(tokenData), nil
}

// TokenAuthFromSecret retrieves token authentication credentials from a Kubernetes secret.
//
// The function expects the secret to contain "token" field.
// The field is required and the function will return an error if it is missing.
func TokenAuthFromSecret(ctx context.Context, secret *corev1.Secret) (TokenAuth, error) {
	tokenData, exists := secret.Data[KeyToken]
	if !exists {
		return "", &KeyNotFoundError{Key: KeyToken, Secret: secret}
	}

	return TokenAuth(tokenData), nil
}

// SSHAuthFromSecret retrieves SSH authentication credentials from a Kubernetes secret.
//
// The function expects the secret to contain "identity" and "known_hosts" fields.
// Both fields are required and the function will return an error if either is missing.
// Optional "identity.pub" and "password" fields can be present.
func SSHAuthFromSecret(ctx context.Context, secret *corev1.Secret) (*SSHAuth, error) {
	privateKeyData, exists := secret.Data[KeySSHPrivateKey]
	if !exists {
		return nil, &KeyNotFoundError{Key: KeySSHPrivateKey, Secret: secret}
	}

	knownHostsData, exists := secret.Data[KeySSHKnownHosts]
	if !exists {
		return nil, &KeyNotFoundError{Key: KeySSHKnownHosts, Secret: secret}
	}

	auth := &SSHAuth{
		PrivateKey: privateKeyData,
		KnownHosts: string(knownHostsData),
	}

	if publicKeyData, exists := secret.Data[KeySSHPublicKey]; exists {
		auth.PublicKey = publicKeyData
	}

	if passwordData, exists := secret.Data[KeyPassword]; exists {
		auth.Password = string(passwordData)
	}

	return auth, nil
}

// GitHubAppDataFromSecret retrieves GitHub App authentication data from a Kubernetes secret.
//
// The function expects the secret to contain "githubAppID", "githubAppInstallationID", and
// "githubAppPrivateKey" fields. All three fields are required and the function will return
// an error if any is missing. Optional "githubAppBaseURL" field can be present for GitHub
// Enterprise Server instances.
func GitHubAppDataFromSecret(ctx context.Context, secret *corev1.Secret) (GitHubAppData, error) {
	_, hasAppID := secret.Data[KeyGitHubAppID]
	_, hasInstallationID := secret.Data[KeyGitHubAppInstallationID]
	_, hasPrivateKey := secret.Data[KeyGitHubAppPrivateKey]

	// Complete absence - return KeyNotFoundError (will be ignored by trySetAuth)
	if !hasAppID && !hasInstallationID && !hasPrivateKey {
		return nil, &KeyNotFoundError{Key: KeyGitHubAppID, Secret: secret}
	}

	// Check for required fields - partial presence is an error
	if !hasAppID {
		return nil, &KeyNotFoundError{Key: KeyGitHubAppID, Secret: secret}
	}
	if !hasInstallationID {
		return nil, &KeyNotFoundError{Key: KeyGitHubAppInstallationID, Secret: secret}
	}
	if !hasPrivateKey {
		return nil, &KeyNotFoundError{Key: KeyGitHubAppPrivateKey, Secret: secret}
	}

	data := GitHubAppData{
		KeyGitHubAppID:             secret.Data[KeyGitHubAppID],
		KeyGitHubAppInstallationID: secret.Data[KeyGitHubAppInstallationID],
		KeyGitHubAppPrivateKey:     secret.Data[KeyGitHubAppPrivateKey],
	}

	if baseURLData, exists := secret.Data[KeyGitHubAppBaseURL]; exists {
		data[KeyGitHubAppBaseURL] = baseURLData
	}

	return data, nil
}

func getTLSCertificateData(secret *corev1.Secret, logger logr.Logger) (*tlsCertificateData, error) {
	return newTLSCertificateData(secret, logger)
}

func buildTLSConfig(certData *tlsCertificateData, config *tlsConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		// Note: InsecureSkipVerify is explicitly set to false in accordance with Flux security policy.
		// TLS certificates must be validated using CA certificates or the system trust store.
		InsecureSkipVerify: false,
	}

	if certData.hasCertPair() {
		cert, err := tls.X509KeyPair(certData.cert, certData.key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TLS certificate and key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if certData.hasCA() {
		var caCertPool *x509.CertPool
		if config.useSystemCertPool {
			var err error
			caCertPool, err = x509.SystemCertPool()
			if err != nil {
				return nil, fmt.Errorf("cannot retrieve system certificate pool: %w", err)
			}
		} else {
			caCertPool = x509.NewCertPool()
		}

		if !caCertPool.AppendCertsFromPEM(certData.caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// trySetAuth is a helper function to reduce repetition in AuthMethodsFromSecret.
// It ignores KeyNotFoundError (complete absence of authentication data) but propagates
// other errors including BasicAuthMalformedError (partial/malformed authentication data).
func trySetAuth[T any](ctx context.Context, secret *corev1.Secret, target *T, fn func(context.Context, *corev1.Secret) (T, error)) error {
	if result, err := fn(ctx, secret); err == nil {
		*target = result
	} else if !errors.Is(err, ErrKeyNotFound) {
		// Propagate all errors except KeyNotFoundError
		// This includes BasicAuthMalformedError and other malformed authentication errors
		var tlsErr *TLSValidationError
		if errors.As(err, &tlsErr) {
			// TLS validation errors are ignored (for now, to maintain compatibility)
			return nil
		}
		return err
	}
	return nil
}
