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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/fluxcd/pkg/git/gogit/fs"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	git2go "github.com/libgit2/git2go/v34"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/libgit2/transport"
	"github.com/fluxcd/pkg/git/repository"
)

// ClientName is the string representation of Client.
const ClientName = "libgit2"

type Client struct {
	path       string
	repository *git2go.Repository
	remote     *git2go.Remote
	authOpts   *git.AuthOptions
	repoFS     billy.Filesystem
	proxyAddr  string
	// transportOptsURL is the backbone of how we use our own smart transports
	// without having to rely on libgit2 callbacks (since they are inflexible
	// and unstable).
	// The smart transport contract provided by libgit2 doesn't support any
	// kind of dependency injection, so it's not possible to pass certain
	// app level settings like creds, timeout, etc. down to the transport.
	// transportOptsURL serves as unique key for a particular set of transport
	// options for a Git repository, which is then used to fetch these options
	// at the transport level.
	transportOptsURL    string
	credentialsOverHTTP bool
}

var _ repository.Client = &Client{}

type ClientOption func(*Client) error

func NewClient(path string, authOpts *git.AuthOptions, clientOpts ...ClientOption) (*Client, error) {
	securePath, err := git.SecurePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path %s: %w", path, err)
	}

	l := &Client{
		path:     securePath,
		authOpts: authOpts,
	}

	if len(clientOpts) == 0 {
		clientOpts = append(clientOpts, WithDiskStorage())
	}

	for _, clientOpt := range clientOpts {
		if err := clientOpt(l); err != nil {
			return nil, err
		}
	}

	if l.repoFS == nil {
		return nil, errors.New("unable to create client with a nil repo filesystem")
	}

	return l, nil
}

func WithDiskStorage() ClientOption {
	return func(c *Client) error {
		c.repoFS = fs.New(c.path)
		return nil
	}
}

func WithMemoryStorage() ClientOption {
	return func(c *Client) error {
		c.repoFS = memfs.New()
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

func WithProxy(proxyAddr string) ClientOption {
	return func(c *Client) error {
		c.proxyAddr = proxyAddr
		return nil
	}
}

func (l *Client) Init(ctx context.Context, url, branch string) error {
	if err := l.validateUrl(url); err != nil {
		return err
	}

	if l.repository != nil {
		return nil
	}
	repo, err := git2go.InitRepository(l.path, false)
	if err != nil {
		return fmt.Errorf("unable to init repository for '%s': %w", url, libGit2Error(err))
	}

	l.registerTransportOptions(ctx, url)

	repo.SetHead(fmt.Sprintf("refs/heads/%s", branch))

	// Set remote URL to transportOptsURL. For any remote Git operation, `SmartSubTransport.Action()`
	// is invoked with the repo's remote URL, (which in our case would be transportOptsURL).
	// This then helps us fetch all the transport options (including the _actual_ remote URL)
	// required for that Git repo and operation at the transport level.
	remote, err := repo.Remotes.Create(git.DefaultRemote, l.transportOptsURL)
	if err != nil {
		// If the remote already exists, lookup the remote.
		if git2go.IsErrorCode(err, git2go.ErrorCodeExists) {
			remote, err = repo.Remotes.Lookup(git.DefaultRemote)
			if err != nil {
				repo.Free()
				return fmt.Errorf("unable to create or lookup remote '%s'", git.DefaultRemote)
			}

			// if the remote URL doesn't match, set the remote URL.
			if remote.Url() != l.transportOptsURL {
				err = repo.Remotes.SetUrl(git.DefaultRemote, l.transportOptsURL)
				if err != nil {
					repo.Free()
					remote.Free()
					return fmt.Errorf("unable to configure remote %s with url %s", git.DefaultRemote, url)
				}

				// referesh the remote
				remote, err = repo.Remotes.Lookup(git.DefaultRemote)
				if err != nil {
					repo.Free()
					return fmt.Errorf("unable to create or lookup remote '%s'", git.DefaultRemote)
				}
			}
		} else {
			repo.Free()
			return fmt.Errorf("unable to create remote for '%s': %w", url, libGit2Error(err))
		}
	}

	l.repository = repo
	l.remote = remote
	return nil
}

func (l *Client) Clone(ctx context.Context, url string, cloneOpts repository.CloneOptions) (*git.Commit, error) {
	if err := l.validateUrl(url); err != nil {
		return nil, err
	}

	checkoutStrat := cloneOpts.CheckoutStrategy
	switch {
	case checkoutStrat.Commit != "":
		return l.cloneCommit(ctx, url, checkoutStrat.Commit, cloneOpts)
	case checkoutStrat.Tag != "":
		return l.cloneTag(ctx, url, checkoutStrat.Tag, cloneOpts)
	case checkoutStrat.SemVer != "":
		return l.cloneSemVer(ctx, url, checkoutStrat.SemVer, cloneOpts)
	default:
		branch := checkoutStrat.Branch
		if branch == "" {
			branch = git.DefaultBranch
		}
		return l.cloneBranch(ctx, url, branch, cloneOpts)
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

func (l *Client) writeFile(path string, reader io.Reader) error {
	if l.repository == nil {
		return git.ErrNoGitRepository
	}

	f, err := l.repoFS.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	if err != nil {
		return err
	}

	return nil
}

func (l *Client) Commit(info git.Commit, commitOpts ...repository.CommitOption) (string, error) {
	if l.repository == nil {
		return "", git.ErrNoGitRepository
	}

	options := &repository.CommitOptions{}
	for _, o := range commitOpts {
		o(options)
	}

	for path, content := range options.Files {
		if err := l.writeFile(path, content); err != nil {
			return "", err
		}
	}

	sl, err := l.repository.StatusList(&git2go.StatusOptions{
		Show:  git2go.StatusShowIndexAndWorkdir,
		Flags: git2go.StatusOptIncludeUntracked,
	})
	if err != nil {
		return "", err
	}
	defer sl.Free()

	unborn, err := l.repository.IsHeadUnborn()
	if err != nil {
		return "", err
	}

	count, err := sl.EntryCount()
	if err != nil {
		return "", err
	}

	// If HEAD is non-existent and there are no changes then exit early.
	if unborn && count == 0 {
		return "", git.ErrNoStagedFiles
	}

	var parentC []*git2go.Commit
	if !unborn {
		ref, err := l.repository.Head()
		if err != nil {
			return "", err
		}
		defer ref.Free()
		head, err := l.repository.LookupCommit(ref.Target())

		if err == nil {
			defer head.Free()
			parentC = append(parentC, head)
		}

		// If there are no changes, then exit by returning the SHA of the commit
		// pointed to by HEAD.
		if count == 0 {
			return head.Id().String(), git.ErrNoStagedFiles
		}
	}

	index, err := l.repository.Index()
	if err != nil {
		return "", err
	}
	defer index.Free()

	if err = util.Walk(l.repoFS, "",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// skip symlinks and any files that are within ".git/"
			if !info.Mode().IsRegular() || strings.HasPrefix(path, ".git") || path == "" {
				return nil
			}

			if err := index.AddByPath(path); err != nil {
				return err
			}
			return nil
		}); err != nil {
		return "", err
	}

	if err := index.Write(); err != nil {
		return "", err
	}

	treeID, err := index.WriteTree()
	if err != nil {
		return "", err
	}

	tree, err := l.repository.LookupTree(treeID)
	if err != nil {
		return "", err
	}
	defer tree.Free()

	sig := &git2go.Signature{
		Name:  info.Author.Name,
		Email: info.Author.Email,
		When:  time.Now(),
	}
	commitID, err := l.repository.CreateCommit("HEAD", sig, sig, info.Message, tree, parentC...)
	if err != nil {
		return "", err
	}

	// return unsigned commit if pgp entity is not provided
	if options.Signer == nil {
		return commitID.String(), nil
	}

	commit, err := l.repository.LookupCommit(commitID)
	if err != nil {
		return "", err
	}
	defer commit.Free()

	signedCommitID, err := commit.WithSignatureUsing(func(commitContent string) (string, string, error) {
		cipherText := new(bytes.Buffer)
		err := openpgp.ArmoredDetachSignText(cipherText, options.Signer, strings.NewReader(commitContent), &packet.Config{})
		if err != nil {
			return "", "", errors.New("error signing payload")
		}

		return cipherText.String(), "", nil
	})
	if err != nil {
		return "", err
	}
	signedCommit, err := l.repository.LookupCommit(signedCommitID)
	if err != nil {
		return "", err
	}
	defer signedCommit.Free()

	newHead, err := l.repository.Head()
	if err != nil {
		return "", err
	}
	defer newHead.Free()

	signedRef, err := l.repository.References.Create(
		newHead.Name(),
		signedCommit.Id(),
		true,
		"repoint to signed commit",
	)
	if err != nil {
		return "", err
	}
	signedRef.Free()

	return signedCommitID.String(), nil
}

func (l *Client) Push(ctx context.Context) error {
	if l.repository == nil {
		return git.ErrNoGitRepository
	}
	callbacks := RemoteCallbacks()

	head, err := l.repository.Head()
	if err != nil {
		return err
	}
	defer head.Free()
	branch, err := head.Branch().Name()
	if err != nil {
		return err
	}

	// calling repo.Push will succeed even if a reference update is
	// rejected; to detect this case, this callback is supplied.
	var callbackErr error
	callbacks.PushUpdateReferenceCallback = func(refname, status string) error {
		if status != "" {
			callbackErr = fmt.Errorf("ref %s rejected: %s", refname, status)
		}
		return nil
	}
	err = l.remote.Push([]string{fmt.Sprintf("refs/heads/%s:refs/heads/%[1]s", branch)}, &git2go.PushOptions{
		RemoteCallbacks: callbacks,
		ProxyOptions:    git2go.ProxyOptions{Type: git2go.ProxyTypeAuto},
	})
	if err != nil {
		return pushError(err, l.remote.Url())
	}

	return callbackErr
}

func (l *Client) SwitchBranch(ctx context.Context, branchName string) error {
	if l.repository == nil {
		return git.ErrNoGitRepository
	}
	callbacks := RemoteCallbacks()

	// Force the fetching of the remote branch.
	err := l.remote.Fetch([]string{branchName}, &git2go.FetchOptions{
		RemoteCallbacks: callbacks,
	}, "")
	if err != nil {
		return fmt.Errorf("cannot fetch remote branch: %w", err)
	}

	remoteBranch, err := l.repository.References.Lookup(fmt.Sprintf("refs/remotes/origin/%s", branchName))
	if err != nil && !git2go.IsErrorCode(err, git2go.ErrorCodeNotFound) {
		return err
	}
	if remoteBranch != nil {
		defer remoteBranch.Free()
	}
	err = nil

	var commit *git2go.Commit
	// tries to get tip commit from remote branch, if it exists.
	// otherwise gets the commit that local head is pointing to.
	if remoteBranch != nil {
		commit, err = l.repository.LookupCommit(remoteBranch.Target())
	} else {
		head, err := l.repository.Head()
		if err != nil {
			return fmt.Errorf("cannot get repo head: %w", err)
		}
		defer head.Free()
		commit, err = l.repository.LookupCommit(head.Target())
	}
	if err != nil {
		return fmt.Errorf("cannot find the head commit: %w", err)
	}
	defer commit.Free()

	localBranch, err := l.repository.References.Lookup(fmt.Sprintf("refs/heads/%s", branchName))
	if err != nil && !git2go.IsErrorCode(err, git2go.ErrorCodeNotFound) {
		return fmt.Errorf("cannot lookup branch '%s': %w", branchName, err)
	}
	if localBranch == nil {
		lb, err := l.repository.CreateBranch(branchName, commit, false)
		if err != nil {
			return fmt.Errorf("cannot create branch '%s': %w", branchName, err)
		}
		defer lb.Free()
		// We could've done something like:
		// localBranch = lb.Reference
		// But for some reason, calling `lb.Free()` AND using it, causes a really
		// nasty crash. Since, we can't avoid calling `lb.Free()`, in order to prevent
		// memory leaks, we don't use `lb` and instead manually lookup the ref.
		localBranch, err = l.repository.References.Lookup(fmt.Sprintf("refs/heads/%s", branchName))
		if err != nil {
			return fmt.Errorf("cannot lookup branch '%s': %w", branchName, err)
		}
	}
	defer localBranch.Free()

	tree, err := l.repository.LookupTree(commit.TreeId())
	if err != nil {
		return fmt.Errorf("cannot lookup tree for branch '%s': %w", branchName, err)
	}
	defer tree.Free()

	err = l.repository.CheckoutTree(tree, &git2go.CheckoutOpts{
		// the remote branch should take precedence if it exists at this point in time.
		Strategy: git2go.CheckoutForce,
	})
	if err != nil {
		return fmt.Errorf("cannot checkout tree for branch '%s': %w", branchName, err)
	}

	ref, err := localBranch.SetTarget(commit.Id(), "")
	if err != nil {
		return fmt.Errorf("cannot update branch '%s' to be at target commit: %w", branchName, err)
	}
	ref.Free()

	return l.repository.SetHead("refs/heads/" + branchName)
}

func (l *Client) IsClean() (bool, error) {
	if l.repository == nil {
		return false, git.ErrNoGitRepository
	}
	sl, err := l.repository.StatusList(&git2go.StatusOptions{
		Show:  git2go.StatusShowIndexAndWorkdir,
		Flags: git2go.StatusOptIncludeUntracked,
	})
	if err != nil {
		return false, err
	}
	defer sl.Free()

	count, err := sl.EntryCount()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}
	return false, nil
}

func (l *Client) Head() (string, error) {
	if l.repository == nil {
		return "", git.ErrNoGitRepository
	}
	head, err := l.repository.Head()
	if err != nil {
		return "", err
	}
	return head.Target().String(), nil
}

func (l *Client) Path() string {
	return l.path
}

func (l *Client) Close() {
	// Reset the remote origin of the repository back to the actual URL.
	transportOpts, found := transport.GetTransportOptions(l.transportOptsURL)
	if found {
		l.repository.Remotes.SetUrl(l.remote.Name(), transportOpts.TargetURL)
	}

	if l.remote != nil {
		l.remote.Free()
	}
	if l.repository != nil {
		l.repository.Free()
	}

	transport.RemoveTransportOptions(l.transportOptsURL)
}

// registerTransportOptions generates a dummy URL which serves as a unique key.
// It then registers a few options mapped to this url/key, that are to be used
// at the transport level.
func (l *Client) registerTransportOptions(ctx context.Context, url string) {
	l.transportOptsURL = getTransportOptsURL(l.authOpts.Transport)

	transportOpts := transport.TransportOptions{
		TargetURL:    url,
		AuthOpts:     l.authOpts,
		ProxyOptions: &git2go.ProxyOptions{Type: git2go.ProxyTypeAuto},
		Context:      ctx,
	}
	if l.proxyAddr != "" {
		transportOpts.ProxyOptions = &git2go.ProxyOptions{
			Type: git2go.ProxyTypeSpecified,
			Url:  l.proxyAddr,
		}
	}

	transport.AddTransportOptions(l.transportOptsURL, transportOpts)
}
