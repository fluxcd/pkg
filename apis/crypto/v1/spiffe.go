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

const (
	// SPIFFETrustDomainSelf is a symbol for the SPIFFE Trust Domain of our own X509-SVID.
	SPIFFETrustDomainSelf = "self"
)

// SPIFFE annotation keys.
const (
	// AnnotationMTLSSPIFFEServerID opts a ServiceAccount into using SPIFFE mTLS
	// and specifies what the server SPIFFE ID should look like for authorization.
	// It can be one of:
	//
	//   - `spiffe://<trust domain>/<path>` as an exact SPIFFE ID.
	//   - `spiffe://<trust domain>/<path prefix>/` as a SPIFFE ID prefix.
	//     The trailing slash identifies a prefix.
	//   - `spiffe://<trust domain>` as a SPIFFE Trust Domain.
	//   - `spiffe://self` as a symbol for the SPIFFE Trust Domain of our own X509-SVID.
	AnnotationMTLSSPIFFEServerID MTLSSPIFFEServerID = GroupMTLSSPIFFE + "/server-id"
)

// SPIFFEPeerAuthorization is a SPIFFE ID parsed either from a
// `*.fluxcd.io` ServiceAccount annotation, or from a controller
// flag. Any field of this struct is only ever set alone. No
// two or more fields will be set simultaneously.
type SPIFFEPeerAuthorization struct {
	ID                 *spiffeid.ID
	Prefix             *spiffeid.ID
	TrustDomain        *spiffeid.TrustDomain
	UseSelfTrustDomain bool
}

// ParseSPIFFEID parses a SPIFFE ID with the following rules:
//
//   - `spiffe://<trust domain>/<path>` as an exact SPIFFE ID.
//   - `spiffe://<trust domain>/<path prefix>/` as a SPIFFE ID prefix.
//     The trailing slash identifies a prefix.
//   - `spiffe://<trust domain>` as a SPIFFE Trust Domain.
//   - `spiffe://self` as a symbol for the SPIFFE Trust Domain of our own X509-SVID.
//
// Exactly one field of the returned SPIFFEPeerAuthorization is set, one for each case above.
func ParseSPIFFEID(value string) (*SPIFFEPeerAuthorization, error) {
	if value == "spiffe://"+SPIFFETrustDomainSelf {
		return &SPIFFEPeerAuthorization{UseSelfTrustDomain: true}, nil
	}

	if prefix, ok := strings.CutSuffix(value, "/"); ok {
		id, err := spiffeid.FromString(prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid SPIFFE ID prefix '%s': %w", value, err)
		}
		if id.Path() == "" {
			return nil, fmt.Errorf("invalid SPIFFE ID prefix '%s': prefix must include a path", value)
		}
		return &SPIFFEPeerAuthorization{Prefix: &id}, nil
	}

	id, err := spiffeid.FromString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid SPIFFE ID '%s': %w", value, err)
	}
	if id.Path() == "" {
		trustDomain := id.TrustDomain()
		return &SPIFFEPeerAuthorization{TrustDomain: &trustDomain}, nil
	}
	return &SPIFFEPeerAuthorization{ID: &id}, nil
}

// MTLSSPIFFEServerID is the type for the SPIFFE mTLS server ID annotation.
type MTLSSPIFFEServerID string

// String implements the fmt.Stringer interface.
func (MTLSSPIFFEServerID) String() string {
	return string(AnnotationMTLSSPIFFEServerID)
}

// Extract parses the MTLSSPIFFEServerID annotation and returns the parsed
// SPIFFE peer authorization. A nil authorization is returned when the
// annotation is not set.
func (MTLSSPIFFEServerID) Extract(saAnnotations map[string]string) (*SPIFFEPeerAuthorization, error) {
	const key = string(AnnotationMTLSSPIFFEServerID)
	value, ok := saAnnotations[key]
	if !ok {
		return nil, nil
	}
	return ParseSPIFFEID(value)
}
