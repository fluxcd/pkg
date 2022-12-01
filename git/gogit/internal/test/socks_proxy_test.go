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

package test

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
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/fluxcd/pkg/ssh"
	. "github.com/onsi/gomega"
)

var proxiedRequests int32
var gitServerAddr *url.URL

func Test_SOCKS5_proxy(t *testing.T) {
	g := NewWithT(t)

	l, err := net.Listen("tcp", ":0")
	g.Expect(err).ToNot(HaveOccurred())
	defer l.Close()
	proxyAddr := fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port)

	conf := &socks5.Config{
		Rules: TestProxyRule{},
	}
	socksServer, err := socks5.New(conf)
	g.Expect(err).ToNot(HaveOccurred())

	go func() {
		socksServer.Serve(l)
	}()

	os.Setenv("ALL_PROXY", fmt.Sprintf("socks5://%s", proxyAddr))
	defer os.Unsetenv("ALL_PROXY")

	atomic.StoreInt32(&proxiedRequests, 0)

	repoPath := "test.git"
	server, err := setupGitServer(repoPath)
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())

	server.KeyDir(filepath.Join(server.Root(), "keys"))

	var repoURL string
	authOpts := &git.AuthOptions{}
	err = server.ListenSSH()
	g.Expect(err).ToNot(HaveOccurred())

	go func() {
		server.StartSSH()
	}()
	defer server.StopSSH()

	kp, err := ssh.NewEd25519Generator().Generate()
	g.Expect(err).ToNot(HaveOccurred())

	repoURL = server.SSHAddress() + "/" + repoPath
	u, err := url.Parse(repoURL)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(u.Host).ToNot(BeEmpty())
	knownhosts, err := ssh.ScanHostKey(u.Host, 5*time.Second, git.HostKeyAlgos, false)
	g.Expect(err).NotTo(HaveOccurred())

	authOpts.Transport = git.SSH
	authOpts.KnownHosts = knownhosts
	authOpts.Identity = kp.PrivateKey

	tmpDir := t.TempDir()
	ggc, err := gogit.NewClient(tmpDir, authOpts)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = ggc.Clone(context.TODO(), repoURL, repository.CloneOptions{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: "main",
		},
		ShallowClone: true,
	})
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(atomic.LoadInt32(&proxiedRequests) > 0).To(Equal(true))
}

type TestProxyRule struct{}

func (dr TestProxyRule) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
	atomic.AddInt32(&proxiedRequests, 1)
	return ctx, true
}
