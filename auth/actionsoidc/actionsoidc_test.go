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

package actionsoidc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fluxcd/pkg/auth/actionsoidc"
)

// makeJWT mints an unsigned-but-parseable JWT carrying the given exp claim.
func makeJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": exp.Unix()})
	signed, err := tok.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to mint test JWT: %v", err)
	}
	return signed
}

func TestFetchToken(t *testing.T) {
	t.Run("fetches token", func(t *testing.T) {
		idToken := makeJWT(t, time.Now().Add(time.Hour))

		var gotAudience, gotAuth, gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAudience = r.URL.Query().Get("audience")
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"value":"` + idToken + `"}`))
		}))
		defer srv.Close()

		t.Setenv(actionsoidc.EnvRequestURL, srv.URL+"/token?api-version=2.0")
		t.Setenv(actionsoidc.EnvRequestToken, "request-token")

		token, err := actionsoidc.FetchToken(context.Background(), "my-audience")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != idToken {
			t.Errorf("token = %q, want %q", token, idToken)
		}
		if gotAudience != "my-audience" {
			t.Errorf("audience query = %q, want my-audience", gotAudience)
		}
		if gotAuth != "bearer request-token" {
			t.Errorf("Authorization = %q, want bearer request-token", gotAuth)
		}
		if gotPath != "/token" {
			t.Errorf("path = %q, want /token (base path preserved)", gotPath)
		}
	})

	t.Run("errors when env vars are unset", func(t *testing.T) {
		t.Setenv(actionsoidc.EnvRequestURL, "")
		t.Setenv(actionsoidc.EnvRequestToken, "")
		_, err := actionsoidc.FetchToken(context.Background(), "aud")
		if err == nil || !strings.Contains(err.Error(), actionsoidc.EnvRequestURL) {
			t.Fatalf("expected error mentioning %s, got: %v", actionsoidc.EnvRequestURL, err)
		}
	})

	t.Run("errors on non-200 response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("denied"))
		}))
		defer srv.Close()
		t.Setenv(actionsoidc.EnvRequestURL, srv.URL)
		t.Setenv(actionsoidc.EnvRequestToken, "request-token")

		_, err := actionsoidc.FetchToken(context.Background(), "aud")
		if err == nil || !strings.Contains(err.Error(), "denied") {
			t.Fatalf("expected error containing response body, got: %v", err)
		}
	})

	t.Run("errors on empty value", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"value":""}`))
		}))
		defer srv.Close()
		t.Setenv(actionsoidc.EnvRequestURL, srv.URL)
		t.Setenv(actionsoidc.EnvRequestToken, "request-token")

		_, err := actionsoidc.FetchToken(context.Background(), "aud")
		if err == nil || !strings.Contains(err.Error(), "did not contain a token") {
			t.Fatalf("expected empty-token error, got: %v", err)
		}
	})
}
