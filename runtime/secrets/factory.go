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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type emptyCheckable interface {
	~string | ~[]byte
}

func validateRequired[T emptyCheckable](value T, fieldName string) error {
	if len(value) == 0 {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

func makeSecret(name, namespace string, secretType corev1.SecretType, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       secretType,
		StringData: data,
	}
}

// TLSSecretOption configures a TLS secret.
type TLSSecretOption func(*tlsCertificateData)

// WithCAData sets the CA certificate data for the TLS secret.
func WithCAData(caData []byte) TLSSecretOption {
	return func(data *tlsCertificateData) {
		data.caCert = caData
	}
}

// WithCertKeyPair sets the certificate and key data for the TLS secret.
func WithCertKeyPair(certData, keyData []byte) TLSSecretOption {
	return func(data *tlsCertificateData) {
		data.cert = certData
		data.key = keyData
	}
}

// MakeTLSSecret creates a Kubernetes TLS secret from certificate data.
//
// The function supports creating secrets with CA certificate only, client certificate
// and key pair only, or both. At least one option must be provided.
func MakeTLSSecret(name, namespace string, opts ...TLSSecretOption) (*corev1.Secret, error) {
	data := &tlsCertificateData{}
	for _, opt := range opts {
		opt(data)
	}

	if err := data.validate(); err != nil {
		return nil, err
	}

	if err := data.validateCertificatePairIfPresent(); err != nil {
		return nil, err
	}

	return data.toSecret(name, namespace), nil
}

// MakeBasicAuthSecret creates a Kubernetes basic auth secret.
//
// The function requires both username and password to be non-empty.
// The resulting secret will be of type kubernetes.io/basic-auth.
func MakeBasicAuthSecret(name, namespace, username, password string) (*corev1.Secret, error) {
	if err := validateRequired(username, "username"); err != nil {
		return nil, err
	}
	if err := validateRequired(password, "password"); err != nil {
		return nil, err
	}

	return makeSecret(name, namespace, corev1.SecretTypeBasicAuth, map[string]string{
		UsernameKey: username,
		PasswordKey: password,
	}), nil
}

// MakeProxySecret creates a Kubernetes secret for proxy configuration.
//
// The function requires a valid proxy address (URL format).
// Optional username and password can be provided for proxy authentication.
// The resulting secret will be of type Opaque.
func MakeProxySecret(name, namespace, address, username, password string) (*corev1.Secret, error) {
	if err := validateRequired(address, "address"); err != nil {
		return nil, err
	}

	if _, err := url.Parse(address); err != nil {
		return nil, fmt.Errorf("invalid proxy address: %w", err)
	}

	data := map[string]string{
		AddressKey: address,
	}

	if username != "" {
		data[UsernameKey] = username
	}
	if password != "" {
		data[PasswordKey] = password
	}

	return makeSecret(name, namespace, corev1.SecretTypeOpaque, data), nil
}

// MakeBearerTokenSecret creates a Kubernetes secret for bearer token authentication.
//
// The function requires a non-empty token value.
// The resulting secret will be of type Opaque with the token stored under the "bearerToken" key.
func MakeBearerTokenSecret(name, namespace, token string) (*corev1.Secret, error) {
	if err := validateRequired(token, "token"); err != nil {
		return nil, err
	}

	return makeSecret(name, namespace, corev1.SecretTypeOpaque, map[string]string{
		BearerTokenKey: token,
	}), nil
}

// MakeTokenSecret creates a Kubernetes secret for generic API token authentication.
//
// The function requires a non-empty token value.
// The resulting secret will be of type Opaque with the token stored under the "token" key.
// This is suitable for various API tokens like GitHub, Slack, Telegram, etc.
func MakeTokenSecret(name, namespace, token string) (*corev1.Secret, error) {
	if err := validateRequired(token, "token"); err != nil {
		return nil, err
	}

	return makeSecret(name, namespace, corev1.SecretTypeOpaque, map[string]string{
		TokenKey: token,
	}), nil
}

// MakeRegistrySecret creates a Kubernetes Docker config secret for container registry authentication.
//
// The function requires server, username, and password to be non-empty.
// It generates a Docker config JSON with base64-encoded auth field containing "username:password".
// The resulting secret will be of type kubernetes.io/dockerconfigjson.
func MakeRegistrySecret(name, namespace, server, username, password string) (*corev1.Secret, error) {
	if err := validateRequired(server, "server"); err != nil {
		return nil, err
	}
	if err := validateRequired(username, "username"); err != nil {
		return nil, err
	}
	if err := validateRequired(password, "password"); err != nil {
		return nil, err
	}

	type dockerAuth struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Auth     string `json:"auth"`
	}

	type dockerConfig struct {
		Auths map[string]dockerAuth `json:"auths"`
	}

	cred := fmt.Sprintf("%s:%s", username, password)
	auth := base64.StdEncoding.EncodeToString([]byte(cred))

	config := dockerConfig{
		Auths: map[string]dockerAuth{
			server: {
				Username: username,
				Password: password,
				Auth:     auth,
			},
		},
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Docker config: %w", err)
	}

	return makeSecret(name, namespace, corev1.SecretTypeDockerConfigJson, map[string]string{
		corev1.DockerConfigJsonKey: string(configData),
	}), nil
}
