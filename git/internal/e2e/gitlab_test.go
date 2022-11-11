//go:build e2e
// +build e2e

/*
// Copyright 2022 The Flux authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
*/

package e2e

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/fluxcd/go-git-providers/gitlab"
	"github.com/fluxcd/go-git-providers/gitprovider"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/libgit2"
	. "github.com/onsi/gomega"
)

const (
	gitlabUsername = "fluxcd-gitprovider-bot"
	gitlabOrgname  = "fluxcd-testing"
	gitlabSSHHost  = "ssh://" + git.DefaultPublicKeyAuthUser + "@" + gitlab.DefaultDomain
	gitlabHTTPHost = "https://" + gitlab.DefaultDomain
)

var (
	gitlabPrivateToken string
)

func TestGitLabE2E(t *testing.T) {
	g := NewWithT(t)
	gitlabPrivateToken = os.Getenv(gitlabPat)
	if gitlabPrivateToken == "" {
		t.Fatalf("could not read gitlab PAT")
	}

	c, err := gitlab.NewClient(gitlabPrivateToken, "", gitprovider.WithDestructiveAPICalls(true))
	g.Expect(err).ToNot(HaveOccurred())
	orgClient := c.OrgRepositories()

	repoInfo := func(proto git.TransportType, repo gitprovider.OrgRepository) (*url.URL, *git.AuthOptions, error) {
		var repoURL *url.URL
		var authOptions *git.AuthOptions
		var err error

		if proto == git.SSH {
			repoURL, err = url.Parse(gitlabSSHHost + "/" + gitlabOrgname + "/" + repo.Repository().GetRepository())
			if err != nil {
				return nil, nil, err
			}

			sshAuth, err := createSSHIdentitySecret(*repoURL)
			if err != nil {
				return nil, nil, err
			}
			dkClient := repo.DeployKeys()
			var readOnly bool
			_, err = dkClient.Create(context.TODO(), gitprovider.DeployKeyInfo{
				Name:     "git-e2e-deploy-key" + randStringRunes(5),
				Key:      sshAuth["identity.pub"],
				ReadOnly: &readOnly,
			})
			if err != nil {
				return nil, nil, err
			}

			authOptions, err = git.NewAuthOptions(*repoURL, sshAuth)
			if err != nil {
				return nil, nil, err
			}
		} else {
			repoURL, err = url.Parse(gitlabHTTPHost + "/" + gitlabOrgname + "/" + repo.Repository().GetRepository())
			if err != nil {
				return nil, nil, err
			}
			authOptions, err = git.NewAuthOptions(*repoURL, map[string][]byte{
				"username": []byte(gitlabUsername),
				"password": []byte(gitlabPrivateToken),
			})
			if err != nil {
				return nil, nil, err
			}
		}
		return repoURL, authOptions, nil
	}

	protocols := []git.TransportType{git.HTTP, git.SSH}
	clients := []string{gogit.ClientName, libgit2.ClientName}

	testFunc := func(t *testing.T, proto git.TransportType, gitClient string) {
		t.Run(fmt.Sprintf("repo created using Clone/%s/%s", gitClient, proto), func(t *testing.T) {
			g := NewWithT(t)

			repoName := fmt.Sprintf("gitlab-e2e-checkout-%s-%s-%s", string(proto), string(gitClient), randStringRunes(5))
			upstreamRepoURL := gitlabHTTPHost + "/" + gitlabOrgname + "/" + repoName

			ref, err := gitprovider.ParseOrgRepositoryURL(upstreamRepoURL)
			g.Expect(err).ToNot(HaveOccurred())
			repo, err := orgClient.Create(context.TODO(), *ref, gitprovider.RepositoryInfo{})
			g.Expect(err).ToNot(HaveOccurred())

			defer repo.Delete(context.TODO())

			err = initRepo(upstreamRepoURL, "main", "../../testdata/git/repo", gitlabUsername, gitlabPrivateToken)
			g.Expect(err).ToNot(HaveOccurred())
			repoURL, authOptions, err := repoInfo(proto, repo)
			g.Expect(err).ToNot(HaveOccurred())

			client, err := newClient(gitClient, t.TempDir(), authOptions, false)
			g.Expect(err).ToNot(HaveOccurred())
			defer client.Close()

			testUsingClone(g, client, repoURL, upstreamRepoInfo{
				url:      upstreamRepoURL,
				username: gitlabUsername,
				password: gitlabPrivateToken,
			})
		})

		t.Run(fmt.Sprintf("repo created using Init/%s/%s", gitClient, proto), func(t *testing.T) {
			g := NewWithT(t)

			repoName := fmt.Sprintf("gitlab-e2e-checkout-%s-%s-%s", string(proto), string(gitClient), randStringRunes(5))
			upstreamRepoURL := gitlabHTTPHost + "/" + gitlabOrgname + "/" + repoName

			ref, err := gitprovider.ParseOrgRepositoryURL(upstreamRepoURL)
			g.Expect(err).ToNot(HaveOccurred())
			repo, err := orgClient.Create(context.TODO(), *ref, gitprovider.RepositoryInfo{})
			g.Expect(err).ToNot(HaveOccurred())

			defer repo.Delete(context.TODO())

			repoURL, authOptions, err := repoInfo(proto, repo)
			g.Expect(err).ToNot(HaveOccurred())

			client, err := newClient(gitClient, t.TempDir(), authOptions, false)
			g.Expect(err).ToNot(HaveOccurred())
			defer client.Close()

			testUsingInit(g, client, repoURL, upstreamRepoInfo{
				url:      upstreamRepoURL,
				username: gitlabUsername,
				password: gitlabPrivateToken,
			})
		})
	}

	for _, client := range clients {
		for _, protocol := range protocols {
			testFunc(t, protocol, client)
		}
	}
}
