/*
Copyright 2024 The Flux authors

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

package git

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/github"
	. "github.com/onsi/gomega"
)

func TestGetCredentials(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)
	tests := []struct {
		name            string
		provider        *ProviderOptions
		wantCredentials *Credentials
		wantErr         error
	}{
		{
			name:    "nil provider options",
			wantErr: errors.New("provider options are not specified"),
		},
		{
			name: "invalid provider",
			provider: &ProviderOptions{
				Name: "invalid provider",
			},
			wantErr: errors.New("invalid provider"),
		},
		{
			name: "get credentials from azure",
			provider: &ProviderOptions{
				Name: ProviderAzure,
				AzureOpts: []azure.OptFunc{
					azure.WithCredential(&azure.FakeTokenCredential{
						Token:     "ado-token",
						ExpiresOn: expiresAt,
					}),
					azure.WithAzureDevOpsScope(),
				},
			},
			wantCredentials: &Credentials{
				BearerToken: "ado-token",
			},
		},
		{
			name: "get credentials from azure without scope",
			provider: &ProviderOptions{
				Name: ProviderAzure,
				AzureOpts: []azure.OptFunc{
					azure.WithCredential(&azure.FakeTokenCredential{
						Token:     "ado-token",
						ExpiresOn: expiresAt,
					}),
				},
			},
			wantCredentials: &Credentials{
				BearerToken: "ado-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			creds, expiry, err := GetCredentials(context.TODO(), tt.provider)

			if tt.wantCredentials != nil {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(*creds).To(Equal(*tt.wantCredentials))

				expectedCredBytes := []byte(tt.wantCredentials.BearerToken)
				receivedCredBytes := []byte(creds.BearerToken)
				g.Expect(receivedCredBytes).To(Equal(expectedCredBytes))
				g.Expect(expiry).To(Equal(expiresAt))
			} else {
				g.Expect(creds).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			}
		})
	}
}

func TestGetCredentials_GitHub(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)
	tests := []struct {
		name            string
		accessToken     *github.FakeAccessToken
		statusCode      int
		wantCredentials *Credentials
		wantErr         bool
	}{
		{
			name:       "get credentials from github success",
			statusCode: http.StatusOK,
			accessToken: &github.FakeAccessToken{
				Token:     "access-token",
				ExpiresAt: expiresAt,
			},
			wantCredentials: &Credentials{
				Username: GitHubAccessTokenUsername,
				Password: "access-token",
			},
		},
		{
			name:       "get credentials from github failure",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				var response []byte
				var err error
				if tt.accessToken != nil {
					response, err = json.Marshal(tt.accessToken)
					g.Expect(err).ToNot(HaveOccurred())
				}
				w.Write(response)
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			pk, err := github.CreateTestPrivateKey()
			g.Expect(err).ToNot(HaveOccurred())

			providerOpts := &ProviderOptions{
				Name: ProviderGitHub,
				GitHubOpts: []github.OptFunc{github.WithApiURL(srv.URL), github.WithAppID(123),
					github.WithInstllationID(456), github.WithPrivateKey(pk)},
			}

			creds, expiry, err := GetCredentials(context.TODO(), providerOpts)
			if tt.wantCredentials != nil {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(*creds).To(Equal(*tt.wantCredentials))

				g.Expect(creds.Username).To(Equal(tt.wantCredentials.Username))
				g.Expect(creds.Password).To(Equal(tt.wantCredentials.Password))
				g.Expect(expiry).To(Equal(expiresAt))
			} else {
				g.Expect(creds).To(BeNil())
				g.Expect(err).To(HaveOccurred())
			}
		})
	}

}
