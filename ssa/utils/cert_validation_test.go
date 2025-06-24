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

package utils_test

import (
	"encoding/base64"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fluxcd/pkg/ssa/utils"
)

// exampleCert was generated from crypto/tls/generate_cert.go with the following command:
//
//	go run generate_cert.go  --rsa-bits 2048 --host example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h - from
//
// this example is from https://github.com/kubernetes/kubernetes/blob/04d2f336419b5a824cb96cb88462ef18a90d619d/staging/src/k8s.io/apiserver/pkg/util/webhook/validation_test.go
// Base64 encoded because caBundle field expects base64 string when stored in unstructured.Unstructured
var exampleCert = base64.StdEncoding.EncodeToString([]byte(`-----BEGIN CERTIFICATE-----
MIIDIDCCAgigAwIBAgIRALYg7UBIx7aeUpwohjIBhUEwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBANJuxq11hL2nB6nygf5/q7JRkPZCYuXwkaqZm7Bk8e9+WzEy9/EW
QtRP92IuKB8XysLY7a/vh9WOcUMw9zBICP754pBIUjgt2KveEYABDSkrAVWIGIO9
IN6crS3OvHiMKyShCvqMMho9wxyTbtnl3lrlcxVyLCmMahnoSyIwWiQ3TMT81eKt
FGEYXa8XEIJJFRX6wxtCgw0PqQy/NLM+G1QvYyKLSLm2cKUGH1A9RfAlMzsICOOf
Rx+/zCAgAfXnjg0SUXfgOjc/Y8EdVyMmBfCWMfovbpwCwULxlEDHHsjVZy5azZjm
E2AYW94BSdRd745M7fudchS6+9rGJi9lc5kCAwEAAaNvMG0wDgYDVR0PAQH/BAQD
AgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0O
BBYEFL/WGYyHD90dPKo8SswyPSydkwG/MBYGA1UdEQQPMA2CC2V4YW1wbGUuY29t
MA0GCSqGSIb3DQEBCwUAA4IBAQAS9qnl6mTF/HHRZSfQypxBj1lsDwYz99PsDAyw
hoXetTVmkejsPe9EcQ5eBRook6dFIevXN9bY5dxYSjWoSg/kdsihJ3FsJsmAQEtK
eM8ko9uvtZ+i0LUfg2l3kima1/oX0MCvnuePGgl7quyBhGUeg5tOudiX07hETWPW
Kt/FgMvfzK63pqcJpLj2+2pnmieV3ploJjw1sIAboR3W5LO/9XgRK3h1vr1BbplZ
dhv6TGB0Y1Zc9N64gh0A3xDOrBSllAWYw/XM6TodhvahFyE48fYSFBZVfZ3TZTfd
Bdcg8G2SMXDSZoMBltEIO7ogTjNAqNUJ8MWZFNZz6HnE8UJC
-----END CERTIFICATE-----`))

func TestRemoveCABundleFromCRD(t *testing.T) {
	tests := []struct {
		name           string
		object         *unstructured.Unstructured
		expectRemoved  bool
		expectedBundle string
	}{
		{
			name: "removes invalid CA bundle from CRD",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "examples.example.com",
					},
					"spec": map[string]interface{}{
						"conversion": map[string]interface{}{
							"webhook": map[string]interface{}{
								"clientConfig": map[string]interface{}{
									"caBundle": "invalid-cert-data",
								},
							},
						},
					},
				},
			},
			expectRemoved: true,
		},
		{
			name: "removes invalid base64 CA bundle from CRD",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "tests.example.com",
					},
					"spec": map[string]interface{}{
						"group": "example.com",
						"conversion": map[string]interface{}{
							"webhook": map[string]interface{}{
								"clientConfig": map[string]interface{}{
									"caBundle": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t", // partial cert header in base64
								},
							},
						},
					},
				},
			},
			expectRemoved: true,
		},
		{
			name: "preserves valid CA bundle in CRD",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "examples.example.com",
					},
					"spec": map[string]interface{}{
						"conversion": map[string]interface{}{
							"webhook": map[string]interface{}{
								"clientConfig": map[string]interface{}{
									"caBundle": exampleCert,
								},
							},
						},
					},
				},
			},
			expectRemoved:  false,
			expectedBundle: exampleCert,
		},
		{
			name: "ignores non-CRD objects",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "test-config",
					},
					"data": map[string]interface{}{
						"caBundle": "invalid-cert-data",
					},
				},
			},
			expectRemoved: false,
		},
		{
			name: "handles CRD without CA bundle",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "examples.example.com",
					},
					"spec": map[string]interface{}{
						"group": "example.com",
					},
				},
			},
			expectRemoved: false,
		},
		{
			name: "handles CRD with empty CA bundle",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "tests.example.com",
					},
					"spec": map[string]interface{}{
						"group": "example.com",
						"conversion": map[string]interface{}{
							"webhook": map[string]interface{}{
								"clientConfig": map[string]interface{}{
									"caBundle": "",
								},
							},
						},
					},
				},
			},
			expectRemoved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := tt.object.DeepCopy()

			utils.RemoveCABundleFromCRD(obj)

			caBundle, found, err := unstructured.NestedString(obj.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
			if err != nil {
				t.Fatal(err)
			}

			if tt.expectRemoved {
				if found && caBundle != "" {
					t.Errorf("Expected CA bundle to be removed, but found: %s", caBundle)
				}
			} else if tt.expectedBundle != "" {
				if !found || caBundle != tt.expectedBundle {
					t.Errorf("Expected CA bundle to be preserved as %s, got: %s", tt.expectedBundle, caBundle)
				}
			}

			if obj.GetKind() == "ConfigMap" {
				originalData, _, _ := unstructured.NestedString(tt.object.Object, "data", "caBundle")
				currentData, found, err := unstructured.NestedString(obj.Object, "data", "caBundle")
				if err != nil {
					t.Fatal(err)
				}
				if !found || currentData != originalData {
					t.Errorf("Expected ConfigMap data to be unchanged, got: %s", currentData)
				}
			}

			if obj.GetKind() == "CustomResourceDefinition" && !tt.expectRemoved && tt.expectedBundle == "" {
				group, found, err := unstructured.NestedString(obj.Object, "spec", "group")
				if err != nil {
					t.Fatal(err)
				}
				if !found && group != "example.com" {
					t.Errorf("Group should remain unchanged from example.com, group: %s", group)
				}
			}
		})
	}
}
