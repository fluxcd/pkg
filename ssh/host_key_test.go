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
	"net"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
)

func TestScanHost(t *testing.T) {
	g := NewWithT(t)

	startSSH := func(listener net.Listener, cfg *ssh.ServerConfig) {
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
			wantErr: "no common algorithm for host key; client offered: [ssh-rsa], server offered: [ssh-ed25519]",
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

			go startSSH(listener, sshConfig)

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
