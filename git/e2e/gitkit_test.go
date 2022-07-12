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
package e2e

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	extgogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/gittestserver"
	"github.com/fluxcd/pkg/ssh"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func TestGitKitE2E(t *testing.T) {
	gitServer, err := gittestserver.NewTempGitServer()
	if err != nil {
		log.Fatal(err)
	}
	username := "test-user"
	password := "test-pswd"
	gitServer.Auth(username, password)
	gitServer.AutoCreate()
	if err := gitServer.StartHTTP(); err != nil {
		log.Fatal(err)
	}
	gitServer.KeyDir(filepath.Join(gitServer.Root(), "keys"))
	if err := gitServer.ListenSSH(); err != nil {
		log.Fatal(err)
	}
	go func() {
		gitServer.StartSSH()
	}()
	defer gitServer.StopSSH()

	protocols := []git.TransportType{git.SSH, git.HTTP}
	clients := []string{git.GoGitClient}

	testFunc := func(t *testing.T, proto git.TransportType, c string) {
		t.Run("repo created using Clone", func(t *testing.T) {
			g := NewWithT(t)
			var client git.GitClient
			tmp := t.TempDir()
			var repoURL *url.URL
			var authOptions *git.AuthOptions
			repoName := fmt.Sprintf("gitkit-e2e-checkout-%s", string(proto))
			if proto == git.SSH {
				repoURL, err = url.Parse(gitServer.SSHAddress() + "/" + repoName)
				g.Expect(err).ToNot(HaveOccurred())
				sshAuth, err := createSSHIdentitySecret(*repoURL)
				g.Expect(err).ToNot(HaveOccurred())
				authOptions, err = git.NewAuthOptions(*repoURL, sshAuth)
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				repoURL, err = url.Parse(gitServer.HTTPAddressWithCredentials() + "/" + repoName)
				g.Expect(err).ToNot(HaveOccurred())
				authOptions, err = git.NewAuthOptions(*repoURL, nil)
				g.Expect(err).ToNot(HaveOccurred())
			}
			upstreamRepoPath := filepath.Join(gitServer.Root(), repoName)

			if c == git.GoGitClient {
				g.Expect(err).ToNot(HaveOccurred())
				client, err = gogit.NewGoGitClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
			}
			// init repo on server
			err = gitServer.InitRepo("../testdata/git/repo", "main", repoName)
			g.Expect(err).ToNot(HaveOccurred())

			// clone repo
			_, err = client.Clone(context.TODO(), repoURL.String(), git.CheckoutOptions{
				Branch: "main",
			})
			g.Expect(err).ToNot(HaveOccurred())

			// commit and push to origin
			err = client.WriteFile("test", strings.NewReader(randStringRunes(10)))
			g.Expect(err).ToNot(HaveOccurred())
			cc, err := client.Commit(mockCommitInfo(), nil)
			g.Expect(err).ToNot(HaveOccurred())

			err = client.Push(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())

			headCommit, _, err := headCommitWithBranch(upstreamRepoPath, "main", c)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(headCommit).To(Equal(cc))

			// switch to a new branch
			err = client.SwitchBranch(context.TODO(), "new")
			g.Expect(err).ToNot(HaveOccurred())

			// commit to and push new branch
			err = client.WriteFile("test", strings.NewReader(randStringRunes(10)))
			g.Expect(err).ToNot(HaveOccurred())
			cc, err = client.Commit(mockCommitInfo(), nil)
			g.Expect(err).ToNot(HaveOccurred())

			err = client.Push(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())
			headCommit, branch, err := headCommitWithBranch(upstreamRepoPath, "new", c)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(headCommit).To(Equal(cc))
			g.Expect(branch).To(Equal("new"))

			// switch to a branch behind the current branch, commit and push
			err = client.SwitchBranch(context.TODO(), "main")
			g.Expect(err).ToNot(HaveOccurred())
			err = client.WriteFile("test", strings.NewReader(randStringRunes(10)))
			g.Expect(err).ToNot(HaveOccurred())
			_, err = client.Commit(mockCommitInfo(), nil)
			g.Expect(err).ToNot(HaveOccurred())
			err = client.Push(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())
		})
		t.Run("repo created using Init", func(t *testing.T) {
			g := NewWithT(t)
			var client git.GitClient
			tmp := t.TempDir()
			var repoURL *url.URL
			var authOptions *git.AuthOptions
			repoName := fmt.Sprintf("gitkit-e2e-init-%s", string(proto))
			if proto == git.SSH {
				repoURL, err = url.Parse(gitServer.SSHAddress() + "/" + repoName)
				g.Expect(err).ToNot(HaveOccurred())
				sshAuth, err := createSSHIdentitySecret(*repoURL)
				g.Expect(err).ToNot(HaveOccurred())
				authOptions, err = git.NewAuthOptions(*repoURL, sshAuth)
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				repoURL, err = url.Parse(gitServer.HTTPAddressWithCredentials() + "/" + repoName)
				g.Expect(err).ToNot(HaveOccurred())
				authOptions, err = git.NewAuthOptions(*repoURL, nil)
				g.Expect(err).ToNot(HaveOccurred())
			}
			upstreamRepoPath := filepath.Join(gitServer.Root(), repoName)

			if c == git.GoGitClient {
				g.Expect(err).ToNot(HaveOccurred())
				client, err = gogit.NewGoGitClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
			}

			// Create a new repository
			err = client.Init(context.TODO(), repoURL.String(), "main")
			g.Expect(err).ToNot(HaveOccurred())

			err = client.WriteFile("test", strings.NewReader(randStringRunes(10)))
			g.Expect(err).ToNot(HaveOccurred())
			cc, err := client.Commit(mockCommitInfo(), nil)
			g.Expect(err).ToNot(HaveOccurred())

			err = client.Push(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())

			headCommit, _, err := headCommitWithBranch(upstreamRepoPath, "main", c)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(headCommit).To(Equal(cc))

			err = client.SwitchBranch(context.TODO(), "new")
			g.Expect(err).ToNot(HaveOccurred())

			err = client.WriteFile("test", strings.NewReader(randStringRunes(10)))
			g.Expect(err).ToNot(HaveOccurred())

			cc, err = client.Commit(mockCommitInfo(), nil)
			g.Expect(err).ToNot(HaveOccurred())

			err = client.Push(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())
			headCommit, branch, err := headCommitWithBranch(upstreamRepoPath, "new", c)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(headCommit).To(Equal(cc))
			g.Expect(branch).To(Equal("new"))

			err = client.SwitchBranch(context.TODO(), "main")
			g.Expect(err).ToNot(HaveOccurred())
			err = client.WriteFile("test", strings.NewReader(randStringRunes(10)))
			g.Expect(err).ToNot(HaveOccurred())
			_, err = client.Commit(mockCommitInfo(), nil)
			g.Expect(err).ToNot(HaveOccurred())
			err = client.Push(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
	for _, client := range clients {
		for _, protocol := range protocols {
			testFunc(t, protocol, client)
		}
	}
}

func headCommitWithBranch(url, branch, client string) (string, string, error) {
	tmp, err := os.MkdirTemp("", randStringRunes(5))
	if err != nil {
		return "", "", err
	}
	if client == git.GoGitClient {
		repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
			URL:           url,
			ReferenceName: plumbing.NewBranchReferenceName(branch),
		})
		if err != nil {
			return "", "", err
		}
		head, err := repo.Head()
		if err != nil {
			return "", "", err
		}
		return head.Hash().String(), head.Name().Short(), nil
	}
	return "", "", errors.New("unsupported git client")
}

func mockCommitInfo() git.Commit {
	return git.Commit{
		Author: git.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
		Message: "testing",
	}
}

func createSSHIdentitySecret(repoURL url.URL) (map[string][]byte, error) {
	knownhosts, err := ssh.ScanHostKey(repoURL.Host, 5*time.Second, []string{}, false)
	if err != nil {
		return nil, err
	}
	keygen := ssh.NewRSAGenerator(2048)
	pair, err := keygen.Generate()
	if err != nil {
		return nil, err
	}
	data := map[string][]byte{
		"known_hosts":  knownhosts,
		"identity":     pair.PrivateKey,
		"identity.pub": pair.PublicKey,
	}
	return data, nil
}

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func fileStorer(path string) (storage.Storer, error) {
	dot, err := osfs.New(path).Chroot(extgogit.GitDirName)
	if err != nil {
		return nil, err
	}
	return filesystem.NewStorage(dot, cache.NewObjectLRUDefault()), nil
}
