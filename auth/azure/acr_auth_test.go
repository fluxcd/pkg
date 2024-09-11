/*
Copyright 2023 The Flux authors

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

package azure

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
)

func TestProvider_GetACRAuthConfig(t *testing.T) {
	g := NewWithT(t)

	expiresAt := time.Now().UTC().Add(time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer:    "auth.microsoft.com",
		Subject:   "fluxcd",
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})

	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())
	tokenStr, err := token.SignedString(pk)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name            string
		tokenCredential azcore.TokenCredential
		responseBody    string
		statusCode      int
		wantErr         bool
		wantAuthConfig  authn.AuthConfig
	}{
		{
			name:            "success",
			tokenCredential: &FakeTokenCredential{Token: "foo"},
			responseBody:    fmt.Sprintf(`{"refresh_token": "%s"}`, tokenStr),
			statusCode:      http.StatusOK,
			wantAuthConfig: authn.AuthConfig{
				Username: "00000000-0000-0000-0000-000000000000",
				Password: tokenStr,
			},
		},
		{
			name:            "fail to get access token",
			tokenCredential: &FakeTokenCredential{Err: errors.New("no access token")},
			wantErr:         true,
		},
		{
			name:            "error from exchange service",
			tokenCredential: &FakeTokenCredential{Token: "foo"},
			responseBody:    `[{"code": "111","message": "error message 1"}]`,
			statusCode:      http.StatusInternalServerError,
			wantErr:         true,
		},
	}
	//
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			// Run a test server.
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})
			provider := NewProvider(WithCredential(tt.tokenCredential))
			auth, expiry, err := provider.GetACRAuthConfig(context.TODO(), srv.URL)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			if tt.statusCode == http.StatusOK {
				g.Expect(auth).To(Equal(tt.wantAuthConfig))
				g.Expect(time.Now().UTC().Add(expiry)).To(BeTemporally("~", expiresAt, time.Second))
			}
		})
	}
}

func TestGetScopeProviderOption(t *testing.T) {
	tests := []struct {
		host   string
		scopes []string
	}{
		{"foo.azurecr.io", []string{}},
		{"foo.azurecr.cn", []string{"https://management.chinacloudapi.cn/.default"}},
		{"foo.azurecr.de", []string{}},
		{"foo.azurecr.us", []string{"https://management.usgovcloudapi.net/.default"}},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			g := NewWithT(t)
			opt := GetScopeProiderOption(tt.host)
			if len(tt.scopes) == 0 {
				g.Expect(opt).To(BeNil())
			} else {
				scopes := NewProvider(opt).scopes
				g.Expect(scopes).To(Equal(tt.scopes))
			}
		})
	}
}

func Test_exchanger_ExchangeACRAccessToken(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		statusCode   int
		wantErr      bool
		wantToken    string
	}{
		{
			name: "successful",
			responseBody: `{
	"access_token": "aaaaa",
	"refresh_token": "bbbbb",
	"resource": "ccccc",
	"token_type": "ddddd"
}`,
			statusCode: http.StatusOK,
			wantToken:  "bbbbb",
		},
		{
			name:       "fail",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:         "invalid response",
			responseBody: "foo",
			statusCode:   http.StatusOK,
			wantErr:      true,
		},
		{
			name: "error response",
			responseBody: `[
	{
		"code": "111",
		"message": "error message 1"
	},
	{
		"code": "112",
		"message": "error message 2"
	}
]`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			ex := newExchanger(srv.URL)
			token, err := ex.ExchangeACRAccessToken("some-access-token")
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if tt.statusCode == http.StatusOK {
				g.Expect(token).To(Equal(tt.wantToken))
			}
		})
	}
}
