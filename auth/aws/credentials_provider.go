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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/fluxcd/pkg/auth"
)

type credentialsProvider struct {
	ctx  context.Context
	opts []auth.Option
}

// NewCredentialsProvider creates a new credentials provider for the given options.
func NewCredentialsProvider(ctx context.Context, opts ...auth.Option) aws.CredentialsProvider {
	return &credentialsProvider{ctx, opts}
}

// Retrieve implements aws.CredentialsProvider.
// The context is ignored, use the constructor to set the context.
// This is because some callers of the library pass context.Background()
// when calling this method (e.g. SOPS), so to ensure we have a real
// context we pass it in the constructor.
func (c *credentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	token, err := auth.GetAccessToken(c.ctx, Provider{}, c.opts...)
	if err != nil {
		return aws.Credentials{}, err
	}
	awsCreds, ok := token.(*Credentials)
	if !ok {
		return aws.Credentials{}, fmt.Errorf("failed to cast token to AWS token: %T", token)
	}
	return aws.Credentials{
		AccessKeyID:     *awsCreds.AccessKeyId,
		SecretAccessKey: *awsCreds.SecretAccessKey,
		SessionToken:    *awsCreds.SessionToken,
		Expires:         *awsCreds.Expiration,
		CanExpire:       true,
	}, nil
}
