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
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/elazarl/goproxy"
	git2go "github.com/libgit2/git2go/v34"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/gittestserver"
)

type cleanupFunc func()

const repoPath = "bar/test-reponame"

func Test_HTTP_proxy(t *testing.T) {

	type testCase struct {
		name          string
		url           string
		proxyType     git2go.ProxyType
		setupGitProxy func(g *WithT, proxy *goproxy.ProxyHttpServer, proxiedRequests *int32) (*git.AuthOptions, cleanupFunc)
		wantUsedProxy bool
		tls           bool
	}

	g := NewWithT(t)

	// Get a free port for proxy to use.
	l, err := net.Listen("tcp", ":0")
	g.Expect(err).ToNot(HaveOccurred())
	proxyAddr := fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port)
	g.Expect(l.Close()).ToNot(HaveOccurred())

	// Create the git server to be used for hosts covered under NO_PROXY
	noProxyGitServer, err := setupGitServer(repoPath)
	g.Expect(err).ToNot(HaveOccurred())

	err = noProxyGitServer.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())
	defer noProxyGitServer.StopHTTP()

	tests := []testCase{
		{
			name:          "env var: HTTP_PROXY",
			url:           "http://example.com/bar/test-reponame",
			proxyType:     git2go.ProxyTypeAuto,
			setupGitProxy: setupHTTPGitProxy,
			wantUsedProxy: true,
		},
		{
			name:      "env var: HTTPS_PROXY",
			url:       "https://example.com/bar/test-reponame",
			proxyType: git2go.ProxyTypeAuto,
			setupGitProxy: func(g *WithT, proxy *goproxy.ProxyHttpServer, proxiedRequests *int32) (*git.AuthOptions, cleanupFunc) {
				// Create the git server.
				gitServer, err := setupGitServer(repoPath)
				g.Expect(err).ToNot(HaveOccurred())

				username := "test-user"
				password := "test-password"
				gitServer.Auth(username, password)

				// Start the HTTPS server.
				examplePublicKey, err := os.ReadFile("../../testdata/certs/server.pem")
				g.Expect(err).ToNot(HaveOccurred())
				examplePrivateKey, err := os.ReadFile("../../testdata/certs/server-key.pem")
				g.Expect(err).ToNot(HaveOccurred())
				exampleCA, err := os.ReadFile("../../testdata/certs/ca.pem")
				g.Expect(err).ToNot(HaveOccurred())
				err = gitServer.StartHTTPS(examplePublicKey, examplePrivateKey, exampleCA, "example.com")
				g.Expect(err).ToNot(HaveOccurred())

				u, err := url.Parse(gitServer.HTTPAddress())
				g.Expect(err).ToNot(HaveOccurred())

				// The request is being forwarded to the local test git server in this handler.
				// The certificate used here is valid for both example.com and localhost.
				var proxyHandler goproxy.FuncHttpsHandler = func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
					// Check if the host matches with the git server address and the user-agent is the expected git client.
					userAgent := ctx.Req.Header.Get("User-Agent")
					if strings.Contains(host, "example.com") && strings.Contains(userAgent, "libgit2") {
						atomic.AddInt32(proxiedRequests, 1)
						return goproxy.OkConnect, u.Host
					}
					// Reject if it isn't our request.
					return goproxy.RejectConnect, host
				}
				proxy.OnRequest().HandleConnect(proxyHandler)

				return &git.AuthOptions{
						Transport: git.HTTPS,
						Username:  username,
						Password:  password,
						CAFile:    exampleCA,
					}, func() {
						os.RemoveAll(gitServer.Root())
						gitServer.StopHTTP()
					}
			},
			wantUsedProxy: true,
			tls:           true,
		},
		{
			name:      "env var: NO_PROXY",
			url:       noProxyGitServer.HTTPAddress() + "/" + repoPath,
			proxyType: git2go.ProxyTypeAuto,
			setupGitProxy: func(g *WithT, proxy *goproxy.ProxyHttpServer, proxiedRequests *int32) (*git.AuthOptions, cleanupFunc) {
				var proxyHandler goproxy.FuncHttpsHandler = func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
					// We shouldn't hit the proxy so we just want to check for any interaction, then reject.
					atomic.AddInt32(proxiedRequests, 1)
					return goproxy.RejectConnect, host
				}
				proxy.OnRequest().HandleConnect(proxyHandler)

				return &git.AuthOptions{
						Transport: git.HTTP,
					}, func() {
					}
			},
			wantUsedProxy: false,
		},
		{
			name:          "specified proxy host",
			url:           "http://example.com/bar/test-reponame",
			proxyType:     git2go.ProxyTypeSpecified,
			setupGitProxy: setupHTTPGitProxy,
			wantUsedProxy: true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Run a proxy server.
			proxy := goproxy.NewProxyHttpServer()
			proxy.Verbose = true
			proxiedRequests := int32(0)
			authOpts, cleanup := tt.setupGitProxy(g, proxy, &proxiedRequests)
			defer cleanup()

			proxyServer := http.Server{
				Addr:    proxyAddr,
				Handler: proxy,
			}
			l, err := net.Listen("tcp", proxyServer.Addr)
			g.Expect(err).ToNot(HaveOccurred())
			if tt.tls {
				go proxyServer.ServeTLS(l, "../../testdata/certs/server.pem", "../../testdata/certs/server-key.pem")
			} else {
				go proxyServer.Serve(l)
			}
			defer proxyServer.Close()

			// Set proxy related env vars.
			os.Setenv("HTTPS_PROXY", fmt.Sprintf("https://%s", proxyAddr))
			defer os.Unsetenv("HTTPS_PROXY")

			os.Setenv("HTTP_PROXY", fmt.Sprintf("http://%s", proxyAddr))
			defer os.Unsetenv("HTTP_PROXY")

			os.Setenv("NO_PROXY", "127.0.0.1")
			defer os.Unsetenv("NO_PROXY")

			tmpDir := t.TempDir()

			transportOpts := TransportOptions{
				TargetURL: tt.url,
				AuthOpts:  authOpts,
				ProxyOptions: &git2go.ProxyOptions{
					Type: tt.proxyType,
				},
			}
			if tt.proxyType == git2go.ProxyTypeSpecified {
				transportOpts.ProxyOptions.Url = fmt.Sprintf("http://%s", proxyAddr)
			}

			transportOptsURL := fmt.Sprintf("http://proxy-http-test-%d", i)
			AddTransportOptions(transportOptsURL, transportOpts)
			defer RemoveTransportOptions(transportOptsURL)

			repo, err := git2go.Clone(transportOptsURL, tmpDir, &git2go.CloneOptions{
				CheckoutOptions: git2go.CheckoutOptions{
					Strategy: git2go.CheckoutForce,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			defer repo.Free()

			g.Expect(atomic.LoadInt32(&proxiedRequests) > 0).To(Equal(tt.wantUsedProxy))
		})
	}
}

func setupGitServer(repoPath string) (*gittestserver.GitServer, error) {
	gitServer, err := gittestserver.NewTempGitServer()
	if err != nil {
		return nil, err
	}

	// Initialize a git repo.
	err = gitServer.InitRepo("../../testdata/git/repo", "main", repoPath)
	if err != nil {
		return nil, err
	}
	return gitServer, nil
}

func setupHTTPGitProxy(g *WithT, proxy *goproxy.ProxyHttpServer, proxiedRequests *int32) (*git.AuthOptions, cleanupFunc) {
	// Create the git server.
	gitServer, err := setupGitServer(repoPath)
	g.Expect(err).ToNot(HaveOccurred())

	err = gitServer.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())

	u, err := url.Parse(gitServer.HTTPAddress())
	g.Expect(err).ToNot(HaveOccurred())

	// The request is being forwarded to the local test git server in this handler.
	// The certificate used here is valid for both example.com and localhost.
	var proxyHandler goproxy.FuncReqHandler = func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		userAgent := req.Header.Get("User-Agent")
		if strings.Contains(req.Host, "example.com") && strings.Contains(userAgent, "git") {
			atomic.AddInt32(proxiedRequests, 1)
			req.Host = u.Host
			req.URL.Host = req.Host
			return req, nil
		}
		// Reject if it isn't our request.
		return req, goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusForbidden, "")
	}
	proxy.OnRequest().Do(proxyHandler)

	return &git.AuthOptions{
			Transport: git.HTTP,
		}, func() {
			os.RemoveAll(gitServer.Root())
			gitServer.StopHTTP()
		}
}
