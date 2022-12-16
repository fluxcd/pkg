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
	"net/url"
	"path/filepath"
	"time"

	extgogit "github.com/fluxcd/go-git/v5"
	"github.com/fluxcd/go-git/v5/config"
	"github.com/fluxcd/go-git/v5/plumbing"
	"github.com/fluxcd/go-git/v5/plumbing/cache"
	"github.com/fluxcd/go-git/v5/plumbing/object"
	"github.com/fluxcd/go-git/v5/storage"
	"github.com/fluxcd/go-git/v5/storage/filesystem"
	"github.com/fluxcd/go-git/v5/storage/memory"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit/fs"
	"github.com/fluxcd/pkg/git/repository"
)

// ClientName is the string representation of Client.
const ClientName = "go-git"

// Client implements repository.Client.
type Client struct {
	*repository.DiscardCloser
	path                 string
	repository           *extgogit.Repository
	authOpts             *git.AuthOptions
	storer               storage.Storer
	worktreeFS           billy.Filesystem
	forcePush            bool
	credentialsOverHTTP  bool
	useDefaultKnownHosts bool
	singleBranch         bool
}

var _ repository.Client = &Client{}

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
		// Default to single branch as it is the most performant option.
		singleBranch: true,
	}

	if len(clientOpts) == 0 {
		clientOpts = append(clientOpts, WithDiskStorage())
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

// WithSingleBranch indicates whether only the references of a single
// branch will be fetched during cloning operations.
// For read-only clones, and for single branch write operations,
// a single branch is advised for performance reasons.
//
// For write operations that require multiple branches, for example,
// cloning from main and pushing into a feature branch, this should be
// disabled. Otherwise a second fetch will be required to get the state
// of the target branch, which won't work against some Git servers due
// to MULTI_ACK not being implemented in go-git.
//
// By default this is enabled.
func WithSingleBranch(singleBranch bool) ClientOption {
	return func(c *Client) error {
		c.singleBranch = singleBranch
		return nil
	}
}

func WithDiskStorage() ClientOption {
	return func(c *Client) error {
		wt := fs.New(c.path)
		dot := fs.New(filepath.Join(c.path, extgogit.GitDirName))

		c.storer = filesystem.NewStorage(dot, cache.NewObjectLRUDefault())
		c.worktreeFS = wt
		return nil
	}
}

func WithMemoryStorage() ClientOption {
	return func(c *Client) error {
		c.storer = memory.NewStorage()
		c.worktreeFS = memfs.New()
		return nil
	}
}

// WithForcePush enables the use of force push for all push operations
// back to the Git repository.
// By default this is disabled.
func WithForcePush() ClientOption {
	return func(c *Client) error {
		c.forcePush = true
		return nil
	}
}

// WithInsecureCredentialsOverHTTP enables credentials being used over
// HTTP. This is not recommended for production environments.
func WithInsecureCredentialsOverHTTP() ClientOption {
	return func(c *Client) error {
		c.credentialsOverHTTP = true
		return nil
	}
}

// WithFallbackToDefaultKnownHosts enables falling back to the default known_hosts
// of the machine if the provided auth options don't provide it.
func WithFallbackToDefaultKnownHosts() ClientOption {
	return func(c *Client) error {
		c.useDefaultKnownHosts = true
		return nil
	}
}

func (g *Client) Init(ctx context.Context, url, branch string) error {
	if err := g.validateUrl(url); err != nil {
		return err
	}

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

func (g *Client) Clone(ctx context.Context, url string, cloneOpts repository.CloneOptions) (*git.Commit, error) {
	if err := g.validateUrl(url); err != nil {
		return nil, err
	}

	checkoutStrat := cloneOpts.CheckoutStrategy
	switch {
	case checkoutStrat.Commit != "":
		return g.cloneCommit(ctx, url, checkoutStrat.Commit, cloneOpts)
	case checkoutStrat.Tag != "":
		return g.cloneTag(ctx, url, checkoutStrat.Tag, cloneOpts)
	case checkoutStrat.SemVer != "":
		return g.cloneSemVer(ctx, url, checkoutStrat.SemVer, cloneOpts)
	default:
		branch := checkoutStrat.Branch
		if branch == "" {
			branch = git.DefaultBranch
		}
		return g.cloneBranch(ctx, url, branch, cloneOpts)
	}
}

func (g *Client) validateUrl(u string) error {
	ru, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("cannot parse url: %w", err)
	}

	if g.credentialsOverHTTP {
		return nil
	}

	httpOrEmpty := ru.Scheme == string(git.HTTP) || ru.Scheme == ""
	if httpOrEmpty && ru.User != nil {
		return errors.New("URL cannot contain credentials when using HTTP")
	}

	if httpOrEmpty && g.authOpts != nil &&
		(g.authOpts.Username != "" || g.authOpts.Password != "") {
		return errors.New("basic auth cannot be sent over HTTP")
	}

	return nil
}

func (g *Client) writeFile(path string, reader io.Reader) error {
	if g.repository == nil {
		return git.ErrNoGitRepository
	}

	f, err := g.worktreeFS.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	return err
}

func (g *Client) Commit(info git.Commit, commitOpts ...repository.CommitOption) (string, error) {
	if g.repository == nil {
		return "", git.ErrNoGitRepository
	}

	options := &repository.CommitOptions{}
	for _, o := range commitOpts {
		o(options)
	}

	wt, err := g.repository.Worktree()
	if err != nil {
		return "", err
	}

	var changed bool
	for path, content := range options.Files {
		if err := g.writeFile(path, content); err != nil {
			return "", err
		}
		if _, err = wt.Add(path); err != nil {
			return "", fmt.Errorf("cannot stage file %s: %w", path, err)
		}
		changed = true
	}

	status, err := wt.Status()
	if err != nil {
		return "", err
	}

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

	authMethod, err := transportAuth(g.authOpts, g.useDefaultKnownHosts)
	if err != nil {
		return fmt.Errorf("failed to construct auth method with options: %w", err)
	}

	return g.repository.PushContext(ctx, &extgogit.PushOptions{
		Force:      g.forcePush,
		RemoteName: extgogit.DefaultRemoteName,
		Auth:       authMethod,
		Progress:   nil,
		CABundle:   caBundle(g.authOpts),
	})
}

// SwitchBranch switches the current branch to the given branch name.
//
// No new references are fetched from the remote during the process,
// this is to ensure that the same flow can be used across all Git
// servers, regardless of them requiring MULTI_ACK or not. Once MULTI_ACK
// is implemented in go-git, this can be revisited.
//
// If more than one remote branch state is required, create the gogit
// client using WithSingleBranch(false). This will fetch all remote
// branches as part of the initial clone. Note that this is fully
// compatible with shallow clones.
//
// The following cases are handled:
// - Branch does not exist results in one being created using HEAD
// of the worktree.
// - Branch exists only remotely, results in a local branch being
// created tracking the remote HEAD.
// - Branch exists only locally, results in a checkout to the
// existing branch.
// - Branch exists locally and remotely, the local branch will take
// precendece.
//
// To override a remote branch with the state from the current branch,
// (i.e. image automation controller), use WithForcePush(true) in
// combination with WithSingleBranch(true). This will ignore the
// remote branch's existence.
func (g *Client) SwitchBranch(ctx context.Context, branchName string) error {
	if g.repository == nil {
		return git.ErrNoGitRepository
	}

	wt, err := g.repository.Worktree()
	if err != nil {
		return fmt.Errorf("failed to load worktree: %w", err)
	}

	// Assumes both local and remote branches exists until proven otherwise.
	remote, local := true, true
	remRefName := plumbing.NewRemoteReferenceName(extgogit.DefaultRemoteName, branchName)
	remRef, err := g.repository.Reference(remRefName, true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		remote = false
	} else if err != nil {
		return fmt.Errorf("could not fetch remote reference '%s': %w", branchName, err)
	}

	refName := plumbing.NewBranchReferenceName(branchName)
	_, err = g.repository.Reference(refName, true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		local = false
	} else if err != nil {
		return fmt.Errorf("could not fetch local reference '%s': %w", branchName, err)
	}

	create := false
	// If the remote branch exists, but not the local branch, create a local
	// reference to the remote's HEAD.
	if remote && !local {
		branchRef := plumbing.NewHashReference(refName, remRef.Hash())

		err = g.repository.Storer.SetReference(branchRef)
		if err != nil {
			return fmt.Errorf("could not create reference to remote HEAD '%s': %w", branchRef.Hash().String(), err)
		}
	} else if !remote && !local {
		// If the target branch does not exist locally or remotely, create a new
		// branch using the current worktree HEAD.
		create = true
	}

	err = wt.Checkout(&extgogit.CheckoutOptions{
		Branch: refName,
		Create: create,
	})
	if err != nil {
		return fmt.Errorf("could not checkout to branch '%s': %w", branchName, err)
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
