/*
Copyright 2021 The Flux authors

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
	"fmt"
	"io"
	"net/url"

	"github.com/ProtonMail/go-crypto/openpgp"
)

const (
	DefaultRemote            = "origin"
	DefaultBranch            = "master"
	DefaultPublicKeyAuthUser = "git"
)

// CloneOptions are the options used for a Git clone.
type CloneOptions struct {
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

// CheckoutStrategy provides options to checkout a repository to a target.
type CheckoutStrategy struct {
	// Branch to checkout. If supported by the client, it can be combined
	// with Commit.
	Branch string

	// Tag to checkout, takes precedence over Branch.
	Tag string

	// SemVer tag expression to checkout, takes precedence over Tag.
	SemVer string `json:"semver,omitempty"`

	// Commit SHA1 to checkout, takes precedence over Tag and SemVer.
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

type TransportType string

const (
	SSH   TransportType = "ssh"
	HTTPS TransportType = "https"
	HTTP  TransportType = "http"
)

// AuthOptions are the authentication options for the Transport of
// communication with a remote origin.
type AuthOptions struct {
	Transport  TransportType
	Host       string
	Username   string
	Password   string
	Identity   []byte
	KnownHosts []byte
	CAFile     []byte
}

// KexAlgos hosts the key exchange algorithms to be used for SSH connections.
// If empty, Go's default is used instead.
var KexAlgos []string

// HostKeyAlgos holds the HostKey algorithms that the SSH client will advertise
// to the server. If empty, Go's default is used instead.
var HostKeyAlgos []string

// Validate the AuthOptions against the defined Transport.
func (o AuthOptions) Validate() error {
	switch o.Transport {
	case HTTPS, HTTP:
		if o.Username == "" && o.Password != "" {
			return fmt.Errorf("invalid '%s' auth option: 'password' requires 'username' to be set", o.Transport)
		}
	case SSH:
		if o.Host == "" {
			return fmt.Errorf("invalid '%s' auth option: 'host' is required", o.Transport)
		}
		if len(o.Identity) == 0 {
			return fmt.Errorf("invalid '%s' auth option: 'identity' is required", o.Transport)
		}
		if len(o.KnownHosts) == 0 {
			return fmt.Errorf("invalid '%s' auth option: 'known_hosts' is required", o.Transport)
		}
	case "":
		return fmt.Errorf("no transport type set")
	default:
		return fmt.Errorf("unknown transport '%s'", o.Transport)
	}
	return nil
}

// NewAuthOptions constructs an AuthOptions object from the given map and URL.
// If the map is empty, it returns a minimal AuthOptions object after
// validating the result.
func NewAuthOptions(u url.URL, data map[string][]byte) (*AuthOptions, error) {
	opts := newAuthOptions(u)
	if len(data) > 0 {
		opts.Username = string(data["username"])
		opts.Password = string(data["password"])
		opts.CAFile = data["caFile"]
		opts.Identity = data["identity"]
		opts.KnownHosts = data["known_hosts"]
	}

	if opts.Username == "" {
		opts.Username = u.User.Username()
	}

	// We fallback to using "git" as the username when cloning Git
	// repositories through SSH since that's the conventional username used
	// by Git providers.
	if opts.Username == "" && opts.Transport == SSH {
		opts.Username = DefaultPublicKeyAuthUser
	}
	if opts.Password == "" {
		opts.Password, _ = u.User.Password()
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	return opts, nil
}

// newAuthOptions returns a minimal AuthOptions object constructed from
// the given URL.
func newAuthOptions(u url.URL) *AuthOptions {
	opts := &AuthOptions{
		Transport: TransportType(u.Scheme),
		Host:      u.Host,
	}

	return opts
}
