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
		t.Skip(skippedMessage)
	}

	for _, tt := range []struct {
		name string
		skip bool
		opts []jobOption
	}{
		{
			name: "controller-level workload identity",
			skip: false,
		},
		{
			name: "object-level workload identity (impersonation)",
			skip: false,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeImpersonation)},
		},
		{
			name: "object-level workload identity (direct access)",
			skip: !testWIDirectAccess,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeDirectAccess)},
		},
		{
			name: "object-level workload identity (impersonation, federation)",
			skip: !testWIFederation,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeImpersonationFederation)},
		},
		{
			name: "object-level workload identity (direct access, federation)",
			skip: !testWIDirectAccess || !testWIFederation,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeDirectAccessFederation)},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip(skippedMessage)
			}

			ctx := context.TODO()
			tmpDir := t.TempDir()

			if err := setUpGitRepository(ctx, tmpDir); err != nil {
				t.Fatalf("failed setting up GitRepository: %v", err)
			}

			args := []string{
				"-category=git",
				fmt.Sprintf("-provider=%s", *targetProvider),
				fmt.Sprintf("-repo=%s", testGitCfg.applicationRepositoryWithoutUser),
			}
			testjobExecutionWithArgs(t, args, tt.opts...)
		})
	}
}

func TestGitCloneUsingSSH(t *testing.T) {
	if !testGit {
		t.Skip("Skipping git test, not supported for provider")
	}

	ctx := context.TODO()
	tmpDir := t.TempDir()

	if err := setUpGitRepository(ctx, tmpDir); err != nil {
		t.Fatalf("failed setting up GitRepository: %v", err)
	}
	t.Run("Git ssh credential test", func(t *testing.T) {
		args := []string{
			"-category=git",
			"-git-ssh=true",
			fmt.Sprintf("-provider=%s", *targetProvider),
			fmt.Sprintf("-repo=%s", gitSSHURL),
		}
		testjobExecutionWithArgs(t, args)
	})
}
