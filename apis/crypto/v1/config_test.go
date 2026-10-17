/*
Copyright 2026 The Flux authors

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

package v1

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func ann(kv ...string) map[string]string {
	m := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

func TestValidateConfig(t *testing.T) {
	const uuid = "11111111-1111-1111-1111-111111111111"

	optInKubernetes := ann(
		AnnotationCredentialProvider, CredentialProviderKubernetes,
		AnnotationCredentialType, CredentialTypeJWT,
	)

	tests := []struct {
		name    string
		opts    []Option
		wantErr string
	}{
		{
			name: "defaults",
			opts: []Option{WithServiceProvider(ProviderGCP)},
		},
		{
			name: "kubernetes jwt",
			opts: []Option{WithAnnotations(optInKubernetes)},
		},
		{
			name: "gcp rfc0010 annotation without opt-in",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithAnnotations(ann(AnnotationGCPWorkloadIdentityProvider, "projects/1/locations/global/workloadIdentityPools/p/providers/x")),
			},
		},
		{
			name: "mTLS only",
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org"))},
		},
		{
			name: "mTLS with opt-in",
			opts: []Option{WithAnnotations(ann(
				AnnotationMTLSSPIFFEServerID, "spiffe://example.org",
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
			))},
		},
		{
			name: "mTLS with GitHub Enterprise Server",
			opts: []Option{
				WithServiceProvider(ProviderGitHub),
				WithGitHubEnterpriseServer(true),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org")),
			},
		},
		{
			name: "expiration with kubernetes",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationCredentialExpiration, "5m",
			))},
		},
		{
			name: "audiences with jwt",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationCredentialAudiences, "audience",
			))},
		},
		{
			name: "transit engine pair",
			opts: []Option{
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationTransitEngineLoginPath, "auth/jwt/login",
					AnnotationTransitEngineRole, "role",
				)),
			},
		},
		{
			name: "generic OCI with opt-in and username",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderGeneric),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialUsername, "user",
				)),
			},
		},
		{
			name: "generic OCI static",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(true),
			},
		},
		{
			name: "dockerhub",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderDockerHub),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialUsername, "user",
					AnnotationDockerHubOIDCConnectionID, uuid,
				)),
			},
		},
		{
			name: "aws",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
		},
		{
			name: "azure",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationAzureTenantID, uuid,
					AnnotationAzureClientID, uuid,
				)),
			},
		},
		{
			name: "kubeconfig cluster",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithKubeConfigCluster("arn:aws:eks:us-east-1:123456789012:cluster/test-cluster"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
		},

		// Errors.
		{
			name:    "new annotation without opt-in",
			opts:    []Option{WithAnnotations(ann(AnnotationCredentialAudiences, "audience"))},
			wantErr: "requires annotations",
		},
		{
			name:    "provider without type",
			opts:    []Option{WithAnnotations(ann(AnnotationCredentialProvider, CredentialProviderKubernetes))},
			wantErr: "must be set together",
		},
		{
			name:    "type without provider",
			opts:    []Option{WithAnnotations(ann(AnnotationCredentialType, CredentialTypeJWT))},
			wantErr: "must be set together",
		},
		{
			name: "kubernetes x509",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeX509,
			))},
			wantErr: "not supported by the kubernetes credential provider",
		},
		{
			name: "azure x509",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeX509,
				)),
			},
			wantErr: "not supported by the azure service provider",
		},
		{
			name: "mTLS with borrowed cloud annotation without opt-in",
			opts: []Option{WithAnnotations(ann(
				AnnotationMTLSSPIFFEServerID, "spiffe://example.org",
				AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
			))},
			wantErr: "must be set together with any authentication annotation",
		},
		{
			name: "mTLS with certSecretRef",
			opts: []Option{
				WithCertSecretRef(true),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org")),
			},
			wantErr: ".spec.certSecretRef",
		},
		{
			name: "mTLS with secretRef certs",
			opts: []Option{
				WithSecretRefCerts(true),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org")),
			},
			wantErr: ".spec.secretRef",
		},
		{
			name: "mTLS with public provider",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org")),
			},
			wantErr: "not allowed with service provider aws",
		},
		{
			name: "mTLS with GitHub.com",
			opts: []Option{
				WithServiceProvider(ProviderGitHub),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org")),
			},
			wantErr: "not allowed with service provider github",
		},
		{
			name: "invalid kubeconfig cluster",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithKubeConfigCluster("nope"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
			wantErr: "invalid kubeconfig ConfigMap data key cluster",
		},
		{
			name: "kubeconfig legacy audiences",
			opts: []Option{
				WithKubeConfigAudiences("audience"),
				WithAnnotations(optInKubernetes),
			},
			wantErr: "legacy audiences ConfigMap key",
		},
		{
			name: "sops vault configmap flag",
			opts: []Option{
				WithSOPSVaultConfigMap(true),
				WithAnnotations(optInKubernetes),
			},
			wantErr: "--sops-vault-configmap",
		},
		{
			name: "legacy bucket sts provider",
			opts: []Option{
				WithSTSEndpoint(ProviderAWS, "https://sts.amazonaws.com"),
				WithAnnotations(optInKubernetes),
			},
			wantErr: "Bucket.spec.sts.provider",
		},
		{
			name: "generic OCI imagePullSecrets with annotations",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(true),
				WithAnnotations(optInKubernetes),
			},
			wantErr: ".imagePullSecrets",
		},
		{
			name: "generic OCI without opt-in",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderGeneric),
			},
			wantErr: "requires annotations",
		},
		{
			name: "username outside OCI",
			opts: []Option{
				WithServiceProvider(ProviderGeneric),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialUsername, "user",
				)),
			},
			wantErr: "only valid for the OCI APIs",
		},
		{
			name: "username with aws provider",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialUsername, "user",
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
			wantErr: "only valid for the generic and dockerhub",
		},
		{
			name: "audiences with x509",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderSPIFFE,
				AnnotationCredentialType, CredentialTypeX509,
				AnnotationCredentialAudiences, "audience",
			))},
			wantErr: "only supported by the jwt credential type",
		},
		{
			name: "audiences with dockerhub",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderDockerHub),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialAudiences, "audience",
				)),
			},
			wantErr: "not supported by the dockerhub",
		},
		{
			name: "expiration with spiffe",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderSPIFFE,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationCredentialExpiration, "5m",
			))},
			wantErr: "only supported by the kubernetes credential provider",
		},
		{
			name: "transit engine login path only",
			opts: []Option{
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationTransitEngineLoginPath, "auth/jwt/login",
				)),
			},
			wantErr: "must be set together",
		},
		{
			name: "dockerhub without username",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderDockerHub),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationDockerHubOIDCConnectionID, uuid,
				)),
			},
			wantErr: "required for the dockerhub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateConfig(tt.opts...)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got error %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfigResult(t *testing.T) {
	got, err := ValidateConfig(
		WithOCI(true),
		WithServiceProvider(ProviderAWS),
		WithKubeConfigCluster("arn:aws:eks:us-east-1:123456789012:cluster/test-cluster"),
		WithAnnotations(ann(
			AnnotationCredentialProvider, CredentialProviderKubernetes,
			AnnotationCredentialType, CredentialTypeJWT,
			AnnotationCredentialExpiration, "5m",
			AnnotationCredentialAudiences, "audience",
			AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
			AnnotationAWSSTSEndpoint, "https://sts.us-east-1.amazonaws.com",
		)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Credential.Provider != CredentialProviderKubernetes || got.Credential.Type != CredentialTypeJWT {
		t.Fatalf("got credentials (%q, %q)", got.Credential.Provider, got.Credential.Type)
	}
	if got.Credential.Expiration != 5*time.Minute {
		t.Fatalf("got expiration %v", got.Credential.Expiration)
	}
	if len(got.Credential.Audiences) != 1 || got.Credential.Audiences[0] != "audience" {
		t.Fatalf("got audiences %v", got.Credential.Audiences)
	}
	if got.AWS == nil {
		t.Fatal("got nil AWS config")
	}
	if got.AWS.RoleARN != "arn:aws:iam::123456789012:role/role" {
		t.Fatalf("got AWS RoleARN %q", got.AWS.RoleARN)
	}
	if got.AWS.STSEndpoint != "https://sts.us-east-1.amazonaws.com" {
		t.Fatalf("got AWS STSEndpoint %q", got.AWS.STSEndpoint)
	}
	if got.AWS.Cluster.Region != "us-east-1" || got.AWS.Cluster.Name != "test-cluster" {
		t.Fatalf("got AWS cluster (%q, %q)", got.AWS.Cluster.Region, got.AWS.Cluster.Name)
	}
}

func TestValidateConfigAzureResult(t *testing.T) {
	const uuid = "11111111-1111-1111-1111-111111111111"
	got, err := ValidateConfig(
		WithServiceProvider(ProviderAzure),
		WithKubeConfigCluster("/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster"),
		WithAnnotations(ann(
			AnnotationCredentialProvider, CredentialProviderKubernetes,
			AnnotationCredentialType, CredentialTypeJWT,
			AnnotationAzureTenantID, uuid,
			AnnotationAzureClientID, uuid,
		)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Azure == nil {
		t.Fatal("got nil Azure config")
	}
	if got.Azure.TenantID != uuid || got.Azure.ClientID != uuid {
		t.Fatalf("got Azure IDs (%q, %q)", got.Azure.TenantID, got.Azure.ClientID)
	}
	if got.Azure.Cluster.SubscriptionID != "12345678-1234-1234-1234-123456789012" ||
		got.Azure.Cluster.ResourceGroup != "test-rg" || got.Azure.Cluster.Name != "test-cluster" {
		t.Fatalf("got Azure cluster (%q, %q, %q)",
			got.Azure.Cluster.SubscriptionID, got.Azure.Cluster.ResourceGroup, got.Azure.Cluster.Name)
	}
}

func TestValidateConfigMTLSResult(t *testing.T) {
	got, err := ValidateConfig(WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org/foo")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.MTLS == nil || got.MTLS.SPIFFE == nil {
		t.Fatalf("got %+v, want MTLS SPIFFE", got.MTLS)
	}
	if got.MTLS.SPIFFE.ServerID.ID == nil || got.MTLS.SPIFFE.ServerID.ID.String() != "spiffe://example.org/foo" {
		t.Fatalf("got %+v", got.MTLS.SPIFFE.ServerID)
	}
}

func TestValidateConfigDefaults(t *testing.T) {
	got, err := ValidateConfig(WithServiceProvider(ProviderGCP))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Credential.Provider != CredentialProviderKubernetes || got.Credential.Type != CredentialTypeJWT {
		t.Fatalf("got credentials (%q, %q), want backward-compatible defaults",
			got.Credential.Provider, got.Credential.Type)
	}
}

func TestValidateConfigClusterWithoutServiceAccount(t *testing.T) {
	t.Run("aws", func(t *testing.T) {
		got, err := ValidateConfig(
			WithServiceProvider(ProviderAWS),
			WithKubeConfigCluster("arn:aws:eks:us-east-1:123456789012:cluster/test-cluster"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AWS == nil || got.AWS.RoleARN != "" {
			t.Fatalf("got AWS config %+v", got.AWS)
		}
		if got.AWS.Cluster == nil || got.AWS.Cluster.Region != "us-east-1" || got.AWS.Cluster.Name != "test-cluster" {
			t.Fatalf("got AWS cluster %+v", got.AWS.Cluster)
		}
	})

	t.Run("azure", func(t *testing.T) {
		got, err := ValidateConfig(
			WithServiceProvider(ProviderAzure),
			WithKubeConfigCluster("/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Azure == nil || got.Azure.TenantID != "" || got.Azure.ClientID != "" {
			t.Fatalf("got Azure config %+v", got.Azure)
		}
		if got.Azure.Cluster == nil || got.Azure.Cluster.SubscriptionID != "12345678-1234-1234-1234-123456789012" ||
			got.Azure.Cluster.ResourceGroup != "test-rg" || got.Azure.Cluster.Name != "test-cluster" {
			t.Fatalf("got Azure cluster %+v", got.Azure.Cluster)
		}
	})
}

func TestValidateConfigKubeConfigAudiences(t *testing.T) {
	got, err := ValidateConfig(WithKubeConfigAudiences("audience1\naudience2, audience3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"audience1", "audience2", "audience3"}
	if !reflect.DeepEqual(got.Credential.Audiences, want) {
		t.Fatalf("got audiences %v, want %v", got.Credential.Audiences, want)
	}
}

func TestValidateConfigBranches(t *testing.T) {
	const uuid = "11111111-1111-1111-1111-111111111111"

	optIn := ann(
		AnnotationCredentialProvider, CredentialProviderKubernetes,
		AnnotationCredentialType, CredentialTypeJWT,
	)

	tests := []struct {
		name    string
		opts    []Option
		wantErr string
	}{
		// Additional valid branches.
		{
			name: "mTLS self trust domain",
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://self"))},
		},
		{
			name: "mTLS prefix",
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org/foo/"))},
		},
		{
			name: "mTLS exact id",
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org/foo"))},
		},
		{
			name: "annotations without slash",
			opts: []Option{WithAnnotations(map[string]string{"justakey": "x"})},
		},
		{
			name: "generic OCI imagePullSecrets with mTLS",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(true),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org")),
			},
		},
		{
			name: "generic OCI imagePullSecrets with new auth annotation",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(true),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
			wantErr: ".imagePullSecrets",
		},
		{
			name: "aws legacy sts endpoint",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithSTSEndpoint("ldap", "https://sts.amazonaws.com"),
				WithAnnotations(ann(
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
		},
		{
			name: "legacy bucket sts endpoint with annotations allowed",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithSTSEndpoint("ldap", "https://sts.amazonaws.com"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
		},
		{
			name: "azure kubeconfig cluster",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithKubeConfigCluster("/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationAzureTenantID, uuid,
					AnnotationAzureClientID, uuid,
				)),
			},
		},
		{
			name: "gcp kubeconfig cluster",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithKubeConfigCluster("projects/test-project/locations/us-central1/clusters/test-cluster"),
				WithAnnotations(optIn),
			},
		},

		// Error branches.
		{
			name:    "invalid credential provider",
			opts:    []Option{WithAnnotations(ann(AnnotationCredentialProvider, "foo", AnnotationCredentialType, CredentialTypeJWT))},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name:    "invalid credential type",
			opts:    []Option{WithAnnotations(ann(AnnotationCredentialProvider, CredentialProviderKubernetes, AnnotationCredentialType, "foo"))},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name:    "invalid mTLS id",
			opts:    []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "nope"))},
			wantErr: "invalid SPIFFE ID",
		},
		{
			name:    "mTLS prefix without path",
			opts:    []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org/"))},
			wantErr: "prefix must include a path",
		},
		{
			name:    "mTLS invalid prefix",
			opts:    []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerID, "spiffe://example.org/a b/"))},
			wantErr: "invalid SPIFFE ID prefix",
		},
		{
			name: "invalid transit engine login path",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationTransitEngineLoginPath, "bad path!",
				AnnotationTransitEngineRole, "role",
			))},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid transit engine role",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationTransitEngineLoginPath, "auth/jwt/login",
				AnnotationTransitEngineRole, "bad role!",
			))},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid expiration",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationCredentialExpiration, "nope",
			))},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "non-positive expiration",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationCredentialExpiration, "0s",
			))},
			wantErr: "duration must be positive",
		},
		{
			name: "invalid username",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderGeneric),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialUsername, "bad user",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid aws role arn",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "nope",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "missing aws role arn",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(optIn),
			},
			wantErr: "is required",
		},
		{
			name: "invalid aws sts endpoint",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
					AnnotationAWSSTSEndpoint, "nope",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid azure tenant id",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationAzureTenantID, "nope",
					AnnotationAzureClientID, uuid,
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "missing azure tenant id",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationAzureClientID, uuid,
				)),
			},
			wantErr: "is required",
		},
		{
			name: "invalid azure client id",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationAzureTenantID, uuid,
					AnnotationAzureClientID, "nope",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "missing azure client id",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationAzureTenantID, uuid,
				)),
			},
			wantErr: "is required",
		},
		{
			name: "invalid gcp workload identity provider",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationGCPWorkloadIdentityProvider, "nope",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid gcp service account",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationGCPServiceAccount, "nope",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid dockerhub oidc connection id",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderDockerHub),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialUsername, "user",
					AnnotationDockerHubOIDCConnectionID, "nope",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "missing dockerhub oidc connection id",
			opts: []Option{
				WithOCI(true),
				WithServiceProvider(ProviderDockerHub),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialUsername, "user",
				)),
			},
			wantErr: "is required",
		},
		{
			name: "invalid kubeconfig cluster azure",
			opts: []Option{
				WithServiceProvider(ProviderAzure),
				WithKubeConfigCluster("nope"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationAzureTenantID, uuid,
					AnnotationAzureClientID, uuid,
				)),
			},
			wantErr: "invalid kubeconfig ConfigMap data key cluster",
		},
		{
			name: "invalid kubeconfig cluster gcp",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithKubeConfigCluster("nope"),
				WithAnnotations(optIn),
			},
			wantErr: "invalid kubeconfig ConfigMap data key cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateConfig(tt.opts...)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got error %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
