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
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the AWS authentication provider.
const ProviderName = "aws"

// Provider implements the auth.Provider interface for AWS authentication.
type Provider struct{ Implementation }

// GetName implements auth.Provider.
func (Provider) GetName() string {
	return ProviderName
}

// NewControllerToken implements auth.Provider.
func (p Provider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	var o auth.Options
	o.Apply(opts...)

	var awsOpts []func(*config.LoadOptions) error

	stsRegion := o.STSRegion
	if stsRegion == "" {
		// A region is required. Try to get it somewhere else.
		switch {
		// For artifact repositories we can take advantage of the fact that ECR
		// repositories have a region we can use.
		// **Important**: This code path is required for supporting EKS Node Identity
		// for artifact repositories! This is because the environment variable
		// AWS_REGION is set automatically for IRSA or EKS Pod Identity, but
		// not for Node Identity.
		// We strive to support Node Identity for container registry-based APIs because
		// EKS users also use Node Identity for container images, so this allows a
		// simpler/consistent user experience.
		case o.ArtifactRepository != "":
			// We can safely ignore the error here, auth.GetToken() has already called
			// ParseArtifactRepository() and validated the repository at this point.
			ecrRegion, _ := p.ParseArtifactRepository(o.ArtifactRepository)
			stsRegion = ecrRegion
		// EKS sets this environment variable automatically if the controller pod is
		// properly configured with IRSA or EKS Pod Identity, so we can rely on this
		// and communicate this to users since this is controller-level configuration.
		default:
			stsRegion = os.Getenv("AWS_REGION")
			if stsRegion == "" {
				return nil, errors.New("AWS_REGION environment variable is not set in the Flux controller. " +
					"if you have properly configured IAM Roles for Service Accounts (IRSA) or EKS Pod Identity, " +
					"please delete/replace the controller pod so the EKS admission controllers can inject this " +
					"environment variable, or set it manually if the cluster is not EKS")
			}
		}
	}
	awsOpts = append(awsOpts, config.WithRegion(stsRegion))

	if e := o.STSEndpoint; e != "" {
		if err := ValidateSTSEndpoint(e); err != nil {
			return nil, err
		}
		awsOpts = append(awsOpts, config.WithBaseEndpoint(e))
	}

	if hc := o.GetHTTPClient(); hc != nil {
		awsOpts = append(awsOpts, config.WithHTTPClient(hc))
	}

	conf, err := p.impl().LoadDefaultConfig(ctx, awsOpts...)
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
func (p Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	stsRegion := o.STSRegion
	if stsRegion == "" {
		// A region is required. Try to get it somewhere else.
		switch {
		// For artifact repositories we can take advantage of the fact that ECR
		// repositories have a region we can use.
		case o.ArtifactRepository != "":
			// We can safely ignore the error here, auth.GetToken() has already called
			// ParseArtifactRepository() and validated the repository at this point.
			ecrRegion, _ := p.ParseArtifactRepository(o.ArtifactRepository)
			stsRegion = ecrRegion
		// In this case we can't rely on IRSA or EKS Pod Identity for the controller
		// pod because this is object-level configuration, so we show a different
		// error message.
		// In this error message we assume an API that has a region field, e.g. the
		// Bucket API. APIs that can extract the region from the ARN (e.g. KMS) will
		// never reach this code path.
		default:
			return nil, errors.New("an AWS region is required for authenticating with a service account. " +
				"please configure one in the object spec")
		}
	}

	roleARN, err := getRoleARN(serviceAccount)
	if err != nil {
		return nil, err
	}

	roleSessionName := getRoleSessionName(serviceAccount, stsRegion)

	awsOpts := sts.Options{
		Region: stsRegion,
	}

	if e := o.STSEndpoint; e != "" {
		if err := ValidateSTSEndpoint(e); err != nil {
			return nil, err
		}
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
	resp, err := p.impl().AssumeRoleWithWebIdentity(ctx, req, awsOpts)
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

// This regex is sourced from the AWS ECR Credential Helper (https://github.com/awslabs/amazon-ecr-credential-helper).
// It covers both public AWS partitions like amazonaws.com, China partitions like amazonaws.com.cn, and non-public partitions.
const registryPattern = `([0-9+]*).dkr.ecr(?:-fips)?\.([^/.]*)\.(amazonaws\.com[.cn]*|sc2s\.sgov\.gov|c2s\.ic\.gov|cloud\.adc-e\.uk|csp\.hci\.ic\.gov)`

var registryRegex = regexp.MustCompile(registryPattern)

// ParseArtifactRepository implements auth.Provider.
// ParseArtifactRepository returns the ECR region.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	registry, err := auth.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}

	parts := registryRegex.FindAllStringSubmatch(registry, -1)
	if len(parts) < 1 || len(parts[0]) < 3 {
		return "", fmt.Errorf("invalid AWS registry: '%s'. must match %s",
			registry, registryPattern)
	}

	// For issuing AWS registry credentials the ECR region is required.
	ecrRegion := parts[0][2]
	return ecrRegion, nil
}

// NewArtifactRegistryCredentials implements auth.Provider.
func (p Provider) NewArtifactRegistryCredentials(ctx context.Context, ecrRegion string,
	accessToken auth.Token, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	var o auth.Options
	o.Apply(opts...)

	conf := aws.Config{
		Region:      ecrRegion,
		Credentials: accessToken.(*Token).CredentialsProvider(),
	}

	if hc := o.GetHTTPClient(); hc != nil {
		conf.HTTPClient = hc
	}

	resp, err := p.impl().GetAuthorizationToken(ctx, conf)
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
		Authenticator: authn.FromConfig(authn.AuthConfig{
			Username: s[0],
			Password: s[1],
		}),
		ExpiresAt: expiresAt,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
