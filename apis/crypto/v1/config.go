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

// Config is the fully validated configuration extracted from the Flux object
// context and, when object-level workload identity is configured, the
// ServiceAccount's annotations.
type Config struct {
	// Credential acquisition.
	Credential *CredentialConfig

	// Transit engine (SOPS).
	TransitEngine *TransitEngineConfig

	// Service-provider-specific configuration.

	// AWS is the AWS configuration, when the service provider is `aws`.
	AWS *AWSConfig
	// Azure is the Azure configuration, when the service provider is `azure`.
	Azure *AzureConfig
	// GCP is the GCP configuration, when the service provider is `gcp`.
	GCP *GCPConfig
	// DockerHub is the Docker Hub configuration, when the service provider is
	// `dockerhub`.
	DockerHub *DockerHubConfig

	// mTLS.
	MTLS *MTLSConfig
}

// MTLSConfig is the validated mTLS configuration.
type MTLSConfig struct {
	// SPIFFE is the SPIFFE mTLS configuration.
	SPIFFE *MTLSSPIFFEConfig
}

// MTLSSPIFFEConfig is the validated SPIFFE mTLS configuration.
type MTLSSPIFFEConfig struct {
	// ServerID is the SPIFFE ID of the server.
	ServerID SPIFFEPeerAuthorization
}

// CredentialConfig is the validated credential acquisition configuration.
type CredentialConfig struct {
	// Provider is the credential provider, one of `kubernetes` or `spiffe`.
	Provider string
	// Type is the credential type, one of `jwt` or `x509`.
	Type string
	// Expiration is the requested credential expiration.
	Expiration time.Duration
	// Audiences are the target audiences of the credential.
	Audiences []string
	// Username is the optional username to authenticate with the service.
	Username string
}

// TransitEngineConfig is the validated transit engine configuration.
type TransitEngineConfig struct {
	// LoginPath is the HTTP path in the transit engine where the request for
	// login must be sent.
	LoginPath string
	// Role is the role to use when authenticating against the transit engine.
	Role string
}

// AWSConfig is the validated service-provider-specific configuration for AWS.
type AWSConfig struct {
	// RoleARN is the AWS IAM Role ARN.
	RoleARN string
	// STSEndpoint is the resolved AWS STS endpoint.
	STSEndpoint string
	// Cluster is the parsed kubeconfig cluster resource name.
	Cluster *AWSClusterConfig
}

// AWSClusterConfig is the parsed kubeconfig cluster resource name for AWS.
type AWSClusterConfig struct {
	// Region is the AWS region.
	Region string
	// Name is the cluster name.
	Name string
}

// AzureConfig is the validated service-provider-specific configuration for
// Azure.
type AzureConfig struct {
	// TenantID is the Microsoft Entra tenant ID.
	TenantID string
	// ClientID is the Microsoft Entra client ID.
	ClientID string
	// Cluster is the parsed kubeconfig cluster resource name.
	Cluster *AzureClusterConfig
}

// AzureClusterConfig is the parsed kubeconfig cluster resource name for Azure.
type AzureClusterConfig struct {
	// SubscriptionID is the subscription ID.
	SubscriptionID string
	// ResourceGroup is the resource group.
	ResourceGroup string
	// Name is the cluster name.
	Name string
}

// GCPConfig is the validated service-provider-specific configuration for GCP.
type GCPConfig struct {
	// WorkloadIdentityProvider is the GCP Workload Identity Provider.
	WorkloadIdentityProvider string
	// ServiceAccount is the GCP Service Account email.
	ServiceAccount string
}

// DockerHubConfig is the validated service-provider-specific configuration for
// Docker Hub.
type DockerHubConfig struct {
	// OIDCConnectionID is the Docker Hub OIDC connection ID.
	OIDCConnectionID string
}

// Option is a functional option for ValidateConfig. The options struct
// and its fields are intentionally unexported.
type Option func(*options)

// options holds the validation inputs, which are intentionally unexported.
type options struct {
	annotations            map[string]string
	serviceProvider        string
	kubeConfigCluster      string
	kubeConfigAudiences    string
	stsProvider            string
	stsEndpoint            string
	oci                    bool
	githubEnterpriseServer bool
	sopsVaultConfigMap     bool
	certSecretRef          bool
	secretRefCerts         bool
	imagePullSecrets       bool
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

// WithSTSEndpoint sets the legacy `Bucket.spec.sts.provider` and
// `Bucket.spec.sts.endpoint` values, when present.
func WithSTSEndpoint(provider, endpoint string) Option {
	return func(o *options) {
		o.stsProvider = provider
		o.stsEndpoint = endpoint
	}
}

// WithOCI indicates whether the Flux object talks to an OCI registry.
func WithOCI(set bool) Option {
	return func(o *options) {
		o.oci = set
	}
}

// WithGitHubEnterpriseServer indicates that the `github` service provider is a
// GitHub Enterprise Server rather than GitHub.com. It has no effect for any
// other service provider.
func WithGitHubEnterpriseServer(set bool) Option {
	return func(o *options) {
		o.githubEnterpriseServer = set
	}
}

// WithSOPSVaultConfigMap indicates whether the legacy kustomize-controller
// flag `--sops-vault-configmap` is set.
func WithSOPSVaultConfigMap(set bool) Option {
	return func(o *options) {
		o.sopsVaultConfigMap = set
	}
}

// WithCertSecretRef indicates whether `.spec.certSecretRef` is set.
func WithCertSecretRef(set bool) Option {
	return func(o *options) {
		o.certSecretRef = set
	}
}

// WithSecretRefCerts indicates whether certificates or CAs are provided by
// `.spec.secretRef`.
func WithSecretRefCerts(set bool) Option {
	return func(o *options) {
		o.secretRefCerts = set
	}
}

// WithImagePullSecrets indicates whether the ServiceAccount has
// `.imagePullSecrets`.
func WithImagePullSecrets(set bool) Option {
	return func(o *options) {
		o.imagePullSecrets = set
	}
}

// ValidateConfig applies all the API validation rules to the given inputs and
// returns the fully parsed and validated configuration. It must be called even
// when no ServiceAccount is configured, as some inputs (e.g. the kubeconfig
// ConfigMap cluster resource) are validated and parsed regardless.
//
// Breaking changes to this function and its result are allowed until it's GA.
func ValidateConfig(opts ...Option) (*Config, error) {
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
	if o.serviceProvider == ProviderAzure && credType == CredentialTypeX509 {
		return nil, fmt.Errorf("credential type %s is not supported by the %s service provider",
			credType, o.serviceProvider)
	}
	credOptIn := providerOk
	if !credOptIn {
		credProvider = CredentialProviderKubernetes
		credType = CredentialTypeJWT
	}
	cfg := &Config{
		Credential: &CredentialConfig{
			Provider: credProvider,
			Type:     credType,
		},
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
	if serverID, ok := ann[AnnotationMTLSSPIFFEServerID]; ok {
		peer, err := ParseSPIFFEID(serverID)
		if err != nil {
			return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: %w",
				AnnotationMTLSSPIFFEServerID, err)
		}
		if _, ok := findAuthAnnotation(ann); ok && !credOptIn {
			return nil, fmt.Errorf("annotations %s and %s must be set together with any authentication annotation when %s is used",
				AnnotationCredentialProvider, AnnotationCredentialType, AnnotationMTLSSPIFFEServerID)
		}
		if o.certSecretRef {
			return nil, fmt.Errorf("annotation %s is not allowed with .spec.certSecretRef",
				AnnotationMTLSSPIFFEServerID)
		}
		if o.secretRefCerts {
			return nil, fmt.Errorf("annotation %s is not allowed with certificates or CAs from .spec.secretRef",
				AnnotationMTLSSPIFFEServerID)
		}
		switch o.serviceProvider {
		case ProviderAWS, ProviderAzure, ProviderGCP, ProviderDockerHub:
			return nil, fmt.Errorf("annotation %s is not allowed with service provider %s",
				AnnotationMTLSSPIFFEServerID, o.serviceProvider)
		case ProviderGitHub:
			if !o.githubEnterpriseServer {
				return nil, fmt.Errorf("annotation %s is not allowed with service provider %s",
					AnnotationMTLSSPIFFEServerID, o.serviceProvider)
			}
		}
		cfg.MTLS = &MTLSConfig{
			SPIFFE: &MTLSSPIFFEConfig{
				ServerID: *peer,
			},
		}
	}

	// The new OCI service providers require opting into object-level workload
	// identity, except when the `generic` provider uses `.imagePullSecrets`.
	genericStatic := o.oci && o.imagePullSecrets && o.serviceProvider == ProviderGeneric
	if o.oci && (o.serviceProvider == ProviderGeneric || o.serviceProvider == ProviderDockerHub) &&
		!genericStatic && !credOptIn {
		return nil, fmt.Errorf("service provider %s requires annotations %s and %s",
			o.serviceProvider, AnnotationCredentialProvider, AnnotationCredentialType)
	}

	// Conflicts with legacy configuration.
	if newAnn, ok := findNewAnnotation(ann); ok {
		switch {
		case o.kubeConfigAudiences != "":
			return nil, fmt.Errorf("annotation %s is not allowed with the legacy audiences ConfigMap key", newAnn)
		case o.sopsVaultConfigMap:
			return nil, fmt.Errorf("annotation %s is not allowed with the legacy --sops-vault-configmap flag", newAnn)
		case o.stsProvider == ProviderAWS:
			return nil, fmt.Errorf("annotation %s is not allowed with the legacy Bucket.spec.sts.provider set to aws", newAnn)
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
	cfg.TransitEngine = &TransitEngineConfig{
		LoginPath: loginPath,
		Role:      role,
	}

	// Credential expiration.
	if value, ok := ann[AnnotationCredentialExpiration]; ok {
		expiration, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s': %w",
				AnnotationCredentialExpiration, value, err)
		}
		if expiration <= 0 {
			return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s'. duration must be positive",
				AnnotationCredentialExpiration, value)
		}
		if credProvider != CredentialProviderKubernetes {
			return nil, fmt.Errorf("annotation %s is only supported by the %s credential provider",
				AnnotationCredentialExpiration, CredentialProviderKubernetes)
		}
		cfg.Credential.Expiration = expiration
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
		cfg.Credential.Audiences = splitAudiences(value)
	} else if o.kubeConfigAudiences != "" {
		cfg.Credential.Audiences = splitAudiences(o.kubeConfigAudiences)
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
	cfg.Credential.Username = username

	// Service-provider-specific configuration.
	switch o.serviceProvider {
	case ProviderAWS:
		cfg.AWS = &AWSConfig{}
		roleARN, roleARNok, err := extract(ann, AnnotationEKSRoleARN, eksRoleARNRegexp)
		if err != nil {
			return nil, err
		}
		if credOptIn && !roleARNok {
			return nil, fmt.Errorf("annotation %s is required", AnnotationEKSRoleARN)
		}
		cfg.AWS.RoleARN = roleARN
		if o.stsEndpoint != "" {
			cfg.AWS.STSEndpoint = o.stsEndpoint
		} else if value, ok := ann[AnnotationAWSSTSEndpoint]; ok {
			if !awsSTSEndpointRegexp.MatchString(value) {
				return nil, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s'. must match %s",
					AnnotationAWSSTSEndpoint, value, awsSTSEndpointRegexp)
			}
			cfg.AWS.STSEndpoint = value
		}
		if o.kubeConfigCluster != "" {
			matches, err := parseCluster(o.kubeConfigCluster, clusterAWSRegexp)
			if err != nil {
				return nil, err
			}
			cfg.AWS.Cluster = &AWSClusterConfig{
				Region: matches[1],
				Name:   matches[2],
			}
		}
	case ProviderAzure:
		cfg.Azure = &AzureConfig{}
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
		cfg.Azure.TenantID = tenantID
		cfg.Azure.ClientID = clientID
		if o.kubeConfigCluster != "" {
			matches, err := parseCluster(o.kubeConfigCluster, clusterAzureRegexp)
			if err != nil {
				return nil, err
			}
			cfg.Azure.Cluster = &AzureClusterConfig{
				SubscriptionID: matches[1],
				ResourceGroup:  matches[2],
				Name:           matches[3],
			}
		}
	case ProviderGCP:
		cfg.GCP = &GCPConfig{}
		wip, _, err := extract(ann, AnnotationGCPWorkloadIdentityProvider, gcpWIPRegexp)
		if err != nil {
			return nil, err
		}
		gcpSA, _, err := extract(ann, AnnotationGCPServiceAccount, gcpServiceAccountRegexp)
		if err != nil {
			return nil, err
		}
		cfg.GCP.WorkloadIdentityProvider = wip
		cfg.GCP.ServiceAccount = gcpSA
		if o.kubeConfigCluster != "" {
			if _, err := parseCluster(o.kubeConfigCluster, clusterGCPRegexp); err != nil {
				return nil, err
			}
		}
	case ProviderDockerHub:
		cfg.DockerHub = &DockerHubConfig{}
		connectionID, ok, err := extract(ann, AnnotationDockerHubOIDCConnectionID, uuidRegexp)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("annotation %s is required", AnnotationDockerHubOIDCConnectionID)
		}
		cfg.DockerHub.OIDCConnectionID = connectionID
	}

	return cfg, nil
}
