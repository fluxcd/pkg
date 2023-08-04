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
	"io"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	extgogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/fluxcd/pkg/ssh"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

const timeout = time.Second * 20

func testUsingClone(g *WithT, client repository.Client, repoURL *url.URL, upstreamRepo upstreamRepoInfo) {
	// clone repo
	_, err := client.Clone(context.TODO(), repoURL.String(), repository.CloneConfig{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: "main",
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	// commit and push to origin
	cc, err := client.Commit(
		mockCommitInfo(),
		repository.WithFiles(map[string]io.Reader{
			"test1": strings.NewReader(uuid.New().String()),
		}),
	)
	g.Expect(err).ToNot(HaveOccurred(), "first commit")

	// GitHub sometimes takes a long time to propogate its deploy key and this leads
	// to mysterious push errors like "unknown error: ERROR: Unknown public SSH key".
	// This helps us get around that by retrying for a fixed amount of time.
	g.Eventually(func() bool {
		err = client.Push(context.TODO(), repository.PushConfig{})
		if err != nil {
			return false
		}
		return true
	}, timeout).Should(BeTrue())

	headCommit, _, err := headCommitWithBranch(upstreamRepo.url, "main", upstreamRepo.username, upstreamRepo.password)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(headCommit).To(Equal(cc))

	// switch to a new branch
	err = client.SwitchBranch(context.TODO(), "new")
	g.Expect(err).ToNot(HaveOccurred())

	// commit to and push new branch
	cc, err = client.Commit(
		mockCommitInfo(),
		repository.WithFiles(map[string]io.Reader{
			"test2": strings.NewReader(uuid.New().String()),
		}),
	)
	g.Expect(err).ToNot(HaveOccurred(), "second commit")

	err = client.Push(context.TODO(), repository.PushConfig{})
	g.Expect(err).ToNot(HaveOccurred())
	headCommit, branch, err := headCommitWithBranch(upstreamRepo.url, "new", upstreamRepo.username, upstreamRepo.password)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(headCommit).To(Equal(cc))
	g.Expect(branch).To(Equal("new"))

	// switch to a branch behind the current branch, commit and push
	err = client.SwitchBranch(context.TODO(), "main")
	g.Expect(err).ToNot(HaveOccurred())

	_, err = client.Commit(
		mockCommitInfo(),
		repository.WithFiles(map[string]io.Reader{
			"test3": strings.NewReader(uuid.New().String()),
		}),
	)
	g.Expect(err).ToNot(HaveOccurred(), "third commit")
	err = client.Push(context.TODO(), repository.PushConfig{})
	g.Expect(err).ToNot(HaveOccurred())
	headCommit, _, err = headCommitWithBranch(upstreamRepo.url, "new", upstreamRepo.username, upstreamRepo.password)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(headCommit).To(Equal(cc))
}

func testUsingInit(g *WithT, client repository.Client, repoURL *url.URL, upstreamRepo upstreamRepoInfo) {
	// Create a new repository
	err := client.Init(context.TODO(), repoURL.String(), "main")
	g.Expect(err).ToNot(HaveOccurred())

	cc, err := client.Commit(
		mockCommitInfo(),
		repository.WithFiles(map[string]io.Reader{
			"test1": strings.NewReader(uuid.New().String()),
		}),
	)
	g.Expect(err).ToNot(HaveOccurred(), "first commit")

	g.Eventually(func() bool {
		err = client.Push(context.TODO(), repository.PushConfig{})
		if err != nil {
			return false
		}
		return true
	}, timeout).Should(BeTrue())

	headCommit, _, err := headCommitWithBranch(upstreamRepo.url, "main", upstreamRepo.username, upstreamRepo.password)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(headCommit).To(Equal(cc))

	err = client.SwitchBranch(context.TODO(), "new")
	g.Expect(err).ToNot(HaveOccurred())

	cc, err = client.Commit(
		mockCommitInfo(),
		repository.WithFiles(map[string]io.Reader{
			"test2": strings.NewReader(uuid.New().String()),
		}),
	)
	g.Expect(err).ToNot(HaveOccurred(), "second commit")

	err = client.Push(context.TODO(), repository.PushConfig{})
	g.Expect(err).ToNot(HaveOccurred())
	headCommit, branch, err := headCommitWithBranch(upstreamRepo.url, "new", upstreamRepo.username, upstreamRepo.password)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(headCommit).To(Equal(cc))
	g.Expect(branch).To(Equal("new"))

	err = client.SwitchBranch(context.TODO(), "main")
	g.Expect(err).ToNot(HaveOccurred())

	_, err = client.Commit(
		mockCommitInfo(),
		repository.WithFiles(map[string]io.Reader{
			"test3": strings.NewReader(uuid.New().String()),
		}),
	)
	g.Expect(err).ToNot(HaveOccurred(), "third commit")
	err = client.Push(context.TODO(), repository.PushConfig{})
	g.Expect(err).ToNot(HaveOccurred())

	headCommit, _, err = headCommitWithBranch(upstreamRepo.url, "new", upstreamRepo.username, upstreamRepo.password)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(headCommit).To(Equal(cc))
}

func headCommitWithBranch(url, branch, username, password string) (string, string, error) {
	tmp, err := os.MkdirTemp("", randStringRunes(5))
	if err != nil {
		return "", "", err
	}
	var auth transport.AuthMethod
	if username != "" && password != "" {
		auth = &http.BasicAuth{
			Username: username,
			Password: password,
		}
	}
	repo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          auth,
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
	if repoURL.Port() == "" {
		repoURL.Host = repoURL.Hostname() + ":22"
	}
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

type upstreamRepoInfo struct {
	url      string
	username string
	password string
}

func initRepo(tmp, repoURL, branch, fixture, username, password string) error {
	repo, err := extgogit.PlainInit(tmp, false)
	if err != nil {
		return err
	}

	if _, err = repo.CreateRemote(&config.RemoteConfig{
		Name: extgogit.DefaultRemoteName,
		URLs: []string{repoURL},
	}); err != nil {
		return err
	}

	branchRef := plumbing.NewBranchReferenceName(branch)
	if err = repo.CreateBranch(&config.Branch{
		Name:   branch,
		Remote: extgogit.DefaultRemoteName,
		Merge:  branchRef,
	}); err != nil {
		return err
	}
	if err = repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, branchRef)); err != nil {
		return err
	}

	_ = filepath.WalkDir(fixture, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		input, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(tmp, d.Name()), input, 0644)
		if err != nil {
			return err
		}
		return nil
	})

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	_, err = wt.Add(".")
	if err != nil {
		return err
	}

	info := mockCommitInfo()
	_, err = wt.Commit(info.Message, &extgogit.CommitOptions{
		Author: &object.Signature{
			Name:  info.Author.Name,
			Email: info.Author.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	err = repo.Push(&extgogit.PushOptions{
		RemoteName: git.DefaultRemote,
		Auth: &http.BasicAuth{
			Username: username,
			Password: password,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
