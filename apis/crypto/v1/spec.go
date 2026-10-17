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
	"fmt"
	"time"
)

// Spec is the fully validated specification extracted from the Flux object
// context and, when object-level workload identity is configured, the
// ServiceAccount's annotations. It is the shared build result: the auth
// package consumes Auth and the mTLS package consumes MTLS.
type Spec struct {
	// Auth is the validated authentication specification. It is nil when there
	// is no authentication configuration.
	Auth *AuthSpec

	// MTLS is the validated mTLS specification. It is nil when the
	// ServiceAccount did not opt into mTLS.
	MTLS *MTLSSpec
}

// AuthSpec is the validated authentication specification.
type AuthSpec struct {
	// Credential is the credential acquisition specification. It is nil when
	// the ServiceAccount did not opt into object-level workload identity and no
	// legacy audiences are configured.
	Credential *CredentialSpec

	// Transit engine (SOPS).
	TransitEngine *TransitEngineSpec

	// Service-provider-specific specification.

	// AWS is the AWS specification, when the service provider is `aws`.
	AWS *AWSSpec
	// Azure is the Azure specification, when the service provider is `azure`.
	Azure *AzureSpec
	// GCP is the GCP specification, when the service provider is `gcp`.
	GCP *GCPSpec
	// DockerHub is the Docker Hub specification, when the service provider is
	// `dockerhub`.
	DockerHub *DockerHubSpec
}

// MTLSSpec is the validated mTLS specification.
type MTLSSpec struct {
	// SPIFFE is the SPIFFE mTLS specification.
	SPIFFE *MTLSSPIFFESpec
}

// MTLSSPIFFESpec is the validated SPIFFE mTLS specification.
type MTLSSPIFFESpec struct {
	// ServerIDs are the authorized SPIFFE IDs, prefixes or trust domains of
	// the servers.
	ServerIDs []SPIFFEPeerAuthorization
}

// CredentialSpec is the validated credential acquisition specification.
type CredentialSpec struct {
	// Provider is the credential provider, one of `kubernetes` or `spiffe`. It
	// is empty when the ServiceAccount did not opt into object-level workload
	// identity with the `/provider` and `/type` annotations.
	Provider string
	// Type is the credential type, one of `jwt` or `x509`. The `x509` type is
	// supported only by the `aws` and `gcp` service providers. It is empty when
	// the ServiceAccount did not opt into object-level workload identity with
	// the `/provider` and `/type` annotations.
	Type string
	// Expiration is the requested credential expiration.
	Expiration time.Duration
	// Audiences are the target audiences of the credential.
	Audiences []string
	// Username is the optional username to authenticate with the service.
	Username string
}

// OptedIn reports whether the ServiceAccount explicitly opted into
// object-level workload identity with the `/provider` and `/type` annotations.
func (a *AuthSpec) OptedIn() bool {
	return a != nil && a.Credential != nil && a.Credential.Provider != "" && a.Credential.Type != ""
}

// TransitEngineSpec is the validated transit engine specification.
type TransitEngineSpec struct {
	// LoginPath is the HTTP path in the transit engine where the request for
	// login must be sent.
	LoginPath string
	// Role is the role to use when authenticating against the transit engine.
	Role string
}

// AWSSpec is the validated service-provider-specific specification for AWS.
type AWSSpec struct {
	// RoleARN is the AWS IAM Role ARN.
	RoleARN string
	// STSEndpoint is the resolved AWS STS endpoint.
	STSEndpoint string
	// Cluster is the parsed kubeconfig cluster resource name.
	Cluster *AWSClusterSpec
}

// AWSClusterSpec is the parsed kubeconfig cluster resource name for AWS.
type AWSClusterSpec struct {
	// Region is the AWS region.
	Region string
	// Name is the cluster name.
	Name string
}

// AzureSpec is the validated service-provider-specific specification for
// Azure.
type AzureSpec struct {
	// TenantID is the Microsoft Entra tenant ID.
	TenantID string
	// ClientID is the Microsoft Entra client ID.
	ClientID string
	// Cluster is the parsed kubeconfig cluster resource name.
	Cluster *AzureClusterSpec
}

// AzureClusterSpec is the parsed kubeconfig cluster resource name for Azure.
type AzureClusterSpec struct {
	// SubscriptionID is the subscription ID.
	SubscriptionID string
	// ResourceGroup is the resource group.
	ResourceGroup string
	// Name is the cluster name.
	Name string
}

// GCPSpec is the validated service-provider-specific specification for GCP.
type GCPSpec struct {
	// WorkloadIdentityProvider is the GCP Workload Identity Provider.
	WorkloadIdentityProvider string
	// ServiceAccount is the GCP Service Account email.
	ServiceAccount string
}

// DockerHubSpec is the validated service-provider-specific specification for
// Docker Hub.
type DockerHubSpec struct {
	// OIDCConnectionID is the Docker Hub OIDC connection ID.
	OIDCConnectionID string
}

// Option is a functional option for BuildSpec. The options struct
// and its fields are intentionally unexported.
type Option func(*options)

// options holds the validation inputs, which are intentionally unexported.
type options struct {
	// Global properties.

	annotations     map[string]string
	serviceProvider string
	certSecretRef   bool
	secretRefAuth   bool

	// OCI properties (OCIRepository, ImageRepository and ImagePolicy).

	oci              bool
	imagePullSecrets bool

	// Git properties (GitRepository and ImageUpdateAutomation).

	secretRefCerts         bool
	githubEnterpriseServer bool

	// STS properties (Bucket).

	stsProvider string
	stsEndpoint string

	// kubeconfig properties (Kustomization and HelmRelease).

	kubeConfigCluster   string
	kubeConfigAudiences string

	// Decryption properties (Kustomization).

	sopsVaultConfigMap bool
}

// WithAnnotations sets the ServiceAccount annotations.
func WithAnnotations(annotations map[string]string) Option {
	return func(o *options) {
		o.annotations = annotations
	}
}

// WithServiceProvider sets the Flux object's service provider, e.g. `aws`.
func WithServiceProvider(provider string) Option {
	return func(o *options) {
		o.serviceProvider = provider
	}
}

// WithCertSecretRef indicates that `.spec.certSecretRef` is set.
func WithCertSecretRef() Option {
	return func(o *options) {
		o.certSecretRef = true
	}
}

// WithSecretRefAuth indicates that `.spec.secretRef` provides static
// authentication credentials.
func WithSecretRefAuth() Option {
	return func(o *options) {
		o.secretRefAuth = true
	}
}

// WithOCI indicates that the Flux object talks to an OCI registry.
func WithOCI() Option {
	return func(o *options) {
		o.oci = true
	}
}

// WithImagePullSecrets indicates that the ServiceAccount has
// `.imagePullSecrets`.
func WithImagePullSecrets() Option {
	return func(o *options) {
		o.imagePullSecrets = true
	}
}

// WithSecretRefCerts indicates that certificates or CAs are provided by
// `.spec.secretRef`.
func WithSecretRefCerts() Option {
	return func(o *options) {
		o.secretRefCerts = true
	}
}

// WithGitHubEnterpriseServer indicates that the `github` service provider is a
// GitHub Enterprise Server rather than GitHub.com. It has no effect for any
// other service provider.
func WithGitHubEnterpriseServer() Option {
	return func(o *options) {
		o.githubEnterpriseServer = true
	}
}

// WithSTSEndpoint sets the legacy `Bucket.spec.sts.provider` and
// `Bucket.spec.sts.endpoint` values, when present.
func WithSTSEndpoint(provider, endpoint string) Option {
	return func(o *options) {
		o.stsProvider = provider
		o.stsEndpoint = endpoint
	}
}

// WithKubeConfigCluster sets the value of the `cluster` key of the kubeconfig
// ConfigMap, when present.
func WithKubeConfigCluster(cluster string) Option {
	return func(o *options) {
		o.kubeConfigCluster = cluster
	}
}

// WithKubeConfigAudiences sets the value of the legacy `audiences` key of the
// kubeconfig ConfigMap, when present.
func WithKubeConfigAudiences(audiences string) Option {
	return func(o *options) {
		o.kubeConfigAudiences = audiences
	}
}

// WithSOPSVaultConfigMap indicates that the legacy kustomize-controller flag
// `--sops-vault-configmap` is set.
func WithSOPSVaultConfigMap() Option {
	return func(o *options) {
		o.sopsVaultConfigMap = true
	}
}

// BuildSpec applies all the API validation rules to the given inputs and
// returns the fully parsed and validated specification. It must be called even
// when no ServiceAccount is configured, as some inputs (e.g. the kubeconfig
// ConfigMap cluster resource) are validated and parsed regardless.
func BuildSpec(opts ...Option) (*Spec, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	ann := o.annotations

	// Credential provider and type.
	credProvider, providerOk, err := extract(ann, AnnotationCredentialProvider, credentialProviderRegexp)
	if err != nil {
		return nil, err
	}
	credType, typeOk, err := extract(ann, AnnotationCredentialType, credentialTypeRegexp)
	if err != nil {
		return nil, err
	}
	if providerOk != typeOk {
		return nil, fmt.Errorf("annotations %s and %s must be set together",
			AnnotationCredentialProvider, AnnotationCredentialType)
	}
	if credProvider == CredentialProviderKubernetes && credType != CredentialTypeJWT {
		return nil, fmt.Errorf("credential type %s is not supported by the %s credential provider",
			credType, credProvider)
	}
	// Only the service providers with support for exchanging X.509
	// certificates for credentials are the ones that support the x509
	// credential type.
	if credType == CredentialTypeX509 {
		switch o.serviceProvider {
		case ProviderAWS, ProviderGCP:
		case "":
			return nil, fmt.Errorf("credential type %s is not supported without a service provider",
				credType)
		default:
			return nil, fmt.Errorf("credential type %s is not supported by the %s service provider",
				credType, o.serviceProvider)
		}
	}
	credOptIn := providerOk
	if !credOptIn {
		credProvider = CredentialProviderKubernetes
		credType = CredentialTypeJWT
	}
	spec := &Spec{}
	var auth AuthSpec
	if credOptIn {
		auth.Credential = &CredentialSpec{
			Provider: credProvider,
			Type:     credType,
		}
	}

	// Every annotation introduced by this RFC, except the mTLS ones, requires
	// opting into object-level workload identity.
	if !credOptIn {
		if key, ok := findNewAuthAnnotation(ann); ok {
			return nil, fmt.Errorf("annotation %s requires annotations %s and %s",
				key, AnnotationCredentialProvider, AnnotationCredentialType)
		}
	}

	// mTLS.
	if serverIDsValue, ok := ann[AnnotationMTLSSPIFFEServerIDs]; ok {
		serverIDs := splitList(serverIDsValue)
		if len(serverIDs) == 0 {
			return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: must specify at least one SPIFFE ID",
				AnnotationMTLSSPIFFEServerIDs)
		}
		peers := make([]SPIFFEPeerAuthorization, 0, len(serverIDs))
		for _, serverID := range serverIDs {
			peer, err := ParseSPIFFEID(serverID)
			if err != nil {
				return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: %w",
					AnnotationMTLSSPIFFEServerIDs, err)
			}
			peers = append(peers, *peer)
		}
		if _, ok := findAuthAnnotation(ann); ok && !credOptIn {
			return nil, fmt.Errorf("annotations %s and %s must be set together with any authentication annotation when %s is used",
				AnnotationCredentialProvider, AnnotationCredentialType, AnnotationMTLSSPIFFEServerIDs)
		}
		if o.certSecretRef {
			return nil, fmt.Errorf("annotation %s is not allowed with .spec.certSecretRef",
				AnnotationMTLSSPIFFEServerIDs)
		}
		if o.secretRefCerts {
			return nil, fmt.Errorf("annotation %s is not allowed with certificates or CAs from .spec.secretRef",
				AnnotationMTLSSPIFFEServerIDs)
		}
		// The mTLS annotation is only supported by the `generic` service
		// provider and by GitHub Enterprise Server. An empty service provider
		// is allowed because mTLS is independent of credential acquisition and
		// is not bound to a service provider.
		switch o.serviceProvider {
		case "", ProviderGeneric:
		case ProviderGitHub:
			if !o.githubEnterpriseServer {
				return nil, fmt.Errorf("annotation %s is not allowed with service provider %s",
					AnnotationMTLSSPIFFEServerIDs, o.serviceProvider)
			}
		default:
			return nil, fmt.Errorf("annotation %s is not allowed with service provider %s",
				AnnotationMTLSSPIFFEServerIDs, o.serviceProvider)
		}
		spec.MTLS = &MTLSSpec{
			SPIFFE: &MTLSSPIFFESpec{
				ServerIDs: peers,
			},
		}
	}

	// The `dockerhub` service provider only supports credential acquisition, so
	// it always requires opting into object-level workload identity.
	if o.oci && o.serviceProvider == ProviderDockerHub && !credOptIn {
		return nil, fmt.Errorf("service provider %s requires annotations %s and %s",
			o.serviceProvider, AnnotationCredentialProvider, AnnotationCredentialType)
	}

	// The `generic` service provider does not require opting into object-level
	// workload identity. Without the opt-in annotations it authenticates with
	// the static credentials from the ServiceAccount's `.imagePullSecrets` when
	// present, or anonymously. This must not be an error: a ServiceAccount that
	// does not opt in, e.g. the default ServiceAccount, is a valid input and the
	// caller falls back to static credentials or anonymous access.
	genericStatic := o.oci && o.imagePullSecrets && o.serviceProvider == ProviderGeneric

	// Conflicts with legacy settings.
	if newAnn, ok := findNewAnnotation(ann); ok {
		switch {
		case o.kubeConfigAudiences != "":
			return nil, fmt.Errorf("annotation %s is not allowed with the legacy audiences ConfigMap key", newAnn)
		case o.sopsVaultConfigMap:
			return nil, fmt.Errorf("annotation %s is not allowed with the legacy --sops-vault-configmap flag", newAnn)
		case o.stsProvider == ProviderAWS:
			return nil, fmt.Errorf("annotation %s is not allowed with the legacy Bucket.spec.sts.provider set to aws", newAnn)
		case o.secretRefAuth:
			if key, ok := findNewAuthAnnotation(ann); ok {
				return nil, fmt.Errorf("annotation %s is not allowed with static credentials in .spec.secretRef", key)
			}
		case genericStatic:
			if key, ok := findNewAuthAnnotation(ann); ok {
				return nil, fmt.Errorf("annotation %s is not allowed with .imagePullSecrets", key)
			}
		}
	}

	// Transit engine (SOPS).
	loginPath, loginPathOk, err := extract(ann, AnnotationTransitEngineLoginPath, transitEngineLoginPathRegexp)
	if err != nil {
		return nil, err
	}
	role, roleOk, err := extract(ann, AnnotationTransitEngineRole, transitEngineRoleRegexp)
	if err != nil {
		return nil, err
	}
	if loginPathOk != roleOk {
		return nil, fmt.Errorf("annotations %s and %s must be set together",
			AnnotationTransitEngineLoginPath, AnnotationTransitEngineRole)
	}
	if loginPathOk {
		auth.TransitEngine = &TransitEngineSpec{
			LoginPath: loginPath,
			Role:      role,
		}
	}

	// Credential expiration.
	if value, ok := ann[AnnotationCredentialExpiration]; ok {
		expiration, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s': %w",
				AnnotationCredentialExpiration, value, err)
		}
		if expiration < 10*time.Minute {
			return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s'. duration must be at least 10m",
				AnnotationCredentialExpiration, value)
		}
		if credProvider != CredentialProviderKubernetes {
			return nil, fmt.Errorf("annotation %s is only supported by the %s credential provider",
				AnnotationCredentialExpiration, CredentialProviderKubernetes)
		}
		auth.Credential.Expiration = expiration
	}

	// Credential audiences.
	if value, ok := ann[AnnotationCredentialAudiences]; ok {
		if credType != CredentialTypeJWT {
			return nil, fmt.Errorf("annotation %s is only supported by the %s credential type",
				AnnotationCredentialAudiences, CredentialTypeJWT)
		}
		if o.serviceProvider == ProviderDockerHub {
			return nil, fmt.Errorf("annotation %s is not supported by the %s service provider",
				AnnotationCredentialAudiences, ProviderDockerHub)
		}
		auth.Credential.Audiences = splitList(value)
	} else if o.kubeConfigAudiences != "" {
		if audiences := splitList(o.kubeConfigAudiences); len(audiences) > 0 {
			if auth.Credential == nil {
				auth.Credential = &CredentialSpec{}
			}
			auth.Credential.Audiences = audiences
		}
	}

	// Credential username.
	username, usernameOk, err := extract(ann, AnnotationCredentialUsername, credentialUsernameRegexp)
	if err != nil {
		return nil, err
	}
	if usernameOk {
		if !o.oci {
			return nil, fmt.Errorf("annotation %s is only valid for the OCI APIs",
				AnnotationCredentialUsername)
		}
		if o.serviceProvider != ProviderGeneric && o.serviceProvider != ProviderDockerHub {
			return nil, fmt.Errorf("annotation %s is only valid for the %s and %s service providers",
				AnnotationCredentialUsername, ProviderGeneric, ProviderDockerHub)
		}
	}
	if o.oci && o.serviceProvider == ProviderDockerHub && !usernameOk {
		return nil, fmt.Errorf("annotation %s is required for the %s service provider",
			AnnotationCredentialUsername, ProviderDockerHub)
	}
	if username != "" {
		auth.Credential.Username = username
	}

	// Service-provider-specific specification.
	switch o.serviceProvider {
	case ProviderAWS:
		var aws AWSSpec
		roleARN, roleARNok, err := extract(ann, AnnotationEKSRoleARN, eksRoleARNRegexp)
		if err != nil {
			return nil, err
		}
		if credOptIn && !roleARNok {
			return nil, fmt.Errorf("annotation %s is required", AnnotationEKSRoleARN)
		}
		aws.RoleARN = roleARN
		if o.stsEndpoint != "" {
			if err := validateSTSEndpoint(o.stsEndpoint); err != nil {
				return nil, fmt.Errorf("invalid Bucket.spec.sts.endpoint: %w", err)
			}
			aws.STSEndpoint = o.stsEndpoint
		}
		if value, ok := ann[AnnotationAWSSTSEndpoint]; ok {
			if err := validateSTSEndpoint(value); err != nil {
				return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: %w",
					AnnotationAWSSTSEndpoint, err)
			}
			if aws.STSEndpoint == "" {
				aws.STSEndpoint = value
			}
		}
		if o.kubeConfigCluster != "" {
			matches, err := parseCluster(o.kubeConfigCluster, clusterAWSRegexp)
			if err != nil {
				return nil, err
			}
			aws.Cluster = &AWSClusterSpec{
				Region: matches[1],
				Name:   matches[2],
			}
		}
		if aws.RoleARN != "" || aws.STSEndpoint != "" || aws.Cluster != nil {
			auth.AWS = &aws
		}
	case ProviderAzure:
		var azure AzureSpec
		tenantID, tenantIDOk, err := extract(ann, AnnotationAzureTenantID, uuidRegexp)
		if err != nil {
			return nil, err
		}
		if credOptIn && !tenantIDOk {
			return nil, fmt.Errorf("annotation %s is required", AnnotationAzureTenantID)
		}
		clientID, clientIDOk, err := extract(ann, AnnotationAzureClientID, uuidRegexp)
		if err != nil {
			return nil, err
		}
		if credOptIn && !clientIDOk {
			return nil, fmt.Errorf("annotation %s is required", AnnotationAzureClientID)
		}
		azure.TenantID = tenantID
		azure.ClientID = clientID
		if o.kubeConfigCluster != "" {
			matches, err := parseCluster(o.kubeConfigCluster, clusterAzureRegexp)
			if err != nil {
				return nil, err
			}
			azure.Cluster = &AzureClusterSpec{
				SubscriptionID: matches[1],
				ResourceGroup:  matches[2],
				Name:           matches[3],
			}
		}
		if azure.TenantID != "" || azure.ClientID != "" || azure.Cluster != nil {
			auth.Azure = &azure
		}
	case ProviderGCP:
		var gcp GCPSpec
		wip, _, err := extract(ann, AnnotationGCPWorkloadIdentityProvider, gcpWIPRegexp)
		if err != nil {
			return nil, err
		}
		gcpSA, _, err := extract(ann, AnnotationGCPServiceAccount, gcpServiceAccountRegexp)
		if err != nil {
			return nil, err
		}
		// Without a workload identity provider, the GKE built-in workload
		// identity provider is used. It manages the OIDC token audience itself
		// and rejects OIDC tokens with a custom audience, so credential
		// audiences are not supported.
		if wip == "" && auth.Credential != nil && len(auth.Credential.Audiences) > 0 {
			return nil, fmt.Errorf("audiences are not supported by the %s service provider without annotation %s: "+
				"the GKE built-in workload identity provider rejects OIDC tokens with a custom audience",
				ProviderGCP, AnnotationGCPWorkloadIdentityProvider)
		}
		// The GKE built-in workload identity provider expects OIDC tokens issued
		// through the Kubernetes TokenRequest API, so it can only be used with
		// the kubernetes credential provider. Any other credential provider,
		// e.g. a SPIFFE JWT-SVID issued by a SPIFFE trust domain, cannot be
		// validated by the GKE built-in provider and requires a user-created
		// workload identity provider.
		if wip == "" && credProvider != CredentialProviderKubernetes {
			return nil, fmt.Errorf("annotation %s is required for the %s service provider with the %s credential provider: "+
				"the GKE built-in workload identity provider is only supported with the %s credential provider",
				AnnotationGCPWorkloadIdentityProvider, ProviderGCP, credProvider, CredentialProviderKubernetes)
		}
		gcp.WorkloadIdentityProvider = wip
		gcp.ServiceAccount = gcpSA
		if o.kubeConfigCluster != "" {
			if _, err := parseCluster(o.kubeConfigCluster, clusterGCPRegexp); err != nil {
				return nil, err
			}
		}
		if gcp.WorkloadIdentityProvider != "" || gcp.ServiceAccount != "" {
			auth.GCP = &gcp
		}
	case ProviderDockerHub:
		var dockerHub DockerHubSpec
		connectionID, ok, err := extract(ann, AnnotationDockerHubOIDCConnectionID, uuidRegexp)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("annotation %s is required", AnnotationDockerHubOIDCConnectionID)
		}
		dockerHub.OIDCConnectionID = connectionID
		auth.DockerHub = &dockerHub
	}

	if auth.Credential != nil || auth.TransitEngine != nil ||
		auth.AWS != nil || auth.Azure != nil || auth.GCP != nil || auth.DockerHub != nil {
		spec.Auth = &auth
	}

	return spec, nil
}
