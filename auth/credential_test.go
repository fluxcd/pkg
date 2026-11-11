/*
Copyright 2025 The Flux authors

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

package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fluxcd/pkg/auth"
)

type mockCredential struct {
	token string
}

func (m *mockCredential) GetDuration() time.Duration {
	return time.Hour
}

func TestJWTCredential_Identity(t *testing.T) {
	mint := func(t *testing.T, claims jwt.MapClaims) string {
		t.Helper()
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return token
	}
	identity := func(token string) string {
		return (&auth.JWTCredential{Token: token}).Identity()
	}
	base := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss":    "https://issuer.example.com",
			"sub":    "subject",
			"aud":    "audience",
			"custom": "value",
			"jti":    "token-id",
			"exp":    time.Now().Add(time.Hour).Unix(),
			"iat":    time.Now().Unix(),
		}
	}

	first := mint(t, base())
	if a, b := identity(first), identity(first); a != b {
		t.Errorf("identity is not stable for the same token: %q != %q", a, b)
	}

	for _, mutate := range []func(jwt.MapClaims){
		func(c jwt.MapClaims) { c["iss"] = "https://other-issuer.example.com" },
		func(c jwt.MapClaims) { c["sub"] = "other-subject" },
		func(c jwt.MapClaims) { c["aud"] = "other-audience" },
		func(c jwt.MapClaims) { c["custom"] = "other-value" },
		func(c jwt.MapClaims) { c["jti"] = "other-token-id" },
		func(c jwt.MapClaims) { c["exp"] = time.Now().Add(2 * time.Hour).Unix() },
		func(c jwt.MapClaims) { c["iat"] = time.Now().Add(time.Minute).Unix() },
	} {
		claims := base()
		mutate(claims)
		if a, b := identity(first), identity(mint(t, claims)); a == b {
			t.Errorf("identity did not change for mutated claims")
		}
	}

	if a, b := identity("invalid-token"), identity("invalid-token"); a == "" || a != b {
		t.Errorf("malformed token identity = %q, want non-empty and stable", a)
	}
}
