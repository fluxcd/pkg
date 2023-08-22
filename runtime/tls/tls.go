/*
Copyright 2021 The Flux authors

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

package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
)

const (
	ClientCertIdentifier = "certFile"
	ClientKeyIdentifier  = "keyFile"
	CACertIdentifier     = "caFile"
)

// ConfigFromSecret returns a TLS config created from the content of the secret.
// An error is returned if the secret does not contain a ClientCertIdentifier and ClientKeyIdentifier, or a
// CACertIdentifier.
func ConfigFromSecret(certSecret *corev1.Secret) (*tls.Config, error) {
	validSecret := false
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	clientCert, clientCertOk := certSecret.Data[ClientCertIdentifier]
	clientKey, clientKeyOk := certSecret.Data[ClientKeyIdentifier]
	if clientKeyOk != clientCertOk {
		return nil, fmt.Errorf("found one of %s or %s, and expected both or neither", ClientCertIdentifier, ClientKeyIdentifier)
	}
	if clientCertOk && clientKeyOk {
		validSecret = true
		cert, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}

	if caCert, ok := certSecret.Data[CACertIdentifier]; ok {
		validSecret = true
		sysCerts, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		sysCerts.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = sysCerts
	}

	if !validSecret {
		return nil, fmt.Errorf("no %s and %s, or %s found in secret", ClientCertIdentifier, ClientKeyIdentifier, CACertIdentifier)
	}

	return tlsConfig, nil
}

// TransportFromSecret returns a HTTP transport with a TLS config created from the content of the secret.
// An error is returned if the secret does not contain a ClientCertIdentifier and ClientKeyIdentifier, or a
// CACertIdentifier.
func TransportFromSecret(certSecret *corev1.Secret) (*http.Transport, error) {
	tlsConfig, err := ConfigFromSecret(certSecret)
	if err != nil {
		return nil, err
	}
	return &http.Transport{TLSClientConfig: tlsConfig}, nil
}
