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
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	extgogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/fluxcd/pkg/git"
)

// ClientName is the string representation of Client.
const ClientName = "go-git"

var (
	// DirtyRepositoryErr represents the case in which a repository is
	// dirty after a clone. This is caused by go-git filesystem changing
	// the worktree content. An example of such occurrence is when the
	// repository contains a symlink pointing to a absolute path outside
	// the filesystem chroot - this is only a problem with osfs.
	DirtyRepositoryErr error = errors.New("repository is dirty (check for absolute symlinks)")
)

// Client implements git.RepositoryClient.
type Client struct {
	*git.DiscardRepositoryCloser
	path       string
	repository *extgogit.Repository
	authOpts   *git.AuthOptions
	storer     storage.Storer
	worktreeFS billy.Filesystem
}

var _ git.RepositoryClient = &Client{}

type ClientOption func(*Client) error

// NewClient returns a new GoGitClient.
func NewClient(path string, authOpts *git.AuthOptions, clientOpts ...ClientOption) (*Client, error) {
	securePath, err := git.SecurePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path %s: %w", path, err)
	}

	g := &Client{
		path:     securePath,
		authOpts: authOpts,
	}

	if len(clientOpts) == 0 {
		clientOpts = append(clientOpts, WithDiskStorage)
	}

	for _, clientOpt := range clientOpts {
		if err := clientOpt(g); err != nil {
			return nil, err
		}
	}

	if g.storer == nil {
		return nil, errors.New("unable to create client with a nil storer")
	}
	if g.worktreeFS == nil {
		return nil, errors.New("unable to create client with a nil worktree filesystem")
	}

	return g, nil
}

func WithStorer(s storage.Storer) ClientOption {
	return func(c *Client) error {
		c.storer = s
		return nil
	}
}
func WithWorkTreeFS(wt billy.Filesystem) ClientOption {
	return func(c *Client) error {
		c.worktreeFS = wt
		return nil
	}
}

func WithDiskStorage(g *Client) error {
	wt := osfs.New(g.path)
	dot, err := wt.Chroot(extgogit.GitDirName)
	if err != nil {
		return err
	}

	g.storer = filesystem.NewStorage(dot, cache.NewObjectLRUDefault())
	g.worktreeFS = wt
	return nil
}

func WithMemoryStorage(g *Client) error {
	g.storer = memory.NewStorage()
	g.worktreeFS = memfs.New()
	return nil
}

func (g *Client) Init(ctx context.Context, url, branch string) error {
	if g.repository != nil {
		return nil
	}

	r, err := extgogit.Init(g.storer, g.worktreeFS)
	if err != nil {
		return err
	}

	if _, err = r.CreateRemote(&config.RemoteConfig{
		Name: extgogit.DefaultRemoteName,
		URLs: []string{url},
	}); err != nil {
		return err
	}

	branchRef := plumbing.NewBranchReferenceName(branch)
	if err = r.CreateBranch(&config.Branch{
		Name:   branch,
		Remote: extgogit.DefaultRemoteName,
		Merge:  branchRef,
	}); err != nil {
		return err
	}
	// PlainInit assumes the initial branch to always be master, we can
	// overwrite this by setting the reference of the Storer to a new
	// symbolic reference (as there are no commits yet) that points
	// the HEAD to our new branch.
	if err = r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, branchRef)); err != nil {
		return err
	}

	g.repository = r
	return nil
}

func (g *Client) Clone(ctx context.Context, url string, cloneOpts git.CloneOptions) (commit *git.Commit, err error) {
	checkoutStrat := cloneOpts.CheckoutStrategy
	switch {
	case checkoutStrat.Commit != "":
		commit, err = g.cloneCommit(ctx, url, checkoutStrat.Commit, cloneOpts)
	case checkoutStrat.Tag != "":
		commit, err = g.cloneTag(ctx, url, checkoutStrat.Tag, cloneOpts)
	case checkoutStrat.SemVer != "":
		commit, err = g.cloneSemVer(ctx, url, checkoutStrat.SemVer, cloneOpts)
	default:
		branch := checkoutStrat.Branch
		if branch == "" {
			branch = git.DefaultBranch
		}
		commit, err = g.cloneBranch(ctx, url, branch, cloneOpts)
	}

	// If the clone operation errored or it was a no-op,
	// in both cases the clean check below won't be required.
	if err != nil ||
		(commit != nil && !git.IsConcreteCommit(*commit)) {
		return
	}

	clean, err := g.IsClean()
	if err != nil {
		return nil, err
	}

	// Refer to DirtyRepositoryErr's documentation for more info.
	if !clean {
		commit = nil
		err = DirtyRepositoryErr
	}

	return
}

func (g *Client) writeFile(path string, reader io.Reader) error {
	if g.repository == nil {
		return git.ErrNoGitRepository
	}

	wt, err := g.repository.Worktree()
	if err != nil {
		return err
	}

	f, err := wt.Filesystem.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	return err
}

func (g *Client) Commit(info git.Commit, commitOpts ...git.CommitOption) (string, error) {
	if g.repository == nil {
		return "", git.ErrNoGitRepository
	}

	options := &git.CommitOptions{}
	for _, o := range commitOpts {
		o(options)
	}

	for path, content := range options.Files {
		if err := g.writeFile(path, content); err != nil {
			return "", err
		}
	}

	wt, err := g.repository.Worktree()
	if err != nil {
		return "", err
	}

	status, err := wt.Status()
	if err != nil {
		return "", err
	}

	var changed bool
	for file := range status {
		_, _ = wt.Add(file)
		changed = true
	}

	if !changed {
		head, err := g.repository.Head()
		if err != nil {
			return "", err
		}
		return head.Hash().String(), git.ErrNoStagedFiles
	}

	opts := &extgogit.CommitOptions{
		Author: &object.Signature{
			Name:  info.Author.Name,
			Email: info.Author.Email,
			When:  time.Now(),
		},
	}

	if options.Signer != nil {
		opts.SignKey = options.Signer
	}

	commit, err := wt.Commit(info.Message, opts)
	if err != nil {
		return "", err
	}
	return commit.String(), nil
}

func (g *Client) Push(ctx context.Context) error {
	if g.repository == nil {
		return git.ErrNoGitRepository
	}

	authMethod, err := transportAuth(g.authOpts)
	if err != nil {
		return fmt.Errorf("failed to construct auth method with options: %w", err)
	}

	return g.repository.PushContext(ctx, &extgogit.PushOptions{
		RemoteName: extgogit.DefaultRemoteName,
		Auth:       authMethod,
		Progress:   nil,
		CABundle:   caBundle(g.authOpts),
	})
}

func (g *Client) SwitchBranch(ctx context.Context, branchName string) error {
	if g.repository == nil {
		return git.ErrNoGitRepository
	}

	wt, err := g.repository.Worktree()
	if err != nil {
		return fmt.Errorf("failed to load worktree: %w", err)
	}
	authMethod, err := transportAuth(g.authOpts)
	if err != nil {
		return fmt.Errorf("failed to construct auth method with options: %w", err)
	}

	_, err = g.repository.Branch(branchName)
	var create bool
	if err == extgogit.ErrBranchNotFound {
		create = true
	} else if err != nil {
		return err
	}

	err = wt.Checkout(&extgogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: create,
	})
	if err != nil {
		return fmt.Errorf("could not checkout to branch '%s': %w", branchName, err)
	}

	g.repository.FetchContext(ctx, &extgogit.FetchOptions{
		RemoteName: extgogit.DefaultRemoteName,
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%[1]s", branchName, extgogit.DefaultRemoteName)),
		},
		Auth: authMethod,
	})
	ref, err := g.repository.Reference(plumbing.ReferenceName(fmt.Sprintf("/refs/remotes/origin/%s", branchName)), true)

	// If remote ref doesn't exist, no need to reset to remote target commit, exit early.
	if err == plumbing.ErrReferenceNotFound {
		return nil
	} else if err != nil {
		return fmt.Errorf("could not fetch remote reference '%s': %w", branchName, err)
	}

	err = wt.Reset(&extgogit.ResetOptions{
		Commit: ref.Hash(),
		Mode:   extgogit.HardReset,
	})
	if err != nil {
		return fmt.Errorf("could not reset branch to be at commit '%s': %w", ref.Hash().String(), err)
	}
	return nil
}

func (g *Client) IsClean() (bool, error) {
	if g.repository == nil {
		return false, git.ErrNoGitRepository
	}
	wt, err := g.repository.Worktree()
	if err != nil {
		return false, err
	}
	status, err := wt.Status()
	if err != nil {
		return false, err
	}
	return status.IsClean(), nil
}

func (g *Client) Head() (string, error) {
	if g.repository == nil {
		return "", git.ErrNoGitRepository
	}
	head, err := g.repository.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func (g *Client) Path() string {
	return g.path
}
