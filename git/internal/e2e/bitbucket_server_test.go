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
	"strings"
	"testing"

	bitbucket "github.com/fluxcd/go-git-providers/stash"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/libgit2"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
)

const (
	projectKey  = "FLUX"
	sshPort     = "7999"
	stashToken  = "STASH_TOKEN"
	stashUser   = "STASH_USER"
	stashDomain = "STASH_DOMAIN"
)

var (
	bitbucketServerToken    string
	bitbucketServerUser     string
	bitbucketServerHost     string
	bitbucketServerUsername string
	deployKey               *bitbucket.DeployKey
)

func TestBitbucketServerE2E(t *testing.T) {
	g := NewWithT(t)
	bitbucketServerToken = os.Getenv(stashToken)
	if bitbucketServerToken == "" {
		t.Fatalf("could not read bitbucket token")
	}
	bitbucketServerUsername = os.Getenv(stashUser)
	if bitbucketServerUsername == "" {
		t.Fatalf("could not read bitbucket user")
	}
	bitbucketServerHost = os.Getenv(stashDomain)
	if bitbucketServerHost == "" {
		t.Fatalf("could not read bitbucket domain")
	}
	bitbucketServerHTTPHost := "https://" + bitbucketServerHost + "/scm"

	bitbucketURL, err := url.Parse(bitbucketServerHost)
	g.Expect(err).ToNot(HaveOccurred())
	bitbucketServerSSHHost := "ssh://" + git.DefaultPublicKeyAuthUser + "@" + bitbucketURL.Hostname() + ":" + sshPort

	c, err := bitbucket.NewClient(nil, bitbucketServerHTTPHost, nil, logr.Discard(), bitbucket.WithAuth(bitbucketServerUsername, bitbucketServerToken))
	g.Expect(err).ToNot(HaveOccurred())

	repoInfo := func(proto git.TransportType, repo *bitbucket.Repository) (*url.URL, *git.AuthOptions, error) {
		var repoURL *url.URL
		var authOptions *git.AuthOptions
		var err error

		if proto == git.SSH {
			repoURL, err = url.Parse(bitbucketServerSSHHost + "/" + strings.ToLower(projectKey) + "/" + repo.Name)
			if err != nil {
				return nil, nil, err
			}

			sshAuth, err := createSSHIdentitySecret(*repoURL)
			if err != nil {
				return nil, nil, err
			}
			dkClient := c.DeployKeys
			deployKey, err = dkClient.Create(context.TODO(), &bitbucket.DeployKey{
				Key: bitbucket.Key{
					Text: string(sshAuth["identity.pub"]),
				},
				Permission: "REPO_WRITE",
				Repository: *repo,
			})
			if err != nil {
				return nil, nil, err
			}

			authOptions, err = git.NewAuthOptions(*repoURL, sshAuth)
			if err != nil {
				return nil, nil, err
			}
		} else {
			repoURL, err = url.Parse(bitbucketServerHTTPHost + "/" + strings.ToLower(projectKey) + "/" + repo.Name)
			if err != nil {
				return nil, nil, err
			}
			authOptions, err = git.NewAuthOptions(*repoURL, map[string][]byte{
				"username": []byte(bitbucketServerUsername),
				"password": []byte(bitbucketServerToken),
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

			repoName := fmt.Sprintf("github-e2e-checkout-%s-%s-%s", string(proto), string(gitClient), randStringRunes(5))
			upstreamRepoURL := bitbucketServerHTTPHost + "/" + projectKey + "/" + repoName

			repo, err := c.Repositories.Create(context.TODO(), projectKey, &bitbucket.Repository{
				Name:          repoName,
				ScmID:         "git",
				DefaultBranch: "main",
			})
			g.Expect(err).ToNot(HaveOccurred())

			defer c.Repositories.Delete(context.TODO(), projectKey, repoName)

			err = initRepo(upstreamRepoURL, "main", "../../testdata/git/repo", bitbucketServerUsername, bitbucketServerToken)
			g.Expect(err).ToNot(HaveOccurred())
			repoURL, authOptions, err := repoInfo(proto, repo)
			g.Expect(err).ToNot(HaveOccurred())

			client, err := newClient(gitClient, t.TempDir(), authOptions, false)
			g.Expect(err).ToNot(HaveOccurred())
			defer client.Close()

			testUsingClone(g, client, repoURL, upstreamRepoInfo{
				url:      upstreamRepoURL,
				username: bitbucketServerUsername,
				password: bitbucketServerToken,
			})
		})

		t.Run(fmt.Sprintf("repo created using Init/%s/%s", gitClient, proto), func(t *testing.T) {
			g := NewWithT(t)

			repoName := fmt.Sprintf("github-e2e-checkout-%s-%s-%s", string(proto), string(gitClient), randStringRunes(5))
			upstreamRepoURL := bitbucketServerHTTPHost + "/" + projectKey + "/" + repoName

			repo, err := c.Repositories.Create(context.TODO(), projectKey, &bitbucket.Repository{
				Name:          repoName,
				ScmID:         "git",
				DefaultBranch: "main",
			})
			g.Expect(err).ToNot(HaveOccurred())

			defer c.Repositories.Delete(context.TODO(), projectKey, repoName)

			repoURL, authOptions, err := repoInfo(proto, repo)
			g.Expect(err).ToNot(HaveOccurred())

			client, err := newClient(gitClient, t.TempDir(), authOptions, false)
			g.Expect(err).ToNot(HaveOccurred())
			defer client.Close()

			testUsingInit(g, client, repoURL, upstreamRepoInfo{
				url:      upstreamRepoURL,
				username: bitbucketServerUsername,
				password: bitbucketServerToken,
			})
		})
	}

	for _, client := range clients {
		for _, protocol := range protocols {
			testFunc(t, protocol, client)
		}
	}
}
