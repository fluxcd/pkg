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

package aws_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signerv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	ecrpublictypes "github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	. "github.com/onsi/gomega"
)

type mockImplementation struct {
	t *testing.T

	publicECR bool

	expectEKSAPICall bool

	argRoleARN         string
	argRoleSessionName string
	argOIDCToken       string
	argRegion          string
	argSTSEndpoint     string
	argProxyURL        *url.URL
	argCredsProvider   aws.CredentialsProvider
	argClusterName     string

	returnCreds        aws.Credentials
	returnUsername     string
	returnPassword     string
	returnEndpoint     string
	returnCAData       string
	returnPresignedURL string

	// AssumeRole fields
	argAssumeRoleARN           string
	argAssumeRoleSessionName   string
	argAssumeRoleCredsProvider aws.CredentialsProvider
	returnAssumeRoleCreds      aws.Credentials
	returnAssumeRoleErr        error
}

type mockHTTPPresigner struct {
	t              *testing.T
	argClusterName string
	returnURL      string
}

type mockCredentialsProvider struct{ aws.Credentials }

func (m *mockImplementation) LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	var o config.LoadOptions
	for _, optFn := range optFns {
		optFn(&o)
	}
	g.Expect(o.Region).To(Equal(m.argRegion))
	g.Expect(o.BaseEndpoint).To(Equal(m.argSTSEndpoint))
	g.Expect(o.HTTPClient).NotTo(BeNil())
	g.Expect(o.HTTPClient.(*http.Client)).NotTo(BeNil())
	g.Expect(o.HTTPClient.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(o.HTTPClient.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(o.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := o.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	return aws.Config{Credentials: &mockCredentialsProvider{m.returnCreds}}, nil
}

func (m *mockImplementation) AssumeRoleWithWebIdentity(ctx context.Context, params *sts.AssumeRoleWithWebIdentityInput, options sts.Options) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(params).NotTo(BeNil())
	g.Expect(params.RoleArn).NotTo(BeNil())
	g.Expect(*params.RoleArn).To(Equal(m.argRoleARN))
	g.Expect(params.RoleSessionName).NotTo(BeNil())
	g.Expect(*params.RoleSessionName).To(Equal(m.argRoleSessionName))
	g.Expect(params.WebIdentityToken).NotTo(BeNil())
	g.Expect(*params.WebIdentityToken).To(Equal(m.argOIDCToken))
	g.Expect(options.Region).To(Equal(m.argRegion))
	g.Expect(options.BaseEndpoint).NotTo(BeNil())
	g.Expect(*options.BaseEndpoint).To(Equal(m.argSTSEndpoint))
	g.Expect(options.HTTPClient).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	return &sts.AssumeRoleWithWebIdentityOutput{
		Credentials: &ststypes.Credentials{
			AccessKeyId:     aws.String(m.returnCreds.AccessKeyID),
			SecretAccessKey: aws.String(m.returnCreds.SecretAccessKey),
			SessionToken:    aws.String(m.returnCreds.SessionToken),
			Expiration:      aws.Time(m.returnCreds.Expires),
		},
	}, nil
}

func (m *mockImplementation) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, options sts.Options) (*sts.AssumeRoleOutput, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(params).NotTo(BeNil())
	g.Expect(params.RoleArn).NotTo(BeNil())
	g.Expect(*params.RoleArn).To(Equal(m.argAssumeRoleARN))
	g.Expect(params.RoleSessionName).NotTo(BeNil())
	g.Expect(*params.RoleSessionName).To(Equal(m.argAssumeRoleSessionName))
	g.Expect(options.Region).To(Equal(m.argRegion))
	if m.argAssumeRoleCredsProvider != nil {
		g.Expect(options.Credentials).To(Equal(m.argAssumeRoleCredsProvider))
	}
	if m.argSTSEndpoint != "" {
		g.Expect(options.BaseEndpoint).NotTo(BeNil())
		g.Expect(*options.BaseEndpoint).To(Equal(m.argSTSEndpoint))
	}
	g.Expect(options.HTTPClient).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	if m.returnAssumeRoleErr != nil {
		return nil, m.returnAssumeRoleErr
	}
	return &sts.AssumeRoleOutput{
		Credentials: &ststypes.Credentials{
			AccessKeyId:     aws.String(m.returnAssumeRoleCreds.AccessKeyID),
			SecretAccessKey: aws.String(m.returnAssumeRoleCreds.SecretAccessKey),
			SessionToken:    aws.String(m.returnAssumeRoleCreds.SessionToken),
			Expiration:      aws.Time(m.returnAssumeRoleCreds.Expires),
		},
	}, nil
}

func (m *mockImplementation) GetAuthorizationToken(ctx context.Context, cfg aws.Config) (any, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(m.publicECR).To(BeFalse())
	m.checkGetAuthorizationToken(ctx, cfg)
	return &ecr.GetAuthorizationTokenOutput{
		AuthorizationData: []ecrtypes.AuthorizationData{{
			AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte(m.returnUsername + ":" + m.returnPassword))),
			ExpiresAt:          aws.Time(m.returnCreds.Expires),
		}},
	}, nil
}

func (m *mockImplementation) GetPublicAuthorizationToken(ctx context.Context, cfg aws.Config) (any, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(m.publicECR).To(BeTrue())
	m.checkGetAuthorizationToken(ctx, cfg)
	return &ecrpublic.GetAuthorizationTokenOutput{
		AuthorizationData: &ecrpublictypes.AuthorizationData{
			AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte(m.returnUsername + ":" + m.returnPassword))),
			ExpiresAt:          aws.Time(m.returnCreds.Expires),
		},
	}, nil
}

func (m *mockImplementation) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, options eks.Options) (*eks.DescribeClusterOutput, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(m.expectEKSAPICall).To(BeTrue())
	g.Expect(params).NotTo(BeNil())
	g.Expect(params.Name).NotTo(BeNil())
	g.Expect(*params.Name).To(Equal(m.argClusterName))
	g.Expect(options.Region).To(Equal(m.argRegion))
	g.Expect(options.Credentials).To(Equal(m.argCredsProvider))
	g.Expect(options.HTTPClient).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	return &eks.DescribeClusterOutput{
		Cluster: &ekstypes.Cluster{
			Name:     aws.String(m.argClusterName),
			Endpoint: aws.String(m.returnEndpoint),
			CertificateAuthority: &ekstypes.Certificate{
				Data: aws.String(m.returnCAData),
			},
		},
	}, nil
}

func (m *mockImplementation) PresignGetCallerIdentity(ctx context.Context, optFn func(*sts.PresignOptions), options sts.Options) (*signerv4.PresignedHTTPRequest, error) {
	m.t.Helper()

	g := NewWithT(m.t)

	// Check that optFn adds the presigner with the custom EKS headers to the options.
	g.Expect(optFn).NotTo(BeNil())
	mockPresigner := &mockHTTPPresigner{
		t:              m.t,
		argClusterName: m.argClusterName,
		returnURL:      m.returnPresignedURL,
	}
	var presignOpts sts.PresignOptions
	presignOpts.Presigner = mockPresigner
	optFn(&presignOpts)
	g.Expect(presignOpts.Presigner).NotTo(Equal(mockPresigner))
	req, _ := http.NewRequest("POST", "https://sts.amazonaws.com/", nil)
	signingTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	signerOptFn := func(opts *signerv4.SignerOptions) { opts.LogSigning = true }
	creds := aws.Credentials{
		AccessKeyID:     "access-key-id",
		SecretAccessKey: "secret-access-key",
		SessionToken:    "session-token",
	}
	presignedURL, presignedHeader, err := presignOpts.Presigner.PresignHTTP(
		ctx, creds, req, "payload-hash", "sts", "us-east-1", signingTime, signerOptFn)
	g.Expect(presignedURL).To(Equal(m.returnPresignedURL))
	g.Expect(presignedHeader).To(Equal(http.Header{"foo": []string{"bar"}}))
	g.Expect(err).To(MatchError("mock presign error"))

	// Check the sts options.
	g.Expect(options.Region).To(Equal(m.argRegion))
	g.Expect(options.Credentials).To(Equal(m.argCredsProvider))
	if m.argSTSEndpoint != "" {
		g.Expect(options.BaseEndpoint).NotTo(BeNil())
		g.Expect(*options.BaseEndpoint).To(Equal(m.argSTSEndpoint))
	} else {
		g.Expect(options.BaseEndpoint).To(BeNil())
	}
	g.Expect(options.HTTPClient).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := options.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))

	return &signerv4.PresignedHTTPRequest{
		URL: m.returnPresignedURL,
	}, nil
}

func (m *mockImplementation) checkGetAuthorizationToken(ctx context.Context, cfg aws.Config) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(cfg.Region).To(Equal(m.argRegion))
	g.Expect(cfg.Credentials).To(Equal(m.argCredsProvider))
	g.Expect(cfg.HTTPClient).NotTo(BeNil())
	g.Expect(cfg.HTTPClient.(*http.Client)).NotTo(BeNil())
	g.Expect(cfg.HTTPClient.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(cfg.HTTPClient.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(cfg.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := cfg.HTTPClient.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
}

func (m *mockHTTPPresigner) PresignHTTP(ctx context.Context, credentials aws.Credentials,
	r *http.Request, payloadHash string, service string, region string, signingTime time.Time,
	optFns ...func(*signerv4.SignerOptions)) (url string, signedHeader http.Header, err error) {

	m.t.Helper()

	g := NewWithT(m.t)

	// Check args.
	g.Expect(ctx).NotTo(BeNil())
	g.Expect(credentials.AccessKeyID).To(Equal("access-key-id"))
	g.Expect(credentials.SecretAccessKey).To(Equal("secret-access-key"))
	g.Expect(credentials.SessionToken).To(Equal("session-token"))
	g.Expect(r).NotTo(BeNil())
	g.Expect(r.Method).To(Equal("POST"))
	g.Expect(r.URL.String()).To(Equal("https://sts.amazonaws.com/"))
	g.Expect(r.Header.Get("x-k8s-aws-id")).To(Equal(m.argClusterName))
	g.Expect(r.Header.Get("X-Amz-Expires")).To(Equal("900"))
	g.Expect(payloadHash).To(Equal("payload-hash"))
	g.Expect(service).To(Equal("sts"))
	g.Expect(region).To(Equal("us-east-1"))
	g.Expect(signingTime).To(Equal(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)))
	g.Expect(optFns).To(HaveLen(1))
	optFn := optFns[0]
	g.Expect(optFn).NotTo(BeNil())
	var signerOpts signerv4.SignerOptions
	optFn(&signerOpts)
	g.Expect(signerOpts).To(Equal(signerv4.SignerOptions{LogSigning: true}))

	return m.returnURL, http.Header{"foo": []string{"bar"}}, errors.New("mock presign error")
}

func (m *mockCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return m.Credentials, nil
}
