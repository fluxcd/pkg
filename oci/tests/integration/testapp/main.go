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
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/fluxcd/pkg/oci/auth/login"
)

// registry and repo flags are to facilitate testing of two login scenarios:
//   - when the repository contains the full address, including registry host,
//     e.g. foo.azurecr.io/bar.
//   - when the repository contains only the repository name and registry name
//     is provided separately, e.g. registry: foo.azurecr.io, repo: bar.
var (
	registry  = flag.String("registry", "", "registry of the repository")
	repo      = flag.String("repo", "", "repository to list")
	oidcLogin = flag.Bool("oidc-login", false, "login with OIDCLogin function")
)

func main() {
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	opts := login.ProviderOptions{
		AwsAutoLogin:   true,
		GcpAutoLogin:   true,
		AzureAutoLogin: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if *repo == "" {
		panic("must provide -repo value")
	}

	var loginURL string
	var auth authn.Authenticator
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
