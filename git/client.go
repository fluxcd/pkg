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

package git

import (
	"context"
)

// RepositoryReader knows how to perform local and remote read operations
// on a Git repository.
type RepositoryReader interface {
	// Clone clones a repository from the provided url using the options provided.
	// It returns a Commit object describing the Git commit that the repository
	// HEAD points to. If the repository is empty, it returns a nil Commit.
	Clone(ctx context.Context, url string, cloneOpts CloneOptions) (*Commit, error)
	// IsClean returns whether the working tree is clean.
	IsClean() (bool, error)
	// Head returns the hash of the current HEAD of the repo.
	Head() (string, error)
	// Path returns the path of the repository.
	Path() string
	RepositoryCloser
}

// RepositoryWriter knows how to perform local and remote write operations
// on a Git repository.
type RepositoryWriter interface {
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
	Commit(info Commit, commitOpts ...CommitOption) (string, error)
	RepositoryCloser
}

// RepositoryCloser knows how to perform any operations that need to happen
// at the end of the lifecycle of a RepositoryWriter/RepositoryReader.
// When this is not required by the implementation, it can simply embed an
// anonymous pointer to DiscardRepositoryCloser.
type RepositoryCloser interface {
	// Close closes any resources that need to be closed at the end of
	// a Git repository client's lifecycle.
	Close()
}

// RepositoryClient knows how to perform local and remote operations on
// a Git repository.
type RepositoryClient interface {
	RepositoryReader
	RepositoryWriter
}

// DiscardRepositoryCloser is a RepositoryCloser which discards calls to Close().
type DiscardRepositoryCloser struct{}

func (c *DiscardRepositoryCloser) Close() {}
