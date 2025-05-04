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

package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
)

// registry and repo flags are to facilitate testing of two login scenarios:
//   - when the repository contains the full address, including registry host,
//     e.g. foo.azurecr.io/bar.
//   - when the repository contains only the repository name and registry name
//     is provided separately, e.g. registry: foo.azurecr.io, repo: bar.
var (
	registry = flag.String("registry", "", "registry of the repository")
	repo     = flag.String("repo", "", "git/oci repository to list")
	category = flag.String("category", "", "Test category to run - oci/git")
	provider = flag.String("provider", "", "Supported git oidc provider - azure")
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	if *category == "oci" {
		checkOci(ctx)
	} else if *category == "git" {
		checkGit(ctx)
	} else {
		panic("unsupported category")
	}
}

func checkOci(ctx context.Context) {
	if *repo == "" {
		panic("must provide -repo value")
	}

	var loginURL string
	var authenticator authn.Authenticator
	var ref name.Reference
	var err error

	if *registry != "" {
		// Registry and repository are separate.
		log.Printf("registry: %s, repo: %s\n", *registry, *repo)
		loginURL = *registry
		ref, err = name.ParseReference(strings.Join([]string{*registry, *repo}, "/"))
	} else {
		// Repository contains the registry host address.
		log.Println("repo:", *repo)
		loginURL = *repo
		ref, err = name.ParseReference(*repo)
	}
	if err != nil {
		panic(err)
	}

	for _, provider := range []auth.Provider{aws.Provider{}, azure.Provider{}, gcp.Provider{}} {
		if _, err = provider.ParseArtifactRepository(loginURL); err == nil {
			authenticator, err = authutils.GetArtifactRegistryCredentials(ctx, provider.GetName(), loginURL)
			break
		}
	}

	if err != nil {
		panic(err)
	}
	log.Println("logged in")

	var options []remote.Option
	options = append(options, remote.WithAuth(authenticator))
	options = append(options, remote.WithContext(ctx))

	tags, err := remote.List(ref.Context(), options...)
	if err != nil {
		panic(err)
	}
	log.Println("tags:", tags)
}

func checkGit(ctx context.Context) {
	u, err := url.Parse(*repo)
	if err != nil {
		panic(err)
	}

	var authData map[string][]byte
	authOpts, err := git.NewAuthOptions(*u, authData)
	if err != nil {
		panic(err)
	}
	authOpts.ProviderOpts = &git.ProviderOptions{
		Name: *provider,
		AuthOpts: []auth.Option{
			auth.WithScopes(azure.ScopeDevOps),
		},
	}
	cloneDir, err := os.MkdirTemp("", "test-clone")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(cloneDir)
	c, err := gogit.NewClient(cloneDir, authOpts, gogit.WithSingleBranch(false), gogit.WithDiskStorage())
	if err != nil {
		panic(err)
	}

	_, err = c.Clone(ctx, *repo, repository.CloneConfig{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: "main",
		},
	})
	if err != nil {
		panic(err)
	}

	log.Println("Successfully cloned repository ")
	// Check file from clone.
	fPath := filepath.Join(cloneDir, "configmap.yaml")
	if _, err := os.Stat(fPath); os.IsNotExist(err) {
		panic("expected artifact configmap.yaml to exist in clone dir")
	}

	// read the whole file at once
	contents, err := os.ReadFile(fPath)
	if err != nil {
		panic(err)
	}
	log.Println(string(contents))
}
