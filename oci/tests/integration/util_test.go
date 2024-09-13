//go:build integration
// +build integration

/*
Copyright 2024 The Flux authors

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

package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/go-git/go-git/v5/plumbing"
)

const (
	// default branch to be used when cloning git repositories
	defaultBranch = "main"
)

// Clones the git repository specified in the test config and commits a config
// map yaml into the repository
func setUpGitRepository(ctx context.Context, tmpDir string) error {
	c, err := getRepository(ctx, tmpDir, testGitCfg.applicationRepository, defaultBranch, testGitCfg.defaultAuthOpts)

	if err != nil {
		return err
	}

	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: foobar`
	branchName := defaultBranch

	files := make(map[string]io.Reader)
	files["configmap.yaml"] = strings.NewReader(manifest)
	return commitAndPushAll(ctx, c, files, branchName)
}

// Uses git package to get auth options
func getAuthOpts(repoURL string, authData map[string][]byte) (*git.AuthOptions, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, err
	}

	return git.NewAuthOptions(*u, authData)
}

// getRepository clones the specified branch of the git repository
func getRepository(ctx context.Context, dir, repoURL, branchName string, authOpts *git.AuthOptions) (*gogit.Client, error) {
	c, err := gogit.NewClient(dir, authOpts, gogit.WithSingleBranch(false), gogit.WithDiskStorage())
	if err != nil {
		return nil, err
	}

	_, err = c.Clone(ctx, repoURL, repository.CloneConfig{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: branchName,
		},
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// commitAndPushAll creates a commit and pushes the changes using gogit client
func commitAndPushAll(ctx context.Context, client *gogit.Client, files map[string]io.Reader, branchName string) error {
	err := client.SwitchBranch(ctx, branchName)
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return err
	}

	_, err = client.Commit(git.Commit{
		Author: git.Signature{
			Name:  git.DefaultPublicKeyAuthUser,
			Email: "test@example.com",
			When:  time.Now(),
		},
	}, repository.WithFiles(files))
	if err != nil {
		if errors.Is(err, git.ErrNoStagedFiles) {
			return nil
		}

		return err
	}

	err = client.Push(ctx, repository.PushConfig{})
	if err != nil {
		return fmt.Errorf("unable to push: %s", err)
	}

	return nil
}
