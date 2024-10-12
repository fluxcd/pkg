/*
Copyright 2022 The Flux authors

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

package gcp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

const testValidGCRImage = "gcr.io/foo/bar:v1"

type fakeTokenSource struct {
	token *oauth2.Token
	err   error
}

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	return f.token, f.err
}

func TestGetLoginAuth(t *testing.T) {
	tests := []struct {
		name           string
		token          *oauth2.Token
		tokenErr       error
		wantErr        bool
		wantAuthConfig authn.AuthConfig
	}{
		{
			name: "success",
			token: &oauth2.Token{
				AccessToken: "some-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(10 * time.Second),
			},
			wantAuthConfig: authn.AuthConfig{
				Username: "oauth2accesstoken",
				Password: "some-token",
			},
		},
		{
			name:     "fail",
			tokenErr: fmt.Errorf("token error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create fake token source
			fakeTS := &fakeTokenSource{
				token: tt.token,
				err:   tt.tokenErr,
			}

			gc := NewClient().WithTokenSource(fakeTS)
			a, expiresAt, err := gc.getLoginAuth(context.TODO())
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if !tt.wantErr {
				g.Expect(expiresAt).To(BeTemporally("~", tt.token.Expiry, time.Second))
				g.Expect(a).To(Equal(tt.wantAuthConfig))
			}
		})
	}
}

func TestValidHost(t *testing.T) {
	tests := []struct {
		host   string
		result bool
	}{
		{"gcr.io", true},
		{"foo.gcr.io", true},
		{"foo-docker.pkg.dev", true},
		{"docker.io", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ValidHost(tt.host)).To(Equal(tt.result))
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name      string
		autoLogin bool
		image     string
		token     *oauth2.Token
		tokenErr  error
		testOIDC  bool
		wantErr   bool
	}{
		{
			name:      "no auto login",
			autoLogin: false,
			image:     testValidGCRImage,
			wantErr:   true,
		},
		{
			name:      "with auto login",
			autoLogin: true,
			image:     testValidGCRImage,
			testOIDC:  true,
			token: &oauth2.Token{
				AccessToken: "some-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(10 * time.Second),
			},
		},
		{
			name:      "login failure",
			autoLogin: true,
			image:     testValidGCRImage,
			tokenErr:  fmt.Errorf("token error"),
			testOIDC:  true,
			wantErr:   true,
		},
		{
			name:      "non GCR image",
			autoLogin: true,
			image:     "foo/bar:v1",
			token: &oauth2.Token{
				AccessToken: "some-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(10 * time.Second),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			ref, err := name.ParseReference(tt.image)
			g.Expect(err).ToNot(HaveOccurred())

			// Create fake token source
			fakeTS := &fakeTokenSource{
				token: tt.token,
				err:   tt.tokenErr,
			}

			gc := NewClient().WithTokenSource(fakeTS)

			_, err = gc.Login(context.TODO(), tt.autoLogin, tt.image, ref)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.testOIDC {
				_, err = gc.OIDCLogin(context.TODO())
				g.Expect(err != nil).To(Equal(tt.wantErr))
			}
		})
	}
}
