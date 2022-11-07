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
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	extgogit "github.com/fluxcd/go-git/v5"
	"github.com/fluxcd/go-git/v5/config"
	"github.com/fluxcd/go-git/v5/plumbing"
	"github.com/fluxcd/go-git/v5/plumbing/object"
	"github.com/fluxcd/go-git/v5/plumbing/transport"
	"github.com/fluxcd/go-git/v5/storage/memory"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/gitutil"
	"github.com/fluxcd/pkg/version"
)

func (g *Client) cloneBranch(ctx context.Context, url, branch string, opts git.CloneOptions) (*git.Commit, error) {
	if g.authOpts == nil {
		return nil, fmt.Errorf("unable to checkout repo with an empty set of auth options")
	}
	authMethod, err := transportAuth(g.authOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to construct auth method with options: %w", err)
	}

	ref := plumbing.NewBranchReferenceName(branch)
	// check if previous revision has changed before attempting to clone
	if opts.LastObservedCommit != "" {
		head, err := getRemoteHEAD(ctx, url, ref, g.authOpts, authMethod)
		if err != nil {
			return nil, err
		}

		if head != "" && head == opts.LastObservedCommit {
			// Construct a non-concrete commit with the existing information.
			// Split the revision and take the last part as the hash.
			// Example revision: main/43d7eb9c49cdd49b2494efd481aea1166fc22b67
			var hash git.Hash
			ss := strings.Split(head, "/")
			if len(ss) > 1 {
				hash = git.Hash(ss[len(ss)-1])
			} else {
				hash = git.Hash(ss[0])
			}
			c := &git.Commit{
				Hash:      hash,
				Reference: plumbing.NewBranchReferenceName(branch).String(),
			}
			return c, nil
		}
	}

	var depth int
	if opts.ShallowClone {
		depth = 1
	}
	cloneOpts := &extgogit.CloneOptions{
		URL:               url,
		Auth:              authMethod,
		RemoteName:        git.DefaultRemote,
		ReferenceName:     plumbing.NewBranchReferenceName(branch),
		SingleBranch:      true,
		NoCheckout:        false,
		Depth:             depth,
		RecurseSubmodules: recurseSubmodules(opts.RecurseSubmodules),
		Progress:          nil,
		Tags:              extgogit.NoTags,
		CABundle:          caBundle(g.authOpts),
	}

	repo, err := extgogit.CloneContext(ctx, g.storer, g.worktreeFS, cloneOpts)
	if err != nil {
		if err == transport.ErrRepositoryNotFound || isRemoteBranchNotFoundErr(err, ref.String()) {
			return nil, git.ErrRepositoryNotFound{
				Message: fmt.Sprintf("unable to clone: %s", err),
				URL:     url,
			}
		}
		// Directly cloning an empty Git repo to a directory fails with this error.
		// We check for the error and then init a new Git repo in that directory
		// (which represents an empty repository).
		if err == transport.ErrEmptyRemoteRepository {
			if err = os.RemoveAll(g.path); err == nil {
				if err = g.Init(ctx, url, branch); err == nil {
					return nil, nil
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("unable to clone '%s': %w", url, gitutil.GoGitError(err))
		}
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("unable to resolve HEAD of branch '%s': %w", branch, err)
	}
	cc, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("unable to resolve commit object for HEAD '%s': %w", head.Hash(), err)
	}
	g.repository = repo
	return buildCommitWithRef(cc, ref)
}

func (g *Client) cloneTag(ctx context.Context, url, tag string, opts git.CloneOptions) (*git.Commit, error) {
	if g.authOpts == nil {
		return nil, fmt.Errorf("unable to checkout repo with an empty set of auth options")
	}

	authMethod, err := transportAuth(g.authOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to construct auth method with options: %w", err)
	}

	ref := plumbing.NewTagReferenceName(tag)
	// check if previous revision has changed before attempting to clone
	if opts.LastObservedCommit != "" {
		head, err := getRemoteHEAD(ctx, url, ref, g.authOpts, authMethod)
		if err != nil {
			return nil, err
		}

		if head != "" && head == opts.LastObservedCommit {
			// Construct a non-concrete commit with the existing information.
			// Split the revision and take the last part as the hash.
			// Example revision: 6.1.4/bf09377bfd5d3bcac1e895fa8ce52dc76695c060
			var hash git.Hash
			ss := strings.Split(head, "/")
			if len(ss) > 1 {
				hash = git.Hash(ss[len(ss)-1])
			} else {
				hash = git.Hash(ss[0])
			}
			c := &git.Commit{
				Hash:      hash,
				Reference: ref.String(),
			}
			return c, nil
		}
	}

	var depth int
	if opts.ShallowClone {
		depth = 1
	}
	cloneOpts := &extgogit.CloneOptions{
		URL:               url,
		Auth:              authMethod,
		RemoteName:        git.DefaultRemote,
		ReferenceName:     plumbing.NewTagReferenceName(tag),
		SingleBranch:      true,
		NoCheckout:        false,
		Depth:             depth,
		RecurseSubmodules: recurseSubmodules(opts.RecurseSubmodules),
		Progress:          nil,
		Tags:              extgogit.NoTags,
		CABundle:          caBundle(g.authOpts),
	}

	repo, err := extgogit.CloneContext(ctx, g.storer, g.worktreeFS, cloneOpts)
	if err != nil {
		if err == transport.ErrEmptyRemoteRepository || err == transport.ErrRepositoryNotFound || isRemoteBranchNotFoundErr(err, ref.String()) {
			return nil, git.ErrRepositoryNotFound{
				Message: fmt.Sprintf("unable to clone: %s", err),
				URL:     url,
			}
		}
		return nil, fmt.Errorf("unable to clone '%s': %w", url, gitutil.GoGitError(err))
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("unable to resolve HEAD of tag '%s': %w", tag, err)
	}
	cc, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("unable to resolve commit object for HEAD '%s': %w", head.Hash(), err)
	}
	g.repository = repo
	return buildCommitWithRef(cc, ref)
}

func (g *Client) cloneCommit(ctx context.Context, url, commit string, opts git.CloneOptions) (*git.Commit, error) {
	authMethod, err := transportAuth(g.authOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to construct auth method with options: %w", err)
	}
	cloneOpts := &extgogit.CloneOptions{
		URL:               url,
		Auth:              authMethod,
		RemoteName:        git.DefaultRemote,
		SingleBranch:      false,
		NoCheckout:        true,
		RecurseSubmodules: recurseSubmodules(opts.RecurseSubmodules),
		Progress:          nil,
		Tags:              extgogit.NoTags,
		CABundle:          caBundle(g.authOpts),
	}
	if opts.Branch != "" {
		cloneOpts.SingleBranch = true
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Branch)
	}

	repo, err := extgogit.CloneContext(ctx, g.storer, g.worktreeFS, cloneOpts)
	if err != nil {
		if err == transport.ErrEmptyRemoteRepository || err == transport.ErrRepositoryNotFound ||
			isRemoteBranchNotFoundErr(err, cloneOpts.ReferenceName.String()) {
			return nil, git.ErrRepositoryNotFound{
				Message: fmt.Sprintf("unable to clone: %s", err),
				URL:     url,
			}
		}
		return nil, fmt.Errorf("unable to clone '%s': %w", url, gitutil.GoGitError(err))
	}

	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("unable to open repo worktree: %w", err)
	}
	cc, err := repo.CommitObject(plumbing.NewHash(commit))
	if err != nil {
		return nil, fmt.Errorf("unable to resolve commit object for '%s': %w", commit, err)
	}
	err = w.Checkout(&extgogit.CheckoutOptions{
		Hash:  cc.Hash,
		Force: true,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to checkout commit '%s': %w", commit, err)
	}
	g.repository = repo
	return buildCommitWithRef(cc, cloneOpts.ReferenceName)
}

func (g *Client) cloneSemVer(ctx context.Context, url, semverTag string, opts git.CloneOptions) (*git.Commit, error) {
	verConstraint, err := semver.NewConstraint(semverTag)
	if err != nil {
		return nil, fmt.Errorf("semver parse error: %w", err)
	}

	authMethod, err := transportAuth(g.authOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to construct auth method with options: %w", err)
	}
	var depth int
	if opts.ShallowClone {
		depth = 1
	}
	cloneOpts := &extgogit.CloneOptions{
		URL:               url,
		Auth:              authMethod,
		RemoteName:        git.DefaultRemote,
		NoCheckout:        false,
		Depth:             depth,
		RecurseSubmodules: recurseSubmodules(opts.RecurseSubmodules),
		Progress:          nil,
		Tags:              extgogit.AllTags,
		CABundle:          caBundle(g.authOpts),
	}

	repo, err := extgogit.CloneContext(ctx, g.storer, g.worktreeFS, cloneOpts)
	if err != nil {
		if err == transport.ErrEmptyRemoteRepository || err == transport.ErrRepositoryNotFound {
			return nil, git.ErrRepositoryNotFound{
				Message: fmt.Sprintf("unable to clone: %s", err),
				URL:     url,
			}
		}
		return nil, fmt.Errorf("unable to clone '%s': %w", url, gitutil.GoGitError(err))
	}

	repoTags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("unable to list tags: %w", err)
	}

	tags := make(map[string]string)
	tagTimestamps := make(map[string]time.Time)
	if err = repoTags.ForEach(func(t *plumbing.Reference) error {
		revision := plumbing.Revision(t.Name().String())
		hash, err := repo.ResolveRevision(revision)
		if err != nil {
			return fmt.Errorf("unable to resolve tag revision: %w", err)
		}
		commit, err := repo.CommitObject(*hash)
		if err != nil {
			return fmt.Errorf("unable to resolve commit of a tag revision: %w", err)
		}
		tagTimestamps[t.Name().Short()] = commit.Committer.When

		tags[t.Name().Short()] = t.Strings()[1]
		return nil
	}); err != nil {
		return nil, err
	}

	var matchedVersions semver.Collection
	for tag := range tags {
		v, err := version.ParseVersion(tag)
		if err != nil {
			continue
		}
		if !verConstraint.Check(v) {
			continue
		}
		matchedVersions = append(matchedVersions, v)
	}
	if len(matchedVersions) == 0 {
		return nil, fmt.Errorf("no match found for semver: %s", semverTag)
	}

	// Sort versions
	sort.SliceStable(matchedVersions, func(i, j int) bool {
		left := matchedVersions[i]
		right := matchedVersions[j]

		if !left.Equal(right) {
			return left.LessThan(right)
		}

		// Having tag target timestamps at our disposal, we further try to sort
		// versions into a chronological order. This is especially important for
		// versions that differ only by build metadata, because it is not considered
		// a part of the comparable version in Semver
		return tagTimestamps[left.Original()].Before(tagTimestamps[right.Original()])
	})
	v := matchedVersions[len(matchedVersions)-1]
	t := v.Original()

	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("unable to open Git worktree: %w", err)
	}

	ref := plumbing.NewTagReferenceName(t)
	err = w.Checkout(&extgogit.CheckoutOptions{
		Branch: ref,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to checkout tag '%s': %w", t, err)
	}
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("unable to resolve HEAD of tag '%s': %w", t, err)
	}
	cc, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("unable to resolve commit object for HEAD '%s': %w", head.Hash(), err)
	}
	g.repository = repo
	return buildCommitWithRef(cc, ref)
}

func recurseSubmodules(recurse bool) extgogit.SubmoduleRescursivity {
	if recurse {
		return extgogit.DefaultSubmoduleRecursionDepth
	}
	return extgogit.NoRecurseSubmodules
}

func getRemoteHEAD(ctx context.Context, url string, ref plumbing.ReferenceName,
	authOpts *git.AuthOptions, authMethod transport.AuthMethod) (string, error) {
	config := &config.RemoteConfig{
		Name: git.DefaultRemote,
		URLs: []string{url},
	}
	remote := extgogit.NewRemote(memory.NewStorage(), config)
	listOpts := &extgogit.ListOptions{
		Auth:     authMethod,
		CABundle: authOpts.CAFile,
	}
	refs, err := remote.ListContext(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("unable to list remote for '%s': %w", url, err)
	}

	head := filterRefs(refs, ref)
	return head, nil
}

func filterRefs(refs []*plumbing.Reference, currentRef plumbing.ReferenceName) string {
	for _, ref := range refs {
		if ref.Name().String() == currentRef.String() {
			return fmt.Sprintf("%s/%s", currentRef.Short(), ref.Hash().String())
		}
	}

	return ""
}

func buildSignature(s object.Signature) git.Signature {
	return git.Signature{
		Name:  s.Name,
		Email: s.Email,
		When:  s.When,
	}
}

func buildCommitWithRef(c *object.Commit, ref plumbing.ReferenceName) (*git.Commit, error) {
	if c == nil {
		return nil, fmt.Errorf("unable to construct commit: no object")
	}

	// Encode commit components excluding signature into SignedData.
	encoded := &plumbing.MemoryObject{}
	if err := c.EncodeWithoutSignature(encoded); err != nil {
		return nil, fmt.Errorf("unable to encode commit '%s': %w", c.Hash, err)
	}
	reader, err := encoded.Reader()
	if err != nil {
		return nil, fmt.Errorf("unable to encode commit '%s': %w", c.Hash, err)
	}
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to read encoded commit '%s': %w", c.Hash, err)
	}
	return &git.Commit{
		Hash:      []byte(c.Hash.String()),
		Reference: ref.String(),
		Author:    buildSignature(c.Author),
		Committer: buildSignature(c.Committer),
		Signature: c.PGPSignature,
		Encoded:   b,
		Message:   c.Message,
	}, nil
}

func isRemoteBranchNotFoundErr(err error, ref string) bool {
	return strings.Contains(err.Error(), fmt.Sprintf("couldn't find remote ref '%s'", ref))
}
