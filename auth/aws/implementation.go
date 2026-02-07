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

	"github.com/aws/aws-sdk-go-v2/aws"
	signerv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Implementation provides the required methods of the AWS libraries.
type Implementation interface {
	LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error)
	AssumeRoleWithWebIdentity(ctx context.Context, params *sts.AssumeRoleWithWebIdentityInput, options sts.Options) (*sts.AssumeRoleWithWebIdentityOutput, error)
	AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, options sts.Options) (*sts.AssumeRoleOutput, error)
	GetAuthorizationToken(ctx context.Context, cfg aws.Config) (any, error)
	GetPublicAuthorizationToken(ctx context.Context, cfg aws.Config) (any, error)
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, options eks.Options) (*eks.DescribeClusterOutput, error)
	PresignGetCallerIdentity(ctx context.Context, optFn func(*sts.PresignOptions), options sts.Options) (*signerv4.PresignedHTTPRequest, error)
}

type implementation struct{}

func (implementation) LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, optFns...)
}

func (implementation) AssumeRoleWithWebIdentity(ctx context.Context, params *sts.AssumeRoleWithWebIdentityInput, options sts.Options) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	return sts.New(options).AssumeRoleWithWebIdentity(ctx, params)
}

func (implementation) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, options sts.Options) (*sts.AssumeRoleOutput, error) {
	return sts.New(options).AssumeRole(ctx, params)
}

func (implementation) GetAuthorizationToken(ctx context.Context, cfg aws.Config) (any, error) {
	return ecr.NewFromConfig(cfg).GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
}

func (implementation) GetPublicAuthorizationToken(ctx context.Context, cfg aws.Config) (any, error) {
	return ecrpublic.NewFromConfig(cfg).GetAuthorizationToken(ctx, &ecrpublic.GetAuthorizationTokenInput{})
}

func (implementation) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, options eks.Options) (*eks.DescribeClusterOutput, error) {
	return eks.New(options).DescribeCluster(ctx, params)
}

func (implementation) PresignGetCallerIdentity(ctx context.Context, optFn func(*sts.PresignOptions), options sts.Options) (*signerv4.PresignedHTTPRequest, error) {
	return sts.NewPresignClient(sts.New(options)).PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, optFn)
}
