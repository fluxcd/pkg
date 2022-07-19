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
					return nil, nil, fmt.Errorf("error reading response body")
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

	protocols := []git.TransportType{git.SSH, git.HTTP}
	clients := []string{gogit.ClientName}

	testFunc := func(t *testing.T, proto git.TransportType, c string) {
		t.Run(fmt.Sprintf("repo created using Clone/%s", proto), func(t *testing.T) {
			g := NewWithT(t)

			repoName := fmt.Sprintf("gitlab-e2e-checkout-%s-%s", string(proto), randStringRunes(5))
			repoURL, authOptions, err := repoInfo(repoName, proto)
			g.Expect(err).ToNot(HaveOccurred())

			upstreamRepoURL := gitlabHTTPHost + "/" + gitlabUsername + "/" + repoName
			err = initRepo(upstreamRepoURL, "main", "../testdata/git/repo", gitlabUsername, password)
			g.Expect(err).ToNot(HaveOccurred())

			var client git.RepositoryClient
			tmp := t.TempDir()

			if c == gogit.ClientName {
				client, err = gogit.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
			}

			testUsingClone(g, client, repoURL, upstreamRepoInfo{
				url:      upstreamRepoURL,
				username: gitlabUsername,
				password: password,
			})
		})

		t.Run(fmt.Sprintf("repo created using Init/%s", proto), func(t *testing.T) {
			g := NewWithT(t)

			repoName := fmt.Sprintf("gitlab-e2e-checkout-%s-%s", string(proto), randStringRunes(5))
			repoURL, authOptions, err := repoInfo(repoName, proto)
			g.Expect(err).ToNot(HaveOccurred())
			upstreamRepoURL := gitlabHTTPHost + "/" + gitlabUsername + "/" + repoName

			var client git.RepositoryClient
			tmp := t.TempDir()

			if c == gogit.ClientName {
				client, err = gogit.NewClient(tmp, authOptions)
				g.Expect(err).ToNot(HaveOccurred())
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
