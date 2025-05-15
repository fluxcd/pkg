//go:build integration
// +build integration

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

package integration

import (
	"context"
	"fmt"
	"testing"
)

func TestGitCloneUsingProvider(t *testing.T) {
	if !testGit {
		t.Skip("Skipping git test, not supported for provider")
	}

	ctx := context.TODO()
	tmpDir := t.TempDir()

	if err := setUpGitRepository(ctx, tmpDir); err != nil {
		t.Fatalf("failed setting up GitRepository: %v", err)
	}
	t.Run("Git oidc credential test", func(t *testing.T) {
		args := []string{
			"-category=git",
			fmt.Sprintf("-provider=%s", *targetProvider),
			fmt.Sprintf("-repo=%s", testGitCfg.applicationRepositoryWithoutUser),
		}
		testjobExecutionWithArgs(t, args)
	})
}

func TestGitCloneUsingObjectLevelWorkloadIdentity(t *testing.T) {
	if !testGit {
		t.Skip("Skipping git test, not supported for provider")
	}

	ctx := context.TODO()
	tmpDir := t.TempDir()

	if err := setUpGitRepository(ctx, tmpDir); err != nil {
		t.Fatalf("failed setting up GitRepository: %v", err)
	}
	t.Run("Git oidc credential test", func(t *testing.T) {
		args := []string{
			"-category=git",
			fmt.Sprintf("-provider=%s", *targetProvider),
			fmt.Sprintf("-repo=%s", testGitCfg.applicationRepositoryWithoutUser),
		}
		testjobExecutionWithArgs(t, args, withObjectLevelWI(objectLevelWIModeImpersonation))
	})
}

func TestGitCloneUsingObjectLevelWorkloadIdentityWithDirectAccess(t *testing.T) {
	if !testGit {
		t.Skip("Skipping git test, not supported for provider")
	}

	if !testWIDirectAccess {
		t.Skip("Skipping workload identity direct access test, not supported for provider")
	}

	ctx := context.TODO()
	tmpDir := t.TempDir()

	if err := setUpGitRepository(ctx, tmpDir); err != nil {
		t.Fatalf("failed setting up GitRepository: %v", err)
	}
	t.Run("Git oidc credential test", func(t *testing.T) {
		args := []string{
			"-category=git",
			fmt.Sprintf("-provider=%s", *targetProvider),
			fmt.Sprintf("-repo=%s", testGitCfg.applicationRepositoryWithoutUser),
		}
		testjobExecutionWithArgs(t, args, withObjectLevelWI(objectLevelWIModeDirectAccess))
	})
}
