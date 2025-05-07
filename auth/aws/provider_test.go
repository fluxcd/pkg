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
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
)

func TestProvider_NewControllerToken(t *testing.T) {
	impl := &mockImplementation{
		t:              t,
		argRegion:      "us-east-1",
		argProxyURL:    &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argSTSEndpoint: "https://sts.amazonaws.com",
		returnCreds:    awssdk.Credentials{AccessKeyID: "access-key-id"},
	}

	for _, tt := range []struct {
		name               string
		stsEndpoint        string
		artifactRepository string
		skipSTSRegion      bool
		err                string
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
		{
			name:          "missing region",
			stsEndpoint:   "https://sts.amazonaws.com",
			skipSTSRegion: true,
			err: "AWS_REGION environment variable is not set in the Flux controller. " +
				"if you have properly configured IAM Roles for Service Accounts (IRSA) or EKS Pod Identity, " +
				"please delete/replace the controller pod so the EKS admission controllers can inject this " +
				"environment variable, or set it manually if the cluster is not EKS",
		},
		{
			name:               "missing region but can extract from artifact repository",
			stsEndpoint:        "https://sts.amazonaws.com",
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1",
			skipSTSRegion:      true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if !tt.skipSTSRegion {
				t.Setenv("AWS_REGION", "us-east-1")
			}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
				auth.WithArtifactRepository(tt.artifactRepository),
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewControllerToken(context.Background(), opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&aws.Token{Credentials: types.Credentials{
					AccessKeyId:     awssdk.String("access-key-id"),
					SecretAccessKey: awssdk.String(""),
					SessionToken:    awssdk.String(""),
					Expiration:      awssdk.Time(time.Time{}),
				}}))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(token).To(BeNil())
			}
		})
	}
}

func TestProvider_NewTokenForServiceAccount(t *testing.T) {
	impl := &mockImplementation{
		t:                  t,
		argRegion:          "us-east-1",
		argRoleARN:         "arn:aws:iam::1234567890:role/some-role",
		argRoleSessionName: "test-sa.test-ns.us-east-1.fluxcd.io",
		argOIDCToken:       "oidc-token",
		argProxyURL:        &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argSTSEndpoint:     "https://sts.amazonaws.com",
		returnCreds:        awssdk.Credentials{AccessKeyID: "access-key-id"},
	}

	for _, tt := range []struct {
		name               string
		annotations        map[string]string
		stsEndpoint        string
		artifactRepository string
		skipSTSRegion      bool
		err                string
	}{
		{
			name:        "valid",
			annotations: map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1234567890:role/some-role"},
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "invalid sts endpoint",
			annotations: map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1234567890:role/some-role"},
			stsEndpoint: "https://something.amazonaws.com",
			err:         `invalid STS endpoint: 'https://something.amazonaws.com'. must match ^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`,
		},
		{
			name:          "missing region",
			annotations:   map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1234567890:role/some-role"},
			stsEndpoint:   "https://sts.amazonaws.com",
			skipSTSRegion: true,
			err: "an AWS region is required for authenticating with a service account. " +
				"please configure one in the object spec",
		},
		{
			name:               "missing region but can extract from artifact repository",
			annotations:        map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1234567890:role/some-role"},
			stsEndpoint:        "https://sts.amazonaws.com",
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1",
			skipSTSRegion:      true,
		},
		{
			name:        "invalid role ARN",
			annotations: map[string]string{"eks.amazonaws.com/role-arn": "foobar"},
			stsEndpoint: "https://sts.amazonaws.com",
			err:         "invalid eks.amazonaws.com/role-arn annotation: 'foobar'. must match ^arn:aws:iam::[0-9]{1,30}:role/.{1,200}$",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			oidcToken := "oidc-token"
			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "test-ns",
					Annotations: tt.annotations,
				},
			}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
				auth.WithArtifactRepository(tt.artifactRepository),
			}

			if !tt.skipSTSRegion {
				opts = append(opts, auth.WithSTSRegion("us-east-1"))
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, serviceAccount, opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&aws.Token{Credentials: types.Credentials{
					AccessKeyId:     awssdk.String("access-key-id"),
					SecretAccessKey: awssdk.String(""),
					SessionToken:    awssdk.String(""),
					Expiration:      awssdk.Time(time.Time{}),
				}}))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(token).To(BeNil())
			}
		})
	}
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	g := NewWithT(t)

	impl := &mockImplementation{
		t:                t,
		argRegion:        "us-east-1",
		argProxyURL:      &url.URL{Scheme: "http", Host: "proxy.example.com"},
		argCredsProvider: credentials.NewStaticCredentialsProvider("access-key-id", "secret-access-key", "session-token"),
		returnUsername:   "username",
		returnPassword:   "password",
	}

	ecrRegion := "us-east-1"
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
	creds, err := provider.NewArtifactRegistryCredentials(
		context.Background(), ecrRegion, accessToken, opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(creds).To(Equal(&auth.ArtifactRegistryCredentials{
		Authenticator: authn.FromConfig(authn.AuthConfig{
			Username: "username",
			Password: "password",
		}),
	}))
}

func TestProvider_ParseArtifactRepository(t *testing.T) {
	tests := []struct {
		artifactRepository string
		expectedRegion     string
		expectValid        bool
	}{
		{
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1",
			expectedRegion:     "us-east-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo",
			expectedRegion:     "us-east-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com",
			expectedRegion:     "us-east-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com/v2/part/part",
			expectedRegion:     "us-east-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.cn-north-1.amazonaws.com.cn/foo",
			expectedRegion:     "cn-north-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr-fips.us-gov-west-1.amazonaws.com",
			expectedRegion:     "us-gov-west-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.us-secret-region.sc2s.sgov.gov",
			expectedRegion:     "us-secret-region",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr-fips.us-ts-region.c2s.ic.gov",
			expectedRegion:     "us-ts-region",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.uk-region.cloud.adc-e.uk",
			expectedRegion:     "uk-region",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.us-ts-region.csp.hci.ic.gov",
			expectedRegion:     "us-ts-region",
			expectValid:        true,
		},
		{
			artifactRepository: "gcr.io/foo/bar:baz",
			expectValid:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.artifactRepository, func(t *testing.T) {
			g := NewWithT(t)

			region, err := aws.Provider{}.ParseArtifactRepository(tt.artifactRepository)
			g.Expect(err == nil).To(Equal(tt.expectValid))
			g.Expect(region).To(Equal(tt.expectedRegion))
		})
	}
}
