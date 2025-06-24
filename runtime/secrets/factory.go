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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeTLSSecret creates a Kubernetes TLS secret from certificate data.
//
// The function accepts certificate, private key, and optional CA certificate data.
// If both certData and keyData are provided, they will be validated as a valid TLS pair.
// Empty data fields will be omitted from the resulting secret.
func MakeTLSSecret(name, namespace string, certData, keyData, caData []byte) (*corev1.Secret, error) {
	if len(certData) > 0 && len(keyData) > 0 {
		if _, err := tls.X509KeyPair(certData, keyData); err != nil {
			return nil, fmt.Errorf("invalid TLS certificate and key pair: %w", err)
		}
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       corev1.SecretTypeTLS,
		StringData: make(map[string]string),
	}

	if len(certData) > 0 {
		secret.StringData[TLSCertKey] = string(certData)
	}
	if len(keyData) > 0 {
		secret.StringData[TLSKeyKey] = string(keyData)
	}
	if len(caData) > 0 {
		secret.StringData[CACertKey] = string(caData)
	}

	return secret, nil
}

// MakeBasicAuthSecret creates a Kubernetes basic auth secret.
//
// The function requires both username and password to be non-empty.
// The resulting secret will be of type kubernetes.io/basic-auth.
func MakeBasicAuthSecret(name, namespace, username, password string) (*corev1.Secret, error) {
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeBasicAuth,
		StringData: map[string]string{
			UsernameKey: username,
			PasswordKey: password,
		},
	}, nil
}

// MakeProxySecret creates a Kubernetes secret for proxy configuration.
//
// The function requires a valid proxy address (URL format).
// Optional username and password can be provided for proxy authentication.
// The resulting secret will be of type Opaque.
func MakeProxySecret(name, namespace, address, username, password string) (*corev1.Secret, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}

	if _, err := url.Parse(address); err != nil {
		return nil, fmt.Errorf("invalid proxy address: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			ProxyAddressKey: address,
		},
	}

	if username != "" {
		secret.StringData[UsernameKey] = username
	}
	if password != "" {
		secret.StringData[PasswordKey] = password
	}

	return secret, nil
}

// MakeBearerTokenSecret creates a Kubernetes secret for bearer token authentication.
//
// The function requires a non-empty token value.
// The resulting secret will be of type Opaque with the token stored under the "bearerToken" key.
func MakeBearerTokenSecret(name, namespace, token string) (*corev1.Secret, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			BearerTokenKey: token,
		},
	}, nil
}

// MakeTokenSecret creates a Kubernetes secret for generic API token authentication.
//
// The function requires a non-empty token value.
// The resulting secret will be of type Opaque with the token stored under the "token" key.
// This is suitable for various API tokens like GitHub, Slack, Telegram, etc.
func MakeTokenSecret(name, namespace, token string) (*corev1.Secret, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			TokenKey: token,
		},
	}, nil
}

// MakeRegistrySecret creates a Kubernetes Docker config secret for container registry authentication.
//
// The function requires server, username, and password to be non-empty.
// It generates a Docker config JSON with base64-encoded auth field containing "username:password".
// The resulting secret will be of type kubernetes.io/dockerconfigjson.
func MakeRegistrySecret(name, namespace, server, username, password string) (*corev1.Secret, error) {
	if server == "" {
		return nil, fmt.Errorf("server is required")
	}
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
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

	configData, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Docker config: %w", err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			corev1.DockerConfigJsonKey: string(configData),
		},
	}, nil
}
