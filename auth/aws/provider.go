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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/eks"
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

	confOpts := []func(*config.LoadOptions) error{
		config.WithHTTPClient(o.GetHTTPClient()),
	}

	stsRegion := o.STSRegion
	if stsRegion == "" {
		// EKS sets this environment variable automatically if the controller pod is
		// properly configured with IRSA or EKS Pod Identity, so we can rely on it.
		stsRegion = os.Getenv("AWS_REGION")
		if stsRegion == "" {
			return nil, errors.New("AWS_REGION environment variable is not set in the Flux controller. " +
				"if you have properly configured IAM Roles for Service Accounts (IRSA) or EKS Pod Identity, " +
				"please delete/replace the controller pod so the EKS admission controllers can inject this " +
				"environment variable, or set it manually if the cluster is not EKS")
		}
	}
	confOpts = append(confOpts, config.WithRegion(stsRegion))

	if e := o.STSEndpoint; e != "" {
		if err := ValidateSTSEndpoint(e); err != nil {
			return nil, err
		}
		confOpts = append(confOpts, config.WithBaseEndpoint(e))
	}

	conf, err := p.impl().LoadDefaultConfig(ctx, confOpts...)
	if err != nil {
		return nil, err
	}
	creds, err := conf.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, err
	}

	return newTokenFromAWSCredentials(&creds), nil
}

// GetAudiences implements auth.Provider.
func (Provider) GetAudiences(context.Context, corev1.ServiceAccount) ([]string, error) {
	return []string{"sts.amazonaws.com"}, nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	return getRoleARN(serviceAccount)
}

// NewTokenForServiceAccount implements auth.Provider.
func (p Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	stsRegion, err := getSTSRegionForObjectLevel(&o)
	if err != nil {
		return nil, err
	}

	roleARN, err := getRoleARN(serviceAccount)
	if err != nil {
		return nil, err
	}

	roleSessionName := getRoleSessionNameForServiceAccount(serviceAccount, stsRegion)

	stsOpts := sts.Options{
		Region:     stsRegion,
		HTTPClient: o.GetHTTPClient(),
	}

	if e := o.STSEndpoint; e != "" {
		if err := ValidateSTSEndpoint(e); err != nil {
			return nil, err
		}
		stsOpts.BaseEndpoint = &e
	}

	req := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleARN,
		RoleSessionName:  &roleSessionName,
		WebIdentityToken: &oidcToken,
	}
	resp, err := p.impl().AssumeRoleWithWebIdentity(ctx, req, stsOpts)
	if err != nil {
		return nil, err
	}
	if resp.Credentials == nil {
		return nil, fmt.Errorf("credentials are nil")
	}

	creds := &Credentials{*resp.Credentials}
	if creds.Expiration == nil {
		creds.Expiration = &time.Time{}
	}

	return creds, nil
}

// GetImpersonationAnnotationKey implements auth.ProviderWithImpersonation.
func (Provider) GetImpersonationAnnotationKey() string {
	return "assume-role"
}

type impersonation struct {
	RoleARN string `json:"roleARN"`
}

func (i impersonation) String() string {
	return i.RoleARN
}

// GetIdentityForImpersonation implements auth.ProviderWithImpersonation.
func (Provider) GetIdentityForImpersonation(identity json.RawMessage) (fmt.Stringer, error) {
	var id impersonation
	if err := json.Unmarshal(identity, &id); err != nil {
		return nil, fmt.Errorf("failed to unmarshal impersonation identity: %w", err)
	}
	if !roleARNRegex.MatchString(id.RoleARN) {
		return nil, fmt.Errorf("invalid role ARN in impersonation identity: '%s'. must match %s",
			id.RoleARN, roleARNPattern)
	}
	return &id, nil
}

// NewTokenForIdentity implements auth.ProviderWithImpersonation.
func (p Provider) NewTokenForIdentity(ctx context.Context, token auth.Token,
	identity fmt.Stringer, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	stsRegion, err := getSTSRegionForObjectLevel(&o)
	if err != nil {
		return nil, err
	}

	roleARN := identity.(*impersonation).RoleARN

	roleName, err := getRoleNameFromARN(roleARN)
	if err != nil {
		return nil, err
	}
	roleSessionName := getRoleSessionNameForImpersonation(roleName, stsRegion)

	stsOpts := sts.Options{
		Region:      stsRegion,
		Credentials: token.(*Credentials).provider(),
		HTTPClient:  o.GetHTTPClient(),
	}

	if e := o.STSEndpoint; e != "" {
		if err := ValidateSTSEndpoint(e); err != nil {
			return nil, err
		}
		stsOpts.BaseEndpoint = &e
	}

	req := &sts.AssumeRoleInput{
		RoleArn:         &roleARN,
		RoleSessionName: &roleSessionName,
	}
	resp, err := p.impl().AssumeRole(ctx, req, stsOpts)
	if err != nil {
		return nil, err
	}
	if resp.Credentials == nil {
		return nil, fmt.Errorf("credentials are nil")
	}

	creds := &Credentials{*resp.Credentials}
	if creds.Expiration == nil {
		creds.Expiration = &time.Time{}
	}

	return creds, nil
}

// GetAccessTokenOptionsForArtifactRepository implements auth.ArtifactRegistryCredentialsProvider.
func (p Provider) GetAccessTokenOptionsForArtifactRepository(artifactRepository string) ([]auth.Option, error) {
	// AWS requires a region for getting access credentials. To avoid requiring
	// two regions to be passed in the Flux APIs we leverage the region present
	// in the ECR repository.
	// **Important**: This code path is required for supporting the identity of
	// the EKS node! The AWS_REGION environment variable is only automatically
	// set for IRSA and EKS Pod Identity. We strive to support the identity of
	// the node for artifact repository APIs because EKS users also use it for
	// for pulling container images to spin up pods inside the cluster, so this
	// allows a simpler user experience setting up ECR authentication only once.
	registryInput, err := p.ParseArtifactRepository(artifactRepository)
	if err != nil {
		return nil, err
	}
	ecrRegion := getECRRegionFromRegistryInput(registryInput)
	return []auth.Option{auth.WithSTSRegion(ecrRegion)}, nil
}

// This regex is sourced from the AWS ECR Credential Helper (https://github.com/awslabs/amazon-ecr-credential-helper).
// It covers both public AWS partitions like amazonaws.com, China partitions like amazonaws.com.cn, and non-public partitions.
const registryPattern = `([0-9+]*).dkr.ecr(?:-fips)?\.([^/.]*)\.(amazonaws\.com[.cn]*|amazonaws\.eu|sc2s\.sgov\.gov|c2s\.ic\.gov|cloud\.adc-e\.uk|csp\.hci\.ic\.gov)`

const publicECR = "public.ecr.aws"

var registryRegex = regexp.MustCompile(registryPattern)

// ParseArtifactRepository implements auth.ArtifactRegistryCredentialsProvider.
// ParseArtifactRepository returns the ECR region, unless the registry
// is public.ecr.aws, in which case it returns public.ecr.aws.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	registry, err := auth.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}

	if registry == publicECR {
		return publicECR, nil
	}

	parts := registryRegex.FindAllStringSubmatch(registry, -1)
	if len(parts) < 1 || len(parts[0]) < 3 {
		return "", fmt.Errorf("invalid AWS registry: '%s'. must match %s",
			registry, registryPattern)
	}

	ecrRegion := parts[0][2]
	return ecrRegion, nil
}

func getECRRegionFromRegistryInput(registryInput string) string {
	if registryInput == publicECR {
		// Region is required to be us-east-1 for public ECR:
		// https://docs.aws.amazon.com/AmazonECR/latest/public/public-registry-auth.html#public-registry-auth-token
		return "us-east-1"
	}
	return registryInput
}

// NewArtifactRegistryCredentials implements auth.ArtifactRegistryCredentialsProvider.
func (p Provider) NewArtifactRegistryCredentials(ctx context.Context, registryInput string,
	accessToken auth.Token, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	var o auth.Options
	o.Apply(opts...)

	authTokenFunc := p.impl().GetAuthorizationToken
	if registryInput == publicECR {
		authTokenFunc = p.impl().GetPublicAuthorizationToken
	}

	conf := aws.Config{
		Region:      getECRRegionFromRegistryInput(registryInput),
		Credentials: accessToken.(*Credentials).provider(),
		HTTPClient:  o.GetHTTPClient(),
	}

	respAny, err := authTokenFunc(ctx, conf)
	if err != nil {
		return nil, err
	}

	// Parse the authorization token.
	var token string
	var expiresAt time.Time
	switch resp := respAny.(type) {
	case *ecr.GetAuthorizationTokenOutput:
		if len(resp.AuthorizationData) == 0 {
			return nil, fmt.Errorf("no authorization data returned")
		}
		if resp.AuthorizationData[0].AuthorizationToken == nil {
			return nil, fmt.Errorf("authorization token is nil")
		}
		if resp.AuthorizationData[0].ExpiresAt == nil {
			return nil, fmt.Errorf("authorization token expiration is nil")
		}
		token = *resp.AuthorizationData[0].AuthorizationToken
		expiresAt = *resp.AuthorizationData[0].ExpiresAt
	case *ecrpublic.GetAuthorizationTokenOutput:
		if resp.AuthorizationData == nil {
			return nil, fmt.Errorf("no authorization data returned")
		}
		if resp.AuthorizationData.AuthorizationToken == nil {
			return nil, fmt.Errorf("authorization token is nil")
		}
		if resp.AuthorizationData.ExpiresAt == nil {
			return nil, fmt.Errorf("authorization token expiration is nil")
		}
		token = *resp.AuthorizationData.AuthorizationToken
		expiresAt = *resp.AuthorizationData.ExpiresAt
	}
	b, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse authorization token: %w", err)
	}
	s := strings.Split(string(b), ":")
	if len(s) != 2 {
		return nil, fmt.Errorf("invalid authorization token format")
	}
	return &auth.ArtifactRegistryCredentials{
		Authenticator: &authn.Basic{
			Username: s[0],
			Password: s[1],
		},
		ExpiresAt: expiresAt,
	}, nil
}

// GetAccessTokenOptionsForCluster implements auth.RESTConfigProvider.
func (Provider) GetAccessTokenOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {
	var o auth.Options
	o.Apply(opts...)
	// ClusterResource is always needed for AWS as we need to extract the region.
	region, _, err := parseCluster(o.ClusterResource)
	if err != nil {
		return nil, err
	}
	return [][]auth.Option{{auth.WithSTSRegion(region)}}, nil
}

// NewRESTConfig implements auth.RESTConfigProvider.
//
// Reference:
// https://docs.aws.amazon.com/eks/latest/best-practices/identity-and-access-management.html#_controlling_access_to_eks_clusters
func (p Provider) NewRESTConfig(ctx context.Context, accessTokens []auth.Token,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	// The expiration for an EKS restconfig is always 15 minutes, see the reference above.
	// Let's record time.Now() on the beginning of the procedure to be on the safe side.
	expiresAt := time.Now().Add(15 * time.Minute)

	creds := accessTokens[0].(*Credentials).provider()

	var o auth.Options
	o.Apply(opts...)
	hc := o.GetHTTPClient()

	// ClusterResource is always needed for AWS as we need to extract the region.
	cluster := o.ClusterResource
	region, clusterName, err := parseCluster(cluster)
	if err != nil {
		return nil, err
	}

	// Describe the cluster resource to get missing CA or endpoint.
	host := o.ClusterAddress
	caData := []byte(o.CAData)
	if host == "" || len(caData) == 0 {
		describeInput := &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		}
		eksOpts := eks.Options{
			Region:      region,
			Credentials: creds,
			HTTPClient:  hc,
		}
		clusterResource, err := p.impl().DescribeCluster(ctx, describeInput, eksOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to describe EKS cluster '%s': %w", cluster, err)
		}

		// Update host and CA with cluster details.
		if host == "" {
			host = *clusterResource.Cluster.Endpoint
		}
		if len(caData) == 0 {
			caData, err = base64.StdEncoding.DecodeString(*clusterResource.Cluster.CertificateAuthority.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode EKS CA certificate: %w", err)
			}
		}
	}

	// Build token. See reference above.
	presignOpts := func(po *sts.PresignOptions) {
		po.Presigner = &eksHTTPPresignerV4{
			HTTPPresignerV4: po.Presigner,
			clusterName:     clusterName,
		}
	}
	stsOpts := sts.Options{
		Region:      region,
		Credentials: creds,
		HTTPClient:  hc,
	}
	if e := o.STSEndpoint; e != "" {
		if err := ValidateSTSEndpoint(e); err != nil {
			return nil, err
		}
		stsOpts.BaseEndpoint = &e
	}
	presignedReq, err := p.impl().PresignGetCallerIdentity(ctx, presignOpts, stsOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to presign GetCallerIdentity request: %w", err)
	}
	token := fmt.Sprintf("k8s-aws-v1.%s", base64.RawURLEncoding.EncodeToString([]byte(presignedReq.URL)))

	// Build and return the REST config.
	return &auth.RESTConfig{
		Host:        host,
		BearerToken: token,
		CAData:      caData,
		ExpiresAt:   expiresAt,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
