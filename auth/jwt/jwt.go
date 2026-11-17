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
// The signing algorithm is derived from the key type for key types that map to a
// single unambiguous algorithm:
//
//	ed25519.PrivateKey  -> EdDSA
//	*ecdsa.PrivateKey   -> ES256 / ES384 / ES512 (by curve: P-256 / P-384 / P-521)
//	*mldsa.PrivateKey   -> ML-DSA-44 / ML-DSA-65 / ML-DSA-87 (from the JWK "alg")
//
// ML-DSA keys use the RFC 9964 "AKP" key type, whose key material does not
// identify a parameter set: "kty":"AKP" with opaque "pub"/"priv" bytes may be
// any of ML-DSA-44, ML-DSA-65 or ML-DSA-87. RFC 9964 therefore requires the
// "alg" member on every AKP key, and for AKP it is the only source of the
// algorithm. It is still never trusted on its own: it must name an ML-DSA
// algorithm and the key must be an ML-DSA key, so it cannot disagree with the
// key.
//
// RSA is intentionally unsupported: an RSA key does not determine a single
// algorithm (RS256/384/512, PS256/384/512), so signing one would require the
// library to pick on the caller's behalf.
package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// SigningKey is a private signing key, parsed from a JWK, that mints signed JWTs
// using the algorithm determined by the key.
type SigningKey struct {
	key any
	alg jose.SignatureAlgorithm
	kid string
}

// ParseJWK parses jwk, a single JSON Web Key, and returns its private signing
// key. The key must be of a type that maps to a single signing algorithm: an
// Ed25519 private key (kty "OKP", crv "Ed25519"), an ECDSA private key (kty
// "EC", crv "P-256", "P-384", or "P-521"), or an ML-DSA private key (kty "AKP",
// RFC 9964), all carrying the private component. RSA keys are rejected because
// their algorithm is ambiguous.
//
// An AKP key carries no "crv", so its "alg" member is required and selects the
// ML-DSA parameter set. For every other key type "alg" is ignored.
func ParseJWK(jwk string) (*SigningKey, error) {
	// Validate the AKP "alg" member before unmarshalling: an AKP key cannot be
	// decoded without it, and go-jose's error for a missing or unknown value
	// does not say what is wrong.
	var probe struct {
		Kty string `json:"kty"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal([]byte(jwk), &probe); err == nil && probe.Kty == "AKP" {
		switch jose.SignatureAlgorithm(probe.Alg) {
		case jose.MLDSA44, jose.MLDSA65, jose.MLDSA87:
		default:
			return nil, fmt.Errorf("unsupported AKP JWK algorithm %q: "+
				"want ML-DSA-44, ML-DSA-65 or ML-DSA-87", probe.Alg)
		}
	}

	var k jose.JSONWebKey
	if err := json.Unmarshal([]byte(jwk), &k); err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %w", err)
	}

	alg, err := signingAlgorithmForKey(k.Key, k.Algorithm)
	if err != nil {
		return nil, err
	}

	return &SigningKey{key: k.Key, alg: alg, kid: k.KeyID}, nil
}

// signingAlgorithmForKey returns the signing algorithm for the private key. It
// is derived from the key type, except for ML-DSA where RFC 9964 makes the JWK
// "alg" member the only description of the parameter set. It errors for key
// types whose algorithm is not unambiguous (RSA) or that are not private
// signing keys.
func signingAlgorithmForKey(key any, jwkAlg string) (jose.SignatureAlgorithm, error) {
	switch k := key.(type) {
	case ed25519.PrivateKey:
		return jose.EdDSA, nil
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P256():
			return jose.ES256, nil
		case elliptic.P384():
			return jose.ES384, nil
		case elliptic.P521():
			return jose.ES512, nil
		default:
			return "", fmt.Errorf("unsupported ECDSA curve %q", k.Curve.Params().Name)
		}
	case *mldsa.PrivateKey:
		switch alg := jose.SignatureAlgorithm(jwkAlg); alg {
		case jose.MLDSA44, jose.MLDSA65, jose.MLDSA87:
			return alg, nil
		default:
			return "", fmt.Errorf("unsupported AKP JWK algorithm %q: "+
				"want ML-DSA-44, ML-DSA-65 or ML-DSA-87", jwkAlg)
		}
	default:
		return "", fmt.Errorf("unsupported JWK key type %T: "+
			"want an Ed25519, ECDSA (P-256/P-384/P-521) or ML-DSA private key", key)
	}
}

// clockSkewLeeway backdates the "nbf" claim so a verifier whose clock runs
// slightly behind the issuer's does not reject a freshly minted token as not yet
// valid. It does not extend "exp": the token still expires ttl after issuance.
const clockSkewLeeway = 30 * time.Second

// registeredClaims is the RFC 7519 claim set Issue emits. It mirrors the field
// order and encoding of the standard claims.
type registeredClaims struct {
	Issuer    string   `json:"iss,omitempty"`
	Subject   string   `json:"sub,omitempty"`
	Audience  []string `json:"aud,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	NotBefore int64    `json:"nbf,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	ID        string   `json:"jti,omitempty"`
}

// Issue mints a compact-serialized JWT signed with the key, using the algorithm
// determined by the key. The signing key's id is set in the "kid" header field.
// The token carries all seven registered claims (RFC 7519): iss, sub, and
// every audience in auds, iat at the current time, nbf backdated by a small
// clock-skew leeway, exp ttl after issuance, and a random jti. aud is always
// encoded as a JSON array, which RFC 7519 permits for any number of audiences.
func (k *SigningKey) Issue(iss, sub string, auds []string, ttl time.Duration) (string, error) {
	jti, err := newJTI()
	if err != nil {
		return "", err
	}

	now := time.Now()
	payload, err := json.Marshal(registeredClaims{
		Issuer:    iss,
		Subject:   sub,
		Audience:  auds,
		IssuedAt:  now.Unix(),
		NotBefore: now.Add(-clockSkewLeeway).Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		ID:        jti,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}

	opts := (&jose.SignerOptions{}).WithType("JWT")
	if k.kid != "" {
		opts = opts.WithHeader("kid", k.kid)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: k.alg, Key: k.key}, opts)
	if err != nil {
		return "", fmt.Errorf("failed to create signer: %w", err)
	}

	obj, err := signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}
	signed, err := obj.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("failed to serialize JWT: %w", err)
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
