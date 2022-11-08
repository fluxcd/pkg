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
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	git2go "github.com/libgit2/git2go/v34"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/gitutil"
	"github.com/fluxcd/pkg/version"
)

func (l *Client) cloneBranch(ctx context.Context, url, branch string, opts git.CloneOptions) (_ *git.Commit, err error) {
	defer recoverPanic(&err)

	err = l.Init(ctx, url, branch)
	if err != nil {
		return nil, err
	}

	remoteCallBacks := RemoteCallbacks()
	// Open remote connection.
	err = l.remote.ConnectFetch(&remoteCallBacks, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch-connect to remote '%s': %w", url, gitutil.LibGit2Error(err))
	}
	defer l.remote.Disconnect()

	// When the last observed revision is set, check whether it is still the
	// same at the remote branch. If so, short-circuit the clone operation here.
	if opts.LastObservedCommit != "" {
		heads, err := l.remote.Ls(branch)
		if err != nil {
			return nil, fmt.Errorf("unable to remote ls for '%s': %w", url, gitutil.LibGit2Error(err))
		}
		if len(heads) > 0 {
			hash := heads[0].Id.String()
			remoteHead := fmt.Sprintf("%s/%s", branch, hash)
			if remoteHead == opts.LastObservedCommit {
				// Construct a non-concrete commit with the existing information.
				c := &git.Commit{
					Hash:      git.Hash(hash),
					Reference: "refs/heads/" + branch,
				}
				return c, nil
			}
		}
	}

	// Limit the fetch operation to the specific branch, to decrease network usage.
	err = l.remote.Fetch([]string{branch},
		&git2go.FetchOptions{
			DownloadTags:    git2go.DownloadTagsNone,
			RemoteCallbacks: remoteCallBacks,
		},
		"")
	if err != nil {
		return nil, fmt.Errorf("unable to fetch remote '%s': %w", url, gitutil.LibGit2Error(err))
	}

	isEmpty, err := l.repository.IsEmpty()
	if err != nil {
		return nil, fmt.Errorf("unable to check if cloned repo '%s' is empty: %w", url, err)
	}
	if isEmpty {
		return nil, nil
	}

	branchRef, err := l.repository.References.Lookup(fmt.Sprintf("refs/remotes/origin/%s", branch))
	if err != nil {
		return nil, fmt.Errorf("unable to lookup branch '%s' for '%s': %w", branch, url, gitutil.LibGit2Error(err))
	}
	defer branchRef.Free()

	upstreamCommit, err := l.repository.LookupCommit(branchRef.Target())
	if err != nil {
		return nil, fmt.Errorf("unable to lookup commit '%s' for '%s': %w", branch, url, gitutil.LibGit2Error(err))
	}
	defer upstreamCommit.Free()

	// We try to lookup the branch (and create it if it doesn't exist), so that we can
	// switch the repo to the specified branch. This is done so that users of this api
	// can expect the repo to be at the desired branch, when cloned.
	localBranch, err := l.repository.LookupBranch(branch, git2go.BranchLocal)
	if git2go.IsErrorCode(err, git2go.ErrorCodeNotFound) {
		localBranch, err = l.repository.CreateBranch(branch, upstreamCommit, false)
		if err != nil {
			return nil, fmt.Errorf("unable to create local branch '%s': %w", branch, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("unable to lookup branch '%s': %w", branch, err)
	}
	defer localBranch.Free()

	tree, err := l.repository.LookupTree(upstreamCommit.TreeId())
	if err != nil {
		return nil, fmt.Errorf("unable to lookup tree for branch '%s': %w", branch, err)
	}
	defer tree.Free()

	err = l.repository.CheckoutTree(tree, &git2go.CheckoutOpts{
		// the remote branch should take precedence if it exists at this point in time.
		Strategy: git2go.CheckoutForce,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to checkout tree for branch '%s': %w", branch, err)
	}

	// Set the current head to point to the requested branch.
	err = l.repository.SetHead("refs/heads/" + branch)
	if err != nil {
		return nil, fmt.Errorf("unable to set HEAD to branch '%s':%w", branch, err)
	}

	// Use the current worktree's head as reference for the commit to be returned.
	head, err := l.repository.Head()
	if err != nil {
		return nil, fmt.Errorf("unable to resolve HEAD: %w", err)
	}
	defer head.Free()

	cc, err := l.repository.LookupCommit(head.Target())
	if err != nil {
		return nil, fmt.Errorf("unable to lookup HEAD commit '%s' for branch '%s': %w", head.Target(), branch, err)
	}
	defer cc.Free()

	return buildCommit(cc, "refs/heads/"+branch), nil
}

func (l *Client) cloneTag(ctx context.Context, url, tag string, opts git.CloneOptions) (_ *git.Commit, err error) {
	defer recoverPanic(&err)

	remoteCallBacks := RemoteCallbacks()
	err = l.Init(ctx, url, git.DefaultBranch)
	if err != nil {
		return nil, err
	}
	// Open remote connection.
	err = l.remote.ConnectFetch(&remoteCallBacks, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch-connect to remote '%s': %w", url, gitutil.LibGit2Error(err))
	}
	defer l.remote.Disconnect()

	// When the last observed revision is set, check whether it is still the
	// same at the remote branch. If so, short-circuit the clone operation here.
	if opts.LastObservedCommit != "" {
		heads, err := l.remote.Ls(tag)
		if err != nil {
			return nil, fmt.Errorf("unable to remote ls for '%s': %w", url, gitutil.LibGit2Error(err))
		}
		if len(heads) > 0 {
			hash := heads[0].Id.String()
			remoteHEAD := fmt.Sprintf("%s/%s", tag, hash)
			var same bool
			if remoteHEAD == opts.LastObservedCommit {
				same = true
			} else if len(heads) > 1 {
				hash = heads[1].Id.String()
				remoteAnnotatedHEAD := fmt.Sprintf("%s/%s", tag, hash)
				if remoteAnnotatedHEAD == opts.LastObservedCommit {
					same = true
				}
			}
			if same {
				// Construct a non-concrete commit with the existing information.
				c := &git.Commit{
					Hash:      git.Hash(hash),
					Reference: "refs/tags/" + tag,
				}
				return c, nil
			}
		}
	}

	err = l.remote.Fetch([]string{tag},
		&git2go.FetchOptions{
			DownloadTags:    git2go.DownloadTagsAuto,
			RemoteCallbacks: remoteCallBacks,
		},
		"")

	if err != nil {
		return nil, fmt.Errorf("unable to fetch remote '%s': %w", url, gitutil.LibGit2Error(err))
	}

	cc, err := checkoutDetachedDwim(l.repository, tag)
	if err != nil {
		return nil, err
	}
	defer cc.Free()
	return buildCommit(cc, "refs/tags/"+tag), nil
}

func (l *Client) cloneCommit(ctx context.Context, url, commit string, opts git.CloneOptions) (_ *git.Commit, err error) {
	defer recoverPanic(&err)

	l.registerTransportOptions(ctx, url)

	repo, err := git2go.Clone(l.transportOptsURL, l.path, &git2go.CloneOptions{
		FetchOptions: git2go.FetchOptions{
			DownloadTags:    git2go.DownloadTagsNone,
			RemoteCallbacks: RemoteCallbacks(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to clone '%s': %w", url, gitutil.LibGit2Error(err))
	}

	l.repository = repo
	remote, err := repo.Remotes.Lookup(git.DefaultRemote)
	if err != nil {
		return nil, fmt.Errorf("unable to lookup remote origin: %w", err)
	}
	l.remote = remote

	oid, err := git2go.NewOid(commit)
	if err != nil {
		return nil, fmt.Errorf("unable to create oid for '%s': %w", commit, err)
	}
	cc, err := checkoutDetachedHEAD(repo, oid)
	if err != nil {
		return nil, fmt.Errorf("git checkout error: %w", err)
	}
	return buildCommit(cc, ""), nil
}

func (l *Client) cloneSemVer(ctx context.Context, url, semverTag string, opts git.CloneOptions) (_ *git.Commit, err error) {
	defer recoverPanic(&err)

	verConstraint, err := semver.NewConstraint(semverTag)
	if err != nil {
		return nil, fmt.Errorf("semver parse error: %w", err)
	}

	l.registerTransportOptions(ctx, url)

	repo, err := git2go.Clone(l.transportOptsURL, l.path, &git2go.CloneOptions{
		FetchOptions: git2go.FetchOptions{
			DownloadTags:    git2go.DownloadTagsAll,
			RemoteCallbacks: RemoteCallbacks(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to clone '%s': %w", url, gitutil.LibGit2Error(err))
	}
	l.repository = repo
	remote, err := repo.Remotes.Lookup(git.DefaultRemote)
	if err != nil {
		return nil, fmt.Errorf("unable to lookup remote origin: %w", err)
	}
	l.remote = remote

	tags := make(map[string]string)
	tagTimestamps := make(map[string]time.Time)
	if err := repo.Tags.Foreach(func(name string, id *git2go.Oid) error {
		cleanName := strings.TrimPrefix(name, "refs/tags/")
		// The given ID can refer to both a commit and a tag, as annotated tags contain additional metadata.
		// Due to this, first attempt to resolve it as a simple tag (commit), but fallback to attempting to
		// resolve it as an annotated tag in case this results in an error.
		if c, err := repo.LookupCommit(id); err == nil {
			defer c.Free()
			// Use the commit metadata as the decisive timestamp.
			tagTimestamps[cleanName] = c.Committer().When
			tags[cleanName] = name
			return nil
		}
		t, err := repo.LookupTag(id)
		if err != nil {
			return fmt.Errorf("could not lookup '%s' as simple or annotated tag: %w", cleanName, err)
		}
		defer t.Free()
		commit, err := t.Peel(git2go.ObjectCommit)
		if err != nil {
			return fmt.Errorf("could not get commit for tag '%s': %w", t.Name(), err)
		}
		defer commit.Free()
		c, err := commit.AsCommit()
		if err != nil {
			return fmt.Errorf("could not get commit object for tag '%s': %w", t.Name(), err)
		}
		defer c.Free()
		tagTimestamps[t.Name()] = c.Committer().When
		tags[t.Name()] = name
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

	cc, err := checkoutDetachedDwim(repo, t)
	if err != nil {
		return nil, err
	}
	defer cc.Free()
	return buildCommit(cc, "refs/tags/"+t), nil
}

// checkoutDetachedDwim attempts to perform a detached HEAD checkout by first DWIMing the short name
// to get a concrete reference, and then calling checkoutDetachedHEAD.
func checkoutDetachedDwim(repo *git2go.Repository, name string) (*git2go.Commit, error) {
	ref, err := repo.References.Dwim(name)
	if err != nil {
		return nil, fmt.Errorf("unable to find '%s': %w", name, err)
	}
	defer ref.Free()
	c, err := ref.Peel(git2go.ObjectCommit)
	if err != nil {
		return nil, fmt.Errorf("could not get commit for ref '%s': %w", ref.Name(), err)
	}
	defer c.Free()
	cc, err := c.AsCommit()
	if err != nil {
		return nil, fmt.Errorf("could not get commit object for ref '%s': %w", ref.Name(), err)
	}
	defer cc.Free()
	return checkoutDetachedHEAD(repo, cc.Id())
}

// checkoutDetachedHEAD attempts to perform a detached HEAD checkout for the given commit.
func checkoutDetachedHEAD(repo *git2go.Repository, oid *git2go.Oid) (*git2go.Commit, error) {
	cc, err := repo.LookupCommit(oid)
	if err != nil {
		return nil, fmt.Errorf("git commit '%s' not found: %w", oid.String(), err)
	}
	if err = repo.SetHeadDetached(cc.Id()); err != nil {
		cc.Free()
		return nil, fmt.Errorf("could not detach HEAD at '%s': %w", oid.String(), err)
	}
	if err = repo.CheckoutHead(&git2go.CheckoutOptions{
		Strategy: git2go.CheckoutForce,
	}); err != nil {
		cc.Free()
		return nil, fmt.Errorf("git checkout error: %w", err)
	}
	return cc, nil
}

func buildCommit(c *git2go.Commit, ref string) *git.Commit {
	sig, msg, _ := c.ExtractSignature()
	return &git.Commit{
		Hash:      []byte(c.Id().String()),
		Reference: ref,
		Author:    buildSignature(c.Author()),
		Committer: buildSignature(c.Committer()),
		Signature: sig,
		Encoded:   []byte(msg),
		Message:   c.Message(),
	}
}

func buildSignature(s *git2go.Signature) git.Signature {
	return git.Signature{
		Name:  s.Name,
		Email: s.Email,
		When:  s.When,
	}
}

func recoverPanic(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("recovered from git2go panic: %v", r)
	}
}
