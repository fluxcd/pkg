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

package gogit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	extgogit "github.com/fluxcd/go-git/v5"
	"github.com/fluxcd/go-git/v5/plumbing"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/fluxcd/pkg/gittestserver"
)

func TestNewClient(t *testing.T) {
	g := NewWithT(t)

	outside := "../outside"
	ggc, err := NewClient(outside, nil)
	g.Expect(err).ToNot(HaveOccurred())

	wd, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ggc.path).To(Equal(filepath.Join(wd, "outside")))
}

func TestInit(t *testing.T) {
	g := NewWithT(t)

	tmp := t.TempDir()

	ggc, err := NewClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())

	err = ggc.Init(context.TODO(), "https://github.com/fluxcd/flux2", "main")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ggc.repository).ToNot(BeNil())

	_, err = os.Stat(tmp)
	g.Expect(err).ToNot(HaveOccurred())

	remotes, err := ggc.repository.Remotes()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(remotes)).To(Equal(1))
	g.Expect(remotes[0].Config().Name).To(Equal(git.DefaultRemote))
	g.Expect(remotes[0].Config().URLs[0]).To(Equal("https://github.com/fluxcd/flux2"))

	outside := "../outside"
	ggc, err = NewClient(outside, nil)
	g.Expect(err).ToNot(HaveOccurred())

	err = ggc.Init(context.TODO(), "https://github.com/fluxcd/flux2", "main")
	g.Expect(err).ToNot(HaveOccurred())

	wd, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	// path outside the working dir is resolved as a child of the working dir
	expectedPath := filepath.Join(wd, "outside")
	defer os.RemoveAll(expectedPath)

	_, err = os.Stat(expectedPath)
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_writeFile(t *testing.T) {
	g := NewWithT(t)

	tmp := t.TempDir()
	repo, err := extgogit.PlainInit(tmp, false)
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())
	ggc.repository = repo

	err = ggc.writeFile("test", strings.NewReader("testing gogit write"))
	g.Expect(err).ToNot(HaveOccurred())
	cont, err := os.ReadFile(filepath.Join(tmp, "test"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(cont)).To(Equal("testing gogit write"))

	fileStr := "absolute path is resolved as relative"
	err = ggc.writeFile("/outside/test2", strings.NewReader(fileStr))
	g.Expect(err).ToNot(HaveOccurred())

	expectedPath := filepath.Join(tmp, "outside", "test2")
	defer os.RemoveAll(expectedPath)

	cont, err = os.ReadFile(expectedPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(cont)).To(Equal(fileStr))

	relPathContent := "rel path outside repo"
	err = ggc.writeFile("../tmp/test3", strings.NewReader(relPathContent))
	g.Expect(err).ToNot(HaveOccurred())

	relExpectedPath := filepath.Join(tmp, "tmp", "test3")
	defer os.RemoveAll(relExpectedPath)

	cont, err = os.ReadFile(relExpectedPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(cont)).To(Equal(relPathContent))
}

func TestCommit(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())

	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, "test.git")
	g.Expect(err).ToNot(HaveOccurred())
	tmp := t.TempDir()
	repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
		URL: filepath.Join(server.Root(), "test.git"),
	})
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())
	ggc.repository = repo

	// No new commit made when there are no changes in the repo.
	ref, err := ggc.repository.Head()
	g.Expect(err).ToNot(HaveOccurred())
	hash := ref.Hash().String()
	cc, err := ggc.Commit(git.Commit{
		Author: git.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	g.Expect(err).To(Equal(git.ErrNoStagedFiles))
	g.Expect(hash).To(Equal(cc))

	entity, err := openpgp.NewEntity("Test User", "git-testing", "test@example.com", nil)
	g.Expect(err).ToNot(HaveOccurred())

	keyPassphrase := "abcdef0123456789"
	err = entity.PrivateKey.Encrypt([]byte(keyPassphrase))
	g.Expect(err).ToNot(HaveOccurred())

	cc, err = ggc.Commit(
		git.Commit{
			Author: git.Signature{
				Name:  "Test User",
				Email: "test@example.com",
			},
			Message: "testing",
		},
		repository.WithFiles(map[string]io.Reader{
			"test": strings.NewReader("testing gogit commit"),
		}),
		repository.WithSigner(entity),
		repository.WithSignerPassphrase(keyPassphrase),
	)

	g.Expect(err).ToNot(HaveOccurred())
	// New commit should not match the old one.
	g.Expect(cc).ToNot(Equal(hash))

	ref, err = ggc.repository.Head()
	g.Expect(err).ToNot(HaveOccurred())
	obj, err := ggc.repository.CommitObject(ref.Hash())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj.PGPSignature).ToNot(BeEmpty())
}

func TestPush(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())
	server.Auth("test-user", "test-pass")

	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, "test.git")
	g.Expect(err).ToNot(HaveOccurred())

	err = server.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())
	defer server.StopHTTP()

	tmp := t.TempDir()
	repoURL := server.HTTPAddressWithCredentials() + "/" + "test.git"
	auth, err := transportAuth(&git.AuthOptions{
		Transport: git.HTTP,
		Username:  "test-user",
		Password:  "test-pass",
	}, false)
	g.Expect(err).ToNot(HaveOccurred())

	repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
		URL:        repoURL,
		Auth:       auth,
		RemoteName: git.DefaultRemote,
		Tags:       extgogit.NoTags,
	})
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())
	ggc.repository = repo

	cc, err := commitFile(repo, "test", "testing gogit push", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	err = ggc.Push(context.TODO())
	g.Expect(err).ToNot(HaveOccurred())

	repo, err = extgogit.PlainClone(t.TempDir(), false, &extgogit.CloneOptions{
		URL:  repoURL,
		Auth: auth,
	})
	g.Expect(err).ToNot(HaveOccurred())
	ref, err := repo.Head()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ref.Hash().String()).To(Equal(cc.String()))
}

func TestForcePush(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())
	server.Auth("test-user", "test-pass")

	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, "test.git")
	g.Expect(err).ToNot(HaveOccurred())

	err = server.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())
	defer server.StopHTTP()

	repoURL := server.HTTPAddressWithCredentials() + "/" + "test.git"
	auth, err := transportAuth(&git.AuthOptions{
		Transport: git.HTTP,
		Username:  "test-user",
		Password:  "test-pass",
	}, false)
	g.Expect(err).ToNot(HaveOccurred())

	tmp1 := t.TempDir()
	repo1, err := extgogit.PlainClone(tmp1, false, &extgogit.CloneOptions{
		URL:        repoURL,
		Auth:       auth,
		RemoteName: git.DefaultRemote,
		Tags:       extgogit.NoTags,
	})
	g.Expect(err).ToNot(HaveOccurred())

	_, err = commitFile(repo1, "test", "first push", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	ggc1, err := NewClient(tmp1, nil)
	g.Expect(err).ToNot(HaveOccurred())
	ggc1.repository = repo1

	tmp2 := t.TempDir()
	repo2, err := extgogit.PlainClone(tmp2, false, &extgogit.CloneOptions{
		URL:        repoURL,
		Auth:       auth,
		RemoteName: git.DefaultRemote,
		Tags:       extgogit.NoTags,
	})
	g.Expect(err).ToNot(HaveOccurred())

	cc2, err := commitFile(repo2, "test", "first push from second clone", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	ggc2, err := NewClient(tmp2, nil, WithDiskStorage(), WithForcePush())
	g.Expect(err).ToNot(HaveOccurred())
	ggc2.repository = repo2

	// First push from ggc1 should work.
	err = ggc1.Push(context.TODO())
	g.Expect(err).ToNot(HaveOccurred())

	// Force push from ggc2 should override ggc1.
	err = ggc2.Push(context.TODO())
	g.Expect(err).ToNot(HaveOccurred())

	// Follow-up push from ggc1 errors.
	_, err = commitFile(repo1, "test", "amend file again", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	err = ggc1.Push(context.TODO())
	g.Expect(err).To(HaveOccurred())

	repo, err := extgogit.PlainClone(t.TempDir(), false, &extgogit.CloneOptions{
		URL:  repoURL,
		Auth: auth,
	})
	g.Expect(err).ToNot(HaveOccurred())
	ref, err := repo.Head()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ref.Hash().String()).To(Equal(cc2.String()))
}

func TestSwitchBranch(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(g *WithT, path string) string
		changeRepo   func(g *WithT, c *Client) string
		branch       string
		forcePush    bool
		singleBranch bool
	}{
		{
			name: "switch to a branch ahead of the current branch",
			setupFunc: func(g *WithT, repoURL string) string {
				tmp := t.TempDir()
				repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
					RemoteName:    git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				err = createBranch(repo, "ahead")
				g.Expect(err).ToNot(HaveOccurred())

				cc, err := commitFile(repo, "test", "testing gogit switch ahead branch", time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.Push(&extgogit.PushOptions{
					RemoteName: git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())
				return cc.String()
			},
			branch: "ahead",
		},
		{
			name: "switch to a branch that exists locally and remotely",
			setupFunc: func(g *WithT, repoURL string) string {
				tmp := t.TempDir()
				repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
					RemoteName:    git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				err = createBranch(repo, "ahead")
				g.Expect(err).ToNot(HaveOccurred())

				cc, err := commitFile(repo, "test", "I live in the remote branch", time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.Push(&extgogit.PushOptions{
					RemoteName: git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())
				return cc.String()
			},
			changeRepo: func(g *WithT, c *Client) string {
				wt, err := c.repository.Worktree()
				g.Expect(err).ToNot(HaveOccurred())

				err = wt.Checkout(&extgogit.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("ahead"),
					Create: true,
				})
				g.Expect(err).ToNot(HaveOccurred())

				cc, err := commitFile(c.repository, "new change", "local branch is warmer though", time.Now())
				g.Expect(err).ToNot(HaveOccurred())

				err = wt.Checkout(&extgogit.CheckoutOptions{
					Branch: plumbing.Master,
				})
				g.Expect(err).ToNot(HaveOccurred())

				return cc.String()
			},
			branch: "ahead",
		},
		{
			name: "singlebranch: ignore a branch that exists in the remote",
			setupFunc: func(g *WithT, repoURL string) string {
				tmp := t.TempDir()
				repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
					RemoteName:    git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				head, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())

				err = createBranch(repo, "singlebranch-ahead")
				g.Expect(err).ToNot(HaveOccurred())

				_, err = commitFile(repo, "test", "I am going to be treated as stale", time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.Push(&extgogit.PushOptions{
					RemoteName: git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				return head.Hash().String()
			},
			branch:       "singlebranch-ahead",
			singleBranch: true,
		},
		{
			name: "switch to a branch behind the current branch",
			setupFunc: func(g *WithT, repoURL string) string {
				tmp := t.TempDir()
				repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
					RemoteName:    git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				err = createBranch(repo, "behind")
				g.Expect(err).ToNot(HaveOccurred())
				ref, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())
				hash := ref.Hash().String()

				wt, err := repo.Worktree()
				g.Expect(err).ToNot(HaveOccurred())
				err = wt.Checkout(&extgogit.CheckoutOptions{
					Branch: plumbing.ReferenceName("refs/heads/" + git.DefaultBranch),
				})
				g.Expect(err).ToNot(HaveOccurred())

				_, err = commitFile(repo, "test", "testing gogit switch behind branch", time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.Push(&extgogit.PushOptions{
					RemoteName: git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				return hash
			},
			branch: "behind",
		},
		{
			name:      "switch to a branch that doesn't exist on remote",
			setupFunc: nil,
			branch:    "new",
		},
		{
			name: "force push: switch to a branch ahead of the current branch",
			setupFunc: func(g *WithT, repoURL string) string {
				tmp := t.TempDir()
				repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
					RemoteName:    git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				err = createBranch(repo, "ahead")
				g.Expect(err).ToNot(HaveOccurred())

				cc, err := commitFile(repo, "test", "testing gogit switch ahead branch", time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.Push(&extgogit.PushOptions{
					RemoteName: git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())
				return cc.String()
			},
			branch:    "ahead",
			forcePush: true,
		},
		{
			name: "force push: switch to a branch behind the current branch",
			setupFunc: func(g *WithT, repoURL string) string {
				tmp := t.TempDir()
				repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
					RemoteName:    git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				err = createBranch(repo, "behind")
				g.Expect(err).ToNot(HaveOccurred())
				ref, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())
				hash := ref.Hash().String()

				wt, err := repo.Worktree()
				g.Expect(err).ToNot(HaveOccurred())
				err = wt.Checkout(&extgogit.CheckoutOptions{
					Branch: plumbing.ReferenceName("refs/heads/" + git.DefaultBranch),
				})
				g.Expect(err).ToNot(HaveOccurred())

				_, err = commitFile(repo, "test", "testing gogit switch behind branch", time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.Push(&extgogit.PushOptions{
					RemoteName: git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				return hash
			},
			branch:    "behind",
			forcePush: true,
		},
		{
			name:      "force push: switch to a branch that doesn't exist on remote",
			setupFunc: nil,
			branch:    "new",
			forcePush: true,
		},
		{
			name: "force: ignore a branch that exists in the remote",
			setupFunc: func(g *WithT, repoURL string) string {
				tmp := t.TempDir()
				repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
					RemoteName:    git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				head, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())

				err = createBranch(repo, "singlebranch-ahead")
				g.Expect(err).ToNot(HaveOccurred())

				_, err = commitFile(repo, "test", "remote change that will be overwritten", time.Now())
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.Push(&extgogit.PushOptions{
					RemoteName: git.DefaultRemote,
				})
				g.Expect(err).ToNot(HaveOccurred())

				return head.Hash().String()
			},
			branch:       "singlebranch-ahead",
			singleBranch: true,
			forcePush:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			server, err := gittestserver.NewTempGitServer()
			g.Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(server.Root())

			err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, "test.git")
			g.Expect(err).ToNot(HaveOccurred())

			err = server.StartHTTP()
			g.Expect(err).ToNot(HaveOccurred())
			defer server.StopHTTP()

			var expectedHash string
			if tt.setupFunc != nil {
				expectedHash = tt.setupFunc(g, filepath.Join(server.Root(), "test.git"))
			}

			repoURL := server.HTTPAddressWithCredentials() + "/" + "test.git"
			tmp := t.TempDir()
			repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
				URL:           repoURL,
				ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
				RemoteName:    git.DefaultRemote,
				SingleBranch:  tt.singleBranch,
			})
			g.Expect(err).ToNot(HaveOccurred())

			if tt.setupFunc == nil {
				head, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())
				expectedHash = head.Hash().String()
			}

			ggc, err := NewClient(tmp, nil)
			g.Expect(err).ToNot(HaveOccurred())
			ggc.repository = repo
			ggc.forcePush = tt.forcePush

			if tt.changeRepo != nil {
				expectedHash = tt.changeRepo(g, ggc)
			}

			err = ggc.SwitchBranch(context.TODO(), tt.branch)
			g.Expect(err).ToNot(HaveOccurred())

			ref, err := ggc.repository.Head()
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(ref.Name().Short()).To(Equal(tt.branch))
			g.Expect(ref.Hash().String()).To(Equal(expectedHash))
		})
	}
}

func TestIsClean(t *testing.T) {
	g := NewWithT(t)

	repo, path, err := initRepo(t.TempDir())
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(path)

	_, err = commitFile(repo, "clean", "testing gogit is clean", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewClient(path, nil)
	g.Expect(err).ToNot(HaveOccurred())
	ggc.repository = repo

	clean, err := ggc.IsClean()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clean).To(BeTrue())

	wt, err := repo.Worktree()
	g.Expect(err).ToNot(HaveOccurred())
	_, err = wt.Filesystem.Create("dirty")
	g.Expect(err).ToNot(HaveOccurred())

	clean, err = ggc.IsClean()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clean).To(BeFalse())
}

func TestHead(t *testing.T) {
	g := NewWithT(t)

	repo, path, err := initRepo(t.TempDir())
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(path)

	hash, err := commitFile(repo, "clean", "testing gogit head", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewClient(path, nil)
	g.Expect(err).ToNot(HaveOccurred())

	ggc.repository = repo

	cc, err := ggc.Head()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(hash.String()).To(Equal(cc))
}

func TestValidateUrl(t *testing.T) {
	tests := []struct {
		name                string
		transport           git.TransportType
		username            string
		password            string
		bearerToken         string
		url                 string
		credentialsOverHttp bool
		expectedError       string
	}{
		{
			name:          "blocked: basic auth over http",
			transport:     git.HTTP,
			username:      "user",
			password:      "pass",
			url:           "http://url",
			expectedError: "basic auth cannot be sent over HTTP",
		},
		{
			name:                "allowed: basic auth over http with insecure enabled",
			transport:           git.HTTP,
			username:            "user",
			password:            "pass",
			url:                 "http://url",
			credentialsOverHttp: true,
		},
		{
			name:      "allowed: basic auth over https",
			transport: git.HTTPS,
			username:  "user",
			password:  "pass",
			url:       "https://url",
		},
		{
			name:          "blocked: bearer token over http",
			transport:     git.HTTP,
			bearerToken:   "token",
			url:           "http://url",
			expectedError: "bearer token cannot be sent over HTTP",
		},
		{
			name:                "allowed: bearer token over http with insecure enabled",
			transport:           git.HTTP,
			bearerToken:         "token",
			url:                 "http://url",
			credentialsOverHttp: true,
		},
		{
			name:        "allowed: bearer token over https",
			transport:   git.HTTPS,
			bearerToken: "token",
			url:         "https://url",
		},
		{
			name:          "blocked: basic auth and bearer token at the same time over http",
			transport:     git.HTTP,
			username:      "user",
			password:      "pass",
			bearerToken:   "token",
			url:           "http://url",
			expectedError: "basic auth and bearer token cannot be set at the same time",
		},
		{
			name:          "blocked: basic auth and bearer token at the same time over https",
			transport:     git.HTTPS,
			username:      "user",
			password:      "pass",
			bearerToken:   "token",
			url:           "https://url",
			expectedError: "basic auth and bearer token cannot be set at the same time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := []ClientOption{WithDiskStorage()}
			if tt.credentialsOverHttp {
				opts = append(opts, WithInsecureCredentialsOverHTTP())
			}

			ggc, err := NewClient(t.TempDir(), &git.AuthOptions{
				Transport:   tt.transport,
				Username:    tt.username,
				Password:    tt.password,
				BearerToken: tt.bearerToken,
			}, opts...)
			g.Expect(err).ToNot(HaveOccurred())

			err = ggc.validateUrl(tt.url)

			if tt.expectedError == "" {
				g.Expect(err).To(BeNil())
			} else {
				g.Expect(err).ToNot(BeNil())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedError))
			}
		})
	}
}
