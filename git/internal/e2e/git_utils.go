//go:build e2e
// +build e2e

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

package e2e

import (
	"fmt"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/libgit2"
	"github.com/fluxcd/pkg/git/repository"
)

func newClient(gitClient, tmp string, authOptions *git.AuthOptions, insecure bool) (repository.Client, error) {
	switch gitClient {
	case gogit.ClientName:
		if insecure {
			return gogit.NewClient(tmp, authOptions, gogit.WithInsecureCredentialsOverHTTP(), gogit.WithDiskStorage())
		}
		return gogit.NewClient(tmp, authOptions)
	case libgit2.ClientName:
		if insecure {
			return libgit2.NewClient(tmp, authOptions, libgit2.WithInsecureCredentialsOverHTTP(), libgit2.WithDiskStorage())
		}
		return libgit2.NewClient(tmp, authOptions)
	}
	return nil, fmt.Errorf("invalid git client name: %s", gitClient)
}
