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

package auth

import (
	"net/http"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/cache"
)

// Option is a functional option for the auth package.
type Option func(*Options)

// Options contains options for configuring the behavior of the provider methods.
// Not all providers/methods support all options.
type Options struct {
	Client client.Client
	Cache  *cache.TokenCache

	// Object being reconciled.
	ObjectReference ObjectReference
	CacheOperation  string

	// ServiceAccount for object-level workload identity.
	ServiceAccount     *corev1.ServiceAccount
	ServiceAccountName string

	// Credential acquisition.
	CredentialProvider   string
	CredentialType       string
	CredentialExpiration time.Duration
	CredentialUsername   string
	CredentialAudiences  []string

	// Request configuration.
	Scopes        []string
	ProxyURL      *url.URL
	GitURL        *url.URL
	AllowShellOut bool

	// Remote Kubernetes cluster connection.
	ClusterResource string
	ClusterAddress  string
	CAData          string

	// AWS.
	AWSRoleARN     string
	AWSRegion      string
	AWSSTSEndpoint string
	AWSClusterName string

	// Azure.
	AzureTenantID       string
	AzureClientID       string
	AzureSubscriptionID string
	AzureResourceGroup  string
	AzureClusterName    string

	// GCP.
	GCPWorkloadIdentityProvider string
	GCPServiceAccount           string

	// Docker Hub.
	DockerHubOIDCConnectionID string
}

// ObjectReference identifies the Flux object being reconciled. It is used for
// cache entries and, later, for SPIFFE.
type ObjectReference struct {
	APIGroup  string
	Kind      string
	Name      string
	Namespace string
	UID       string
}

// InvolvedObject returns the cache involved object for the options.
func (o *Options) InvolvedObject() cache.InvolvedObject {
	return cache.InvolvedObject{
		Kind:      o.ObjectReference.Kind,
		Name:      o.ObjectReference.Name,
		Namespace: o.ObjectReference.Namespace,
		Operation: o.CacheOperation,
	}
}

// OptionsFromConfig returns the options derived from a validated configuration
// produced by github.com/fluxcd/pkg/apis/crypto.
func OptionsFromConfig(cfg *cryptov1.Config) []Option {
	if cfg == nil {
		return nil
	}
	var opts []Option
	if c := cfg.Credential; c != nil {
		if c.Provider != "" {
			opts = append(opts, WithCredentialProvider(c.Provider))
		}
		if c.Type != "" {
			opts = append(opts, WithCredentialType(c.Type))
		}
		if c.Expiration != 0 {
			opts = append(opts, WithCredentialExpiration(c.Expiration))
		}
		if c.Username != "" {
			opts = append(opts, WithCredentialUsername(c.Username))
		}
		if len(c.Audiences) > 0 {
			opts = append(opts, WithCredentialAudiences(c.Audiences...))
		}
	}
	if a := cfg.AWS; a != nil {
		if a.RoleARN != "" {
			opts = append(opts, WithAWSRoleARN(a.RoleARN))
		}
		if a.STSEndpoint != "" {
			opts = append(opts, WithAWSSTSEndpoint(a.STSEndpoint))
		}
		if a.Cluster != nil {
			if a.Cluster.Region != "" {
				opts = append(opts, WithAWSRegion(a.Cluster.Region))
			}
			if a.Cluster.Name != "" {
				opts = append(opts, WithAWSClusterName(a.Cluster.Name))
			}
		}
	}
	if a := cfg.Azure; a != nil {
		if a.TenantID != "" {
			opts = append(opts, WithAzureTenantID(a.TenantID))
		}
		if a.ClientID != "" {
			opts = append(opts, WithAzureClientID(a.ClientID))
		}
		if a.Cluster != nil {
			if a.Cluster.SubscriptionID != "" {
				opts = append(opts, WithAzureSubscriptionID(a.Cluster.SubscriptionID))
			}
			if a.Cluster.ResourceGroup != "" {
				opts = append(opts, WithAzureResourceGroup(a.Cluster.ResourceGroup))
			}
			if a.Cluster.Name != "" {
				opts = append(opts, WithAzureClusterName(a.Cluster.Name))
			}
		}
	}
	if g := cfg.GCP; g != nil {
		if g.WorkloadIdentityProvider != "" {
			opts = append(opts, WithGCPWorkloadIdentityProvider(g.WorkloadIdentityProvider))
		}
		if g.ServiceAccount != "" {
			opts = append(opts, WithGCPServiceAccount(g.ServiceAccount))
		}
	}
	if d := cfg.DockerHub; d != nil && d.OIDCConnectionID != "" {
		opts = append(opts, WithDockerHubOIDCConnectionID(d.OIDCConnectionID))
	}
	return opts
}

// ShouldGetServiceAccountToken returns true if a ServiceAccount token should be
// retrieved for object-level workload identity. The ServiceAccount is provided
// by the caller, either as an object or as a name/namespace reference.
func (o *Options) ShouldGetServiceAccountToken() bool {
	if o.ServiceAccount != nil {
		return true
	}
	return o.ObjectReference.Namespace != "" && o.ServiceAccountName != ""
}

// WithClient sets the controller-runtime client for the provider.
func WithClient(client client.Client) Option {
	return func(o *Options) {
		o.Client = client
	}
}

// WithServiceAccountName sets the ServiceAccount name for the token.
func WithServiceAccountName(name string) Option {
	return func(o *Options) {
		o.ServiceAccountName = name
	}
}

// WithServiceAccount sets the resolved ServiceAccount object for the token.
// When set, it takes precedence over the ServiceAccount name option.
func WithServiceAccount(serviceAccount *corev1.ServiceAccount) Option {
	return func(o *Options) {
		o.ServiceAccount = serviceAccount
	}
}

// WithCredentialProvider sets the credential provider, one of `kubernetes` or
// `spiffe`.
func WithCredentialProvider(provider string) Option {
	return func(o *Options) {
		o.CredentialProvider = provider
	}
}

// WithCredentialType sets the credential type, one of `jwt` or `x509`.
func WithCredentialType(credentialType string) Option {
	return func(o *Options) {
		o.CredentialType = credentialType
	}
}

// WithCredentialExpiration sets the requested credential expiration.
func WithCredentialExpiration(expiration time.Duration) Option {
	return func(o *Options) {
		o.CredentialExpiration = expiration
	}
}

// WithCredentialUsername sets the optional username to authenticate with the
// service.
func WithCredentialUsername(username string) Option {
	return func(o *Options) {
		o.CredentialUsername = username
	}
}

// WithAWSRoleARN sets the AWS IAM Role ARN.
func WithAWSRoleARN(roleARN string) Option {
	return func(o *Options) {
		o.AWSRoleARN = roleARN
	}
}

// WithAzureTenantID sets the Microsoft Entra tenant ID.
func WithAzureTenantID(tenantID string) Option {
	return func(o *Options) {
		o.AzureTenantID = tenantID
	}
}

// WithAzureClientID sets the Microsoft Entra client ID.
func WithAzureClientID(clientID string) Option {
	return func(o *Options) {
		o.AzureClientID = clientID
	}
}

// WithAzureSubscriptionID sets the Azure subscription ID.
func WithAzureSubscriptionID(subscriptionID string) Option {
	return func(o *Options) {
		o.AzureSubscriptionID = subscriptionID
	}
}

// WithAzureResourceGroup sets the Azure resource group.
func WithAzureResourceGroup(resourceGroup string) Option {
	return func(o *Options) {
		o.AzureResourceGroup = resourceGroup
	}
}

// WithAzureClusterName sets the Azure AKS cluster name.
func WithAzureClusterName(clusterName string) Option {
	return func(o *Options) {
		o.AzureClusterName = clusterName
	}
}

// WithGCPWorkloadIdentityProvider sets the GCP Workload Identity Provider.
func WithGCPWorkloadIdentityProvider(wip string) Option {
	return func(o *Options) {
		o.GCPWorkloadIdentityProvider = wip
	}
}

// WithGCPServiceAccount sets the GCP Service Account email.
func WithGCPServiceAccount(serviceAccount string) Option {
	return func(o *Options) {
		o.GCPServiceAccount = serviceAccount
	}
}

// WithDockerHubOIDCConnectionID sets the Docker Hub OIDC connection ID.
func WithDockerHubOIDCConnectionID(connectionID string) Option {
	return func(o *Options) {
		o.DockerHubOIDCConnectionID = connectionID
	}
}

// GetServiceAccount returns the ServiceAccount object for the token. If the
// object was not set explicitly, it is built from the configured name and
// namespace.
func (o *Options) GetServiceAccount() corev1.ServiceAccount {
	if o.ServiceAccount != nil {
		return *o.ServiceAccount
	}
	return corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.ServiceAccountName,
			Namespace: o.ObjectReference.Namespace,
		},
	}
}

// WithCache sets the token cache.
func WithCache(cache cache.TokenCache) Option {
	return func(o *Options) {
		o.Cache = &cache
	}
}

// WithObjectReference sets the reference to the Flux object being reconciled.
func WithObjectReference(ref ObjectReference) Option {
	return func(o *Options) {
		o.ObjectReference = ref
	}
}

// WithCacheOperation sets the operation performed on the Flux object, for cache
// entry purposes.
func WithCacheOperation(operation string) Option {
	return func(o *Options) {
		o.CacheOperation = operation
	}
}

// WithCredentialAudiences sets the audiences for the Kubernetes ServiceAccount token.
func WithCredentialAudiences(audiences ...string) Option {
	return func(o *Options) {
		o.CredentialAudiences = audiences
	}
}

// WithScopes sets the scopes for the token.
func WithScopes(scopes ...string) Option {
	return func(o *Options) {
		o.Scopes = scopes
	}
}

// WithAWSRegion sets the region for the AWS STS service.
func WithAWSRegion(stsRegion string) Option {
	return func(o *Options) {
		o.AWSRegion = stsRegion
	}
}

// WithAWSSTSEndpoint sets the endpoint for the AWS STS service.
func WithAWSSTSEndpoint(stsEndpoint string) Option {
	return func(o *Options) {
		o.AWSSTSEndpoint = stsEndpoint
	}
}

// WithAWSClusterName sets the AWS EKS cluster name.
func WithAWSClusterName(clusterName string) Option {
	return func(o *Options) {
		o.AWSClusterName = clusterName
	}
}

// WithProxyURL sets a *url.URL for an HTTP/S proxy for acquiring the token.
func WithProxyURL(proxyURL url.URL) Option {
	return func(o *Options) {
		o.ProxyURL = &proxyURL
	}
}

// WithGitURL sets the Git repository URL used by Git credential providers.
func WithGitURL(gitURL url.URL) Option {
	return func(o *Options) {
		o.GitURL = &gitURL
	}
}

// WithCAData sets the CA data for credentials that require a CA,
// e.g. for Kubernetes REST config.
func WithCAData(caData string) Option {
	return func(o *Options) {
		o.CAData = caData
	}
}

// WithClusterResource sets the cluster resource for creating a REST config.
// Must be the fully qualified name of the cluster resource in the cloud
// provider API.
func WithClusterResource(clusterResource string) Option {
	return func(o *Options) {
		o.ClusterResource = clusterResource
	}
}

// WithClusterAddress sets the cluster address for creating a REST config.
// This address is used to select the correct cluster endpoint and CA data
// when the provider has a list of endpoints to choose from, or to simply
// validate the address against the cluster resource when the provider
// returns a single endpoint. This is optional, providers returning a list
// of endpoints will select the first one if no address is provided.
func WithClusterAddress(clusterAddress string) Option {
	return func(o *Options) {
		o.ClusterAddress = clusterAddress
	}
}

// WithAllowShellOut allows the provider to shell out to binary tools
// for acquiring controller tokens. MUST be used only by the Flux CLI,
// i.e. in the github.com/fluxcd/flux2 Git repository.
func WithAllowShellOut() Option {
	return func(o *Options) {
		o.AllowShellOut = true
	}
}

// Apply applies the given slice of Option(s) to the Options struct.
func (o *Options) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// GetHTTPClient returns a *http.Client with appropriate timeouts and proxy settings.
// The client includes a 10-second timeout to prevent indefinite hangs during token acquisition.
func (o *Options) GetHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if o.ProxyURL != nil {
		transport.Proxy = http.ProxyURL(o.ProxyURL)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}
