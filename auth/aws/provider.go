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
	"net/url"
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
	"github.com/aws/smithy-go/aws-http-auth/credentials"
	"github.com/aws/smithy-go/aws-http-auth/sigv4"
	v4 "github.com/aws/smithy-go/aws-http-auth/v4"
	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the AWS authentication provider.
const (
	ProviderName = "aws"

	// codeCommitCanonicalTimestampFormat is the SigV4 canonical timestamp
	// format (ISO 8601 basic, no separators, no fractional seconds) embedded
	// in the CodeCommit Git password and used as the canonical request time
	// during signing. The trailing 'Z' is appended separately by the caller.
	codeCommitCanonicalTimestampFormat = "20060102T150405"

	// codeCommitSignatureValidity is the server-side replay window for
	// SigV4-signed CodeCommit Git requests. AWS documents that the password
	// generated for HTTPS access to a CodeCommit repository stops working
	// after about 15 minutes:
	// https://docs.aws.amazon.com/codecommit/latest/userguide/troubleshooting-ch.html
	codeCommitSignatureValidity = 15 * time.Minute
)

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
		// In this case we can't rely on IRSA or EKS Pod Identity for the controller
		// pod because this is object-level configuration, so we show a different
		// error message.
		// In this error message we assume an API that has a region field, e.g. the
		// Bucket API. APIs that can extract the region from the ARN (e.g. KMS) will
		// never reach this code path.
		return nil, errors.New("an AWS region is required for authenticating with a service account. " +
			"please configure one in the object spec")
	}

	roleARN, err := getRoleARN(serviceAccount)
	if err != nil {
		return nil, err
	}

	roleSessionName := getRoleSessionName(serviceAccount, stsRegion)

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

// GetAccessTokenOptionsForArtifactRepository implements auth.Provider.
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
// It covers the public AWS partition (amazonaws.com), the China partitions (amazonaws.com.cn), the European Sovereign
// Cloud (amazonaws.eu) and the non-public partitions, as well as the dual-stack hostnames used for IPv6 access
// (on.aws and on.amazonwebservices.com.cn).
// The pattern is anchored so that it matches the registry host in full rather than any substring of it, which is what
// makes a match proof that the host belongs to ECR.
const registryPattern = `^([0-9]{12})\.dkr[.-]ecr(?:-fips)?\.([a-zA-Z0-9][a-zA-Z0-9_-]{0,99})\.(amazonaws\.(?:com(?:\.cn)?|eu)|on\.(?:aws|amazonwebservices\.com\.cn)|sc2s\.sgov\.gov|c2s\.ic\.gov|cloud\.adc-e\.uk|csp\.hci\.ic\.gov)$`

// The public ECR registry is reachable over an IPv4-only hostname and a dual-stack one.
const (
	publicECR          = "public.ecr.aws"
	publicECRDualStack = "ecr-public.aws.com"
)

var registryRegex = regexp.MustCompile(registryPattern)

// ParseArtifactRepository implements auth.Provider.
// ParseArtifactRepository returns the ECR region, unless the registry is one of
// the public ECR hostnames, in which case it returns public.ecr.aws.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	registry, err := auth.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}

	// Both public hostnames are normalized to publicECR, which is what selects
	// the public authorization token API and the us-east-1 region downstream.
	if registry == publicECR || registry == publicECRDualStack {
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

// NewArtifactRegistryCredentials implements auth.Provider.
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
		Authenticator: authn.FromConfig(authn.AuthConfig{
			Username: s[0],
			Password: s[1],
		}),
		ExpiresAt: expiresAt,
	}, nil
}

// GetAccessTokenOptionsForCluster implements auth.Provider.
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

// NewRESTConfig implements auth.Provider.
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

// ParseCodeCommitURL parses an AWS CodeCommit HTTPS Git URL and returns the
// scheme+host, region, and repository name.
// Supports standard, FIPS, and China partition URLs:
//
//	https://git-codecommit.{region}.amazonaws.com/v1/repos/{repository}
//	https://git-codecommit-fips.{region}.amazonaws.com/v1/repos/{repository}
//	https://git-codecommit.{region}.amazonaws.com.cn/v1/repos/{repository}
//
// See: https://docs.aws.amazon.com/codecommit/latest/userguide/regions.html#regions-git
func ParseCodeCommitURL(rawURL string) (host, region, repo string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid CodeCommit URL %q: %w", rawURL, err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", "", "", fmt.Errorf("AWS CodeCommit authentication requires an HTTPS Git URL")
	}
	hostname := u.Hostname()
	urlSplit := strings.Split(hostname, ".")
	if len(urlSplit) < 4 ||
		!(strings.HasPrefix(hostname, "git-codecommit.") || strings.HasPrefix(hostname, "git-codecommit-fips.")) ||
		!(strings.HasSuffix(hostname, ".amazonaws.com") || strings.HasSuffix(hostname, ".amazonaws.com.cn")) {
		return "", "", "", fmt.Errorf("invalid AWS CodeCommit Git URL: %s", u.Host)
	}
	region = urlSplit[1]

	pathParts := strings.Split(strings.TrimLeft(u.Path, "/"), "/")
	if len(pathParts) != 3 || pathParts[0] != "v1" || pathParts[1] != "repos" || pathParts[2] == "" {
		return "", "", "", fmt.Errorf("invalid CodeCommit URL %q: path must be /v1/repos/{repository}", rawURL)
	}
	repo = pathParts[2]
	host = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	return host, region, repo, nil
}

// getRegionFromCodeCommitURL extracts the AWS region from a CodeCommit HTTPS
// git URL (e.g. https://git-codecommit.us-east-1.amazonaws.com/...).
// Returns an error if the URL is nil, not HTTPS, or not a valid CodeCommit URL.
// https://docs.aws.amazon.com/codecommit/latest/userguide/regions.html#regions-git
func getRegionFromCodeCommitURL(gitURL *url.URL) (string, error) {
	if gitURL == nil {
		return "", fmt.Errorf("Git URL must be specified for AWS CodeCommit authentication")
	}
	_, region, _, err := ParseCodeCommitURL(gitURL.String())
	return region, err
}

// GetAccessTokenOptionsForGitRepository implements auth.GitCredentialsProvider.
// AWS requires a region for obtaining access credentials. To avoid requiring
// callers to pass a region in addition to the CodeCommit URL, we extract the
// region from the URL and inject it as STSRegion so that object-level workload
// identity (which requires an explicit region) works without extra config.
func (Provider) GetAccessTokenOptionsForGitRepository(gitURL *url.URL) ([]auth.Option, error) {
	region, err := getRegionFromCodeCommitURL(gitURL)
	if err != nil {
		return nil, err
	}
	return []auth.Option{auth.WithSTSRegion(region)}, nil
}

// ParseGitRepository implements auth.GitCredentialsProvider.
// It validates the URL is a CodeCommit URL and returns the URL string so that
// it is included in the cache key: CodeCommit credentials are a SigV4 signature
// over the request URL, so distinct URLs must map to distinct cache entries.
func (Provider) ParseGitRepository(gitURL *url.URL) (string, error) {
	if _, err := getRegionFromCodeCommitURL(gitURL); err != nil {
		return "", err
	}
	return gitURL.String(), nil
}

// NewGitCredentials implements auth.GitCredentialsProvider.
func (Provider) NewGitCredentials(_ context.Context, gitInput string,
	accessToken auth.Token, _ ...auth.Option) (*auth.GitCredentials, error) {

	gitURL, err := url.Parse(gitInput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CodeCommit URL: %w", err)
	}
	region, err := getRegionFromCodeCommitURL(gitURL)
	if err != nil {
		return nil, err
	}

	creds, ok := accessToken.(*Credentials)
	if !ok {
		return nil, fmt.Errorf("failed to cast token to AWS token: %T", accessToken)
	}

	req, err := http.NewRequest("GIT", gitURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build CodeCommit signing request: %w", err)
	}
	req.Host = gitURL.Host

	signingTime := time.Now().UTC()

	signer := sigv4.New(func(o *v4.SignerOptions) {
		o.HeaderRules = signerHeaderHostOnly{}
		o.DisableUnsignedPayloadSentinel = true
		o.CanonicalTimeFormat = codeCommitCanonicalTimestampFormat
	})
	signInput := &sigv4.SignRequestInput{
		Request: req,
		Service: "codecommit",
		Region:  region,
		Credentials: credentials.Credentials{
			AccessKeyID:     *creds.AccessKeyId,
			SecretAccessKey: *creds.SecretAccessKey,
			SessionToken:    *creds.SessionToken,
			Expires:         *creds.Expiration,
		},
		Time: signingTime,
	}

	if err := signer.SignRequest(signInput); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	authHeader := req.Header.Get("Authorization")
	_, after, _ := strings.Cut(authHeader, "Signature=")
	signature := after

	username := strings.Join([]string{*creds.AccessKeyId, *creds.SessionToken}, "%")
	password := signingTime.Format(codeCommitCanonicalTimestampFormat) + "Z" + signature

	// The signed password is invalid once either the server's replay window
	// elapses or the underlying credentials expire, whichever comes first.
	expiresAt := signingTime.Add(codeCommitSignatureValidity)
	if creds.Expiration.Before(expiresAt) {
		expiresAt = *creds.Expiration
	}

	return &auth.GitCredentials{
		Username:  username,
		Password:  password,
		ExpiresAt: expiresAt,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}

type signerHeaderHostOnly struct{}

func (signerHeaderHostOnly) IsSigned(h string) bool {
	return h == "host"
}
