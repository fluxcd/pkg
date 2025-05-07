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
	"net/http"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	. "github.com/onsi/gomega"
)

type mockImplementation struct {
	t *testing.T

	argRoleARN         string
	argRoleSessionName string
	argOIDCToken       string
	argRegion          string
	argSTSEndpoint     string
	argProxyURL        *url.URL
	argCredsProvider   aws.CredentialsProvider

	returnCreds    aws.Credentials
	returnUsername string
	returnPassword string
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

func (m *mockImplementation) GetAuthorizationToken(ctx context.Context, cfg aws.Config) (*ecr.GetAuthorizationTokenOutput, error) {
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
	return &ecr.GetAuthorizationTokenOutput{
		AuthorizationData: []ecrtypes.AuthorizationData{{
			AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte(m.returnUsername + ":" + m.returnPassword))),
		}},
	}, nil
}

func (m *mockCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return m.Credentials, nil
}
