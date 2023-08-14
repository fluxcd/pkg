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
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"
)

const (
	DefaultRemote            = "origin"
	DefaultBranch            = "master"
	DefaultPublicKeyAuthUser = "git"
)

// CloneConfig provides configuration options for a Git clone.
type CloneConfig struct {
	// CheckoutStrategy defines a strategy to use while checking out
	// the cloned repository to a specific target.
	CheckoutStrategy

	// RecurseSubmodules defines if submodules should be checked out,
	// not supported by all Implementations.
	RecurseSubmodules bool

	// LastObservedCommit holds the last observed commit hash of a
	// Git repository.
	// If provided, the clone operation will compare it with the HEAD commit
	// of the branch or tag (as configured via CheckoutStrategy) in the remote
	// repository. If they match, cloning will be skipped and a "non-concrete"
	// commit will be returned, which can be verified using `IsConcreteCommit()`.
	// This functionality is not supported when using a semver range or a commit
	// to checkout.
	LastObservedCommit string

	// ShallowClone defines if the repository should be shallow cloned,
	// not supported by all implementations
	ShallowClone bool
}

// PushConfig provides configuration options for a Git push.
type PushConfig struct {
	// Refspecs is a list of refspecs to use for the push operation.
	// For details about Git Refspecs, please see:
	// https://git-scm.com/book/en/v2/Git-Internals-The-Refspec
	Refspecs []string

	// Force, if set to true, will result in a force push.
	Force bool

	// Options is a map specifying the push options that are sent
	// to the Git server when performing a push option. For details, see:
	// https://git-scm.com/docs/git-push#Documentation/git-push.txt---push-optionltoptiongt
	Options map[string]string
}

// CheckoutStrategy provides options to checkout a repository to a target.
type CheckoutStrategy struct {
	// Branch to checkout. If supported by the client, it can be combined
	// with Commit.
	Branch string

	// Tag to checkout, takes precedence over Branch.
	Tag string

	// SemVer tag expression to checkout, takes precedence over Branch and Tag.
	SemVer string `json:"semver,omitempty"`

	// RefName is the reference to checkout to. It must conform to the
	// Git reference format: https://git-scm.com/book/en/v2/Git-Internals-Git-References
	// Examples: "refs/heads/main", "refs/pull/420/head", "refs/tags/v0.1.0"
	// It takes precedence over Branch, Tag and SemVer.
	RefName string

	// Commit SHA1 to checkout, takes precedence over all the other options.
	// If supported by the client, it can be combined with Branch.
	Commit string
}

// CommitOptions provides options to configure a Git commit operation.
type CommitOptions struct {
	// Signer can be used to sign a commit using OpenPGP.
	Signer *openpgp.Entity
	// Files contains file names mapped to the file's content.
	// Its used to write files which are then included in the commit.
	Files map[string]io.Reader
}

// CommitOption defines an option for a commit operation.
type CommitOption func(*CommitOptions)

// WithSigner allows for the commit to be signed using the provided
// OpenPGP signer.
func WithSigner(signer *openpgp.Entity) CommitOption {
	return func(co *CommitOptions) {
		co.Signer = signer
	}
}

// WithFiles instructs the Git client to write the provided files and include
// them in the commit.
// files contains file names as its key and the content of the file as the
// value. If the file already exists, its overwritten.
func WithFiles(files map[string]io.Reader) CommitOption {
	return func(co *CommitOptions) {
		co.Files = files
	}
}
