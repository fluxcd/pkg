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

package utils

import (
	"encoding/base64"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/transport"
)

// validateCABundle validates a CA bundle using the same logic as Kubernetes 1.31+.
// Returns true if the CA bundle is valid, false otherwise.
func validateCABundle(caBundle []byte) bool {
	if len(caBundle) == 0 {
		return true // Empty CA bundle is considered valid
	}

	_, err := transport.TLSConfigFor(&transport.Config{
		TLS: transport.TLSConfig{
			CAData: caBundle,
		},
	})
	return err == nil
}

// isCRDWithInvalidCABundle checks if the given object is a CRD with an invalid CA bundle
// in spec.conversion.webhook.clientConfig.caBundle. Returns true if it's a CRD with
// an invalid CA bundle that should be removed from the SSA patch.
func isCRDWithInvalidCABundle(object *unstructured.Unstructured) bool {
	if !IsCRD(object) {
		return false
	}

	// Get the caBundle from spec.conversion.webhook.clientConfig.caBundle
	caBundle, found, err := unstructured.NestedString(object.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
	if err != nil || !found || caBundle == "" {
		return false
	}

	if decoded, err := base64.StdEncoding.DecodeString(caBundle); err == nil {
		return !validateCABundle([]byte(decoded))
	}
	return true
}

// Remove invalid CA bundle from CRDs to prevent API rejection in Kubernetes 1.31+
func RemoveCABundleFromCRD(object *unstructured.Unstructured) {
	if isCRDWithInvalidCABundle(object) {
		unstructured.RemoveNestedField(object.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
	}
}
