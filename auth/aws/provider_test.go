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
	"net/url"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
)

func TestProvider_NewDefaultToken_Options(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")

	impl := &mockImplementation{
		t:              t,
		argRegion:      "us-east-1",
		argProxyURL:    &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argSTSEndpoint: "https://sts.amazonaws.com",
	}

	for _, tt := range []struct {
		name        string
		stsEndpoint string
		err         string
	}{
		{
			name:        "valid",
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "invalid sts endpoint",
			stsEndpoint: "https://something.amazonaws.com",
			err:         `invalid STS endpoint: 'https://something.amazonaws.com'. must match ^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewDefaultToken(context.Background(), opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).NotTo(BeNil())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(token).To(BeNil())
			}
		})
	}
}

func TestProvider_NewTokenForServiceAccount_Options(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")

	impl := &mockImplementation{
		t:                  t,
		argRegion:          "us-east-1",
		argRoleARN:         "arn:aws:iam::1234567890:role/some-role",
		argRoleSessionName: "test-sa.test-ns.us-east-1.fluxcd.io",
		argOIDCToken:       "oidc-token",
		argProxyURL:        &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argSTSEndpoint:     "https://sts.amazonaws.com",
	}

	oidcToken := "oidc-token"
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::1234567890:role/some-role",
			},
		},
	}

	for _, tt := range []struct {
		name        string
		stsEndpoint string
		err         string
	}{
		{
			name:        "valid",
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "invalid sts endpoint",
			stsEndpoint: "https://something.amazonaws.com",
			err:         `invalid STS endpoint: 'https://something.amazonaws.com'. must match ^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, serviceAccount, opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).NotTo(BeNil())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(token).To(BeNil())
			}
		})
	}
}

func TestProvider_NewArtifactRegistryToken_Options(t *testing.T) {
	g := NewWithT(t)

	impl := &mockImplementation{
		t:                t,
		argRegion:        "us-east-1",
		argProxyURL:      &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argCredsProvider: credentials.NewStaticCredentialsProvider("access-key-id", "secret-access-key", "session-token"),
	}

	artifactRepository := "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo"
	accessToken := &aws.Token{
		Credentials: types.Credentials{
			AccessKeyId:     awssdk.String("access-key-id"),
			SecretAccessKey: awssdk.String("secret-access-key"),
			SessionToken:    awssdk.String("session-token"),
		},
	}
	opts := []auth.Option{
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
	}

	provider := aws.Provider{Implementation: impl}
	token, err := provider.NewArtifactRegistryToken(context.Background(), artifactRepository, accessToken, opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeNil())
}
