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
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the AWS authentication provider.
const ProviderName = "aws"

// Provider implements the auth.Provider interface for AWS authentication.
type Provider struct{}

// GetName implements auth.Provider.
func (Provider) GetName() string {
	return ProviderName
}

// NewDefaultToken implements auth.Provider.
func (Provider) NewDefaultToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	var o auth.Options
	o.Apply(opts...)

	var awsOpts []func(*config.LoadOptions) error

	region := getRegion()
	awsOpts = append(awsOpts, config.WithRegion(region))

	if e := o.STSEndpoint; e != "" {
		awsOpts = append(awsOpts, config.WithBaseEndpoint(e))
	}

	if hc := o.GetHTTPClient(); hc != nil {
		awsOpts = append(awsOpts, config.WithHTTPClient(hc))
	}

	conf, err := config.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return nil, err
	}
	creds, err := conf.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, err
	}

	return newTokenFromAWSCredentials(&creds), nil
}

// GetAudience implements auth.Provider.
func (Provider) GetAudience(ctx context.Context) (string, error) {
	return "sts.amazonaws.com", nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	roleARN, err := getRoleARN(serviceAccount)
	if err != nil {
		return "", err
	}
	return roleARN, nil
}

// NewTokenForServiceAccount implements auth.Provider.
func (Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	roleARN, err := getRoleARN(serviceAccount)
	if err != nil {
		return nil, err
	}

	roleSessionName := getRoleSessionName(serviceAccount)

	var awsOpts sts.Options

	region := getRegion()
	awsOpts.Region = region

	if e := o.STSEndpoint; e != "" {
		awsOpts.BaseEndpoint = &e
	}

	if u := o.ProxyURL; u != nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(u)
		httpClient := &http.Client{Transport: transport}
		awsOpts.HTTPClient = httpClient
	}

	req := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleARN,
		RoleSessionName:  &roleSessionName,
		WebIdentityToken: &oidcToken,
	}
	resp, err := sts.New(awsOpts).AssumeRoleWithWebIdentity(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Credentials == nil {
		return nil, fmt.Errorf("credentials are nil")
	}

	token := &Token{*resp.Credentials}
	if token.Expiration == nil {
		token.Expiration = &time.Time{}
	}

	return token, nil
}

// GetArtifactCacheKey implements auth.Provider.
func (Provider) GetArtifactCacheKey(artifactRepository string) string {
	if _, region, ok := ParseRegistry(artifactRepository); ok {
		return region
	}
	return ""
}

// NewArtifactRegistryToken implements auth.Provider.
func (Provider) NewArtifactRegistryToken(ctx context.Context, artifactRepository string,
	accessToken auth.Token, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	_, region, ok := ParseRegistry(artifactRepository)
	if !ok {
		return nil, fmt.Errorf("invalid ecr repository: '%s'", artifactRepository)
	}

	credsProvider := accessToken.(*Token).Provider()

	conf := aws.Config{
		Region:      region,
		Credentials: credsProvider,
	}

	if hc := o.GetHTTPClient(); hc != nil {
		conf.HTTPClient = hc
	}

	resp, err := ecr.NewFromConfig(conf).GetAuthorizationToken(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Parse the authorization token.
	if len(resp.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data returned")
	}
	tokenResp := resp.AuthorizationData[0]
	if tokenResp.AuthorizationToken == nil {
		return nil, fmt.Errorf("authorization token is nil")
	}
	token := *tokenResp.AuthorizationToken
	b, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse authorization token: %w", err)
	}
	s := strings.Split(string(b), ":")
	if len(s) != 2 {
		return nil, fmt.Errorf("invalid authorization token format")
	}
	var expiresAt time.Time
	if exp := tokenResp.ExpiresAt; exp != nil {
		expiresAt = *exp
	}
	return &auth.ArtifactRegistryCredentials{
		Username:  s[0],
		Password:  s[1],
		ExpiresAt: expiresAt,
	}, nil
}
