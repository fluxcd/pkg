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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/pkg/git/signatures"
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
	// ReferencingTag is the tag that points to this commit.
	ReferencingTag *Tag
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

// wrapper function to ensure backwards compatibility
func (c *Commit) Verify(keyRings ...string) (string, error) {
	return c.VerifyGPG(keyRings...)
}

// Verify the Signature of the commit with the given key rings.
// It returns the fingerprint of the key the signature was verified
// with, or an error. It does not verify the signature of the referencing
// tag (if present). Users are expected to explicitly verify the referencing
// tag's signature using `c.ReferencingTag.Verify()`
func (c *Commit) VerifyGPG(keyRings ...string) (string, error) {
	fingerprint, err := signatures.VerifyPGPSignature(c.Signature, c.Encoded, keyRings...)
	if err != nil {
		return "", fmt.Errorf("unable to verify Git commit: %w", err)
	}
	return fingerprint, nil
}

// VerifySSH verifies the SSH signature of the commit with the given authorized keys.
// It returns the fingerprint of the key the signature was verified with, or an error.
// It does not verify the signature of the referencing tag (if present). Users are
// expected to explicitly verify the referencing tag's signature using `c.ReferencingTag.VerifySSH()`
func (c *Commit) VerifySSH(authorizedKeys ...string) (string, error) {
	fingerprint, err := signatures.VerifySSHSignature(c.Signature, c.Encoded, authorizedKeys...)
	if err != nil {
		return "", fmt.Errorf("unable to verify Git commit SSH signature: %w", err)
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

// Tag represents a Git tag.
type Tag struct {
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

// wrapper function to ensure backwards compatibility
func (t *Tag) Verify(keyRings ...string) (string, error) {
	return t.VerifyGPG(keyRings...)
}

// Verify the Signature of the tag with the given key rings.
// It returns the fingerprint of the key the signature was verified
// with, or an error.
func (t *Tag) VerifyGPG(keyRings ...string) (string, error) {
	fingerprint, err := signatures.VerifyPGPSignature(t.Signature, t.Encoded, keyRings...)
	if err != nil {
		return "", fmt.Errorf("unable to verify Git tag: %w", err)
	}
	return fingerprint, nil
}

// VerifySSH verifies the SSH signature of the tag with the given authorized keys.
// It returns the fingerprint of the key the signature was verified with, or an error.
func (t *Tag) VerifySSH(authorizedKeys ...string) (string, error) {
	fingerprint, err := signatures.VerifySSHSignature(t.Signature, t.Encoded, authorizedKeys...)
	if err != nil {
		return "", fmt.Errorf("unable to verify Git tag SSH signature: %w", err)
	}
	return fingerprint, nil
}

// String returns a short string representation of the tag in the format
// of <name@hash>, for eg: "1.0.0@a0c14dc8580a23f79bc654faa79c4f62b46c2c22"
// If the tag is lightweight, it won't have a hash, so it'll simply return
// the tag name, i.e. "1.0.0".
func (t *Tag) String() string {
	if len(t.Hash) == 0 {
		return t.Name
	}
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

// IsAnnotatedTag returns true if the provided tag is annotated.
func IsAnnotatedTag(t Tag) bool {
	return len(t.Encoded) > 0
}

// IsSignedTag returns true if the provided tag has a signature.
func IsSignedTag(t Tag) bool {
	return t.Signature != ""
}

// IsPGPSigned returns true if the commit has a PGP signature.
func (c *Commit) IsPGPSigned() bool {
	return signatures.IsPGPSignature(c.Signature)
}

// IsSSHSigned returns true if the commit has an SSH signature.
func (c *Commit) IsSSHSigned() bool {
	return signatures.IsSSHSignature(c.Signature)
}

// SignatureType returns the type of the commit signature as a string.
// It returns "pgp" for PGP signatures, "ssh" for SSH signatures,
// and "unknown" for unrecognized or empty signatures.
func (c *Commit) SignatureType() string {
	return signatures.GetSignatureType(c.Signature)
}

// IsPGPSigned returns true if the tag has a PGP signature.
func (t *Tag) IsPGPSigned() bool {
	return signatures.IsPGPSignature(t.Signature)
}

// IsSSHSigned returns true if the tag has an SSH signature.
func (t *Tag) IsSSHSigned() bool {
	return signatures.IsSSHSignature(t.Signature)
}

// SignatureType returns the type of the tag signature as a string.
// It returns "pgp" for PGP signatures, "ssh" for SSH signatures,
// and "unknown" for unrecognized or empty signatures.
func (t *Tag) SignatureType() string {
	return signatures.GetSignatureType(t.Signature)
}
