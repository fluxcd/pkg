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
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	git2go "github.com/libgit2/git2go/v34"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/libgit2/internal/test"
	"github.com/fluxcd/pkg/gittestserver"
	"github.com/fluxcd/pkg/ssh"
)

// knownHostsFixture is known_hosts fixture in the expected
// format.
var knownHostsFixture = `github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
`

// To fetch latest knownhosts for source.developers.google.com run:
// ssh-keyscan -p 2022 source.developers.google.com
//
// Expected hash (used in the cases) can get found with:
// ssh-keyscan -p 2022 source.developers.google.com | ssh-keygen -l -f -
var knownHostsFixtureWithPort = `[source.developers.google.com]:2022 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBB5Iy4/cq/gt/fPqe3uyMy4jwv1Alc94yVPxmnwNhBzJqEV5gRPiRk5u4/JJMbbu9QUVAguBABxL7sBZa5PH/xY=`

// This is an incorrect known hosts entry, that does not aligned with
// the normalized format and therefore won't match.
var knownHostsFixtureUnormalized = `source.developers.google.com:2022 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBB5Iy4/cq/gt/fPqe3uyMy4jwv1Alc94yVPxmnwNhBzJqEV5gRPiRk5u4/JJMbbu9QUVAguBABxL7sBZa5PH/xY=`

func TestSSHAction_clientConfig(t *testing.T) {
	kp, err := ssh.GenerateKeyPair(ssh.RSA_4096)
	if err != nil {
		t.Fatalf("could not generate keypair: %s", err)
	}
	tests := []struct {
		name             string
		authOpts         *git.AuthOptions
		expectedUsername string
		expectedAuthLen  int
		expectErr        string
	}{
		{
			name:      "nil SSHTransportOptions returns an error",
			authOpts:  nil,
			expectErr: "cannot create ssh client config from nil ssh auth options",
		},
		{
			name: "valid SSHTransportOptions returns a valid SSHClientConfig",
			authOpts: &git.AuthOptions{
				Identity: kp.PrivateKey,
				Username: "user",
			},
			expectedUsername: "user",
			expectedAuthLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cfg, err := createClientConfig(tt.authOpts)
			if tt.expectErr != "" {
				g.Expect(tt.expectErr).To(Equal(err.Error()))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cfg.User).To(Equal(tt.expectedUsername))
			g.Expect(len(cfg.Auth)).To(Equal(tt.expectedAuthLen))
		})
	}
}

func TestSSHManagedTransport_E2E(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())

	server.KeyDir(filepath.Join(server.Root(), "keys"))

	err = server.ListenSSH()
	g.Expect(err).ToNot(HaveOccurred())

	go func() {
		server.StartSSH()
	}()
	defer server.StopSSH()
	InitManagedTransport()

	kp, err := ssh.NewEd25519Generator().Generate()
	g.Expect(err).ToNot(HaveOccurred())

	repoPath := "test.git"
	err = server.InitRepo("../../testdata/git/repo", git.DefaultBranch, repoPath)
	g.Expect(err).ToNot(HaveOccurred())

	u, err := url.Parse(server.SSHAddress())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(u.Host).ToNot(BeEmpty())
	knownhosts, err := ssh.ScanHostKey(u.Host, 5*time.Second, git.HostKeyAlgos, false)
	g.Expect(err).NotTo(HaveOccurred())

	transportOptsURL := "ssh://git@fake-url"
	sshAddress := server.SSHAddress() + "/" + repoPath
	AddTransportOptions(transportOptsURL, TransportOptions{
		TargetURL: sshAddress,
		AuthOpts: &git.AuthOptions{
			Username:   "user",
			Identity:   kp.PrivateKey,
			KnownHosts: knownhosts,
		},
	})

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

	_, err = test.CommitFile(repo, "test-file", "testing push", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	err = push(tmpDir, git.DefaultBranch)
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_checkKnownHost(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		expectedHost string
		knownHosts   []byte
		hostkey      [32]byte
		want         error
	}{
		{
			name:         "Empty",
			host:         "source.developers.google.com",
			knownHosts:   []byte(""),
			hostkey:      sha256Fingerprint("AGvEpqYNMqsRNIviwyk4J4HM0lEylomDBKOWZsBn434"),
			expectedHost: "source.developers.google.com:2022",
			want:         fmt.Errorf("hostkey verification aborted: no known_hosts found"),
		},
		{
			name:         "Mismatch incorrect known_hosts",
			host:         "source.developers.google.com",
			knownHosts:   []byte(knownHostsFixtureUnormalized),
			hostkey:      sha256Fingerprint("AGvEpqYNMqsRNIviwyk4J4HM0lEylomDBKOWZsBn434"),
			expectedHost: "source.developers.google.com:2022",
			want:         fmt.Errorf("no entries in known_hosts match host '[source.developers.google.com]:2022' with fingerprint 'AGvEpqYNMqsRNIviwyk4J4HM0lEylomDBKOWZsBn434'"),
		},
		{
			name:         "Match when host has port",
			host:         "source.developers.google.com:2022",
			knownHosts:   []byte(knownHostsFixtureWithPort),
			hostkey:      sha256Fingerprint("AGvEpqYNMqsRNIviwyk4J4HM0lEylomDBKOWZsBn434"),
			expectedHost: "source.developers.google.com:2022",
			want:         nil,
		},
		{
			name:         "Match even when host does not have port",
			host:         "source.developers.google.com",
			knownHosts:   []byte(knownHostsFixtureWithPort),
			hostkey:      sha256Fingerprint("AGvEpqYNMqsRNIviwyk4J4HM0lEylomDBKOWZsBn434"),
			expectedHost: "source.developers.google.com:2022",
			want:         nil,
		},
		{
			name:         "Match",
			host:         "github.com",
			knownHosts:   []byte(knownHostsFixture),
			hostkey:      sha256Fingerprint("nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"),
			expectedHost: "github.com",
			want:         nil,
		},
		{
			name:         "Match with port",
			host:         "github.com",
			knownHosts:   []byte(knownHostsFixture),
			hostkey:      sha256Fingerprint("nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"),
			expectedHost: "github.com:22",
			want:         nil,
		},
		{
			// Test case to specifically detect a regression introduced in v0.25.0
			// Ref: https://github.com/fluxcd/image-automation-controller/issues/378
			name:       "Match regardless of order of known_hosts",
			host:       "github.com",
			knownHosts: []byte(knownHostsFixture),
			// Use ecdsa-sha2-nistp256 instead of ssh-rsa
			hostkey:      sha256Fingerprint("p2QAMXNIC1TJYWeIOttrVc98/R1BUFWu3/LiyKgUfQM"),
			expectedHost: "github.com:22",
			want:         nil,
		},
		{
			name:         "Hostkey mismatch",
			host:         "github.com",
			knownHosts:   []byte(knownHostsFixture),
			hostkey:      sha256Fingerprint("ROQFvPThGrW4RuWLoL9tq9I9zJ42fK4XywyRtbOz/EQ"),
			expectedHost: "github.com",
			want:         fmt.Errorf("no entries in known_hosts match host 'github.com' with fingerprint 'ROQFvPThGrW4RuWLoL9tq9I9zJ42fK4XywyRtbOz/EQ'"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			err := checkKnownHost(tt.expectedHost, tt.knownHosts, tt.hostkey[:])
			if tt.want == nil {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(Equal(tt.want))
			}
		})
	}
}

func sha256Fingerprint(in string) [32]byte {
	d, err := base64.RawStdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}
	var out [32]byte
	copy(out[:], d)
	return out
}

func push(path, branch string) error {
	repo, err := git2go.OpenRepository(path)
	if err != nil {
		return err
	}
	defer repo.Free()
	origin, err := repo.Remotes.Lookup("origin")
	if err != nil {
		return err
	}
	defer origin.Free()

	err = origin.Push([]string{fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)}, &git2go.PushOptions{
		ProxyOptions: git2go.ProxyOptions{Type: git2go.ProxyTypeAuto},
	})
	if err != nil {
		return err
	}
	return nil
}
