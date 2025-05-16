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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/fluxcd/pkg/cache"
)

// RESTConfig is a particular type implementing the Token interface
// for Kubernetes REST configurations.
type RESTConfig struct {
	*rest.Config
	ExpiresAt time.Time
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

// GetRESTConfig retrieves a RESTConfig for the given provider,
// cluster resource name and API server address.
func GetRESTConfig(ctx context.Context, provider Provider,
	cluster string, opts ...Option) (*RESTConfig, error) {

	var o Options
	o.Apply(opts...)

	// Convert address to canonical form.
	if a := o.ClusterAddress; a != "" {
		canonicalAddress, err := ParseClusterAddress(a)
		if err != nil {
			return nil, err
		}
		opts = append(opts, WithClusterAddress(canonicalAddress))
	}

	// Prepare a function to create the restconfig if needed.
	newRESTConfig := func() (*RESTConfig, error) {
		conf, err := provider.NewRESTConfig(ctx, cluster, opts...)
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
	accessTokenCacheKey := buildAccessTokenCacheKey(provider, providerAudience,
		providerIdentity, serviceAccount, opts...)
	cacheKey := buildCacheKey(
		fmt.Sprintf("accessTokenCacheKey=%s", accessTokenCacheKey),
		fmt.Sprintf("cluster=%s", cluster))

	// Build involved object details.
	kind := o.InvolvedObject.Kind
	name := o.InvolvedObject.Name
	namespace := o.InvolvedObject.Namespace
	operation := o.InvolvedObject.Operation

	// Get restconfig from cache.
	token, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newRESTConfig()
	}, cache.WithInvolvedObject(kind, name, namespace, operation))
	if err != nil {
		return nil, err
	}

	conf := token.(*RESTConfig)
	return &RESTConfig{
		Config:    rest.CopyConfig(conf.Config),
		ExpiresAt: conf.ExpiresAt,
	}, nil
}
