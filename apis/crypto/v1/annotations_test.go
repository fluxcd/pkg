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

import "testing"

func TestAnnotationStringers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"EKSRoleARN", AnnotationEKSRoleARN.String(), string(AnnotationEKSRoleARN)},
		{"AWSSTSEndpoint", AnnotationAWSSTSEndpoint.String(), string(AnnotationAWSSTSEndpoint)},
		{"AWSCluster", AWSCluster("").String(), string(ClusterPatternAWS)},
		{"AzureTenantID", AnnotationAzureTenantID.String(), string(AnnotationAzureTenantID)},
		{"AzureClientID", AnnotationAzureClientID.String(), string(AnnotationAzureClientID)},
		{"AzureCluster", AzureCluster("").String(), string(ClusterPatternAzure)},
		{"GCPWorkloadIdentityProvider", AnnotationGCPWorkloadIdentityProvider.String(), string(AnnotationGCPWorkloadIdentityProvider)},
		{"GCPServiceAccount", AnnotationGCPServiceAccount.String(), string(AnnotationGCPServiceAccount)},
		{"GCPCluster", GCPCluster("").String(), string(ClusterPatternGCP)},
		{"DockerHubOIDCConnectionID", AnnotationDockerHubOIDCConnectionID.String(), string(AnnotationDockerHubOIDCConnectionID)},
		{"CredentialProvider", AnnotationCredentialProvider.String(), string(AnnotationCredentialProvider)},
		{"CredentialType", AnnotationCredentialType.String(), string(AnnotationCredentialType)},
		{"CredentialExpiration", AnnotationCredentialExpiration.String(), string(AnnotationCredentialExpiration)},
		{"CredentialUsername", AnnotationCredentialUsername.String(), string(AnnotationCredentialUsername)},
		{"CredentialAudiences", AnnotationCredentialAudiences.String(), string(AnnotationCredentialAudiences)},
		{"TransitEngineLoginPath", AnnotationTransitEngineLoginPath.String(), string(AnnotationTransitEngineLoginPath)},
		{"TransitEngineRole", AnnotationTransitEngineRole.String(), string(AnnotationTransitEngineRole)},
		{"MTLSSPIFFEServerID", AnnotationMTLSSPIFFEServerID.String(), string(AnnotationMTLSSPIFFEServerID)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
