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

	// TLSProviderSPIFFE is the TLS provider that uses SPIFFE X.509-SVIDs.
	TLSProviderSPIFFE TLSProvider = "spiffe"
)

// TrustDomainSelf is the special trust domain value that authorizes
// any SVID in the client's own trust domain.
const TrustDomainSelf = "self"

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
	// +kubebuilder:validation:Enum=secret;spiffe
	// +required
	Provider TLSProvider `json:"provider"`
}

// TLSServerAuth configures how the remote service is authenticated.
//
// +kubebuilder:validation:XValidation:rule="(self.provider == 'spiffe') == has(self.spiffe)",message="tls.serverAuth.spiffe must be set if and only if tls.serverAuth.provider is 'spiffe'"
type TLSServerAuth struct {
	// Provider is the provider used to authenticate the remote service.
	// +kubebuilder:validation:Enum=secret;spiffe
	// +required
	Provider TLSProvider `json:"provider"`

	// SPIFFE configures the authorization of the remote service's
	// X.509-SVID.
	// +optional
	SPIFFE *TLSServerAuthSPIFFE `json:"spiffe,omitempty"`
}

// TLSServerAuthSPIFFE configures the authorization of a remote service's
// X.509-SVID. Exactly one of ServerID or TrustDomain must be set.
//
// +kubebuilder:validation:XValidation:rule="has(self.serverID) != has(self.trustDomain)",message="exactly one of tls.serverAuth.spiffe.serverID or tls.serverAuth.spiffe.trustDomain must be set"
type TLSServerAuthSPIFFE struct {
	// ServerID authorizes an exact SPIFFE ID. When it ends with a
	// trailing slash ("/"), it authorizes any SPIFFE ID with that prefix.
	// +optional
	ServerID string `json:"serverID,omitempty"`

	// TrustDomain authorizes any SVID in the given trust domain. The
	// special value 'self' authorizes any SVID in the client's own
	// trust domain.
	// +optional
	TrustDomain string `json:"trustDomain,omitempty"`
}
