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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	extgogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/gittestserver"
)

func TestInit(t *testing.T) {
	g := NewWithT(t)

	tmp := t.TempDir()

	ggc, err := NewGoGitClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())

	err = ggc.Init(context.TODO(), "https://github.com/fluxcd/flux2", "main")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ggc.repository).ToNot(BeNil())

	remotes, err := ggc.repository.Remotes()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(remotes)).To(Equal(1))
	g.Expect(remotes[0].Config().Name).To(Equal(git.DefaultRemote))
	g.Expect(remotes[0].Config().URLs[0]).To(Equal("https://github.com/fluxcd/flux2"))
}

func TestWriteFile(t *testing.T) {
	g := NewWithT(t)

	tmp := t.TempDir()
	repo, err := extgogit.PlainInit(tmp, false)
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewGoGitClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())
	ggc.repository = repo

	err = ggc.WriteFile("test", strings.NewReader("testing gogit write"))
	g.Expect(err).ToNot(HaveOccurred())
	cont, err := os.ReadFile(filepath.Join(tmp, "test"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(cont)).To(Equal("testing gogit write"))
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

	ggc, err := NewGoGitClient(tmp, nil)
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
	}, nil)
	g.Expect(err).To(Equal(git.ErrNoStagedFiles))
	g.Expect(hash).To(Equal(cc))

	// Make changes to the repo.
	err = ggc.WriteFile("test", strings.NewReader("testing gogit commit"))
	g.Expect(err).ToNot(HaveOccurred())

	cc, err = ggc.Commit(git.Commit{
		Author: git.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
		Message: "testing",
	}, nil)
	g.Expect(err).ToNot(HaveOccurred())
	// New commit should not match the old one.
	g.Expect(cc).ToNot(Equal(hash))
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
	})
	g.Expect(err).ToNot(HaveOccurred())

	repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
		URL:        repoURL,
		Auth:       auth,
		RemoteName: git.DefaultRemote,
		Tags:       extgogit.NoTags,
	})
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewGoGitClient(tmp, nil)
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

func TestSwitchBranch(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(g *WithT, path string) string
		branch    string
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

			repoURL := server.HTTPAddressWithCredentials() + "/" + "test.git"
			tmp := t.TempDir()
			repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
				URL:           repoURL,
				ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
				RemoteName:    git.DefaultRemote,
			})
			g.Expect(err).ToNot(HaveOccurred())

			var expectedHash string
			if tt.setupFunc != nil {
				expectedHash = tt.setupFunc(g, filepath.Join(server.Root(), "test.git"))
			} else {
				head, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())
				expectedHash = head.Hash().String()
			}

			ggc, err := NewGoGitClient(tmp, nil)
			g.Expect(err).ToNot(HaveOccurred())
			ggc.repository = repo

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

	repo, path, err := initRepo(t)
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(path)

	_, err = commitFile(repo, "clean", "testing gogit is clean", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewGoGitClient(path, nil)
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

	repo, path, err := initRepo(t)
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(path)

	hash, err := commitFile(repo, "clean", "testing gogit head", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	ggc, err := NewGoGitClient(path, nil)
	g.Expect(err).ToNot(HaveOccurred())

	ggc.repository = repo

	cc, err := ggc.Head()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(hash.String()).To(Equal(cc))
}
