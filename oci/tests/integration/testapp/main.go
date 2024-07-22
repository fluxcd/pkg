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
	"fmt"
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

	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/fluxcd/pkg/oci/auth/login"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	gitAuth "github.com/fluxcd/pkg/auth/git"
)

// registry and repo flags are to facilitate testing of two login scenarios:
//   - when the repository contains the full address, including registry host,
//     e.g. foo.azurecr.io/bar.
//   - when the repository contains only the repository name and registry name
//     is provided separately, e.g. registry: foo.azurecr.io, repo: bar.
var (
	registry  = flag.String("registry", "", "registry of the repository")
	repo      = flag.String("repo", "", "git/oci repository to list")
	oidcLogin = flag.Bool("oidc-login", false, "login with OIDCLogin function")
	category  = flag.String("category", "", "Test category to run - oci/git")
	provider  = flag.String("provider", "", "Supported oidc provider - azure")
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if *category == "oci" {
		checkOci(ctx)
	} else if *category == "git" {
		checkGit(ctx)
	} else {
		panic("unsupported category")
	}
}

func checkOci(ctx context.Context) {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	cache, err := cache.New(5, cache.StoreObjectKeyFunc,
		cache.WithCleanupInterval[cache.StoreObject[authn.Authenticator]](1*time.Second))
	if err != nil {
		panic(err)
	}
	opts := login.ProviderOptions{
		AwsAutoLogin:   true,
		GcpAutoLogin:   true,
		AzureAutoLogin: true,
		Cache:          cache,
	}

	if *repo == "" {
		panic("must provide -repo value")
	}

	var loginURL string
	var auth authn.Authenticator
	var ref name.Reference

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

	if *oidcLogin {
		auth, err = login.NewManager().OIDCLogin(ctx, fmt.Sprintf("https://%s", loginURL), opts)
	} else {
		auth, err = login.NewManager().Login(ctx, loginURL, ref, opts)
	}

	if err != nil {
		panic(err)
	}
	log.Println("logged in")

	var options []remote.Option
	options = append(options, remote.WithAuth(auth))
	options = append(options, remote.WithContext(ctx))

	tags, err := remote.List(ref.Context(), options...)
	if err != nil {
		panic(err)
	}
	log.Println("tags:", tags)
}

func checkGit(ctx context.Context) {
	log.Println("Validating git oidc by cloning repo ", *repo)
	providerCreds, err := getProviderCreds(ctx, *repo, *provider)
	if err != nil {
		panic(err)
	}

	authData := providerCreds.ToSecretData()
	u, err := url.Parse(*repo)
	if err != nil {
		panic(err)
	}

	authOpts, err := git.NewAuthOptions(*u, authData)
	if err != nil {
		panic(err)
	}

	cloneDir, err := os.MkdirTemp("", "test-clone-")
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

func getProviderCreds(ctx context.Context, url, provider string) (*gitAuth.Credentials, error) {
	var providerCreds *gitAuth.Credentials
	switch provider {
	case auth.ProviderAzure:
		authOpts := &gitAuth.AuthOptions{}
		authOpts.ProviderOptions = gitAuth.ProviderOptions{
			AzureOpts: []azure.ProviderOptFunc{
				azure.WithAzureDevOpsScope(),
			},
		}
		c, err := cache.New(10, cache.StoreObjectKeyFunc,
			cache.WithCleanupInterval[cache.StoreObject[gitAuth.Credentials]](5*time.Second))
		if err != nil {
			return nil, err
		}
		authOpts.Cache = c
		providerCreds, err = gitAuth.GetCredentials(ctx, url, provider, authOpts)
		if err != nil {
			return nil, err
		}
		// Add other providers here
	}

	return providerCreds, nil
}
