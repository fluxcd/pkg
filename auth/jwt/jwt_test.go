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

package jwt_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/fluxcd/pkg/auth/jwt"
)

func marshalJWK(t *testing.T, key jose.JSONWebKey) string {
	t.Helper()
	b, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("failed to marshal JWK: %v", err)
	}
	return string(b)
}

func TestParseJWK_Errors(t *testing.T) {
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		jwk     string
		wantErr string
	}{
		{
			name:    "not json",
			jwk:     "{not json",
			wantErr: "failed to parse JWK",
		},
		{
			name:    "rsa rejected as ambiguous",
			jwk:     marshalJWK(t, jose.JSONWebKey{Key: rsaPriv, KeyID: "a", Algorithm: "RS256"}),
			wantErr: "unsupported JWK key type *rsa.PrivateKey",
		},
		{
			name:    "public key only",
			jwk:     marshalJWK(t, jose.JSONWebKey{Key: edPriv.Public(), KeyID: "a", Algorithm: "EdDSA"}),
			wantErr: "unsupported JWK key type ed25519.PublicKey",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jwt.ParseJWK(tt.jwk)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestSigningKey_Issue(t *testing.T) {
	ed25519Pub, ed25519Priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	es256 := ecKey(t, elliptic.P256())
	es384 := ecKey(t, elliptic.P384())
	es521 := ecKey(t, elliptic.P521())

	tests := []struct {
		name    string
		priv    crypto.PrivateKey
		pub     crypto.PublicKey
		wantAlg string
	}{
		{"ed25519", ed25519Priv, ed25519Pub, "EdDSA"},
		{"ecdsa P-256", es256.priv, es256.pub, "ES256"},
		{"ecdsa P-384", es384.priv, es384.pub, "ES384"},
		{"ecdsa P-521", es521.priv, es521.pub, "ES512"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const kid = "my-key"
			jwk := marshalJWK(t, jose.JSONWebKey{Key: tt.priv, KeyID: kid})

			key, err := jwt.ParseJWK(jwk)
			if err != nil {
				t.Fatalf("ParseJWK: %v", err)
			}

			signed, err := key.Issue("https://issuer", "my-subject", []string{"my-audience"}, 10*time.Second)
			if err != nil {
				t.Fatalf("Issue: %v", err)
			}

			claims := gojwt.MapClaims{}
			tok, err := gojwt.NewParser().ParseWithClaims(signed, claims, func(*gojwt.Token) (any, error) {
				return tt.pub, nil
			})
			if err != nil {
				t.Fatalf("token failed to verify: %v", err)
			}

			if tok.Method.Alg() != tt.wantAlg {
				t.Errorf("alg = %q, want %q", tok.Method.Alg(), tt.wantAlg)
			}
			if got := tok.Header["kid"]; got != kid {
				t.Errorf("kid header = %v, want %q", got, kid)
			}

			if got, _ := claims.GetIssuer(); got != "https://issuer" {
				t.Errorf("iss = %q", got)
			}
			if got, _ := claims.GetSubject(); got != "my-subject" {
				t.Errorf("sub = %q", got)
			}
			if got, _ := claims.GetAudience(); len(got) != 1 || got[0] != "my-audience" {
				t.Errorf("aud = %v", got)
			}
			iat, _ := claims.GetIssuedAt()
			nbf, _ := claims.GetNotBefore()
			exp, _ := claims.GetExpirationTime()
			if iat == nil || nbf == nil || exp == nil {
				t.Fatalf("missing time claims: iat=%v nbf=%v exp=%v", iat, nbf, exp)
			}
			if want := iat.Add(-30 * time.Second); !nbf.Equal(want) {
				t.Errorf("nbf = %s, want %s (iat backdated 30s)", nbf, want)
			}
			if d := exp.Sub(iat.Time); d != 10*time.Second {
				t.Errorf("lifetime = %s, want 10s", d)
			}
			if jti, ok := claims["jti"].(string); !ok || jti == "" {
				t.Errorf("jti = %v, want non-empty string", claims["jti"])
			}
		})
	}
}

func TestSigningKey_Issue_MultipleAudiences(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := jwt.ParseJWK(marshalJWK(t, jose.JSONWebKey{Key: priv, KeyID: "k"}))
	if err != nil {
		t.Fatalf("ParseJWK: %v", err)
	}

	signed, err := key.Issue("iss", "sub", []string{"aud-a", "aud-b"}, time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims := gojwt.MapClaims{}
	if _, _, err := gojwt.NewParser().ParseUnverified(signed, claims); err != nil {
		t.Fatalf("ParseUnverified: %v", err)
	}
	// RFC 7519: several audiences are carried as a JSON array.
	if raw, ok := claims["aud"].([]any); !ok || len(raw) != 2 || raw[0] != "aud-a" || raw[1] != "aud-b" {
		t.Errorf("aud = %v, want [aud-a aud-b]", claims["aud"])
	}
}

func TestSigningKey_Issue_FreshJTIPerCall(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk := marshalJWK(t, jose.JSONWebKey{Key: priv, KeyID: "k"})
	key, err := jwt.ParseJWK(jwk)
	if err != nil {
		t.Fatalf("ParseJWK: %v", err)
	}

	seen := make(map[string]bool)
	for range 10 {
		signed, err := key.Issue("iss", "sub", []string{"aud"}, time.Second)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		claims := gojwt.MapClaims{}
		if _, _, err := gojwt.NewParser().ParseUnverified(signed, claims); err != nil {
			t.Fatalf("ParseUnverified: %v", err)
		}
		jti, _ := claims["jti"].(string)
		if seen[jti] {
			t.Fatalf("jti %q reused", jti)
		}
		seen[jti] = true
	}
}

type ecPair struct {
	priv *ecdsa.PrivateKey
	pub  *ecdsa.PublicKey
}

// ecKey generates a deterministic-per-call ECDSA key pair on the given curve.
func ecKey(t *testing.T, curve elliptic.Curve) ecPair {
	t.Helper()
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}
	return ecPair{priv: priv, pub: &priv.PublicKey}
}

// TestSigningKey_Issue_MLDSA covers the RFC 9964 AKP key type for all three
// ML-DSA parameter sets. The JWK carries no "crv", so the "alg" member is the
// only description of which parameter set the key belongs to.
func TestSigningKey_Issue_MLDSA(t *testing.T) {
	tests := []struct {
		name string
		alg  jose.SignatureAlgorithm
		pars mldsa.Parameters
	}{
		{"ML-DSA-44", jose.MLDSA44, mldsa.MLDSA44()},
		{"ML-DSA-65", jose.MLDSA65, mldsa.MLDSA65()},
		{"ML-DSA-87", jose.MLDSA87, mldsa.MLDSA87()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priv, err := mldsa.GenerateKey(tt.pars)
			if err != nil {
				t.Fatal(err)
			}
			const kid = "pq-key"
			jwk := marshalJWK(t, jose.JSONWebKey{
				Key:       priv,
				KeyID:     kid,
				Algorithm: string(tt.alg),
			})

			key, err := jwt.ParseJWK(jwk)
			if err != nil {
				t.Fatalf("ParseJWK: %v", err)
			}

			signed, err := key.Issue("https://issuer", "my-subject", []string{"my-audience"}, time.Minute)
			if err != nil {
				t.Fatalf("Issue: %v", err)
			}

			obj, err := jose.ParseSigned(signed, []jose.SignatureAlgorithm{tt.alg})
			if err != nil {
				t.Fatalf("ParseSigned: %v", err)
			}
			if len(obj.Signatures) != 1 {
				t.Fatalf("signatures = %d, want 1", len(obj.Signatures))
			}
			if got := obj.Signatures[0].Header.KeyID; got != kid {
				t.Errorf("kid header = %q, want %q", got, kid)
			}
			if got := obj.Signatures[0].Header.ExtraHeaders[jose.HeaderType]; got != "JWT" {
				t.Errorf("typ header = %v, want JWT", got)
			}

			payload, err := obj.Verify(priv.Public())
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			claims := map[string]any{}
			if err := json.Unmarshal(payload, &claims); err != nil {
				t.Fatalf("unmarshal claims: %v", err)
			}
			if got := claims["iss"]; got != "https://issuer" {
				t.Errorf("iss = %v", got)
			}
			aud, ok := claims["aud"].([]any)
			if !ok || len(aud) != 1 || aud[0] != "my-audience" {
				t.Errorf("aud = %v, want [my-audience]", claims["aud"])
			}
		})
	}
}

// TestParseJWK_AKPAlgorithm proves the AKP "alg" member is required and must
// name an ML-DSA algorithm, since an AKP key carries no "crv" to fall back on.
func TestParseJWK_AKPAlgorithm(t *testing.T) {
	priv, err := mldsa.GenerateKey(mldsa.MLDSA44())
	if err != nil {
		t.Fatal(err)
	}
	valid := marshalJWK(t, jose.JSONWebKey{
		Key:       priv,
		KeyID:     "k",
		Algorithm: string(jose.MLDSA44),
	})

	mutate := func(t *testing.T, fn func(map[string]any)) string {
		t.Helper()
		raw := map[string]any{}
		if err := json.Unmarshal([]byte(valid), &raw); err != nil {
			t.Fatal(err)
		}
		fn(raw)
		b, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	tests := []struct {
		name    string
		jwk     string
		wantErr string
	}{
		{
			name:    "missing alg",
			jwk:     mutate(t, func(m map[string]any) { delete(m, "alg") }),
			wantErr: "unsupported AKP JWK algorithm",
		},
		{
			name:    "non ML-DSA alg",
			jwk:     mutate(t, func(m map[string]any) { m["alg"] = "ES256" }),
			wantErr: "unsupported AKP JWK algorithm",
		},
		{
			name:    "unregistered ML-DSA variant",
			jwk:     mutate(t, func(m map[string]any) { m["alg"] = "ML-DSA-128" }),
			wantErr: "unsupported AKP JWK algorithm",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jwt.ParseJWK(tt.jwk)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}
