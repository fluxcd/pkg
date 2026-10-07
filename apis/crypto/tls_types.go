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

// TLSProvider is the provider of a TLS certificate.
type TLSProvider string

const (
	// TLSProviderSecret is the TLS provider that uses the static
	// certificates referenced by the API object.
	TLSProviderSecret TLSProvider = "secret"

	// TLSProviderKubernetes is the TLS provider that uses X.509 client
	// certificates issued by Kubernetes for the ServiceAccount configured
	// in the API object. It is supported only for client authentication.
	TLSProviderKubernetes TLSProvider = "kubernetes"

	// TLSProviderSPIFFE is the TLS provider that uses SPIFFE X.509-SVIDs.
	TLSProviderSPIFFE TLSProvider = "spiffe"
)

// SelfSPIFFEID is the special SPIFFE ID value that authorizes any SVID in
// our own trust domain.
const SelfSPIFFEID = "spiffe://self"

// TLS defines the TLS configuration for communicating with a remote
// service. ClientAuth and ServerAuth are independent and may use
// different providers. At least one of them must be set.
//
// +kubebuilder:validation:XValidation:rule="has(self.clientAuth) || has(self.serverAuth)",message="at least one of tls.clientAuth or tls.serverAuth must be set"
type TLS struct {
	// ClientAuth configures how the client authenticates with the
	// remote service.
	// +optional
	ClientAuth *TLSClientAuth `json:"clientAuth,omitempty"`

	// ServerAuth configures how the remote service is authenticated.
	// +optional
	ServerAuth *TLSServerAuth `json:"serverAuth,omitempty"`
}

// TLSClientAuth configures how the client authenticates with the remote
// service.
type TLSClientAuth struct {
	// Provider is the provider of the client certificate.
	// +kubebuilder:validation:Enum=secret;kubernetes;spiffe
	// +required
	Provider TLSProvider `json:"provider"`
}

// TLSServerAuth configures how the remote service is authenticated.
//
// +kubebuilder:validation:XValidation:rule="(self.provider == 'spiffe') == has(self.spiffeID)",message="tls.serverAuth.spiffeID must be set if and only if tls.serverAuth.provider is 'spiffe'"
type TLSServerAuth struct {
	// Provider is the provider used to authenticate the remote service.
	// Kubernetes is not supported because Kubernetes only issues X.509
	// client certificates.
	// +kubebuilder:validation:Enum=secret;spiffe
	// +required
	Provider TLSProvider `json:"provider"`

	// SPIFFEID authorizes the remote service's X.509-SVID. The following
	// forms are supported:
	// - an exact SPIFFE ID, e.g. spiffe://example.org/registry;
	// - a SPIFFE ID prefix when it ends with a trailing slash ("/"),
	//   e.g. spiffe://example.org/registry/;
	// - a trust domain when the SPIFFE ID has no path, e.g.
	//   spiffe://example.org, which authorizes any SVID in that trust
	//   domain;
	// - the special value 'spiffe://self', which authorizes any SVID in
	//   our own trust domain.
	// +optional
	SPIFFEID string `json:"spiffeID,omitempty"`
}
