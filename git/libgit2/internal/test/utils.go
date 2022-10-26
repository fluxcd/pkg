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

package test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git2go "github.com/libgit2/git2go/v34"
)

func CommitFile(repo *git2go.Repository, path, content string, time time.Time) (*git2go.Oid, error) {
	var parentC []*git2go.Commit
	head, err := HeadCommit(repo)
	if err == nil {
		defer head.Free()
		parentC = append(parentC, head)
	}

	index, err := repo.Index()
	if err != nil {
		return nil, err
	}
	defer index.Free()

	if repo.IsBare() {
		blobOID, err := repo.CreateBlobFromBuffer([]byte(content))
		if err != nil {
			return nil, err
		}

		entry := &git2go.IndexEntry{
			Mode: git2go.FilemodeBlob,
			Id:   blobOID,
			Path: path,
		}
		if err := index.Add(entry); err != nil {
			return nil, err
		}
	} else {
		f, err := os.Create(filepath.Join(repo.Workdir(), path))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		io.Copy(f, strings.NewReader(content))

		if err := index.AddByPath(path); err != nil {
			return nil, err
		}
	}

	if err := index.Write(); err != nil {
		return nil, err
	}
	treeID, err := index.WriteTree()
	if err != nil {
		return nil, err
	}
	tree, err := repo.LookupTree(treeID)
	if err != nil {
		return nil, err
	}
	defer tree.Free()

	c, err := repo.CreateCommit("HEAD", MockSignature(time), MockSignature(time), "Committing "+path, tree, parentC...)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func InitRepo(t *testing.T, bare bool) (*git2go.Repository, error) {
	tmpDir := t.TempDir()
	repo, err := git2go.InitRepository(tmpDir, bare)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func MockSignature(time time.Time) *git2go.Signature {
	return &git2go.Signature{
		Name:  "Jane Doe",
		Email: "author@example.com",
		When:  time,
	}
}

func CreateBranch(repo *git2go.Repository, branch string, commit *git2go.Commit) error {
	if commit == nil {
		var err error
		commit, err = HeadCommit(repo)
		if err != nil {
			return err
		}
		defer commit.Free()
	}
	_, err := repo.CreateBranch(branch, commit, false)
	return err
}

func HeadCommit(repo *git2go.Repository) (*git2go.Commit, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	defer head.Free()
	c, err := repo.LookupCommit(head.Target())
	if err != nil {
		return nil, err
	}
	return c, nil
}

func Push(path, branch string, cb git2go.RemoteCallbacks) error {
	repo, err := git2go.OpenRepository(path)
	if err != nil {
		return err
	}
	defer repo.Free()
	origin, err := repo.Remotes.Lookup("origin")
	if err != nil {
		return err
	}
	defer origin.Free()

	err = origin.Push([]string{fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)}, &git2go.PushOptions{
		RemoteCallbacks: cb,
		ProxyOptions:    git2go.ProxyOptions{Type: git2go.ProxyTypeAuto},
	})
	if err != nil {
		return err
	}
	return nil
}
