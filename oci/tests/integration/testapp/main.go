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
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/clusterreader"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
	runtimeClient "github.com/fluxcd/pkg/runtime/client"
)

// registry and repo flags are to facilitate testing of two login scenarios:
//   - when the repository contains the full address, including registry host,
//     e.g. foo.azurecr.io/bar.
//   - when the repository contains only the repository name and registry name
//     is provided separately, e.g. registry: foo.azurecr.io, repo: bar.
var (
	registry      = flag.String("registry", "", "registry of the repository")
	repo          = flag.String("repo", "", "git/oci repository to list")
	gitSSH        = flag.Bool("git-ssh", false, "use git ssh authentication")
	category      = flag.String("category", "", "Test category to run - oci/git/restconfig")
	provider      = flag.String("provider", "", "oidc provider - aws, azure, gcp")
	wiSAName      = flag.String("wisa-name", "", "Name of the Workload Identity Service Account to use for authentication")
	wiSANamespace = flag.String("wisa-namespace", "", "Namespace of the Workload Identity Service Account to use for authentication")
)

var (
	authOpts   []auth.Option
	kubeClient client.Client
	proxyURL   *url.URL
)

func main() {
	flag.Parse()

	// Create in-cluster kube client.
	conf, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}
	kubeClient, err = client.New(conf, client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Configure object-level workload identity.
	if *wiSAName != "" && *wiSANamespace != "" {
		auth.EnableObjectLevelWorkloadIdentity()
		authOpts = append(authOpts, auth.WithServiceAccount(client.ObjectKey{
			Name:      *wiSAName,
			Namespace: *wiSANamespace,
		}, kubeClient))
	}

	// Configure a token cache.
	tokenCache, err := cache.NewTokenCache(100)
	if err != nil {
		panic(err)
	}
	authOpts = append(authOpts, auth.WithCache(*tokenCache, cache.InvolvedObject{
		Kind:      "TestApp",
		Name:      "testapp",
		Namespace: "default",
		Operation: "test",
	}))

	// Create and configure a test proxy.
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	s := &http.Server{
		Addr:    lis.Addr().String(),
		Handler: proxy,
	}
	go func() {
		if err := s.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			panic(err)
		}
	}()
	proxyURL = &url.URL{
		Scheme: "http",
		Host:   lis.Addr().String(),
	}
	authOpts = append(authOpts, auth.WithProxyURL(*proxyURL))

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	switch *category {
	case "oci":
		checkOci(ctx)
	case "git":
		checkGit(ctx)
	case "restconfig":
		checkRESTConfig(ctx)
	default:
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

	providers := []auth.ArtifactRegistryCredentialsProvider{
		aws.Provider{},
		azure.Provider{},
		gcp.Provider{},
	}
	for _, provider := range providers {
		if _, err = provider.ParseArtifactRepository(loginURL); err == nil {
			authenticator, err = authutils.GetArtifactRegistryCredentials(ctx, provider.GetName(), loginURL, authOpts...)
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
	if *gitSSH {
		var authSecret corev1.Secret
		secretKey := client.ObjectKey{
			Name:      "git-ssh-key",
			Namespace: "default",
		}
		if err := kubeClient.Get(ctx, secretKey, &authSecret); err != nil {
			panic(err)
		}
		authData = authSecret.Data
	}

	gitAuthOpts, err := git.NewAuthOptions(*u, authData)
	if err != nil {
		panic(err)
	}
	if !*gitSSH {
		creds, err := authutils.GetGitCredentials(ctx, *provider, authOpts...)
		if err != nil {
			panic(err)
		}
		gitAuthOpts.BearerToken = creds.BearerToken
		gitAuthOpts.Username = creds.Username
		gitAuthOpts.Password = creds.Password
	}
	cloneDir, err := os.MkdirTemp("", "test-clone")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(cloneDir)
	c, err := gogit.NewClient(cloneDir, gitAuthOpts, gogit.WithSingleBranch(false), gogit.WithDiskStorage())
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

func checkRESTConfig(ctx context.Context) {
	// Create client.
	const namespace = "default"
	kubeConfigRef := meta.KubeConfigReference{
		ConfigMapRef: &meta.LocalObjectReference{
			Name: "kubeconfig",
		},
	}
	provider := runtimeClient.ProviderRESTConfigFetcher(authutils.GetRESTConfigFetcher(authOpts...))
	impersonatorOpts := []runtimeClient.ImpersonatorOption{
		runtimeClient.WithPolling(engine.ClusterReaderFactoryFunc(clusterreader.NewDirectClusterReader)),
		runtimeClient.WithKubeConfig(&kubeConfigRef, runtimeClient.KubeConfigOptions{}, namespace, provider),
	}
	impersonator := runtimeClient.NewImpersonator(kubeClient, impersonatorOpts...)
	c, _, err := impersonator.GetClient(ctx)
	if err != nil {
		panic(err)
	}

	// Listing secrets in the default namespace should work, as the user is namespace admin.
	secrets := &corev1.SecretList{}
	if err := c.List(ctx, secrets, &client.ListOptions{Namespace: namespace}); err != nil {
		panic(err)
	}

	// Listing secrets in the kube-system namespace should fail, as the user is not cluster admin.
	secrets = &corev1.SecretList{}
	err = c.List(ctx, secrets, &client.ListOptions{Namespace: "kube-system"})
	if err == nil {
		panic("expected error when listing secrets in kube-system namespace, but got none")
	}
	if !strings.Contains(err.Error(), `cannot list resource "secrets" in API group "" in the namespace "kube-system"`) {
		panic(fmt.Sprintf("expected permission error, got: %s", err.Error()))
	}
}
