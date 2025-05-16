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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/cache"
)

// RESTConfig is a particular type implementing the Token interface
// for Kubernetes REST configurations.
type RESTConfig struct {
	Host        string
	BearerToken string
	CAData      []byte
	ExpiresAt   time.Time
}

// GetDuration implements Token.
func (r *RESTConfig) GetDuration() time.Duration {
	return time.Until(r.ExpiresAt)
}

// ParseClusterAddress parses the given cluster address and returns
// the canonical form https://<lowercase(host)>:<port>.
func ParseClusterAddress(address string) (string, error) {
	if address == "" {
		return "", errors.New("empty address")
	}
	if !strings.HasPrefix(address, "http") {
		address = fmt.Sprintf("https://%s", address)
	}
	u, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("failed to parse Kubernetes API server address '%s': %w", address, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("the Kubernetes API server address '%s' must use https scheme", address)
	}
	host := u.Host
	if u.Port() == "" {
		host += ":443"
	}
	return fmt.Sprintf("https://%s", strings.ToLower(host)), nil
}

// GetRESTConfigDetails retrieves the authentication and connection
// details to a remote Kubernetes cluster for the given provider,
// cluster resource name and API server address.
func GetRESTConfigDetails(ctx context.Context, provider Provider, kubeConfigRef meta.KubeConfigReference,
	namespace string, ctrlClient client.Client, opts ...Option) (*RESTConfig, error) {

	// Configure service account.
	if name := kubeConfigRef.ServiceAccountName; name != "" {
		saKey := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		opts = append(opts, WithServiceAccount(saKey, ctrlClient))
	}

	// Configure address.
	var canonicalAddress string
	if a := kubeConfigRef.Address; a != "" {
		var err error
		canonicalAddress, err = ParseClusterAddress(a)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cluster address '%s': %w", a, err)
		}
		opts = append(opts, WithClusterAddress(canonicalAddress))
	}

	cluster := kubeConfigRef.Cluster

	// First, we need the access tokens. They cannot be retrieved inside the
	// cache lock, otherwise we reach a deadlock.
	accessTokenOpts, err := provider.GetAccessTokenOptionsForCluster(cluster)
	if err != nil {
		return nil, err
	}
	accessTokens := make([]Token, 0, len(accessTokenOpts))
	for i := range accessTokenOpts {
		accessTokenOpts[i] = append(opts, accessTokenOpts[i]...)
		token, err := GetAccessToken(ctx, provider, accessTokenOpts[i]...)
		if err != nil {
			return nil, fmt.Errorf("failed to get access token for cluster: %w", err)
		}
		accessTokens = append(accessTokens, token)
	}

	// Prepare a function to create the restconfig if needed.
	newRESTConfig := func() (*RESTConfig, error) {
		conf, err := provider.NewRESTConfig(ctx, cluster, accessTokens, opts...)
		if err != nil {
			return nil, err
		}
		return conf, nil
	}

	var o Options
	o.Apply(opts...)

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newRESTConfig()
	}

	// Build cache key.
	var serviceAccount *corev1.ServiceAccount
	var providerAudience string
	var providerIdentity string
	if o.ServiceAccount != nil {
		var err error
		serviceAccount, providerAudience, providerIdentity, err =
			getServiceAccountAndProviderInfo(ctx, provider, o.Client, *o.ServiceAccount)
		if err != nil {
			return nil, err
		}
	}
	var cacheKeyParts []string
	for _, atOpts := range accessTokenOpts {
		key := buildAccessTokenCacheKey(provider, providerAudience, providerIdentity, serviceAccount, atOpts...)
		cacheKeyParts = append(cacheKeyParts, fmt.Sprintf("accessTokenCacheKey=%s", key))
	}
	cacheKeyParts = append(cacheKeyParts, fmt.Sprintf("cluster=%s", cluster))
	if canonicalAddress != "" {
		cacheKeyParts = append(cacheKeyParts, fmt.Sprintf("address=%s", canonicalAddress))
	}
	cacheKey := buildCacheKey(cacheKeyParts...)

	// Build involved object details.
	cacheOpts := []cache.Options{cache.WithInvolvedObject(
		o.InvolvedObject.Kind,
		o.InvolvedObject.Name,
		o.InvolvedObject.Namespace,
		o.InvolvedObject.Operation)}

	// Get restconfig from cache.
	token, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newRESTConfig()
	}, cacheOpts...)
	if err != nil {
		return nil, err
	}

	return token.(*RESTConfig), nil
}

// GetRESTConfig retrieves a *rest.Config for the given
// provider, cluster resource name and API server address.
// It wraps GetRESTConfigDetails to provide a *rest.Config
// that will call GetRESTConfigDetails for every HTTP request
// to the remote cluster. This is needed for long-running
// operations that wait on resources until a potentially long
// timeout is reached, like kstatus or whatever Helm does. The
// timeout may be longer than a token's lifetime, so tokens can
// expire during such operations. GetRESTConfigDetails will
// create a fresh token if that happens. If a cache is not set
// in the options, GetRESTConfigDetails will create a new token
// for every HTTP request to the remote cluster.
func GetRESTConfig(ctx context.Context, provider Provider, kubeConfigRef meta.KubeConfigReference,
	namespace string, ctrlClient client.Client, opts ...Option) (*rest.Config, error) {

	details, err := GetRESTConfigDetails(ctx, provider, kubeConfigRef, namespace, ctrlClient, opts...)
	if err != nil {
		return nil, err
	}

	conf := &rest.Config{
		Host:            details.Host,
		TLSClientConfig: rest.TLSClientConfig{CAData: details.CAData},
	}

	conf.Wrap(func(base http.RoundTripper) http.RoundTripper {
		return &restConfigRoundTripper{
			base:          base,
			provider:      provider,
			kubeConfigRef: kubeConfigRef,
			namespace:     namespace,
			ctrlClient:    ctrlClient,
			opts:          opts,
		}
	})

	return conf, nil
}

// restConfigRoundTripper implements the logic described in GetRESTConfig.
type restConfigRoundTripper struct {
	base          http.RoundTripper
	provider      Provider
	kubeConfigRef meta.KubeConfigReference
	namespace     string
	ctrlClient    client.Client
	opts          []Option
}

// RoundTrip implements http.RoundTripper.
func (r *restConfigRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	details, err := GetRESTConfigDetails(req.Context(),
		r.provider, r.kubeConfigRef, r.namespace, r.ctrlClient, r.opts...)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+details.BearerToken)
	return r.base.RoundTrip(req)
}
