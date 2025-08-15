/*
Copyright 2025 The Flux authors

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

package gcp

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	auth "github.com/fluxcd/pkg/auth"
)

type tokenSource struct {
	ctx        context.Context
	kubeClient client.Client
	opts       []auth.Option
}

// NewTokenSource creates a new token source for the given context and options.
func NewTokenSource(ctx context.Context, kubeClient client.Client, opts ...auth.Option) oauth2.TokenSource {
	return &tokenSource{
		ctx:        ctx,
		kubeClient: kubeClient,
		opts:       opts,
	}
}

// Token implements oauth2.TokenSource.
func (t *tokenSource) Token() (*oauth2.Token, error) {
	token, err := auth.GetAccessToken(t.ctx, t.kubeClient, Provider{}, t.opts...)
	if err != nil {
		return nil, err
	}
	gcpToken, ok := token.(*Token)
	if !ok {
		return nil, fmt.Errorf("failed to cast token to GCP token: %T", token)
	}
	return &gcpToken.Token, nil
}
