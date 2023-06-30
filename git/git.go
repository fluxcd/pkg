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

const (
	// HashTypeSHA1 is the SHA1 hash algorithm.
	HashTypeSHA1 = "sha1"
	// HashTypeUnknown is an unknown hash algorithm.
	HashTypeUnknown = "<unknown>"
)

// Hash is the (non-truncated) SHA-1 or SHA-256 hash of a Git commit.
type Hash []byte

// Algorithm returns the algorithm of the hash based on its length.
// This is heuristic, and may not be accurate for truncated user constructed
// hashes. The library itself does not produce truncated hashes.
func (h Hash) Algorithm() string {
	switch len(h) {
	case 40:
		return HashTypeSHA1
	default:
		return HashTypeUnknown
	}
}

// Digest returns a digest of the commit, in the format of "<algorithm>:<hash>".
func (h Hash) Digest() string {
	if len(h) == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%s", h.Algorithm(), h)
}

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
	// Hash is the hash of the commit.
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
	// Message is the commit message, containing arbitrary text.
	Message string
	// ReferencingTag is the parent tag, that points to this commit.
	ReferencingTag *AnnotatedTag
}

// String returns a string representation of the Commit, composed
// out of the last part of the Reference element (if not empty) and Hash.
// For example: 'tag-1@sha1:a0c14dc8580a23f79bc654faa79c4f62b46c2c22',
// for a "refs/tags/tag-1" Reference.
func (c *Commit) String() string {
	if short := strings.SplitAfterN(c.Reference, "/", 3); len(short) == 3 {
		return fmt.Sprintf("%s@%s", short[2], c.Hash.Digest())
	}
	return c.Hash.Digest()
}

// AbsoluteReference returns a string representation of the Commit, composed
// out of the Reference element (if not empty) and Hash.
// For example: 'refs/tags/tag-1@sha1:a0c14dc8580a23f79bc654faa79c4f62b46c2c22'
// for a "refs/tags/tag-1" Reference.
func (c *Commit) AbsoluteReference() string {
	if c.Reference != "" {
		return fmt.Sprintf("%s@%s", c.Reference, c.Hash.Digest())
	}
	return c.Hash.Digest()
}

// Verify the Signature of the commit with the given key rings.
// It returns the fingerprint of the key the signature was verified
// with, or an error. It does not verify the signature of the parent
// tag (if present). Users are expected to explicitly verify the parent
// tag's signature using `c.ReferencingTag.Verify()`
func (c *Commit) Verify(keyRings ...string) (string, error) {
	fingerprint, err := verifySignature(c.Signature, c.Encoded, keyRings...)
	if err != nil {
		return "", fmt.Errorf("unable to verify Git commit: %w", err)
	}
	return fingerprint, nil
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

// AnnotatedTag represents an annotated Git tag.
type AnnotatedTag struct {
	// Hash is the hash of the tag.
	Hash Hash
	// Name is the name of the tag.
	Name string
	// Author is the original author of the tag.
	Author Signature
	// Signature is the PGP signature of the tag.
	Signature string
	// Encoded is the encoded tag, without any signature.
	Encoded []byte
	// Message is the tag message, containing arbitrary text.
	Message string
}

// Verify the Signature of the tag with the given key rings.
// It returns the fingerprint of the key the signature was verified
// with, or an error.
func (t *AnnotatedTag) Verify(keyRings ...string) (string, error) {
	fingerprint, err := verifySignature(t.Signature, t.Encoded, keyRings...)
	if err != nil {
		return "", fmt.Errorf("unable to verify Git tag: %w", err)
	}
	return fingerprint, nil
}

// String returns a short string representation of the tag in the format
// of <name@hash>, for eg: <1.0.0@a0c14dc8580a23f79bc654faa79c4f62b46c2c22>
func (t *AnnotatedTag) String() string {
	return fmt.Sprintf("%s@%s", t.Name, t.Hash.String())
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
// commits have most of the commit metadata and content. In contrast, a partial
// commit may only have some metadata and no commit content.
func IsConcreteCommit(c Commit) bool {
	if c.Hash != nil && c.Encoded != nil {
		return true
	}
	return false
}

func verifySignature(sig string, payload []byte, keyRings ...string) (string, error) {
	if sig == "" {
		return "", fmt.Errorf("unable to verify payload as the provided signature is empty")
	}

	for _, r := range keyRings {
		reader := strings.NewReader(r)
		keyring, err := openpgp.ReadArmoredKeyRing(reader)
		if err != nil {
			return "", fmt.Errorf("unable to read armored key ring: %w", err)
		}
		signer, err := openpgp.CheckArmoredDetachedSignature(keyring, bytes.NewBuffer(payload), bytes.NewBufferString(sig), nil)
		if err == nil {
			return signer.PrimaryKey.KeyIdString(), nil
		}
	}
	return "", fmt.Errorf("unable to verify payload with any of the given key rings")
}
