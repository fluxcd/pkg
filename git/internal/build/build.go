/*
Copyright 2026 The Flux authors

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

package build

import (
	"fmt"
	"io"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/fluxcd/pkg/git"
)

// toGitSignature converts a go-git object.Signature to the
// matching git.Signature value. Named to avoid shadowing the
// signature package name in callers within this module.
func toGitSignature(s object.Signature) git.Signature {
	return git.Signature{
		Name:  s.Name,
		Email: s.Email,
		When:  s.When,
	}
}

func Tag(t *object.Tag, ref plumbing.ReferenceName) (*git.Tag, error) {
	if t == nil {
		return &git.Tag{
			Name: ref.Short(),
		}, nil
	}

	encoded := &plumbing.MemoryObject{}
	if err := t.EncodeWithoutSignature(encoded); err != nil {
		return nil, fmt.Errorf("unable to encode tag '%s': %w", t.Name, err)
	}
	reader, err := encoded.Reader()
	if err != nil {
		return nil, fmt.Errorf("unable to encode tag '%s': %w", t.Name, err)
	}
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to read encoded tag '%s': %w", t.Name, err)
	}

	return &git.Tag{
		Hash:      []byte(t.Hash.String()),
		Name:      t.Name,
		Author:    toGitSignature(t.Tagger),
		Signature: t.PGPSignature,
		Encoded:   b,
		Message:   t.Message,
	}, nil
}

func CommitWithRef(c *object.Commit, t *object.Tag, ref plumbing.ReferenceName) (*git.Commit, error) {
	if c == nil {
		return nil, fmt.Errorf("unable to construct commit: no object")
	}

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
	cc := &git.Commit{
		Hash:      []byte(c.Hash.String()),
		Reference: ref.String(),
		Author:    toGitSignature(c.Author),
		Committer: toGitSignature(c.Committer),
		Signature: c.PGPSignature,
		Encoded:   b,
		Message:   c.Message,
	}

	if ref.IsTag() {
		tt, err := Tag(t, ref)
		if err != nil {
			return nil, err
		}
		cc.ReferencingTag = tt
	}

	return cc, nil
}
