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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TLSConfigFromSecret creates a TLS configuration from a Kubernetes secret.
//
// The function looks for TLS certificate data in the secret using standard
// field names (tls.crt, tls.key, ca.crt). If WithDeprecatedFieldSupport
// option is provided, it will also check deprecated field names (certFile,
// keyFile, caFile) as fallbacks.
//
// Standard field names always take precedence over deprecated ones.
func TLSConfigFromSecret(ctx context.Context, c client.Client, name, namespace string, opts ...Option) (*tls.Config, error) {
	options := makeOptions(opts)

	secret, err := getSecret(ctx, c, name, namespace)
	if err != nil {
		return nil, err
	}

	tlsCert, err := getSecretData(secret, TLSCertKey, TLSCertFileKey, options.supportDeprecatedFields)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS certificate: %w", err)
	}

	tlsKey, err := getSecretData(secret, TLSKeyKey, TLSKeyFileKey, options.supportDeprecatedFields)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS private key: %w", err)
	}

	// CA certificate is optional, ignore error if not found
	caCert, _ := getSecretData(secret, CACertKey, CACertFileKey, options.supportDeprecatedFields)

	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TLS certificate and key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	if len(caCert) > 0 {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// ProxyURLFromSecret creates a proxy URL from a Kubernetes secret.
//
// The function expects the secret to contain an "address" field with the
// proxy URL. Optional "username" and "password" fields can be provided
// for proxy authentication.
func ProxyURLFromSecret(ctx context.Context, c client.Client, name, namespace string) (*url.URL, error) {
	secret, err := getSecret(ctx, c, name, namespace)
	if err != nil {
		return nil, err
	}

	addressData, exists := secret.Data[ProxyAddressKey]
	if !exists {
		return nil, fmt.Errorf("key '%s' not found in secret", ProxyAddressKey)
	}

	address := string(addressData)
	if address == "" {
		return nil, fmt.Errorf("proxy address is empty")
	}

	proxyURL, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy address '%s': %w", address, err)
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
func BasicAuthFromSecret(ctx context.Context, c client.Client, name, namespace string) (string, string, error) {
	secret, err := getSecret(ctx, c, name, namespace)
	if err != nil {
		return "", "", err
	}

	usernameData, exists := secret.Data[UsernameKey]
	if !exists {
		return "", "", fmt.Errorf("key '%s' not found in secret", UsernameKey)
	}

	passwordData, exists := secret.Data[PasswordKey]
	if !exists {
		return "", "", fmt.Errorf("key '%s' not found in secret", PasswordKey)
	}

	return string(usernameData), string(passwordData), nil
}

// PullSecretsFromServiceAccount retrieves all image pull secrets referenced by a service account.
//
// The function resolves all secrets listed in the service account's imagePullSecrets field
// and returns them as a slice. If any referenced secret cannot be found, an error is returned.
func PullSecretsFromServiceAccount(ctx context.Context, c client.Client, name, namespace string) ([]corev1.Secret, error) {
	serviceAccount := &corev1.ServiceAccount{}
	saKey := types.NamespacedName{Name: name, Namespace: namespace}
	if err := c.Get(ctx, saKey, serviceAccount); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("serviceaccount %s/%s not found", namespace, name)
		}
		return nil, fmt.Errorf("failed to get serviceaccount %s/%s: %w", namespace, name, err)
	}

	if len(serviceAccount.ImagePullSecrets) == 0 {
		return []corev1.Secret{}, nil
	}

	secrets := make([]corev1.Secret, 0, len(serviceAccount.ImagePullSecrets))
	for _, imagePullSecret := range serviceAccount.ImagePullSecrets {
		secret, err := getSecret(ctx, c, imagePullSecret.Name, namespace)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}

	return secrets, nil
}

func getSecret(ctx context.Context, c client.Client, name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Name: name, Namespace: namespace}
	if err := c.Get(ctx, secretKey, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("secret %s/%s not found", namespace, name)
		}
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}
	return secret, nil
}

func getSecretData(secret *corev1.Secret, key, fallbackKey string, supportDeprecated bool) ([]byte, error) {
	if data, exists := secret.Data[key]; exists {
		return data, nil
	}

	if supportDeprecated {
		if data, exists := secret.Data[fallbackKey]; exists {
			return data, nil
		}
	}

	return nil, fmt.Errorf("key '%s' not found in secret", key)
}
