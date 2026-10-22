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
	"strings"
	"testing"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
)

type fakeTLSSource struct {
	svid *x509svid.SVID
}

func (f fakeTLSSource) GetX509SVID() (*x509svid.SVID, error) {
	return f.svid, nil
}

func (f fakeTLSSource) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return nil, nil
}

func TestPeerMatcherInvalidAuthorization(t *testing.T) {
	_, err := peerMatcher(nil, cryptov1.SPIFFEPeerAuthorization{})
	if err == nil {
		t.Fatal("expected an error for an empty SPIFFE peer authorization")
	}
}

func TestPeerMatcherSelfInvalidTrustDomain(t *testing.T) {
	source := fakeTLSSource{svid: &x509svid.SVID{}}
	id := spiffeid.RequireFromString("spiffe://example.org/foo")
	prefix := spiffeid.RequireFromString("spiffe://example.org/ns")

	tests := []struct {
		name string
		peer cryptov1.SPIFFEPeerAuthorization
	}{
		{
			name: "exact ID",
			peer: cryptov1.SPIFFEPeerAuthorization{
				ID:                 &id,
				UseSelfTrustDomain: true,
			},
		},
		{
			name: "prefix",
			peer: cryptov1.SPIFFEPeerAuthorization{
				Prefix:             &prefix,
				UseSelfTrustDomain: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := peerMatcher(source, tt.peer)
			if err == nil {
				t.Fatal("expected an error for an empty self trust domain")
			}
			if !strings.Contains(err.Error(), "self trust domain path") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
