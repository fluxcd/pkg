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
	"testing"

	"github.com/fluxcd/pkg/runtime/tls/testdata"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestCert_TlsConfigAll(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			CACertIdentifier:     testdata.ExampleCA,
			ClientCertIdentifier: testdata.ExampleCert,
			ClientKeyIdentifier:  testdata.ExampleKey,
		},
	}
	tlsConfig, err := ConfigFromSecret(secret)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(testdata.ExampleCert, testdata.ExampleKey)
	require.NoError(t, err)
	require.Equal(t, tlsConfig.Certificates[0], cert)
}

func TestCert_TlsConfigNone(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{},
	}
	tlsConfig, err := ConfigFromSecret(secret)
	require.EqualError(t, err, "no certFile and keyFile, or caFile found in secret")
	require.Nil(t, tlsConfig)
}

func TestCert_TlsConfigOnlyCa(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			CACertIdentifier: testdata.ExampleCA,
		},
	}
	tlsConfig, err := ConfigFromSecret(secret)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
}

func TestCert_TlsConfigOnlyClient(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			ClientCertIdentifier: testdata.ExampleCert,
			ClientKeyIdentifier:  testdata.ExampleKey,
		},
	}
	tlsConfig, err := ConfigFromSecret(secret)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
}

func TestCert_TlsConfigMissingKey(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			CACertIdentifier:     testdata.ExampleCA,
			ClientCertIdentifier: testdata.ExampleCert,
		},
	}
	tlsConfig, err := ConfigFromSecret(secret)
	require.EqualError(t, err, "found one of certFile or keyFile, and expected both or neither")
	require.Nil(t, tlsConfig)
}

func TestCert_TlsConfigMissingCert(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			CACertIdentifier:    testdata.ExampleCA,
			ClientKeyIdentifier: testdata.ExampleKey,
		},
	}
	tlsConfig, err := ConfigFromSecret(secret)
	require.EqualError(t, err, "found one of certFile or keyFile, and expected both or neither")
	require.Nil(t, tlsConfig)
}

func TestCert_Transport(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			CACertIdentifier:     testdata.ExampleCA,
			ClientCertIdentifier: testdata.ExampleCert,
			ClientKeyIdentifier:  testdata.ExampleKey,
		},
	}
	transport, err := TransportFromSecret(secret)
	require.NoError(t, err)
	require.NotNil(t, transport.TLSClientConfig)
}

func Fuzz_TlsConfig(f *testing.F) {
	f.Add(testdata.ExampleCert, testdata.ExampleKey, testdata.ExampleCA)

	f.Fuzz(func(t *testing.T,
		clientCertIdentifier, clientKeyIdentifier, caCertIdentifier []byte) {
		secret := &corev1.Secret{
			Data: map[string][]byte{
				CACertIdentifier:     caCertIdentifier,
				ClientCertIdentifier: clientCertIdentifier,
				ClientKeyIdentifier:  clientKeyIdentifier,
			},
		}
		_, _ = ConfigFromSecret(secret)
	})
}
