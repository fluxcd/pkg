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

package repository

import (
	"context"

	"github.com/fluxcd/pkg/git"
)

// Reader knows how to perform local and remote read operations
// on a Git repository.
type Reader interface {
	// Clone clones a repository from the provided url using the options provided.
	// It returns a Commit object describing the Git commit that the repository
	// HEAD points to. If the repository is empty, it returns a nil Commit.
	Clone(ctx context.Context, url string, cloneOpts git.CloneOptions) (*git.Commit, error)
	// IsClean returns whether the working tree is clean.
	IsClean() (bool, error)
	// Head returns the hash of the current HEAD of the repo.
	Head() (string, error)
	// Path returns the path of the repository.
	Path() string
	Closer
}

// Writer knows how to perform local and remote write operations
// on a Git repository.
type Writer interface {
	// Init initializes a repository at the configured path with the remote
	// origin set to url on the provided branch.
	Init(ctx context.Context, url, branch string) error
	// Push pushes the current branch of the repository to origin.
	Push(ctx context.Context) error
	// SwitchBranch switches from the current branch of the repository to the
	// provided branch. If the branch doesn't exist, it is created.
	SwitchBranch(ctx context.Context, branch string) error
	// Commit commits any changes made to the repository. commitOpts is an
	// optional argument which can be provided to configure the commit.
	Commit(info git.Commit, commitOpts ...git.CommitOption) (string, error)
	Closer
}

// Closer knows how to perform any operations that need to happen
// at the end of the lifecycle of a Writer/Reader.
// When this is not required by the implementation, it can simply embed an
// anonymous pointer to DiscardCloser.
type Closer interface {
	// Close closes any resources that need to be closed at the end of
	// a Git repository client's lifecycle.
	Close()
}

// Client knows how to perform local and remote operations on
// a Git repository.
type Client interface {
	Reader
	Writer
}

// DiscardCloser is a Closer which discards calls to Close().
type DiscardCloser struct{}

func (c *DiscardCloser) Close() {}
