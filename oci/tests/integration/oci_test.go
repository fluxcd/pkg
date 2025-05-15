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
	"fmt"
	"strings"
	"testing"
)

func TestOciImageRepositoryListTags(t *testing.T) {
	if len(testRepos) == 0 {
		t.Fatalf("expected testRepos to be set")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			args := []string{
				"-category=oci",
				fmt.Sprintf("-repo=%s", repo),
			}
			testjobExecutionWithArgs(t, args)
		})
	}
}

func TestOciImageRepositoryListTagsUsingObjectLevelWorkloadIdentity(t *testing.T) {
	if !enableWI {
		t.Skip("Skipping test as workload identity is not enabled in env")
	}

	if len(testRepos) == 0 {
		t.Fatalf("expected testRepos to be set")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			args := []string{
				"-category=oci",
				fmt.Sprintf("-repo=%s", repo),
			}
			testjobExecutionWithArgs(t, args, withObjectLevelWI(objectLevelWIModeImpersonation))
		})
	}
}

func TestOciImageRepositoryListTagsUsingObjectLevelWorkloadIdentityWithDirectAccess(t *testing.T) {
	if !testWIDirectAccess {
		t.Skip("Skipping workload identity direct access test, not supported for provider")
	}

	if len(testRepos) == 0 {
		t.Fatalf("expected testRepos to be set")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			args := []string{
				"-category=oci",
				fmt.Sprintf("-repo=%s", repo),
			}
			testjobExecutionWithArgs(t, args, withObjectLevelWI(objectLevelWIModeDirectAccess))
		})
	}
}

func TestOciRepositoryRootLoginListTags(t *testing.T) {
	if len(testRepos) == 0 {
		t.Fatalf("expected testRepos to be set")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			parts := strings.SplitN(repo, "/", 2)
			args := []string{
				"-category=oci",
				fmt.Sprintf("-registry=%s", parts[0]),
				fmt.Sprintf("-repo=%s", parts[1]),
			}
			testjobExecutionWithArgs(t, args)
		})
	}
}

func TestOciOIDCLoginListTags(t *testing.T) {
	if len(testRepos) == 0 {
		t.Fatalf("expected testRepos to be set")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			// Registry only.
			parts := strings.SplitN(repo, "/", 2)
			args := []string{
				"-category=oci",
				fmt.Sprintf("-registry=%s", parts[0]),
				fmt.Sprintf("-repo=%s", parts[1]),
			}
			testjobExecutionWithArgs(t, args)

			// Registry + repo.
			args = []string{
				"-category=oci",
				fmt.Sprintf("-repo=%s", repo),
			}
			testjobExecutionWithArgs(t, args)
		})
	}
}
