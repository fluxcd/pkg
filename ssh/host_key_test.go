//go:build !proxy

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

package ssh

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/armon/go-socks5"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
)

func startSSH(listener net.Listener, cfg *ssh.ServerConfig, g *WithT) {
	conn, err := listener.Accept()
	g.Expect(err).ToNot(HaveOccurred())

	sConn, _, _, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		// the only expected error
		g.Expect(err.Error()).To(ContainSubstring("no common algorithm for host key"))
		return
	}

	sConn.Close()
	listener.Close()
}

func TestScanHost(t *testing.T) {
	tests := []struct {
		keyType        KeyPairType
		sshKeyTypeName string
		wantErr        string
	}{
		{keyType: RSA_4096, sshKeyTypeName: "ssh-rsa"},
		{keyType: ECDSA_P256, sshKeyTypeName: "ecdsa-sha2-nistp256"},
		{keyType: ECDSA_P384, sshKeyTypeName: "ecdsa-sha2-nistp384"},
		{keyType: ECDSA_P521, sshKeyTypeName: "ecdsa-sha2-nistp521"},
		{keyType: ED25519, sshKeyTypeName: "ssh-ed25519"},
		{keyType: ED25519, sshKeyTypeName: "ssh-rsa",
			wantErr: "no common algorithm for host key; we offered: [ssh-rsa], peer offered: [ssh-ed25519]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.sshKeyTypeName, func(t *testing.T) {
			g := NewWithT(t)

			listener, err := net.Listen("tcp", "127.0.0.1:0")
			g.Expect(err).ToNot(HaveOccurred())

			serverAddr := listener.Addr().String()
			g.Expect(serverAddr).ToNot(BeEmpty())

			sshConfig := &ssh.ServerConfig{
				NoClientAuth: true,
			}

			// Generate new keypair for the server to use for HostKeys.
			hkp, err := GenerateKeyPair(tt.keyType)
			g.Expect(err).NotTo(HaveOccurred())
			p, err := ssh.ParseRawPrivateKey(hkp.PrivateKey)
			g.Expect(err).NotTo(HaveOccurred())

			// Add key to server.
			signer, err := ssh.NewSignerFromKey(p)
			g.Expect(err).NotTo(HaveOccurred())
			sshConfig.AddHostKey(signer)

			go startSSH(listener, sshConfig, g)

			kh, err := ScanHostKey(serverAddr, 5*time.Second, []string{tt.sshKeyTypeName}, false)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				// Confirm the returned key is of expected type.
				g.Expect(string(kh)).To(ContainSubstring(tt.sshKeyTypeName))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))

			}

			listener.Close()
		})
	}
}

// this test is partially based on a go-git's TestSOCKS5Proxy
// see https://github.com/go-git/go-git/blob/5f90b841aef24f235002e2fc71bfb1e142f804cf/plumbing/transport/ssh/proxy_test.go#L21
func TestScanHostWithProxy(t *testing.T) {
	g := NewWithT(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).ToNot(HaveOccurred())

	serverAddr := listener.Addr().String()
	g.Expect(serverAddr).ToNot(BeEmpty())

	sshConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}

	// Generate new keypair for the server to use for HostKeys.
	hkp, err := GenerateKeyPair(RSA_4096)
	g.Expect(err).NotTo(HaveOccurred())
	p, err := ssh.ParseRawPrivateKey(hkp.PrivateKey)
	g.Expect(err).NotTo(HaveOccurred())

	// Add key to server.
	signer, err := ssh.NewSignerFromKey(p)
	g.Expect(err).NotTo(HaveOccurred())
	sshConfig.AddHostKey(signer)

	go startSSH(listener, sshConfig, g)

	rule := new(testProxyRule)
	socksServer, err := socks5.New(&socks5.Config{
		Rules: rule,
	})
	g.Expect(err).NotTo(HaveOccurred())
	socksListener, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).ToNot(HaveOccurred())
	go socksServer.Serve(socksListener)

	// we can't set ENV only for this test
	// because Golang proxy package caches ENV checks
	// and there is no method to reset this cache outside of proxy package
	// https://cs.opensource.google/go/x/net/+/refs/tags/v0.57.0:proxy/proxy.go;l=132
	// so we have to run an additional process
	// and then check the request counter in our socks server
	cmd := exec.Command("go", "test", "-tags=proxy")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("SSH_HOST=%s", serverAddr),
		fmt.Sprintf("ALL_PROXY=socks5://127.0.0.1:%d", socksListener.Addr().(*net.TCPAddr).Port),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Child process failed with %v. Output: %s", err, string(output))
	}

	g.Expect(rule.proxiedRequests).Should(BeNumerically(">", 0))
	listener.Close()
	socksListener.Close()
}

type testProxyRule struct {
	proxiedRequests int
}

func (r *testProxyRule) Allow(_ context.Context, _ *socks5.Request) (context.Context, bool) {
	r.proxiedRequests++
	return context.Background(), true
}
