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

package spiffe

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
)

// TLSSource provides the X.509-SVID and trust bundles used to build a TLS
// configuration. It is implemented by *workloadapi.X509Source.
type TLSSource interface {
	x509svid.Source
	x509bundle.Source
}

// BuildTLSConfig returns a TLS configuration for mutual TLS with SPIFFE
// X.509-SVIDs. It presents the X.509-SVID from source and authorizes peers
// matching any of the given SPIFFE IDs, ID prefixes, trust domains or the
// `spiffe://self` forms, where the trust domain is taken from the local
// X.509-SVID.
//
// The X.509-SVID and trust bundle are read from source on every handshake, so
// rotations are picked up automatically. The returned configuration can be
// used for both TLS clients and servers. Callers that need different peer
// authorization in each direction should build a configuration for each
// direction. The TLS hostname is not used for peer verification; the SPIFFE
// identity is verified instead.
func BuildTLSConfig(source TLSSource, peers ...string) (*tls.Config, error) {
	if source == nil {
		return nil, errors.New("SPIFFE TLS source is required")
	}

	authorizer, err := buildAuthorizer(source, peers)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:           tls.VersionTLS13,
		GetCertificate:       tlsconfig.GetCertificate(source),
		GetClientCertificate: tlsconfig.GetClientCertificate(source),
		ClientAuth:           tls.RequireAnyClientCert,
		// Hostname verification does not apply to SPIFFE identities. The peer
		// certificate chain and identity are verified by VerifyPeerCertificate.
		InsecureSkipVerify:    true, //nolint:gosec
		VerifyPeerCertificate: tlsconfig.VerifyPeerCertificate(source, authorizer),
	}, nil
}

func buildAuthorizer(source TLSSource, peers []string) (tlsconfig.Authorizer, error) {
	if len(peers) == 0 {
		return nil, errors.New("at least one SPIFFE peer is required")
	}

	matchers := make([]spiffeid.Matcher, 0, len(peers))
	for i, value := range peers {
		peer, err := cryptov1.ParseSPIFFEID(value)
		if err != nil {
			return nil, fmt.Errorf("invalid SPIFFE peer at index %d: %w", i, err)
		}
		matcher, err := peerMatcher(source, *peer)
		if err != nil {
			return nil, fmt.Errorf("invalid SPIFFE peer at index %d: %w", i, err)
		}
		matchers = append(matchers, matcher)
	}
	return tlsconfig.AdaptMatcher(matchAnyOf(matchers...)), nil
}

func peerMatcher(source TLSSource, peer cryptov1.SPIFFEPeerAuthorization) (spiffeid.Matcher, error) {
	if peer.UseSelfTrustDomain {
		svid, err := source.GetX509SVID()
		if err != nil {
			return nil, fmt.Errorf("failed to get SPIFFE X.509-SVID: %w", err)
		}
		trustDomain := svid.ID.TrustDomain()
		switch {
		case peer.ID != nil:
			id, err := spiffeid.FromPath(trustDomain, peer.ID.Path())
			if err != nil {
				return nil, fmt.Errorf("failed to build SPIFFE ID from self trust domain path: %w", err)
			}
			return spiffeid.MatchID(id), nil
		case peer.Prefix != nil:
			id, err := spiffeid.FromPath(trustDomain, peer.Prefix.Path())
			if err != nil {
				return nil, fmt.Errorf("failed to build SPIFFE ID prefix from self trust domain path: %w", err)
			}
			return spiffeid.MatchIDPrefix(id), nil
		default:
			return spiffeid.MatchMemberOf(trustDomain), nil
		}
	}

	switch {
	case peer.ID != nil:
		return spiffeid.MatchID(*peer.ID), nil
	case peer.Prefix != nil:
		return spiffeid.MatchIDPrefix(*peer.Prefix), nil
	case peer.TrustDomain != nil:
		return spiffeid.MatchMemberOf(*peer.TrustDomain), nil
	default:
		return nil, fmt.Errorf("invalid SPIFFE peer authorization: no valid ID, prefix, or trust domain specified")
	}
}

func matchAnyOf(matchers ...spiffeid.Matcher) spiffeid.Matcher {
	return func(id spiffeid.ID) error {
		for _, match := range matchers {
			if err := match(id); err == nil {
				return nil
			}
		}
		return fmt.Errorf("unexpected SPIFFE ID %q", id)
	}
}
