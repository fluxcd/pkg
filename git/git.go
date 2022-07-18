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
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

type Hash []byte

// String returns the Hash as a string.
func (h Hash) String() string {
	return string(h)
}

// Signature represents an entity which associates a person and a time
// with a commit.
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

// Commit contains all possible information about a Git commit.
type Commit struct {
	// Hash is the SHA1 hash of the commit.
	Hash Hash
	// Reference is the original reference of the commit, for example:
	// 'refs/tags/foo'.
	Reference string
	// Author is the original author of the commit.
	Author Signature
	// Committer is the one performing the commit, might be different from
	// Author.
	Committer Signature
	// Signature is the PGP signature of the commit.
	Signature string
	// Encoded is the encoded commit, without any signature.
	Encoded []byte
	// Message is the commit message, contains arbitrary text.
	Message string
}

// String returns a string representation of the Commit, composed
// out the last part of the Reference element, and/or Hash.
// For example: 'tag-1/a0c14dc8580a23f79bc654faa79c4f62b46c2c22',
// for a "tag-1" tag.
func (c *Commit) String() string {
	if short := strings.SplitAfterN(c.Reference, "/", 3); len(short) == 3 {
		return fmt.Sprintf("%s/%s", short[2], c.Hash)
	}
	return fmt.Sprintf("HEAD/%s", c.Hash)
}

// Verify the Signature of the commit with the given key rings.
// It returns the fingerprint of the key the signature was verified
// with, or an error.
func (c *Commit) Verify(keyRing ...string) (string, error) {
	if c.Signature == "" {
		return "", fmt.Errorf("commit does not have a PGP signature")
	}

	for _, r := range keyRing {
		reader := strings.NewReader(r)
		keyring, err := openpgp.ReadArmoredKeyRing(reader)
		if err != nil {
			return "", fmt.Errorf("unable to read armored key ring: %w", err)
		}
		signer, err := openpgp.CheckArmoredDetachedSignature(keyring, bytes.NewBuffer(c.Encoded), bytes.NewBufferString(c.Signature), nil)
		if err == nil {
			return signer.PrimaryKey.KeyIdString(), nil
		}
	}
	return "", fmt.Errorf("unable to verify commit with any of the given key rings")
}

// ShortMessage returns the first 50 characters of a commit subject.
func (c *Commit) ShortMessage() string {
	subject := strings.Split(c.Message, "\n")[0]
	r := []rune(subject)
	if len(r) > 50 {
		return fmt.Sprintf("%s...", string(r[0:50]))
	}
	return subject
}

// ErrRepositoryNotFound indicates that the repository (or the ref in
// question) does not exist at the given URL.
type ErrRepositoryNotFound struct {
	Message string
	URL     string
}

func (e ErrRepositoryNotFound) Error() string {
	return fmt.Sprintf("%s: git repository: '%s'", e.Message, e.URL)
}

var (
	ErrNoGitRepository = errors.New("no git repository")
	ErrNoStagedFiles   = errors.New("no staged files")
)

// IsConcreteCommit returns if a given commit is a concrete commit. Concrete
// commits have most of commit metadata and commit content. In contrast, a
// partial commit may only have some metadata and no commit content.
func IsConcreteCommit(c Commit) bool {
	if c.Hash != nil && c.Encoded != nil {
		return true
	}
	return false
}
