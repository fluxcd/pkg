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
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	socks5 "github.com/armon/go-socks5"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/ssh"
	git2go "github.com/libgit2/git2go/v34"
	. "github.com/onsi/gomega"
)

var proxiedRequests int32
var gitServerAddr *url.URL

func Test_SSH_proxy(t *testing.T) {
	type testCase struct {
		name          string
		proxyType     git2go.ProxyType
		wantUsedProxy bool
		httpServer    bool
	}

	// we don't have a test case for NO_PROXY because the http proxy tests have a case for that
	// and golang caches the proxy env vars.
	tests := []testCase{
		{
			name:          "env var: ALL_PROXY",
			proxyType:     git2go.ProxyTypeAuto,
			wantUsedProxy: true,
		},
		{
			name:          "specified proxy host",
			proxyType:     git2go.ProxyTypeSpecified,
			wantUsedProxy: true,
		},
		{
			name:          "http through socks",
			proxyType:     git2go.ProxyTypeSpecified,
			wantUsedProxy: true,
			httpServer:    true,
		},
	}

	g := NewWithT(t)

	l, err := net.Listen("tcp", ":0")
	g.Expect(err).ToNot(HaveOccurred())
	defer l.Close()
	proxyAddr := fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port)

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			atomic.StoreInt32(&proxiedRequests, 0)

			conf := &socks5.Config{
				Rules: TestProxyRule{},
			}
			socksServer, err := socks5.New(conf)
			g.Expect(err).ToNot(HaveOccurred())

			go func() {
				socksServer.Serve(l)
			}()

			repoPath := "test.git"
			server, err := setupGitServer(repoPath)
			g.Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(server.Root())

			server.KeyDir(filepath.Join(server.Root(), "keys"))

			os.Setenv("ALL_PROXY", fmt.Sprintf("socks5://%s", proxyAddr))
			defer os.Unsetenv("ALL_PROXY")

			transportOpts := TransportOptions{
				AuthOpts: &git.AuthOptions{},
				ProxyOptions: &git2go.ProxyOptions{
					Type: tt.proxyType,
				},
			}
			if tt.proxyType == git2go.ProxyTypeSpecified {
				transportOpts.ProxyOptions.Url = fmt.Sprintf("socks5://%s", proxyAddr)
			}

			var transportOptsURL string
			if tt.httpServer {
				err = server.StartHTTP()
				g.Expect(err).ToNot(HaveOccurred())
				defer server.StopHTTP()

				transportOpts.AuthOpts.Transport = git.HTTP
				transportOpts.TargetURL = server.HTTPAddress() + "/" + repoPath
				transportOptsURL = fmt.Sprintf("http://sock-proxy%d", i)
			} else {
				err = server.ListenSSH()
				g.Expect(err).ToNot(HaveOccurred())

				go func() {
					server.StartSSH()
				}()
				defer server.StopSSH()

				kp, err := ssh.NewEd25519Generator().Generate()
				g.Expect(err).ToNot(HaveOccurred())

				sshRepoAddr := server.SSHAddress() + "/" + repoPath
				u, err := url.Parse(sshRepoAddr)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(u.Host).ToNot(BeEmpty())
				knownhosts, err := ssh.ScanHostKey(u.Host, 5*time.Second, git.HostKeyAlgos, false)
				g.Expect(err).NotTo(HaveOccurred())

				transportOpts.TargetURL = sshRepoAddr
				transportOpts.AuthOpts.Transport = git.SSH
				transportOpts.AuthOpts.KnownHosts = knownhosts
				transportOpts.AuthOpts.Identity = kp.PrivateKey
				transportOptsURL = fmt.Sprintf("ssh://git@fake-url%d", i)
			}

			AddTransportOptions(transportOptsURL, transportOpts)
			defer RemoveTransportOptions(transportOptsURL)

			tmpDir := t.TempDir()

			// We call git2go.Clone with transportOptsURL, so that the managed ssh transport can
			// fetch the correct set of credentials and the actual target url as well.
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

type TestProxyRule struct{}

func (dr TestProxyRule) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
	atomic.AddInt32(&proxiedRequests, 1)
	return ctx, true
}
