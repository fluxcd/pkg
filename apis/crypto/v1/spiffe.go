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

package v1

import (
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// SPIFFEPeerAuthorization is a SPIFFE ID parsed either from a `*.fluxcd.io`
// ServiceAccount annotation, or from a controller flag. At most one of ID,
// Prefix or TrustDomain is set. UseSelfTrustDomain may be set alone, or
// together with ID or Prefix to indicate that their trust domain must be
// replaced by the trust domain of our own X509-SVID.
type SPIFFEPeerAuthorization struct {
	// ID is set when the annotation is an exact SPIFFE ID.
	ID *spiffeid.ID
	// Prefix is set when the annotation is a SPIFFE ID prefix.
	Prefix *spiffeid.ID
	// TrustDomain is set when the annotation is a SPIFFE Trust Domain.
	TrustDomain *spiffeid.TrustDomain
	// UseSelfTrustDomain is set when the parsed value contains the special
	// `self` symbol. When set alone, it refers to the entire trust domain
	// of our own X509-SVID. When set together with ID or Prefix, the ID or
	// prefix carries the path and its trust domain must be replaced by the
	// trust domain of our own X509-SVID.
	UseSelfTrustDomain bool
}

// ParseSPIFFEID parses a SPIFFE ID with the following rules:
//
//   - `spiffe://<trust domain>` as a SPIFFE Trust Domain.
//   - `spiffe://<trust domain>/<path>` as an exact SPIFFE ID.
//   - `spiffe://<trust domain>/<prefix>/` as a SPIFFE ID prefix.
//     The trailing slash identifies a prefix.
//   - `spiffe://self` as a symbol for the SPIFFE Trust Domain of our own X509-SVID.
//   - `spiffe://self/<path>` as an exact SPIFFE ID in the trust domain of our
//     own X509-SVID.
//   - `spiffe://self/<prefix>/` as a SPIFFE ID prefix in the trust domain
//     of our own X509-SVID.
//
// At most one of ID, Prefix or TrustDomain is set. UseSelfTrustDomain is set
// alone for `spiffe://self`, and together with ID or Prefix for the
// `spiffe://self` path forms. At least one field of the SPIFFEPeerAuthorization
// will always be set i.e. this function never returns a zero-value
// SPIFFEPeerAuthorization.
func ParseSPIFFEID(value string) (*SPIFFEPeerAuthorization, error) {
	// Is it a prefix?
	if prefix, ok := strings.CutSuffix(value, "/"); ok {
		id, err := spiffeid.FromString(prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid SPIFFE ID prefix '%s': %w", value, err)
		}
		if id.Path() == "" {
			return nil, fmt.Errorf("invalid SPIFFE ID prefix '%s': prefix must include a path", value)
		}
		return &SPIFFEPeerAuthorization{
			Prefix:             &id,
			UseSelfTrustDomain: isSelfTrustDomain(id),
		}, nil
	}

	// Not a prefix, parse it.
	id, err := spiffeid.FromString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid SPIFFE ID '%s': %w", value, err)
	}

	// Is it a trust domain only?
	if id.Path() == "" {
		if isSelfTrustDomain(id) {
			return &SPIFFEPeerAuthorization{UseSelfTrustDomain: true}, nil
		}
		trustDomain := id.TrustDomain()
		return &SPIFFEPeerAuthorization{TrustDomain: &trustDomain}, nil
	}

	// Exact SPIFFE ID.
	return &SPIFFEPeerAuthorization{
		ID:                 &id,
		UseSelfTrustDomain: isSelfTrustDomain(id),
	}, nil
}

// isSelfTrustDomain returns true when the ID belongs to the `self` symbol
// trust domain.
func isSelfTrustDomain(id spiffeid.ID) bool {
	return id.TrustDomain().String() == SPIFFETrustDomainSelf
}
