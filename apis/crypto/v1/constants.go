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

import "regexp"

// Top-level group.
const (
	// GroupCore is the core API group for Flux.
	GroupCore = "fluxcd.io"
)

// Level 1 groups.
const (
	// GroupAuth is the API group for authentication-related annotations.
	GroupAuth = "auth." + GroupCore

	// GroupMTLS is the API group for mTLS-related annotations.
	GroupMTLS = "mtls." + GroupCore
)

// Level 2 groups (auth).
const (
	// GroupAuthCredential is the API group for credential-related annotations.
	GroupAuthCredential = "credential." + GroupAuth

	// GroupAuthTransitEngine is the API group for transit engine-related annotations.
	GroupAuthTransitEngine = "transit-engine." + GroupAuth

	// GroupAuthAWS is the API group for AWS-related authentication annotations.
	GroupAuthAWS = "aws." + GroupAuth

	// GroupAuthGCP is the API group for GCP-related authentication annotations.
	GroupAuthGCP = "gcp." + GroupAuth

	// GroupAuthDockerHub is the API group for Docker Hub-related authentication annotations.
	GroupAuthDockerHub = "dockerhub." + GroupAuth
)

// Level 2 groups (mtls).
const (
	// GroupMTLSSPIFFE is the API group for SPIFFE-related mTLS annotations.
	GroupMTLSSPIFFE = "spiffe." + GroupMTLS
)

// Groups borrowed from the cloud providers for UX consistency with the
// managed Kubernetes services.
const (
	// GroupAWSEKS is the API group for AWS EKS-related annotations.
	GroupAWSEKS = "eks.amazonaws.com"

	// GroupAzureWorkloadIdentity is the API group for Azure Workload Identity-related annotations.
	GroupAzureWorkloadIdentity = "azure.workload.identity"

	// GroupGKEIAM is the API group for GKE IAM-related annotations.
	GroupGKEIAM = "iam.gke.io"
)

// Service provider names.
const (
	// ProviderAWS is the AWS service provider.
	ProviderAWS = "aws"

	// ProviderAzure is the Azure service provider.
	ProviderAzure = "azure"

	// ProviderGCP is the GCP service provider.
	ProviderGCP = "gcp"

	// ProviderDockerHub is the Docker Hub service provider.
	ProviderDockerHub = "dockerhub"

	// ProviderGeneric is the service provider for opensource services that
	// accept OIDC tokens directly, using short-lived cryptographic material.
	ProviderGeneric = "generic"

	// ProviderGitHub is the GitHub service provider.
	ProviderGitHub = "github"
)

// Credential provider values.
const (
	// CredentialProviderKubernetes is the credential provider that uses the
	// Kubernetes TokenRequest API for acquiring the credential.
	CredentialProviderKubernetes = "kubernetes"

	// CredentialProviderSPIFFE is the credential provider that uses the
	// SPIFFE Broker API for acquiring the credential.
	CredentialProviderSPIFFE = "spiffe"
)

// Credential type values.
const (
	// CredentialTypeJWT is the credential type for JSON Web Tokens.
	CredentialTypeJWT = "jwt"

	// CredentialTypeX509 is the credential type for X.509 certificates.
	CredentialTypeX509 = "x509"
)

// Credential annotation keys.
const (
	// AnnotationCredentialProvider specifies the provider that issues the
	// credential. It must be one of `kubernetes` or `spiffe`.
	AnnotationCredentialProvider = GroupAuthCredential + "/provider"

	// AnnotationCredentialType specifies the type of the credential.
	// It must be one of `jwt` or `x509`.
	AnnotationCredentialType = GroupAuthCredential + "/type"

	// AnnotationCredentialExpiration specifies the requested expiration time
	// of the credential as a Go duration string, e.g. `5m` for five minutes
	// or `1h` for one hour. It is supported only by the `kubernetes` credential
	// provider.
	AnnotationCredentialExpiration = GroupAuthCredential + "/expiration"

	// AnnotationCredentialAudiences specifies the target audiences of the
	// credential. The value is split on line breaks and commas, leading and
	// trailing whitespace is trimmed and empty entries are dropped. It is
	// supported only by the `jwt` credential type.
	AnnotationCredentialAudiences = GroupAuthCredential + "/audiences"

	// AnnotationCredentialUsername specifies an optional username to use when
	// authenticating with the service.
	AnnotationCredentialUsername = GroupAuthCredential + "/username"
)

// Transit engine annotation keys.
const (
	// AnnotationTransitEngineLoginPath specifies the HTTP path in the transit
	// engine where the request for login must be sent, e.g. `auth/jwt/login`.
	AnnotationTransitEngineLoginPath = GroupAuthTransitEngine + "/login-path"

	// AnnotationTransitEngineRole specifies the role to be used when
	// authenticating against the transit engine.
	AnnotationTransitEngineRole = GroupAuthTransitEngine + "/role"
)

// mTLS annotation keys.
const (
	// AnnotationMTLSSPIFFEServerID opts a ServiceAccount into using SPIFFE mTLS
	// and specifies what the server SPIFFE ID should look like for
	// authorization. It can be one of:
	//
	//   - `spiffe://<trust domain>/<path>` as an exact SPIFFE ID.
	//   - `spiffe://<trust domain>/<path prefix>/` as a SPIFFE ID prefix.
	//     The trailing slash identifies a prefix.
	//   - `spiffe://<trust domain>` as a SPIFFE Trust Domain.
	//   - `spiffe://self` as a symbol for the SPIFFE Trust Domain of our own X509-SVID.
	AnnotationMTLSSPIFFEServerID = GroupMTLSSPIFFE + "/server-id"

	// SPIFFETrustDomainSelf is a symbol for the SPIFFE Trust Domain of our own
	// X509-SVID.
	SPIFFETrustDomainSelf = "self"
)

// AWS annotation keys.
const (
	// AnnotationAWSSTSEndpoint specifies the AWS STS endpoint for the
	// ServiceAccount.
	AnnotationAWSSTSEndpoint = GroupAuthAWS + "/sts-endpoint"

	// AnnotationEKSRoleARN specifies the AWS IAM Role ARN for the ServiceAccount.
	AnnotationEKSRoleARN = GroupAWSEKS + "/role-arn"
)

// GCP annotation keys.
const (
	// AnnotationGCPWorkloadIdentityProvider specifies the GCP Workload Identity
	// Provider for the ServiceAccount.
	AnnotationGCPWorkloadIdentityProvider = GroupAuthGCP + "/workload-identity-provider"

	// AnnotationGCPServiceAccount specifies the GCP Service Account email for
	// the ServiceAccount.
	AnnotationGCPServiceAccount = GroupGKEIAM + "/gcp-service-account"
)

// Azure annotation keys.
const (
	// AnnotationAzureTenantID specifies the Microsoft Entra tenant ID for the
	// ServiceAccount.
	AnnotationAzureTenantID = GroupAzureWorkloadIdentity + "/tenant-id"

	// AnnotationAzureClientID specifies the Microsoft Entra client ID for the
	// ServiceAccount.
	AnnotationAzureClientID = GroupAzureWorkloadIdentity + "/client-id"
)

// Docker Hub annotation keys.
const (
	// AnnotationDockerHubOIDCConnectionID specifies the Docker Hub OIDC
	// connection ID for the ServiceAccount.
	AnnotationDockerHubOIDCConnectionID = GroupAuthDockerHub + "/oidc-connection-id"
)

// Cluster resource name patterns.
const (
	// ClusterPatternAWS is the regex pattern for validating AWS cluster resource names.
	ClusterPatternAWS = `^arn:aws[\w-]*:eks:([^:]{1,100}):[0-9]{1,30}:cluster/(.{1,200})$`

	// ClusterPatternAzure is the regex pattern for validating Azure cluster resource names.
	ClusterPatternAzure = `(?i)^/subscriptions/([^/]{36})/resourceGroups/([^/]{1,200})/providers/Microsoft\.ContainerService/managedClusters/([^/]{1,200})$`

	// ClusterPatternGCP is the regex pattern for validating GCP cluster resource names.
	ClusterPatternGCP = `^projects/[^/]{1,200}/locations/[^/]{1,200}/clusters/[^/]{1,200}$`
)

var (
	credentialProviderRegexp = regexp.MustCompile(`^(kubernetes|spiffe)$`)
	credentialTypeRegexp     = regexp.MustCompile(`^(jwt|x509)$`)
	credentialUsernameRegexp = regexp.MustCompile(`^\S+$`)

	transitEngineLoginPathRegexp = regexp.MustCompile(`^[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*$`)
	transitEngineRoleRegexp      = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

	awsSTSEndpointRegexp    = regexp.MustCompile(`^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`)
	eksRoleARNRegexp        = regexp.MustCompile(`^arn:aws[\w-]*:iam::[0-9]{1,30}:role/.{1,200}$`)
	gcpWIPRegexp            = regexp.MustCompile(`^projects/\d{1,30}/locations/global/workloadIdentityPools/[^/]{1,100}/providers/[^/]{1,100}$`)
	gcpServiceAccountRegexp = regexp.MustCompile(
		`^[a-zA-Z0-9-]{1,100}@[a-zA-Z0-9-]{1,100}\.iam\.gserviceaccount\.com$`)
	uuidRegexp = regexp.MustCompile(
		`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	clusterAWSRegexp   = regexp.MustCompile(ClusterPatternAWS)
	clusterAzureRegexp = regexp.MustCompile(ClusterPatternAzure)
	clusterGCPRegexp   = regexp.MustCompile(ClusterPatternGCP)
)
