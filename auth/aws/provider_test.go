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

func TestProvider_NewTokenForServiceAccount(t *testing.T) {
	for _, tt := range []struct {
		name               string
		roleARN            string
		stsEndpoint        string
		artifactRepository string
		skipSTSRegion      bool
		err                string
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
		{
			name:        "invalid role ARN",
			roleARN:     "foobar",
			stsEndpoint: "https://sts.amazonaws.com",
			err:         `invalid eks.amazonaws.com/role-arn annotation: 'foobar'. must match ^arn:aws[\w-]*:iam::[0-9]{1,30}:role/.{1,200}$`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			impl := &mockImplementation{
				t:                  t,
				argRegion:          "us-east-1",
				argRoleARN:         tt.roleARN,
				argRoleSessionName: "test-sa.test-ns.us-east-1.fluxcd.io",
				argOIDCToken:       "oidc-token",
				argProxyURL:        &url.URL{Scheme: "http", Host: "proxy.example.com"},
				argSTSEndpoint:     "https://sts.amazonaws.com",
				returnCreds:        awssdk.Credentials{AccessKeyID: "access-key-id"},
			}

			oidcToken := "oidc-token"
			serviceAccount := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "test-ns",
					Annotations: map[string]string{"eks.amazonaws.com/role-arn": tt.roleARN},
				},
			}

			opts := []auth.Option{
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.example.com"}),
				auth.WithSTSEndpoint(tt.stsEndpoint),
			}

			if !tt.skipSTSRegion {
				opts = append(opts, auth.WithSTSRegion("us-east-1"))
			}

			provider := aws.Provider{Implementation: impl}
			token, err := provider.NewTokenForServiceAccount(context.Background(), oidcToken, serviceAccount, opts...)

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
	g := NewWithT(t)
	aud, err := aws.Provider{}.GetAudiences(context.Background(), corev1.ServiceAccount{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aud).To(Equal([]string{"sts.amazonaws.com"}))
}

func TestProvider_GetIdentity(t *testing.T) {
	g := NewWithT(t)

	identity, err := aws.Provider{}.GetIdentity(corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::1234567890:role/some-role",
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(identity).To(Equal("arn:aws:iam::1234567890:role/some-role"))
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
				Authenticator: authn.FromConfig(authn.AuthConfig{
					Username: "username",
					Password: "password",
				}),
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
			name:           "cluster address mismatch",
			cluster:        "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
			clusterAddress: "https://different-endpoint.eks.amazonaws.com:443",
			err:            "EKS endpoint 'https://EXAMPLE1234567890123456789012345678.gr7.us-east-1.eks.amazonaws.com' does not match specified address: 'https://different-endpoint.eks.amazonaws.com:443'",
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
				g.Expect(restConfig.Host).To(Equal("https://EXAMPLE1234567890123456789012345678.gr7.us-east-1.eks.amazonaws.com"))
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
