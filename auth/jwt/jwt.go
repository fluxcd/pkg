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

// Package jwt issues self-signed JSON Web Tokens. It parses a private signing
// key from a JSON Web Key (JWK) once and mints compact-serialized tokens on
// demand, stamping the key's id into the token header so verifiers can locate the
// matching public key.
//
// The signing algorithm is derived from the key type, never chosen by the caller
// or read from the JWK's "alg" field, so it can never disagree with the key. Only
// key types that map to a single unambiguous algorithm are supported:
//
//	ed25519.PrivateKey  -> EdDSA
//	*ecdsa.PrivateKey   -> ES256 / ES384 / ES512 (by curve: P-256 / P-384 / P-521)
//
// RSA is intentionally unsupported: an RSA key does not determine a single
// algorithm (RS256/384/512, PS256/384/512), so signing one would require the
// library to pick on the caller's behalf.
package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	gojwt "github.com/golang-jwt/jwt/v5"
)

// SigningKey is a private signing key, parsed from a JWK, that mints signed JWTs
// using the algorithm determined by the key type.
type SigningKey struct {
	key    any
	method gojwt.SigningMethod
	kid    string
}

// ParseJWK parses jwk, a single JSON Web Key, and returns its private signing
// key. The key must be of a type that maps to a single signing algorithm: an
// Ed25519 private key (kty "OKP", crv "Ed25519") or an ECDSA private key (kty
// "EC", crv "P-256", "P-384", or "P-521"), both carrying the private "d"
// component. RSA keys are rejected because their algorithm is ambiguous.
func ParseJWK(jwk string) (*SigningKey, error) {
	var k jose.JSONWebKey
	if err := json.Unmarshal([]byte(jwk), &k); err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %w", err)
	}

	method, err := signingMethodForKey(k.Key)
	if err != nil {
		return nil, err
	}

	return &SigningKey{key: k.Key, method: method, kid: k.KeyID}, nil
}

// signingMethodForKey returns the signing method uniquely determined by the
// private key's type. It errors for key types whose algorithm is not unambiguous
// (RSA) or that are not private signing keys.
func signingMethodForKey(key any) (gojwt.SigningMethod, error) {
	switch k := key.(type) {
	case ed25519.PrivateKey:
		return gojwt.SigningMethodEdDSA, nil
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P256():
			return gojwt.SigningMethodES256, nil
		case elliptic.P384():
			return gojwt.SigningMethodES384, nil
		case elliptic.P521():
			return gojwt.SigningMethodES512, nil
		default:
			return nil, fmt.Errorf("unsupported ECDSA curve %q", k.Curve.Params().Name)
		}
	default:
		return nil, fmt.Errorf("unsupported JWK key type %T: "+
			"want an Ed25519 or ECDSA (P-256/P-384/P-521) private key", key)
	}
}

// clockSkewLeeway backdates the "nbf" claim so a verifier whose clock runs
// slightly behind the issuer's does not reject a freshly minted token as not yet
// valid. It does not extend "exp": the token still expires ttl after issuance.
const clockSkewLeeway = 30 * time.Second

// Issue mints a compact-serialized JWT signed with the key, using the algorithm
// determined by the key type. The signing key's id is set in the "kid" header
// field. The token carries all seven registered claims (RFC 7519): iss, sub, and
// aud as given, iat at the current time, nbf backdated by a small clock-skew
// leeway, exp ttl after issuance, and a random jti.
func (k *SigningKey) Issue(iss, sub, aud string, ttl time.Duration) (string, error) {
	jti, err := newJTI()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := gojwt.RegisteredClaims{
		Issuer:    iss,
		Subject:   sub,
		Audience:  gojwt.ClaimStrings{aud},
		IssuedAt:  gojwt.NewNumericDate(now),
		NotBefore: gojwt.NewNumericDate(now.Add(-clockSkewLeeway)),
		ExpiresAt: gojwt.NewNumericDate(now.Add(ttl)),
		ID:        jti,
	}

	tok := gojwt.NewWithClaims(k.method, claims)
	tok.Header["kid"] = k.kid

	signed, err := tok.SignedString(k.key)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}
	return signed, nil
}

// newJTI returns a random 128-bit token identifier as a hex string.
func newJTI() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("failed to generate JWT ID: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
