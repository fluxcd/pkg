/*
Copyright 2026 The Flux authors

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

package crypto

// CredentialProvider is the provider that issues a short-lived credential.
type CredentialProvider string

const (
	// CredentialProviderKubernetes is the credential provider that uses
	// Kubernetes ServiceAccount tokens or X.509 certificates issued
	// through the Kubernetes certificate signing API as the short-lived
	// credential.
	CredentialProviderKubernetes CredentialProvider = "kubernetes"

	// CredentialProviderSPIFFE is the credential provider that uses the
	// SPIFFE Workload API to fetch SVIDs as the short-lived credential.
	CredentialProviderSPIFFE CredentialProvider = "spiffe"
)

// CredentialType is the type of a short-lived credential.
type CredentialType string

const (
	// CredentialTypeJWT is the credential type for JSON Web Tokens.
	CredentialTypeJWT CredentialType = "jwt"

	// CredentialTypeX509 is the credential type for X.509 certificates.
	CredentialTypeX509 CredentialType = "x509"
)

// Credential defines the configuration for obtaining short-lived
// cryptographic material used to authenticate with a remote service.
//
// +kubebuilder:validation:XValidation:rule="!has(self.audiences) || self.type == 'jwt'",message="credential.audiences can only be set when credential.type is 'jwt'"
// +kubebuilder:validation:XValidation:rule="!has(self.expirationSeconds) || self.provider == 'kubernetes'",message="credential.expirationSeconds can only be set when credential.provider is 'kubernetes'"
type Credential struct {
	// Provider is the provider that issues the short-lived credential.
	// +kubebuilder:validation:Enum=kubernetes;spiffe
	// +required
	Provider CredentialProvider `json:"provider"`

	// Type is the type of the short-lived credential.
	// +kubebuilder:validation:Enum=jwt;x509
	// +required
	Type CredentialType `json:"type"`

	// ExpirationSeconds is the requested duration of validity of the
	// credential in seconds. It is supported only when the provider is
	// 'kubernetes'. The credential is issued for the ServiceAccount
	// configured in the respective field of the enclosing API object,
	// e.g. serviceAccountName, or through the respective controller
	// flag --default.*-service-account, where '.*' matches one of '',
	// '-decryption' or '-kubeconfig'.
	// +optional
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty"`

	// Audiences is the list of audiences to set in JWT credentials.
	// It defaults to the URL of the service being accessed.
	// +optional
	Audiences []string `json:"audiences,omitempty"`

	// Annotations is the set of annotations to configure the credential.
	// It accepts the annotations supported for ServiceAccounts by the
	// Flux auth providers. If an annotation is also set in the referenced
	// ServiceAccount object, the values must match.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}
