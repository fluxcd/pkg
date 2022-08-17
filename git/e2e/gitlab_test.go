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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/libgit2"
)

const (
	gitlabUsername     = "root"
	gitlabHTTPHost     = "http://127.0.0.1:8080"
	gitlabSSHHost      = "ssh://git@127.0.0.1:2222"
	gitlabPat          = "GITLAB_PAT"
	gitlabRootPassword = "GITLAB_ROOT_PASSWORD"
)

var (
	privateToken string
	password     string
)

func TestGitLabE2E(t *testing.T) {
	privateToken = os.Getenv(gitlabPat)
	if privateToken == "" {
		t.Fatalf("could not read gitlab private token")
	}

	password = os.Getenv(gitlabRootPassword)
	if password == "" {
		t.Fatalf("could not read gitlab root password")
	}
	password = strings.TrimSpace(password)

	repoInfo := func(repoName string, proto git.TransportType) (*url.URL, *git.AuthOptions, error) {
		var repoURL *url.URL
		var authOptions *git.AuthOptions
		var err error

		if proto == git.SSH {
			repoURL, err = url.Parse(gitlabSSHHost + "/" + gitlabUsername + "/" + repoName)
			if err != nil {
				return nil, nil, err
			}
			sshAuth, err := createSSHIdentitySecret(*repoURL)
			if err != nil {
				return nil, nil, err
			}

			// ref: https://docs.gitlab.com/15.0/ee/api/users.html#add-ssh-key
			sshKeyApiEndpoint, err := url.Parse(fmt.Sprintf("%s/api/v4/user/keys", gitlabHTTPHost))
			if err != nil {
				return nil, nil, err
			}

			form := url.Values{}
			form.Add("title", randStringRunes(10))
			form.Add("key", string(sshAuth["identity.pub"]))
			req, err := http.NewRequest("POST", sshKeyApiEndpoint.String(), strings.NewReader(form.Encode()))
			if err != nil {
				return nil, nil, err
			}

			req.Header = http.Header{
				"PRIVATE-TOKEN": []string{privateToken},
				"Content-Type":  []string{"multipart/form-data"},
			}

			client := http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return nil, nil, err
			}
			if resp.StatusCode != 201 {
				var body []byte
				_, err = resp.Body.Read(body)
				if err != nil {
					return nil, nil, fmt.Errorf("error reading response body: %w", err)
				}
				return nil, nil, fmt.Errorf("could not register ssh key, resp: %s %s", resp.Status, string(body))
			}

			authOptions, err = git.NewAuthOptions(*repoURL, sshAuth)
			if err != nil {
				return nil, nil, err
			}
		} else {
			repoURL, err = url.Parse(gitlabHTTPHost + "/" + gitlabUsername + "/" + repoName)
			if err != nil {
				return nil, nil, err
			}
			authOptions, err = git.NewAuthOptions(*repoURL, map[string][]byte{
				"username": []byte(gitlabUsername),
				"password": []byte(password),
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
			repoURL, authOptions, err := repoInfo(repoName, proto)
			g.Expect(err).ToNot(HaveOccurred())

			upstreamRepoURL := gitlabHTTPHost + "/" + gitlabUsername + "/" + repoName
			err = initRepo(upstreamRepoURL, "main", "../testdata/git/repo", gitlabUsername, password)
			g.Expect(err).ToNot(HaveOccurred())

			var client git.RepositoryClient
			tmp := t.TempDir()

			switch gitClient {
			case gogit.ClientName:
				client, err = gogit.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
				defer client.Close()
			case libgit2.ClientName:
				client, err = libgit2.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
				defer client.Close()
			default:
				t.Fatalf("invalid git client name: %s", gitClient)
			}

			testUsingClone(g, client, repoURL, upstreamRepoInfo{
				url:      upstreamRepoURL,
				username: gitlabUsername,
				password: password,
			})
		})

		t.Run(fmt.Sprintf("repo created using Init/%s/%s", gitClient, proto), func(t *testing.T) {
			g := NewWithT(t)

			repoName := fmt.Sprintf("gitlab-e2e-checkout-%s-%s-%s", string(proto), string(gitClient), randStringRunes(5))
			repoURL, authOptions, err := repoInfo(repoName, proto)
			g.Expect(err).ToNot(HaveOccurred())
			upstreamRepoURL := gitlabHTTPHost + "/" + gitlabUsername + "/" + repoName

			var client git.RepositoryClient
			tmp := t.TempDir()

			switch gitClient {
			case gogit.ClientName:
				client, err = gogit.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
				defer client.Close()
			case libgit2.ClientName:
				client, err = libgit2.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
				defer client.Close()
			default:
				t.Fatalf("invalid git client name: %s", gitClient)
			}

			testUsingInit(g, client, repoURL, upstreamRepoInfo{
				url:      upstreamRepoURL,
				username: gitlabUsername,
				password: password,
			})
		})
	}

	for _, client := range clients {
		for _, protocol := range protocols {
			testFunc(t, protocol, client)
		}
	}
}
