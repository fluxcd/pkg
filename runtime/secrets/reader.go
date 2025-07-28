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
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TLSConfigFromSecretRef creates a TLS configuration from a Kubernetes secret reference.
//
// The function fetches the secret from the API server and then processes it using
// TLSConfigFromSecret. It supports the same field names and legacy field handling.
//
// The targetURL parameter is used to set the ServerName for proper SNI support
// in virtual hosting environments.
func TLSConfigFromSecretRef(ctx context.Context, c client.Client, secretRef types.NamespacedName, targetURL string) (*tls.Config, error) {
	secret, err := getSecret(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}
	return TLSConfigFromSecret(ctx, secret, targetURL)
}

// ProxyURLFromSecretRef creates a proxy URL from a Kubernetes secret reference.
//
// The function fetches the secret from the API server and then processes it using
// ProxyURLFromSecret. It expects the same field structure for proxy configuration.
func ProxyURLFromSecretRef(ctx context.Context, c client.Client, secretRef types.NamespacedName) (*url.URL, error) {
	secret, err := getSecret(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}
	return ProxyURLFromSecret(ctx, secret)
}

// PullSecretsFromServiceAccountRef retrieves all image pull secrets referenced by a service account.
//
// The function resolves all secrets listed in the service account's imagePullSecrets field
// and returns them as a slice. If any referenced secret cannot be found, an error is returned.
func PullSecretsFromServiceAccountRef(ctx context.Context, c client.Client, saRef types.NamespacedName) ([]corev1.Secret, error) {
	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, saRef, sa); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("serviceaccount '%s' not found", saRef)
		}
		return nil, fmt.Errorf("failed to get serviceaccount '%s': %w", saRef, err)
	}

	secrets := make([]corev1.Secret, 0, len(sa.ImagePullSecrets))
	for _, imagePullSecret := range sa.ImagePullSecrets {
		secretRef := types.NamespacedName{Name: imagePullSecret.Name, Namespace: saRef.Namespace}
		secret, err := getSecret(ctx, c, secretRef)
		if err != nil {
			saRef := client.ObjectKeyFromObject(sa)
			return nil, fmt.Errorf("failed to get image pull secret for serviceaccount '%s': %w", saRef, err)
		}
		secrets = append(secrets, *secret)
	}

	return secrets, nil
}

func getSecret(ctx context.Context, c client.Client, secretRef types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, secretRef, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("secret '%s' not found", secretRef)
		}
		return nil, fmt.Errorf("failed to get secret '%s': %w", secretRef, err)
	}
	return secret, nil
}
