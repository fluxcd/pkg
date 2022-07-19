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
	"net/url"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/gittestserver"
)

func TestGitKitE2E(t *testing.T) {
	g := NewWithT(t)
	gitServer, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())

	username := "test-user"
	password := "test-pswd"
	gitServer.Auth(username, password)
	gitServer.AutoCreate()

	err = gitServer.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())

	gitServer.KeyDir(filepath.Join(gitServer.Root(), "keys"))
	err = gitServer.ListenSSH()
	g.Expect(err).ToNot(HaveOccurred())

	go func() {
		gitServer.StartSSH()
	}()
	defer gitServer.StopSSH()

	repoInfo := func(repoName string, proto git.TransportType, gitServer *gittestserver.GitServer) (*url.URL, *git.AuthOptions, error) {
		var repoURL *url.URL
		var authOptions *git.AuthOptions
		var err error
		if proto == git.SSH {
			repoURL, err = url.Parse(gitServer.SSHAddress() + "/" + repoName)
			if err != nil {
				return nil, nil, err
			}
			sshAuth, err := createSSHIdentitySecret(*repoURL)
			if err != nil {
				return nil, nil, err
			}
			authOptions, err = git.NewAuthOptions(*repoURL, sshAuth)
			if err != nil {
				return nil, nil, err
			}
		} else {
			repoURL, err = url.Parse(gitServer.HTTPAddressWithCredentials() + "/" + repoName)
			if err != nil {
				return nil, nil, err
			}
			if err != nil {
				return nil, nil, err
			}
			authOptions, err = git.NewAuthOptions(*repoURL, nil)
			if err != nil {
				return nil, nil, err
			}
		}
		return repoURL, authOptions, nil
	}

	protocols := []git.TransportType{git.SSH, git.HTTP}
	clients := []string{gogit.ClientName}

	testFunc := func(t *testing.T, proto git.TransportType, c string) {
		t.Run(fmt.Sprintf("repo created using Clone/%s", proto), func(t *testing.T) {
			g := NewWithT(t)
			var client git.RepositoryClient
			tmp := t.TempDir()
			repoName := fmt.Sprintf("gitkit-e2e-checkout-%s", string(proto))

			repoURL, authOptions, err := repoInfo(repoName, proto, gitServer)
			g.Expect(err).ToNot(HaveOccurred())

			if c == gogit.ClientName {
				client, err = gogit.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
			}

			// init repo on server
			err = gitServer.InitRepo("../testdata/git/repo", "main", repoName)
			g.Expect(err).ToNot(HaveOccurred())
			upstreamRepoPath := filepath.Join(gitServer.Root(), repoName)

			testUsingClone(g, client, repoURL, upstreamRepoInfo{
				url: upstreamRepoPath,
			})
		})

		t.Run(fmt.Sprintf("repo created using Init/%s", proto), func(t *testing.T) {
			g := NewWithT(t)
			var client git.RepositoryClient
			tmp := t.TempDir()
			repoName := fmt.Sprintf("gitkit-e2e-init-%s", string(proto))
			upstreamRepoPath := filepath.Join(gitServer.Root(), repoName)

			repoURL, authOptions, err := repoInfo(repoName, proto, gitServer)
			g.Expect(err).ToNot(HaveOccurred())

			if c == gogit.ClientName {
				client, err = gogit.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
			}

			testUsingInit(g, client, repoURL, upstreamRepoInfo{
				url: upstreamRepoPath,
			})
		})
	}
	for _, client := range clients {
		for _, protocol := range protocols {
			testFunc(t, protocol, client)
		}
	}
}
