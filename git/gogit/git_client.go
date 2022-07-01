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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	extgogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/fluxcd/pkg/git"
)

// GoGitClient implements git.GitClient
type GoGitClient struct {
	path       string
	repository *extgogit.Repository
	authOpts   *git.AuthOptions
}

var _ git.GitClient = &GoGitClient{}

// NewGoGitClient returns a new GoGitClient
func NewGoGitClient(path string, authOpts *git.AuthOptions) *GoGitClient {
	return &GoGitClient{
		path:     path,
		authOpts: authOpts,
	}
}

func (g *GoGitClient) Init(ctx context.Context, url, branch string) error {
	if g.repository != nil {
		return nil
	}

	r, err := extgogit.PlainInit(g.path, false)
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

func (g *GoGitClient) Clone(ctx context.Context, url string, checkoutOpts git.CheckoutOptions) (*git.Commit, error) {
	switch {
	case checkoutOpts.Commit != "":
		return g.cloneCommit(ctx, url, checkoutOpts.Commit, checkoutOpts)
	case checkoutOpts.Tag != "":
		return g.cloneTag(ctx, url, checkoutOpts.Tag, checkoutOpts)
	case checkoutOpts.SemVer != "":
		return g.cloneSemVer(ctx, url, checkoutOpts.SemVer, checkoutOpts)
	default:
		branch := checkoutOpts.Branch
		if branch == "" {
			branch = git.DefaultBranch
		}
		return g.cloneBranch(ctx, url, branch, checkoutOpts)
	}
}

func (g *GoGitClient) WriteFile(path string, reader io.Reader) error {
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

func (g *GoGitClient) Commit(info git.Commit, signer *openpgp.Entity) (string, error) {
	if g.repository == nil {
		return "", git.ErrNoGitRepository
	}

	wt, err := g.repository.Worktree()
	if err != nil {
		return "", err
	}

	status, err := wt.Status()
	if err != nil {
		return "", err
	}

	// go-git has [a bug](https://github.com/go-git/go-git/issues/253)
	// whereby it thinks broken symlinks to absolute paths are
	// modified. There's no circumstance in which we want to commit a
	// change to a broken symlink: so, detect and skip those.
	var changed bool
	for file := range status {
		abspath := filepath.Join(g.path, file)
		info, err := os.Lstat(abspath)
		if err != nil {
			return "", fmt.Errorf("checking if %s is a symlink: %w", file, err)
		}
		if info.Mode()&os.ModeSymlink > 0 {
			// symlinks are OK; broken symlinks are probably a result
			// of the bug mentioned above, but not of interest in any
			// case.
			if _, err := os.Stat(abspath); os.IsNotExist(err) {
				continue
			}
		}
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

	commitOpts := &extgogit.CommitOptions{
		Author: &object.Signature{
			Name:  info.Author.Name,
			Email: info.Author.Email,
			When:  time.Now(),
		},
	}

	if signer != nil {
		commitOpts.SignKey = signer
	}

	commit, err := wt.Commit(info.Message, commitOpts)
	if err != nil {
		return "", err
	}
	return commit.String(), nil
}

func (g *GoGitClient) Push(ctx context.Context) error {
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

func (g *GoGitClient) SwitchBranch(ctx context.Context, branchName string) error {
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

func (g *GoGitClient) IsClean() (bool, error) {
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

func (g *GoGitClient) Head() (string, error) {
	if g.repository == nil {
		return "", git.ErrNoGitRepository
	}
	head, err := g.repository.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func (g *GoGitClient) Path() string {
	return g.path
}

func (g *GoGitClient) Cleanup() {}
