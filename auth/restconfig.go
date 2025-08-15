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
	"net/url"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/cache"
)

// RESTConfigProvider is an interface that defines methods for retrieving
// REST configurations for Kubernetes clusters from cloud providers.
type RESTConfigProvider interface {
	Provider

	// GetAccessTokenOptionsForCluster returns the options that must be
	// passed to the provider to retrieve access tokens for a cluster.
	// More than one access token may be required depending on the
	// provider, with different options (e.g. scope). Hence the return
	// type is a slice of []Option.
	GetAccessTokenOptionsForCluster(opts ...Option) ([][]Option, error)

	// NewRESTConfig returns a new RESTConfig that can be used to authenticate
	// with the Kubernetes API server. The access tokens are used for looking
	// up connection details like the API server address and CA certificate
	// data, and for accessing the cluster API server itself via the IAM
	// system of the cloud provider. If it's just a single token or multiple,
	// it depends on the provider.
	NewRESTConfig(ctx context.Context, accessTokens []Token, opts ...Option) (*RESTConfig, error)
}

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

// GetRESTConfig retrieves the authentication and connection
// details to a remote Kubernetes cluster for the given provider,
// cluster resource name and options.
func GetRESTConfig(ctx context.Context, provider RESTConfigProvider, opts ...Option) (*RESTConfig, error) {

	var o Options
	o.Apply(opts...)

	// First, we need the access tokens. They cannot be retrieved inside the
	// cache lock, otherwise we reach a deadlock.
	accessTokenOpts, err := provider.GetAccessTokenOptionsForCluster(opts...)
	if err != nil {
		return nil, err
	}
	accessTokens := make([]Token, 0, len(accessTokenOpts))
	for i := range accessTokenOpts {
		accessTokenOpts[i] = append(slices.Clone(opts), accessTokenOpts[i]...)
		token, err := GetAccessToken(ctx, provider, accessTokenOpts[i]...)
		if err != nil {
			return nil, fmt.Errorf("failed to get access token for cluster: %w", err)
		}
		accessTokens = append(accessTokens, token)
	}

	// Prepare a function to create the restconfig if needed.
	newRESTConfig := func() (*RESTConfig, error) {
		conf, err := provider.NewRESTConfig(ctx, accessTokens, opts...)
		if err != nil {
			return nil, err
		}
		return conf, nil
	}

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newRESTConfig()
	}

	// Build cache key.
	var serviceAccount *corev1.ServiceAccount
	var providerIdentity string
	var audiences []string
	if o.ShouldGetServiceAccountToken() {
		var err error
		saRef := client.ObjectKey{
			Name:      o.ServiceAccountName,
			Namespace: o.ServiceAccountNamespace,
		}
		serviceAccount, audiences, providerIdentity, err =
			getServiceAccountAndProviderInfo(ctx, provider, o.Client, saRef, opts...)
		if err != nil {
			return nil, err
		}
	}
	var cacheKeyParts []string
	for i, atOpts := range accessTokenOpts {
		key := buildAccessTokenCacheKey(provider, audiences, providerIdentity, serviceAccount, atOpts...)
		cacheKeyParts = append(cacheKeyParts, fmt.Sprintf("accessToken%dCacheKey=%s", i, key))
	}
	if c := o.ClusterResource; c != "" {
		cacheKeyParts = append(cacheKeyParts, fmt.Sprintf("cluster=%s", c))
	}
	if a := o.ClusterAddress; a != "" {
		cacheKeyParts = append(cacheKeyParts, fmt.Sprintf("address=%s", a))
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
