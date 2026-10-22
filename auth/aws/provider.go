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

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
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
func (p Provider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Credential, error) {
	var o auth.Options
	o.Apply(opts...)

	confOpts := []func(*config.LoadOptions) error{
		config.WithHTTPClient(o.GetHTTPClient()),
	}

	stsRegion := o.AWSRegion
	if stsRegion == "" {
		// EKS sets this environment variable automatically if the controller pod is
		// properly configured with IRSA or EKS Pod Identity, so we can reuse it.
		stsRegion = os.Getenv("AWS_REGION")
		if stsRegion == "" {
			return nil, errors.New("AWS_REGION environment variable is not set in the Flux controller. " +
				"if you have properly configured IAM Roles for Service Accounts (IRSA) or EKS Pod Identity, " +
				"please delete/replace the controller pod so the EKS admission controllers can inject this " +
				"environment variable, or set it manually if the cluster is not EKS")
		}
	}
	confOpts = append(confOpts, config.WithRegion(stsRegion))

	if e := o.AWSSTSEndpoint; e != "" {
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
func (Provider) GetAudiences(_ context.Context, opts ...auth.Option) ([]string, error) {
	var o auth.Options
	o.Apply(opts...)
	if len(o.CredentialAudiences) > 0 {
		return o.CredentialAudiences, nil
	}
	return []string{cryptov1.DefaultAudienceAWS}, nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(opts ...auth.Option) (string, error) {
	var o auth.Options
	o.Apply(opts...)
	return o.AWSRoleARN, nil
}

// NewAccessCredentialForIdentity implements auth.Provider.
func (p Provider) NewAccessCredentialForIdentity(ctx context.Context, identity auth.Credential,
	opts ...auth.Option) (auth.Credential, error) {

	var o auth.Options
	o.Apply(opts...)

	stsRegion := o.AWSRegion
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

	roleARN := o.AWSRoleARN

	jwtCredential, ok := identity.(*auth.JWTCredential)
	if !ok {
		return nil, fmt.Errorf("expected *auth.JWTCredential, got %T", identity)
	}

	sa := o.ServiceAccountRef()
	roleSessionName := cryptov1.GetAWSRoleSessionName(sa.Name, sa.Namespace, stsRegion)

	stsOpts := sts.Options{
		Region:     stsRegion,
		HTTPClient: o.GetHTTPClient(),
	}

	if e := o.AWSSTSEndpoint; e != "" {
		stsOpts.BaseEndpoint = &e
	}

	req := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleARN,
		RoleSessionName:  &roleSessionName,
		WebIdentityToken: &jwtCredential.Token,
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

// GetAccessCredentialOptionsForArtifactRepository implements auth.Provider.
func (p Provider) GetAccessCredentialOptionsForArtifactRepository(artifactRepository string,
	_ ...auth.Option) ([]auth.Option, error) {
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
	ecrRegion := cryptov1.GetAWSRegionFromArtifactRegistryInput(registryInput)
	return []auth.Option{auth.WithAWSRegion(ecrRegion)}, nil
}

// ParseArtifactRepository implements auth.Provider.
// ParseArtifactRepository returns the ECR region, unless the registry is one of
// the public ECR hostnames, in which case it returns
// cryptov1.RegistryAWSPublicECR.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	return cryptov1.ParseArtifactRepository(ProviderName, artifactRepository)
}

// NewArtifactRegistryCredentials implements auth.Provider.
func (p Provider) NewArtifactRegistryCredentials(ctx context.Context, registryInput string,
	accessCredential auth.Credential, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	var o auth.Options
	o.Apply(opts...)

	authTokenFunc := p.impl().GetAuthorizationToken
	if registryInput == cryptov1.RegistryAWSPublicECR {
		authTokenFunc = p.impl().GetPublicAuthorizationToken
	}

	conf := aws.Config{
		Region:      cryptov1.GetAWSRegionFromArtifactRegistryInput(registryInput),
		Credentials: accessCredential.(*Credentials).provider(),
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

// GetAccessCredentialOptionsForCluster implements auth.Provider.
func (Provider) GetAccessCredentialOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {
	var o auth.Options
	o.Apply(opts...)
	if o.AWSRegion == "" {
		return nil, errors.New("AWS region is required")
	}
	return [][]auth.Option{{auth.WithAWSRegion(o.AWSRegion)}}, nil
}

// NewRESTConfig implements auth.Provider.
//
// Reference:
// https://docs.aws.amazon.com/eks/latest/best-practices/identity-and-access-management.html#_controlling_access_to_eks_clusters
func (p Provider) NewRESTConfig(ctx context.Context, accessCredentials []auth.Credential,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	// The expiration for an EKS restconfig is always 15 minutes, see the reference above.
	// Let's record time.Now() on the beginning of the procedure to be on the safe side.
	expiresAt := time.Now().Add(15 * time.Minute)

	creds := accessCredentials[0].(*Credentials).provider()

	var o auth.Options
	o.Apply(opts...)
	hc := o.GetHTTPClient()

	region := o.AWSRegion
	clusterName := o.AWSClusterName
	if region == "" || clusterName == "" {
		return nil, errors.New("AWS region and cluster name are required")
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
			return nil, fmt.Errorf("failed to describe EKS cluster '%s': %w", clusterName, err)
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
	if e := o.AWSSTSEndpoint; e != "" {
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

// GetAccessCredentialOptionsForGitRepository implements auth.GitCredentialsProvider.
// AWS requires a region for obtaining access credentials. To avoid requiring
// callers to pass a region in addition to the CodeCommit URL, we extract the
// region from the URL and inject it as AWSRegion so that object-level workload
// identity (which requires an explicit region) works without extra config.
func (Provider) GetAccessCredentialOptionsForGitRepository(gitURL *url.URL) ([]auth.Option, error) {
	if gitURL == nil {
		return nil, fmt.Errorf("Git URL must be specified for AWS CodeCommit authentication")
	}
	region, err := cryptov1.GetAWSRegionFromCodeCommitURL(gitURL.String())
	if err != nil {
		return nil, err
	}
	return []auth.Option{auth.WithAWSRegion(region)}, nil
}

// ParseGitRepository implements auth.GitCredentialsProvider.
// It validates the URL is a CodeCommit URL and returns the URL string so that
// it is included in the cache key: CodeCommit credentials are a SigV4 signature
// over the request URL, so distinct URLs must map to distinct cache entries.
func (Provider) ParseGitRepository(gitURL *url.URL) (string, error) {
	if gitURL == nil {
		return "", fmt.Errorf("Git URL must be specified for AWS CodeCommit authentication")
	}
	return cryptov1.ParseGitRepository(ProviderName, gitURL.String())
}

// NewGitCredentials implements auth.GitCredentialsProvider.
func (Provider) NewGitCredentials(_ context.Context, gitInput string,
	accessCredential auth.Credential, _ ...auth.Option) (*auth.GitCredentials, error) {

	gitURL, err := url.Parse(gitInput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CodeCommit URL: %w", err)
	}
	region, err := cryptov1.GetAWSRegionFromCodeCommitURL(gitInput)
	if err != nil {
		return nil, err
	}

	creds, ok := accessCredential.(*Credentials)
	if !ok {
		return nil, fmt.Errorf("failed to cast credential to AWS credential: %T", accessCredential)
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
