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

func TestBuildSpec(t *testing.T) {
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
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org"))},
		},
		{
			name: "mTLS with opt-in",
			opts: []Option{WithAnnotations(ann(
				AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org",
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
			))},
		},
		{
			name: "mTLS with GitHub Enterprise Server",
			opts: []Option{
				WithServiceProvider(ProviderGitHub),
				WithGitHubEnterpriseServer(),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
		},
		{
			name: "expiration with kubernetes",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationCredentialExpiration, "10m",
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
				WithOCI(),
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
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(),
			},
		},
		{
			name: "generic OCI static credentials from secretRef",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithSecretRefAuth(),
			},
		},
		{
			name: "dockerhub",
			opts: []Option{
				WithOCI(),
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
			name: "aws x509",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeX509,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
		},
		{
			name: "gcp x509",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeX509,
					AnnotationGCPWorkloadIdentityProvider, "projects/1/locations/global/workloadIdentityPools/p/providers/x",
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
			name: "generic x509",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeX509,
				)),
			},
			wantErr: "not supported by the generic service provider",
		},
		{
			name: "github x509",
			opts: []Option{
				WithServiceProvider(ProviderGitHub),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeX509,
				)),
			},
			wantErr: "not supported by the github service provider",
		},
		{
			name: "dockerhub x509",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderDockerHub),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeX509,
				)),
			},
			wantErr: "not supported by the dockerhub service provider",
		},
		{
			name: "x509 without service provider",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderSPIFFE,
				AnnotationCredentialType, CredentialTypeX509,
			))},
			wantErr: "not supported without a service provider",
		},
		{
			name: "mTLS with borrowed cloud annotation without opt-in",
			opts: []Option{WithAnnotations(ann(
				AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org",
				AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
			))},
			wantErr: "must be set together with any authentication annotation",
		},
		{
			name: "mTLS with certSecretRef",
			opts: []Option{
				WithCertSecretRef(),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
			wantErr: ".spec.certSecretRef",
		},
		{
			name: "mTLS with secretRef certs",
			opts: []Option{
				WithSecretRefCerts(),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
			wantErr: ".spec.secretRef",
		},
		{
			name: "mTLS with public provider",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
			wantErr: "not allowed with service provider aws",
		},
		{
			name: "mTLS with GitHub.com",
			opts: []Option{
				WithServiceProvider(ProviderGitHub),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
			wantErr: "not allowed with service provider github",
		},
		{
			name: "mTLS with unknown provider",
			opts: []Option{
				WithServiceProvider("unknown"),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
			wantErr: "not allowed with service provider unknown",
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
				WithSOPSVaultConfigMap(),
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
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(),
				WithAnnotations(optInKubernetes),
			},
			wantErr: ".imagePullSecrets",
		},
		{
			name: "generic OCI without opt-in",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
			},
		},
		{
			name: "generic OCI without opt-in and mTLS",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
		},
		{
			name: "dockerhub without opt-in",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderDockerHub),
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
				WithOCI(),
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
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeX509,
					AnnotationCredentialAudiences, "audience",
				)),
			},
			wantErr: "only supported by the jwt credential type",
		},
		{
			name: "audiences with dockerhub",
			opts: []Option{
				WithOCI(),
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
				AnnotationCredentialExpiration, "10m",
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
				WithOCI(),
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
			_, err := BuildSpec(tt.opts...)
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

func TestBuildSpecUnrelatedFluxAnnotations(t *testing.T) {
	unrelated := ann(
		"kustomize.toolkit.fluxcd.io/prune", "enabled",
		"source.toolkit.fluxcd.io/ignore", "true",
		AnnotationGCPWorkloadIdentityProvider, "projects/1/locations/global/workloadIdentityPools/p/providers/x",
	)

	for _, tt := range []struct {
		name string
		opts []Option
	}{
		{
			name: "legacy audiences ConfigMap key",
			opts: []Option{
				WithKubeConfigAudiences("audience"),
				WithAnnotations(unrelated),
			},
		},
		{
			name: "legacy sops vault ConfigMap flag",
			opts: []Option{
				WithSOPSVaultConfigMap(),
				WithAnnotations(unrelated),
			},
		},
		{
			name: "legacy Bucket.spec.sts.provider",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithSTSEndpoint(ProviderAWS, "https://sts.amazonaws.com"),
				WithAnnotations(unrelated),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildSpec(tt.opts...); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildSpecResult(t *testing.T) {
	got, err := BuildSpec(
		WithOCI(),
		WithServiceProvider(ProviderAWS),
		WithKubeConfigCluster("arn:aws:eks:us-east-1:123456789012:cluster/test-cluster"),
		WithAnnotations(ann(
			AnnotationCredentialProvider, CredentialProviderKubernetes,
			AnnotationCredentialType, CredentialTypeJWT,
			AnnotationCredentialExpiration, "10m",
			AnnotationCredentialAudiences, "audience",
			AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
			AnnotationAWSSTSEndpoint, "https://sts.us-east-1.amazonaws.com",
		)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Auth.Credential.Provider != CredentialProviderKubernetes || got.Auth.Credential.Type != CredentialTypeJWT {
		t.Fatalf("got credentials (%q, %q)", got.Auth.Credential.Provider, got.Auth.Credential.Type)
	}
	if !got.Auth.OptedIn() {
		t.Fatal("got OptedIn false, want true")
	}
	if got.Auth.Credential.Expiration != 10*time.Minute {
		t.Fatalf("got expiration %v", got.Auth.Credential.Expiration)
	}
	if len(got.Auth.Credential.Audiences) != 1 || got.Auth.Credential.Audiences[0] != "audience" {
		t.Fatalf("got audiences %v", got.Auth.Credential.Audiences)
	}
	if got.Auth.AWS == nil {
		t.Fatal("got nil AWS spec")
	}
	if got.Auth.AWS.RoleARN != "arn:aws:iam::123456789012:role/role" {
		t.Fatalf("got AWS RoleARN %q", got.Auth.AWS.RoleARN)
	}
	if got.Auth.AWS.STSEndpoint != "https://sts.us-east-1.amazonaws.com" {
		t.Fatalf("got AWS STSEndpoint %q", got.Auth.AWS.STSEndpoint)
	}
	if got.Auth.AWS.Cluster.Region != "us-east-1" || got.Auth.AWS.Cluster.Name != "test-cluster" {
		t.Fatalf("got AWS cluster (%q, %q)", got.Auth.AWS.Cluster.Region, got.Auth.AWS.Cluster.Name)
	}
}

func TestBuildSpecSTSEndpointOption(t *testing.T) {
	got, err := BuildSpec(
		WithServiceProvider(ProviderAWS),
		WithSTSEndpoint("ldap", "https://sts.eu-west-1.amazonaws.com"),
		WithAnnotations(ann(
			AnnotationCredentialProvider, CredentialProviderKubernetes,
			AnnotationCredentialType, CredentialTypeJWT,
			AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
			AnnotationAWSSTSEndpoint, "https://sts.us-east-1.amazonaws.com",
		)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Auth.AWS == nil {
		t.Fatal("got nil AWS spec")
	}
	if got.Auth.AWS.STSEndpoint != "https://sts.eu-west-1.amazonaws.com" {
		t.Fatalf("got AWS STSEndpoint %q", got.Auth.AWS.STSEndpoint)
	}
}

func TestBuildSpecAzureResult(t *testing.T) {
	const uuid = "11111111-1111-1111-1111-111111111111"
	got, err := BuildSpec(
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
	if got.Auth.Azure == nil {
		t.Fatal("got nil Azure spec")
	}
	if got.Auth.Azure.TenantID != uuid || got.Auth.Azure.ClientID != uuid {
		t.Fatalf("got Azure IDs (%q, %q)", got.Auth.Azure.TenantID, got.Auth.Azure.ClientID)
	}
	if got.Auth.Azure.Cluster.SubscriptionID != "12345678-1234-1234-1234-123456789012" ||
		got.Auth.Azure.Cluster.ResourceGroup != "test-rg" || got.Auth.Azure.Cluster.Name != "test-cluster" {
		t.Fatalf("got Azure cluster (%q, %q, %q)",
			got.Auth.Azure.Cluster.SubscriptionID, got.Auth.Azure.Cluster.ResourceGroup, got.Auth.Azure.Cluster.Name)
	}
}

func TestBuildSpecMTLSResult(t *testing.T) {
	got, err := BuildSpec(WithAnnotations(ann(
		AnnotationMTLSSPIFFEServerIDs,
		" spiffe://example.org/foo,\nspiffe://example.org/bar/ , spiffe://self, spiffe://example.org ",
	)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.MTLS == nil || got.MTLS.SPIFFE == nil {
		t.Fatalf("got %+v, want MTLS SPIFFE", got.MTLS)
	}
	serverIDs := got.MTLS.SPIFFE.ServerIDs
	if len(serverIDs) != 4 {
		t.Fatalf("got %d server IDs, want 4: %+v", len(serverIDs), serverIDs)
	}
	if serverIDs[0].ID == nil || serverIDs[0].ID.String() != "spiffe://example.org/foo" {
		t.Fatalf("got %+v, want exact SPIFFE ID", serverIDs[0])
	}
	if serverIDs[1].Prefix == nil || serverIDs[1].Prefix.String() != "spiffe://example.org/bar" {
		t.Fatalf("got %+v, want SPIFFE ID prefix", serverIDs[1])
	}
	if !serverIDs[2].UseSelfTrustDomain || serverIDs[2].ID != nil ||
		serverIDs[2].Prefix != nil || serverIDs[2].TrustDomain != nil {
		t.Fatalf("got %+v, want self trust domain", serverIDs[2])
	}
	if serverIDs[3].TrustDomain == nil || serverIDs[3].TrustDomain.String() != "example.org" {
		t.Fatalf("got %+v, want trust domain", serverIDs[3])
	}
	if got.Auth != nil {
		t.Fatalf("got auth %+v, want nil for non-opted-in ServiceAccount", got.Auth)
	}
}

func TestBuildSpecGenericOCI(t *testing.T) {
	tests := []struct {
		name     string
		opts     []Option
		wantMTLS bool
	}{
		{
			name: "no annotations and no imagePullSecrets",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
			},
		},
		{
			name: "imagePullSecrets",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(),
			},
		},
		{
			name: "mTLS only",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
			wantMTLS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildSpec(tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Auth != nil {
				t.Fatalf("got auth %+v, want nil for non-opted-in ServiceAccount", got.Auth)
			}
			if tt.wantMTLS {
				if got.MTLS == nil || got.MTLS.SPIFFE == nil {
					t.Fatalf("got %+v, want MTLS SPIFFE", got.MTLS)
				}
			} else if got.MTLS != nil {
				t.Fatalf("got MTLS %+v, want nil", got.MTLS)
			}
		})
	}
}

func TestBuildSpecDefaults(t *testing.T) {
	got, err := BuildSpec(WithServiceProvider(ProviderGCP))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Auth != nil {
		t.Fatalf("got auth %+v, want nil for non-opted-in ServiceAccount", got.Auth)
	}
}

func TestBuildSpecTransitEngine(t *testing.T) {
	got, err := BuildSpec(WithAnnotations(ann(
		AnnotationCredentialProvider, CredentialProviderKubernetes,
		AnnotationCredentialType, CredentialTypeJWT,
		AnnotationTransitEngineLoginPath, "auth/jwt/login",
		AnnotationTransitEngineRole, "role",
	)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Auth == nil || got.Auth.TransitEngine == nil {
		t.Fatalf("got auth %+v, want transit engine spec", got.Auth)
	}
	if got.Auth.TransitEngine.LoginPath != "auth/jwt/login" || got.Auth.TransitEngine.Role != "role" {
		t.Fatalf("got transit engine %+v", got.Auth.TransitEngine)
	}
}

func TestBuildSpecClusterWithoutServiceAccount(t *testing.T) {
	t.Run("aws", func(t *testing.T) {
		got, err := BuildSpec(
			WithServiceProvider(ProviderAWS),
			WithKubeConfigCluster("arn:aws:eks:us-east-1:123456789012:cluster/test-cluster"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Auth.AWS == nil || got.Auth.AWS.RoleARN != "" {
			t.Fatalf("got AWS spec %+v", got.Auth.AWS)
		}
		if got.Auth.AWS.Cluster == nil || got.Auth.AWS.Cluster.Region != "us-east-1" || got.Auth.AWS.Cluster.Name != "test-cluster" {
			t.Fatalf("got AWS cluster %+v", got.Auth.AWS.Cluster)
		}
	})

	t.Run("azure", func(t *testing.T) {
		got, err := BuildSpec(
			WithServiceProvider(ProviderAzure),
			WithKubeConfigCluster("/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Auth.Azure == nil || got.Auth.Azure.TenantID != "" || got.Auth.Azure.ClientID != "" {
			t.Fatalf("got Azure spec %+v", got.Auth.Azure)
		}
		if got.Auth.Azure.Cluster == nil || got.Auth.Azure.Cluster.SubscriptionID != "12345678-1234-1234-1234-123456789012" ||
			got.Auth.Azure.Cluster.ResourceGroup != "test-rg" || got.Auth.Azure.Cluster.Name != "test-cluster" {
			t.Fatalf("got Azure cluster %+v", got.Auth.Azure.Cluster)
		}
	})
}

func TestBuildSpecKubeConfigAudiences(t *testing.T) {
	got, err := BuildSpec(WithKubeConfigAudiences("audience1\naudience2, audience3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"audience1", "audience2", "audience3"}
	if !reflect.DeepEqual(got.Auth.Credential.Audiences, want) {
		t.Fatalf("got audiences %v, want %v", got.Auth.Credential.Audiences, want)
	}
	if got.Auth.OptedIn() {
		t.Fatal("got OptedIn true, want false")
	}
	if got.Auth.Credential.Provider != "" || got.Auth.Credential.Type != "" {
		t.Fatalf("got credentials (%q, %q), want empty for non-opted-in ServiceAccount",
			got.Auth.Credential.Provider, got.Auth.Credential.Type)
	}
}

func TestBuildSpecKubeConfigAudiencesEmpty(t *testing.T) {
	got, err := BuildSpec(WithKubeConfigAudiences(" , \n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Auth != nil {
		t.Fatalf("got auth %+v, want nil for empty audiences", got.Auth)
	}
}

func TestBuildSpecBranches(t *testing.T) {
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
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://self"))},
		},
		{
			name: "mTLS prefix",
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org/foo/"))},
		},
		{
			name: "mTLS exact id",
			opts: []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org/foo"))},
		},
		{
			name: "annotations without slash",
			opts: []Option{WithAnnotations(map[string]string{"justakey": "x"})},
		},
		{
			name: "generic OCI imagePullSecrets with mTLS",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(),
				WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org")),
			},
		},
		{
			name: "generic OCI imagePullSecrets with new auth annotation",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithImagePullSecrets(),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
			wantErr: ".imagePullSecrets",
		},
		{
			name: "generic OCI secretRef auth with new auth annotation",
			opts: []Option{
				WithOCI(),
				WithServiceProvider(ProviderGeneric),
				WithSecretRefAuth(),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
				)),
			},
			wantErr: "static credentials in .spec.secretRef",
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
			name: "legacy bucket sts endpoint with valid annotation",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithSTSEndpoint("ldap", "https://sts.us-east-1.amazonaws.com"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
					AnnotationAWSSTSEndpoint, "https://sts.eu-west-1.amazonaws.com",
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
			opts:    []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "nope"))},
			wantErr: "invalid SPIFFE ID",
		},
		{
			name:    "mTLS without server ids",
			opts:    []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, " ,\n "))},
			wantErr: "must specify at least one SPIFFE ID",
		},
		{
			name:    "mTLS prefix without path",
			opts:    []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org/"))},
			wantErr: "prefix must include a path",
		},
		{
			name:    "mTLS invalid prefix",
			opts:    []Option{WithAnnotations(ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org/a b/"))},
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
			name: "expiration below minimum",
			opts: []Option{WithAnnotations(ann(
				AnnotationCredentialProvider, CredentialProviderKubernetes,
				AnnotationCredentialType, CredentialTypeJWT,
				AnnotationCredentialExpiration, "9m",
			))},
			wantErr: "duration must be at least 10m",
		},
		{
			name: "invalid username",
			opts: []Option{
				WithOCI(),
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
			name: "invalid aws sts endpoint path",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
					AnnotationAWSSTSEndpoint, "https://sts.amazonaws.com/path",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid aws sts annotation alongside valid legacy endpoint",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithSTSEndpoint("ldap", "https://sts.us-east-1.amazonaws.com"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
					AnnotationAWSSTSEndpoint, "https://sts.amazonaws.com?foo=bar",
				)),
			},
			wantErr: "invalid ServiceAccount annotation key",
		},
		{
			name: "invalid legacy bucket sts endpoint",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithSTSEndpoint("ldap", "http://sts.amazonaws.com"),
				WithAnnotations(ann(
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
				)),
			},
			wantErr: "invalid Bucket.spec.sts.endpoint",
		},
		{
			name: "invalid legacy bucket sts endpoint alongside valid annotation",
			opts: []Option{
				WithServiceProvider(ProviderAWS),
				WithSTSEndpoint("ldap", "https://sts.amazonaws.com/path"),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationEKSRoleARN, "arn:aws:iam::123456789012:role/role",
					AnnotationAWSSTSEndpoint, "https://sts.us-east-1.amazonaws.com",
				)),
			},
			wantErr: "invalid Bucket.spec.sts.endpoint",
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
			name: "gcp audiences without workload identity provider",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderKubernetes,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationCredentialAudiences, "my-audience",
				)),
			},
			wantErr: "audiences are not supported by the gcp service provider without annotation",
		},
		{
			name: "gcp non-kubernetes credential provider without workload identity provider",
			opts: []Option{
				WithServiceProvider(ProviderGCP),
				WithAnnotations(ann(
					AnnotationCredentialProvider, CredentialProviderSPIFFE,
					AnnotationCredentialType, CredentialTypeJWT,
					AnnotationGCPServiceAccount, "sa@project.iam.gserviceaccount.com",
				)),
			},
			wantErr: "is required for the gcp service provider with the spiffe credential provider",
		},
		{
			name: "invalid dockerhub oidc connection id",
			opts: []Option{
				WithOCI(),
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
				WithOCI(),
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
			_, err := BuildSpec(tt.opts...)
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
