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

	for _, tt := range []struct {
		name string
		skip bool
		opts []jobOption
	}{
		{
			// Only for artifact repositories we support node-level.
			// This test verifies node-level when the cluster is
			// configured without workload identity, and verifies
			// controller-level otherwise.
			name: "node-level or controller-level workload identity",
			skip: false,
		},
		{
			name: "object-level workload identity (impersonation)",
			skip: !enableWI,
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

			for name, repo := range testRepos {
				t.Run(name, func(t *testing.T) {
					args := []string{
						"-category=oci",
						fmt.Sprintf("-repo=%s", repo),
					}
					testjobExecutionWithArgs(t, args, tt.opts...)
				})
			}
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
