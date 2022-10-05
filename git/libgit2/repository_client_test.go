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

package libgit2

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git2go "github.com/libgit2/git2go/v34"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/libgit2/internal/test"
	"github.com/fluxcd/pkg/git/libgit2/transport"
	"github.com/fluxcd/pkg/gittestserver"
)

func TestMain(m *testing.M) {
	err := transport.InitManagedTransport()
	if err != nil {
		panic("could not init managed transport")
	}
	code := m.Run()
	os.Exit(code)
}

func TestNewClient(t *testing.T) {
	g := NewWithT(t)

	outside := "../outside"
	lgc, err := NewClient(outside, nil)
	g.Expect(err).ToNot(HaveOccurred())

	wd, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(lgc.path).To(Equal(filepath.Join(wd, "outside")))
}

func TestInit(t *testing.T) {
	g := NewWithT(t)

	tmp := t.TempDir()
	lgc, err := NewClient(tmp, &git.AuthOptions{
		Transport: git.HTTPS,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = lgc.Init(context.TODO(), "https://github.com/fluxcd/flux2", "main")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(lgc.repository).ToNot(BeNil())
	g.Expect(lgc.remote).ToNot(BeNil())
	g.Expect(lgc.remote.Url()).To(Equal(lgc.transportOptsURL))
	g.Expect(lgc.remote.Name()).To(Equal(git.DefaultRemote))

	outside := "../outside"
	lgc, err = NewClient(outside, &git.AuthOptions{
		Transport: git.HTTPS,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = lgc.Init(context.TODO(), "https://github.com/fluxcd/flux2", "main")
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
	repo, err := git2go.InitRepository(tmp, false)
	g.Expect(err).ToNot(HaveOccurred())
	defer repo.Free()

	lgc, err := NewClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())

	lgc.repository = repo
	err = lgc.writeFile("test", strings.NewReader("testing libgit2 write"))
	g.Expect(err).ToNot(HaveOccurred())
	cont, err := os.ReadFile(filepath.Join(tmp, "test"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(cont)).To(Equal("testing libgit2 write"))

	fileStr := "absolute path is resolved as relative"
	err = lgc.writeFile("/outside/test2", strings.NewReader(fileStr))
	g.Expect(err).ToNot(HaveOccurred())

	expectedPath := filepath.Join(tmp, "outside", "test2")
	defer os.RemoveAll(expectedPath)

	cont, err = os.ReadFile(expectedPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(cont)).To(Equal(fileStr))

	err = lgc.writeFile("../tmp/test3", strings.NewReader("path outside repo"))
	g.Expect(err).To(HaveOccurred())
}

func TestCommit(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())

	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, "test.git")
	g.Expect(err).ToNot(HaveOccurred())
	tmp := t.TempDir()
	repo, err := git2go.Clone(filepath.Join(server.Root(), "test.git"), tmp, &git2go.CloneOptions{
		CheckoutOptions: git2go.CheckoutOptions{
			Strategy: git2go.CheckoutForce,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer repo.Free()

	lgc, err := NewClient(tmp, nil)
	g.Expect(err).ToNot(HaveOccurred())
	defer lgc.Close()

	lgc.repository = repo

	// No new commit made when there are no changes in the repo.
	head, err := lgc.repository.Head()
	g.Expect(err).ToNot(HaveOccurred())
	defer head.Free()
	hash := head.Target().String()
	cc, err := lgc.Commit(git.Commit{
		Author: git.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	g.Expect(err).To(Equal(git.ErrNoStagedFiles))
	g.Expect(hash).To(Equal(cc))

	cc, err = lgc.Commit(
		git.Commit{
			Author: git.Signature{
				Name:  "Test User",
				Email: "test@example.com",
			},
			Message: "testing",
		},
		git.WithFiles(map[string]io.Reader{
			"test": strings.NewReader("testing libgit2 commit"),
		}),
	)
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
	auth := &git.AuthOptions{
		Transport: git.HTTP,
		Username:  "test-user",
		Password:  "test-pass",
	}
	transportOptsURL := getTransportOptsURL(git.HTTP)
	transport.AddTransportOptions(transportOptsURL, transport.TransportOptions{
		TargetURL: repoURL,
		AuthOpts:  auth,
	})
	defer transport.RemoveTransportOptions(transportOptsURL)

	repo, err := git2go.Clone(transportOptsURL, tmp, &git2go.CloneOptions{
		CheckoutOptions: git2go.CheckoutOptions{
			Strategy: git2go.CheckoutForce,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer repo.Free()

	lgc, err := NewClient(tmp, auth)
	g.Expect(err).ToNot(HaveOccurred())
	defer lgc.Close()

	lgc.repository = repo
	lgc.remote, err = repo.Remotes.Lookup(git.DefaultRemote)
	g.Expect(err).ToNot(HaveOccurred())

	cc, err := test.CommitFile(repo, "test", "testing libgit2 push", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	err = lgc.Push(context.TODO())
	g.Expect(err).ToNot(HaveOccurred())

	repo, err = git2go.Clone(transportOptsURL, t.TempDir(), &git2go.CloneOptions{
		CheckoutOptions: git2go.CheckoutOptions{
			Strategy: git2go.CheckoutForce,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	head, err := repo.Head()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(head.Target().String()).To(Equal(cc.String()))
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
				repo, err := git2go.Clone(repoURL, t.TempDir(), &git2go.CloneOptions{
					CheckoutOptions: git2go.CheckoutOptions{
						Strategy: git2go.CheckoutForce,
					},
				})
				g.Expect(err).ToNot(HaveOccurred())
				defer repo.Free()

				commit, err := test.HeadCommit(repo)
				g.Expect(err).ToNot(HaveOccurred())
				defer commit.Free()

				err = test.CreateBranch(repo, "ahead", nil)
				g.Expect(err).ToNot(HaveOccurred())

				tree, err := repo.LookupTree(commit.TreeId())
				g.Expect(err).ToNot(HaveOccurred())
				defer tree.Free()

				err = repo.CheckoutTree(tree, &git2go.CheckoutOpts{
					Strategy: git2go.CheckoutForce,
				})
				g.Expect(err).ToNot(HaveOccurred())
				err = repo.SetHead("refs/heads/ahead")
				g.Expect(err).ToNot(HaveOccurred())

				cc, err := test.CommitFile(repo, "test", "testing libgit2 switch ahead branch", time.Now())
				g.Expect(err).ToNot(HaveOccurred())

				remote, err := repo.Remotes.Lookup(git.DefaultRemote)
				g.Expect(err).ToNot(HaveOccurred())
				defer remote.Free()

				err = remote.Push([]string{"refs/heads/ahead:refs/heads/ahead"}, &git2go.PushOptions{
					RemoteCallbacks: RemoteCallbacks(),
					ProxyOptions:    git2go.ProxyOptions{Type: git2go.ProxyTypeAuto},
				})
				g.Expect(err).ToNot(HaveOccurred())
				return cc.String()
			},
			branch: "ahead",
		},
		{
			name: "switch to a branch behind the current branch",
			setupFunc: func(g *WithT, repoURL string) string {
				repo, err := git2go.Clone(repoURL, t.TempDir(), &git2go.CloneOptions{
					CheckoutOptions: git2go.CheckoutOptions{
						Strategy: git2go.CheckoutForce,
					},
				})
				g.Expect(err).ToNot(HaveOccurred())
				defer repo.Free()

				err = test.CreateBranch(repo, "behind", nil)
				g.Expect(err).ToNot(HaveOccurred())
				head, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())
				hash := head.Target().String()

				_, err = test.CommitFile(repo, "test", "testing gogit switch behind branch", time.Now())
				g.Expect(err).ToNot(HaveOccurred())

				remote, err := repo.Remotes.Lookup(git.DefaultRemote)
				g.Expect(err).ToNot(HaveOccurred())
				defer remote.Free()

				err = remote.Push([]string{fmt.Sprintf("refs/heads/%s:refs/heads/%s", git.DefaultBranch, git.DefaultBranch)}, &git2go.PushOptions{
					RemoteCallbacks: RemoteCallbacks(),
					ProxyOptions:    git2go.ProxyOptions{Type: git2go.ProxyTypeAuto},
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
			auth := &git.AuthOptions{
				Transport: git.HTTP,
			}
			transportOptsURL := getTransportOptsURL(git.HTTP)
			transport.AddTransportOptions(transportOptsURL, transport.TransportOptions{
				TargetURL: repoURL,
				AuthOpts:  auth,
			})
			defer transport.RemoveTransportOptions(transportOptsURL)

			repo, err := git2go.Clone(transportOptsURL, tmp, &git2go.CloneOptions{
				CheckoutOptions: git2go.CheckoutOptions{
					Strategy: git2go.CheckoutForce,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			var expectedHash string
			if tt.setupFunc != nil {
				expectedHash = tt.setupFunc(g, transportOptsURL)
			} else {
				head, err := repo.Head()
				g.Expect(err).ToNot(HaveOccurred())
				expectedHash = head.Target().String()
			}

			lgc, err := NewClient(tmp, auth)
			g.Expect(err).ToNot(HaveOccurred())
			defer lgc.Close()

			lgc.repository = repo
			origin, err := repo.Remotes.Lookup(git.DefaultRemote)
			g.Expect(err).ToNot(HaveOccurred())
			lgc.remote = origin

			err = lgc.SwitchBranch(context.TODO(), tt.branch)
			g.Expect(err).ToNot(HaveOccurred())

			head, err := lgc.repository.Head()
			g.Expect(err).ToNot(HaveOccurred())
			defer head.Free()
			currentBranch, err := head.Branch().Name()
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(currentBranch).To(Equal(tt.branch))
			g.Expect(head.Target().String()).To(Equal(expectedHash))
		})
	}
}

func TestIsClean(t *testing.T) {
	g := NewWithT(t)

	repo, err := test.InitRepo(t, false)
	g.Expect(err).ToNot(HaveOccurred())
	defer repo.Free()
	defer os.RemoveAll(repo.Workdir())

	_, err = test.CommitFile(repo, "clean", "testing libgit2 is clean", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	lgc, err := NewClient(repo.Workdir(), nil)
	g.Expect(err).ToNot(HaveOccurred())

	defer lgc.Close()
	lgc.repository = repo

	clean, err := lgc.IsClean()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clean).To(BeTrue())

	f, err := os.Create(filepath.Join(repo.Workdir(), "dirty"))
	g.Expect(err).ToNot(HaveOccurred())
	defer f.Close()

	clean, err = lgc.IsClean()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clean).To(BeFalse())
}

func TestHead(t *testing.T) {
	g := NewWithT(t)

	repo, err := test.InitRepo(t, false)
	g.Expect(err).ToNot(HaveOccurred())

	hash, err := test.CommitFile(repo, "clean", "testing libgit2 head", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	lgc, err := NewClient(repo.Workdir(), nil)
	g.Expect(err).ToNot(HaveOccurred())
	defer lgc.Close()

	lgc.repository = repo

	cc, err := lgc.Head()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(hash.String()).To(Equal(cc))
}
