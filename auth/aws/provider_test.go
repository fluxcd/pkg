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
	"encoding/json"
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if !tt.skipSTSRegion {
				t.Setenv("AWS_REGION", "us-east-1")
			}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewControllerToken(context.Background(), opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&aws.Credentials{Credentials: types.Credentials{
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

func TestProvider_NewTokenForOIDCToken(t *testing.T) {
	for _, tt := range []struct {
		name          string
		roleARN       string
		stsEndpoint   string
		skipSTSRegion bool
		err           string
	}{
		{
			name:        "valid",
			roleARN:     "arn:aws:iam::1234567890:role/some-role",
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "us gov is valid",
			roleARN:     "arn:aws-us-gov:iam::1234567890:role/some-role",
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "invalid sts endpoint",
			roleARN:     "arn:aws:iam::1234567890:role/some-role",
			stsEndpoint: "https://something.amazonaws.com",
			err:         `invalid STS endpoint: 'https://something.amazonaws.com'. must match ^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`,
		},
		{
			name:          "missing region",
			roleARN:       "arn:aws:iam::1234567890:role/some-role",
			stsEndpoint:   "https://sts.amazonaws.com",
			skipSTSRegion: true,
			err: "an AWS region is required for authenticating with a service account. " +
				"please configure one in the object spec",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:                  t,
				argRegion:          "us-east-1",
				argRoleARN:         tt.roleARN,
				argRoleSessionName: "aws.auth.fluxcd.io",
				argOIDCToken:       "oidc-token",
				argProxyURL:        &url.URL{Scheme: "http", Host: "proxy.example.com"},
				argSTSEndpoint:     "https://sts.amazonaws.com",
				returnCreds:        awssdk.Credentials{AccessKeyID: "access-key-id"},
			}

			oidcToken := "oidc-token"
			identity := &aws.Identity{RoleARN: tt.roleARN}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
			}

			if !tt.skipSTSRegion {
				opts = append(opts, auth.WithSTSRegion("us-east-1"))
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewTokenForOIDCToken(context.Background(), oidcToken, "", identity, opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&aws.Credentials{Credentials: types.Credentials{
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

func TestProvider_GetAudiences(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		g := NewWithT(t)
		oidcAud, exchangeAud, err := aws.Provider{}.GetAudiences(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(oidcAud).To(Equal("sts.amazonaws.com"))
		g.Expect(exchangeAud).To(Equal(""))
	})

	t.Run("explicit audiences", func(t *testing.T) {
		g := NewWithT(t)
		oidcAud, exchangeAud, err := aws.Provider{}.GetAudiences(context.Background(),
			auth.WithAudiences("custom-audience"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(oidcAud).To(Equal("custom-audience"))
		g.Expect(exchangeAud).To(Equal(""))
	})
}

func TestProvider_GetIdentity(t *testing.T) {
	for _, tt := range []struct {
		name        string
		annotations map[string]string
		expected    *aws.Identity
		err         string
	}{
		{
			name: "valid role ARN",
			annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::1234567890:role/some-role",
			},
			expected: &aws.Identity{RoleARN: "arn:aws:iam::1234567890:role/some-role"},
		},
		{
			name: "invalid role ARN",
			annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "foobar",
			},
			err: "invalid eks.amazonaws.com/role-arn annotation",
		},
		{
			name:        "missing annotation",
			annotations: map[string]string{},
			err:         "invalid eks.amazonaws.com/role-arn annotation",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			identity, err := aws.Provider{}.GetIdentity(
				auth.WithServiceAccount(corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: tt.annotations,
					},
				}))

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(identity).To(Equal(tt.expected))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(identity).To(BeNil())
			}
		})
	}
}

func TestProvider_GetIdentity_WithExplicitIdentity(t *testing.T) {
	t.Run("valid explicit identity", func(t *testing.T) {
		g := NewWithT(t)
		identity, err := aws.Provider{}.GetIdentity(
			auth.WithIdentityForOIDCImpersonation(&aws.Identity{RoleARN: "arn:aws:iam::1234567890:role/some-role"}))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(identity).To(Equal(&aws.Identity{RoleARN: "arn:aws:iam::1234567890:role/some-role"}))
	})

	t.Run("invalid explicit identity", func(t *testing.T) {
		g := NewWithT(t)
		identity, err := aws.Provider{}.GetIdentity(
			auth.WithIdentityForOIDCImpersonation(&aws.Identity{RoleARN: "invalid"}))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid IAM role ARN"))
		g.Expect(identity).To(BeNil())
	})

	t.Run("wrong identity type", func(t *testing.T) {
		g := NewWithT(t)
		type wrongIdentity struct{ auth.Identity }
		identity, err := aws.Provider{}.GetIdentity(
			auth.WithIdentityForOIDCImpersonation(&wrongIdentity{}))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid identity type"))
		g.Expect(identity).To(BeNil())
	})

	t.Run("no SA and no identity", func(t *testing.T) {
		g := NewWithT(t)
		identity, err := aws.Provider{}.GetIdentity()
		g.Expect(err).To(MatchError(auth.ErrNoIdentityForOIDCImpersonation))
		g.Expect(identity).To(BeNil())
	})
}

func TestIdentity_Validate(t *testing.T) {
	g := NewWithT(t)

	g.Expect((&aws.Identity{RoleARN: "arn:aws:iam::1234567890:role/some-role"}).Validate()).To(Succeed())
	g.Expect((&aws.Identity{RoleARN: "invalid"}).Validate()).To(HaveOccurred())
	g.Expect((&aws.Identity{RoleARN: ""}).Validate()).To(HaveOccurred())
}

func TestProvider_GetImpersonationAnnotationKey(t *testing.T) {
	g := NewWithT(t)
	g.Expect(aws.Provider{}.GetImpersonationAnnotationKey()).To(Equal("assume-role"))
}

func TestProvider_NewIdentity(t *testing.T) {
	g := NewWithT(t)
	identity := aws.Provider{}.NewIdentity()
	g.Expect(identity).NotTo(BeNil())
	g.Expect(identity.String()).To(Equal(""))
}

func TestIdentity_UnmarshalJSON(t *testing.T) {
	for _, tt := range []struct {
		name     string
		json     string
		expected string
		err      string
	}{
		{
			name:     "valid role ARN",
			json:     `{"roleARN":"arn:aws:iam::123456789012:role/some-role"}`,
			expected: "arn:aws:iam::123456789012:role/some-role",
		},
		{
			name:     "valid us-gov role ARN",
			json:     `{"roleARN":"arn:aws-us-gov:iam::123456789012:role/some-role"}`,
			expected: "arn:aws-us-gov:iam::123456789012:role/some-role",
		},
		{
			name: "invalid role ARN",
			json: `{"roleARN":"foobar"}`,
			err:  "invalid IAM role ARN: 'foobar'",
		},
		{
			name: "empty role ARN",
			json: `{"roleARN":""}`,
			err:  "invalid IAM role ARN: ''",
		},
		{
			name: "invalid JSON",
			json: `{invalid`,
			err:  "invalid character",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			identity := aws.Provider{}.NewIdentity()
			err := json.Unmarshal([]byte(tt.json), identity)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(identity).NotTo(BeNil())
				g.Expect(identity.String()).To(Equal(tt.expected))
			}
		})
	}
}

func TestProvider_NewTokenForNativeToken(t *testing.T) {
	for _, tt := range []struct {
		name        string
		roleARN     string
		stsEndpoint string
		skipRegion  bool
		err         string
	}{
		{
			name:        "valid",
			roleARN:     "arn:aws:iam::123456789012:role/target-role",
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "us gov role ARN",
			roleARN:     "arn:aws-us-gov:iam::123456789012:role/target-role",
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "missing region",
			roleARN:     "arn:aws:iam::123456789012:role/target-role",
			stsEndpoint: "https://sts.amazonaws.com",
			skipRegion:  true,
			err: "an AWS region is required for authenticating with a service account. " +
				"please configure one in the object spec",
		},
		{
			name:        "invalid STS endpoint",
			roleARN:     "arn:aws:iam::123456789012:role/target-role",
			stsEndpoint: "https://something.amazonaws.com",
			err:         `invalid STS endpoint: 'https://something.amazonaws.com'`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:                          t,
				argRegion:                  "us-east-1",
				argAssumeRoleARN:           tt.roleARN,
				argAssumeRoleSessionName:   "aws.auth.fluxcd.io",
				argAssumeRoleCredsProvider: credentials.NewStaticCredentialsProvider("initial-key-id", "initial-secret", "initial-session"),
				argProxyURL:                &url.URL{Scheme: "http", Host: "proxy.example.com"},
				argSTSEndpoint:             tt.stsEndpoint,
				returnAssumeRoleCreds:      awssdk.Credentials{AccessKeyID: "assumed-key-id"},
			}

			// Create the identity via NewIdentity + json.Unmarshal.
			identity := aws.Provider{}.NewIdentity()
			err := json.Unmarshal([]byte(`{"roleARN":"`+tt.roleARN+`"}`), identity)
			g.Expect(err).NotTo(HaveOccurred())

			// Create a mock initial token.
			initialToken := &aws.Credentials{Credentials: types.Credentials{
				AccessKeyId:     awssdk.String("initial-key-id"),
				SecretAccessKey: awssdk.String("initial-secret"),
				SessionToken:    awssdk.String("initial-session"),
			}}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
			}
			if !tt.skipRegion {
				opts = append(opts, auth.WithSTSRegion("us-east-1"))
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewTokenForNativeToken(context.Background(), initialToken, identity, opts...)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(token).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(&aws.Credentials{Credentials: types.Credentials{
					AccessKeyId:     awssdk.String("assumed-key-id"),
					SecretAccessKey: awssdk.String(""),
					SessionToken:    awssdk.String(""),
					Expiration:      awssdk.Time(time.Time{}),
				}}))
			}
		})
	}
}

func TestProvider_NewArtifactRegistryCredentials(t *testing.T) {
	for _, tt := range []struct {
		name               string
		artifactRepository string
		expectedPublicECR  bool
		expectedRegion     string
	}{
		{
			name:               "non public ECR, us-east-1",
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo",
			expectedRegion:     "us-east-1",
			expectedPublicECR:  false,
		},
		{
			name:               "non public ECR, us-west-2",
			artifactRepository: "012345678901.dkr.ecr.us-west-2.amazonaws.com/foo",
			expectedRegion:     "us-west-2",
			expectedPublicECR:  false,
		},
		{
			name:               "public ECR",
			artifactRepository: "public.ecr.aws",
			expectedRegion:     "us-east-1", // Public ECR is always us-east-1
			expectedPublicECR:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:                t,
				publicECR:        tt.expectedPublicECR,
				argRegion:        tt.expectedRegion,
				argProxyURL:      &url.URL{Scheme: "http", Host: "proxy.example.com"},
				argCredsProvider: credentials.NewStaticCredentialsProvider("access-key-id", "secret-access-key", "session-token"),
				returnCreds: awssdk.Credentials{
					AccessKeyID:     "access-key-id",
					SecretAccessKey: "secret-access-key",
					SessionToken:    "session-token",
				},
				returnUsername: "username",
				returnPassword: "password",
			}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
			}

			provider := aws.Provider{Implementation: impl}
			creds, err := auth.GetArtifactRegistryCredentials(
				context.Background(), provider, tt.artifactRepository, opts...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(creds).To(Equal(&auth.ArtifactRegistryCredentials{
				Authenticator: &authn.Basic{
					Username: "username",
					Password: "password",
				},
			}))
		})
	}
}

func TestProvider_GetAccessTokenOptionsForArtifactRepository(t *testing.T) {
	g := NewWithT(t)

	opts, err := aws.Provider{}.GetAccessTokenOptionsForArtifactRepository(
		"012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1")
	g.Expect(err).NotTo(HaveOccurred())

	var o auth.Options
	o.Apply(opts...)

	g.Expect(o.STSRegion).To(Equal("us-east-1"))
}

func TestProvider_ParseArtifactRepository(t *testing.T) {
	tests := []struct {
		artifactRepository string
		expectedRegion     string
		expectValid        bool
	}{
		{
			artifactRepository: "012345678901.dkr.ecr.eusc-de-east-1.amazonaws.eu/foo:v1",
			expectedRegion:     "eusc-de-east-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.eusc-de-east-1.amazonaws.eu/foo",
			expectedRegion:     "eusc-de-east-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.eusc-de-east-1.amazonaws.eu",
			expectedRegion:     "eusc-de-east-1",
			expectValid:        true,
		},
		{
			artifactRepository: "012345678901.dkr.ecr.eusc-de-east-1.amazonaws.eu/v2/part/part",
			expectedRegion:     "eusc-de-east-1",
			expectValid:        true,
		},
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
		{
			artifactRepository: "public.ecr.aws/foo/bar",
			expectedRegion:     "public.ecr.aws",
			expectValid:        true,
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

func TestProvider_NewRESTConfig(t *testing.T) {
	for _, tt := range []struct {
		name           string
		cluster        string
		clusterAddress string
		caData         string
		stsEndpoint    string
		err            string
	}{
		{
			name:    "valid EKS cluster",
			cluster: "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
		},
		{
			name:    "us gov EKS cluster is valid",
			cluster: "arn:aws-us-gov:eks:us-east-1:123456789012:cluster/test-cluster",
		},
		{
			name:           "valid EKS cluster with address match",
			cluster:        "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
			clusterAddress: "https://EXAMPLE1234567890123456789012345678.gr7.us-east-1.eks.amazonaws.com:443",
		},
		{
			name:    "valid EKS cluster with CA",
			cluster: "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
			caData:  "-----BEGIN CERTIFICATE-----",
		},
		{
			name:           "CA and address only. EKS requires cluster to extract region",
			clusterAddress: "https://EXAMPLE1234567890123456789012345678.gr7.us-east-1.eks.amazonaws.com:443",
			caData:         "-----BEGIN CERTIFICATE-----",
			err:            `invalid EKS cluster ARN: ''. must match ^arn:aws[\w-]*:eks:([^:]{1,100}):[0-9]{1,30}:cluster/(.{1,200})$`,
		},
		{
			name:           "valid EKS cluster with address override",
			cluster:        "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
			clusterAddress: "https://different-endpoint.eks.amazonaws.com:443",
		},
		{
			name:        "valid EKS cluster with custom STS endpoint",
			cluster:     "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
			stsEndpoint: "https://sts.amazonaws.com",
		},
		{
			name:        "invalid STS endpoint",
			cluster:     "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
			stsEndpoint: "https://invalid.amazonaws.com",
			err:         `invalid STS endpoint: 'https://invalid.amazonaws.com'. must match ^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`,
		},
		{
			name:    "invalid cluster ARN",
			cluster: "invalid-cluster-arn",
			err:     `invalid EKS cluster ARN: 'invalid-cluster-arn'. must match ^arn:aws[\w-]*:eks:([^:]{1,100}):[0-9]{1,30}:cluster/(.{1,200})$`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:                  t,
				expectEKSAPICall:   tt.clusterAddress == "" || tt.caData == "",
				argRegion:          "us-east-1",
				argClusterName:     "test-cluster",
				argProxyURL:        &url.URL{Scheme: "http", Host: "proxy.example.com"},
				argSTSEndpoint:     tt.stsEndpoint,
				argCredsProvider:   credentials.NewStaticCredentialsProvider("access-key-id", "secret-access-key", "session-token"),
				returnCreds:        awssdk.Credentials{AccessKeyID: "access-key-id", SecretAccessKey: "secret-access-key", SessionToken: "session-token"},
				returnEndpoint:     "https://EXAMPLE1234567890123456789012345678.gr7.us-east-1.eks.amazonaws.com",
				returnCAData:       "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t", // base64 encoded "-----BEGIN CERTIFICATE-----"
				returnPresignedURL: "https://sts.us-east-1.amazonaws.com/?Action=GetCallerIdentity&Version=2011-06-15&X-Amz-Algorithm=AWS4-HMAC-SHA256",
			}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
			}

			if tt.cluster != "" {
				opts = append(opts, auth.WithClusterResource(tt.cluster))
			}

			if tt.clusterAddress != "" {
				opts = append(opts, auth.WithClusterAddress(tt.clusterAddress))
			}

			if tt.caData != "" {
				opts = append(opts, auth.WithCAData(tt.caData))
			}

			if tt.stsEndpoint != "" {
				opts = append(opts, auth.WithSTSEndpoint(tt.stsEndpoint))
			}

			provider := aws.Provider{Implementation: impl}
			restConfig, err := auth.GetRESTConfig(context.Background(), provider, opts...)

			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(restConfig).NotTo(BeNil())
				expectedHost := "https://EXAMPLE1234567890123456789012345678.gr7.us-east-1.eks.amazonaws.com"
				if tt.clusterAddress != "" {
					expectedHost = tt.clusterAddress
				}
				g.Expect(restConfig.Host).To(Equal(expectedHost))
				g.Expect(restConfig.BearerToken).To(Equal("k8s-aws-v1.aHR0cHM6Ly9zdHMudXMtZWFzdC0xLmFtYXpvbmF3cy5jb20vP0FjdGlvbj1HZXRDYWxsZXJJZGVudGl0eSZWZXJzaW9uPTIwMTEtMDYtMTUmWC1BbXotQWxnb3JpdGhtPUFXUzQtSE1BQy1TSEEyNTY"))
				g.Expect(restConfig.CAData).To(Equal([]byte("-----BEGIN CERTIFICATE-----")))
				g.Expect(restConfig.ExpiresAt).To(BeTemporally(">", time.Now().Add(14*time.Minute)))
				g.Expect(restConfig.ExpiresAt).To(BeTemporally("<", time.Now().Add(16*time.Minute)))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(restConfig).To(BeNil())
			}
		})
	}
}

func TestProvider_GetAccessTokenOptionsForCluster(t *testing.T) {
	g := NewWithT(t)

	opts, err := aws.Provider{}.GetAccessTokenOptionsForCluster(
		auth.WithClusterResource("arn:aws:eks:us-west-2:123456789012:cluster/my-cluster"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts).To(HaveLen(1))

	var o auth.Options
	o.Apply(opts[0]...)

	g.Expect(o.STSRegion).To(Equal("us-west-2"))
}
