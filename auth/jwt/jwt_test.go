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

			signed, err := key.Issue("https://issuer", "my-subject", "my-audience", 10*time.Second)
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
		signed, err := key.Issue("iss", "sub", "aud", time.Second)
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
