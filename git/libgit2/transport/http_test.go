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

package transport

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	git2go "github.com/libgit2/git2go/v34"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/libgit2/internal/test"
	"github.com/fluxcd/pkg/gittestserver"
)

func TestHttpAction_CreateClientRequest(t *testing.T) {
	authOpts := git.AuthOptions{
		Username: "user",
		Password: "pwd",
	}
	url := "https://final-target/abc"

	tests := []struct {
		name       string
		assertFunc func(g *WithT, req *http.Request, client *http.Client)
		action     git2go.SmartServiceAction
		authOpts   git.AuthOptions
		transport  *http.Transport
		wantedErr  error
	}{
		{
			name:   "Uploadpack: URL, method and headers are correctly set",
			action: git2go.SmartServiceActionUploadpack,
			transport: &http.Transport{
				Proxy:              http.ProxyFromEnvironment,
				ProxyConnectHeader: map[string][]string{},
			},
			assertFunc: func(g *WithT, req *http.Request, _ *http.Client) {
				g.Expect(req.URL.String()).To(Equal(url + uploadPackAction))
				g.Expect(req.Method).To(Equal(http.MethodPost))
				g.Expect(req.Header).To(BeEquivalentTo(map[string][]string{
					"User-Agent":   {libgit2UserAgent},
					"Content-Type": {uploadPackContentType},
				}))
			},
			wantedErr: nil,
		},
		{
			name:      "UploadpackLs: URL, method and headers are correctly set",
			action:    git2go.SmartServiceActionUploadpackLs,
			transport: &http.Transport{},
			assertFunc: func(g *WithT, req *http.Request, _ *http.Client) {
				g.Expect(req.URL.String()).To(Equal(url + uploadPackLSAction))
				g.Expect(req.Method).To(Equal(http.MethodGet))
				g.Expect(req.Header).To(BeEquivalentTo(map[string][]string{
					"User-Agent": {libgit2UserAgent},
				}))
			},
			wantedErr: nil,
		},
		{
			name:   "Receivepack: URL, method and headers are correctly set",
			action: git2go.SmartServiceActionReceivepack,
			transport: &http.Transport{
				Proxy:              http.ProxyFromEnvironment,
				ProxyConnectHeader: map[string][]string{},
			},
			assertFunc: func(g *WithT, req *http.Request, _ *http.Client) {
				g.Expect(req.URL.String()).To(Equal(url + receivePackAction))
				g.Expect(req.Method).To(Equal(http.MethodPost))
				g.Expect(req.Header).To(BeEquivalentTo(map[string][]string{
					"Content-Type": {receivePackContentType},
					"User-Agent":   {libgit2UserAgent},
				}))
			},
			wantedErr: nil,
		},
		{
			name:      "ReceivepackLs: URL, method and headars are correctly set",
			action:    git2go.SmartServiceActionReceivepackLs,
			transport: &http.Transport{},
			assertFunc: func(g *WithT, req *http.Request, _ *http.Client) {
				g.Expect(req.URL.String()).To(Equal(url + receivePackLSAction))
				g.Expect(req.Method).To(Equal(http.MethodGet))
				g.Expect(req.Header).To(BeEquivalentTo(map[string][]string{
					"User-Agent": {libgit2UserAgent},
				}))
			},
			wantedErr: nil,
		},
		{
			name:   "credentials are correctly configured",
			action: git2go.SmartServiceActionUploadpack,
			transport: &http.Transport{
				Proxy:              http.ProxyFromEnvironment,
				ProxyConnectHeader: map[string][]string{},
			},
			authOpts: authOpts,
			assertFunc: func(g *WithT, req *http.Request, _ *http.Client) {
				g.Expect(req.URL.String()).To(Equal(url + uploadPackAction))
				g.Expect(req.Method).To(Equal(http.MethodPost))

				username, pwd, ok := req.BasicAuth()
				if !ok {
					t.Errorf("could not find Authentication header in request.")
				}
				g.Expect(username).To(Equal("user"))
				g.Expect(pwd).To(Equal("pwd"))
			},
			wantedErr: nil,
		},
		{
			name:      "error when no http.transport provided",
			action:    git2go.SmartServiceActionUploadpack,
			transport: nil,
			wantedErr: fmt.Errorf("failed to create client: transport cannot be nil"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			client, req, err := createClientRequest(url, tt.action, tt.transport, &tt.authOpts)
			if err != nil {
				t.Log(err)
			}
			if tt.wantedErr != nil {
				g.Expect(err).To(Equal(tt.wantedErr))
			} else {
				tt.assertFunc(g, req, client)
			}

		})
	}
}

func TestHTTPManagedTransport_E2E(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())

	user := "test-user"
	pwd := "test-pswd"
	server.Auth(user, pwd)
	server.KeyDir(filepath.Join(server.Root(), "keys"))

	err = server.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())
	defer server.StopHTTP()

	// Force managed transport to be enabled
	InitManagedTransport()

	repoPath := "test.git"
	err = server.InitRepo("../../testdata/git/repo", git.DefaultBranch, repoPath)
	g.Expect(err).ToNot(HaveOccurred())

	tmpDir := t.TempDir()

	// Register the auth options and target url mapped to a unique url.
	id := "http://obj-id"
	AddTransportOptions(id, TransportOptions{
		TargetURL: server.HTTPAddress() + "/" + repoPath,
		AuthOpts: &git.AuthOptions{
			Username: user,
			Password: pwd,
		},
	})

	// We call git2go.Clone with transportOptsURL instead of the actual URL,
	// as the transport action will fetch the actual URL and the required
	// credentials using the it as an identifier.
	repo, err := git2go.Clone(id, tmpDir, &git2go.CloneOptions{
		CheckoutOptions: git2go.CheckoutOptions{
			Strategy: git2go.CheckoutForce,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer repo.Free()

	_, err = test.CommitFile(repo, "test-file", "testing push", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	err = push(tmpDir, git.DefaultBranch)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestTrimActionSuffix(t *testing.T) {
	url := "https://gitlab/repo/podinfo.git"
	tests := []struct {
		name    string
		inURL   string
		wantURL string
	}{
		{
			name:    "ignore other suffixes",
			inURL:   url + "/somethinelese",
			wantURL: url + "/somethinelese",
		},
		{
			name:    "trim" + uploadPackLSAction,
			inURL:   url + uploadPackLSAction,
			wantURL: url,
		},
		{
			name:    "trim" + uploadPackAction,
			inURL:   url + uploadPackAction,
			wantURL: url,
		},
		{
			name:    "trim" + receivePackLSAction,
			inURL:   url + receivePackLSAction,
			wantURL: url,
		},
		{
			name:    "trim" + receivePackAction,
			inURL:   url + receivePackAction,
			wantURL: url,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			gotURL := trimActionSuffix(tt.inURL)
			g.Expect(gotURL).To(Equal(tt.wantURL))
		})
	}
}

func TestHTTPManagedTransport_HandleRedirect(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
	}{
		{name: "http to https", repoURL: "http://github.com/fluxcd/flux2-multi-tenancy"},
		{name: "handle gitlab redirect", repoURL: "https://gitlab.com/stefanprodan/podinfo"},
	}

	// Force managed transport to be enabled
	InitManagedTransport()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tmpDir := t.TempDir()

			id := "http://obj-id"
			AddTransportOptions(id, TransportOptions{
				TargetURL: tt.repoURL,
			})

			// GitHub will cause a 301 and redirect to https
			repo, err := git2go.Clone(id, tmpDir, &git2go.CloneOptions{
				CheckoutOptions: git2go.CheckoutOptions{
					Strategy: git2go.CheckoutForce,
				},
			})

			g.Expect(err).ToNot(HaveOccurred())
			repo.Free()
		})
	}
}
