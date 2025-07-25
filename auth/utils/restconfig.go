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

package utils

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
)

// RESTConfigFetcher is a function that retrieves a *rest.Config for a given
// meta.KubeConfigReference, a namespace, and a controller-runtime client.
type RESTConfigFetcher func(ctx context.Context, ref meta.KubeConfigReference,
	namespace string, ctrlClient client.Client) (*rest.Config, error)

// GetRESTConfigFetcher is a convenience function for controllers that use the
// runtime/client.(*Impersonator) to create controller-runtime clients. To keep
// runtime decoupled from auth, this function closes over the controller-provided
// options and returns a function that can be called by runtime without runtime
// needing to know about the type auth.Option. Usage example:
//
//	provider := authutils.GetRESTConfigFetcher(opts...)
//	impersonatorOpts = append(impersonatorOpts,
//		runtimeclient.WithKubeConfig(ref, kubeConfOpts, namespace, provider))
//
// Controllers that don't use the runtime/client.(*Impersonator) can simply call
// GetRESTConfig directly, passing the options as variadic arguments.
func GetRESTConfigFetcher(opts ...auth.Option) RESTConfigFetcher {
	return func(ctx context.Context, ref meta.KubeConfigReference,
		namespace string, ctrlClient client.Client) (*rest.Config, error) {
		return GetRESTConfig(ctx, ref, namespace, ctrlClient, opts...)
	}
}

// GetRESTConfig retrieves a *rest.Config for the given meta.KubeConfigReference,
// namespace, controller-runtime client and options. It's a convenience wrapper
// for auth.GetRESTConfig so controllers can pass a meta.KubeConfigReference
// object directly without converting it to auth.Option(s).
//
// Additionally, the resulting *rest.Config will call auth.GetRESTConfig for every
// HTTP request to the remote cluster. This is needed for long-running operations
// that wait on resources until a potentially long timeout is reached, like kstatus
// health checks, and whatever Helm does. The timeout may be longer than a token's
// lifetime, so tokens can expire during such operations. auth.GetRESTConfig will
// create a fresh token if that happens.
//
// With the resulting *rest.Config, if a cache is not set in the options, a fresh
// token will be created for every HTTP request sent to the remote cluster.
func GetRESTConfig(ctx context.Context,
	kubeConfigRef meta.KubeConfigReference,
	namespace string, ctrlClient client.Client,
	opts ...auth.Option) (*rest.Config, error) {

	// Get ConfigMap.
	cmKey := client.ObjectKey{
		Name:      kubeConfigRef.ConfigMapRef.Name,
		Namespace: namespace,
	}
	var cm corev1.ConfigMap
	if err := ctrlClient.Get(ctx, cmKey, &cm); err != nil {
		return nil, fmt.Errorf("failed to get configmap %s: %w", cmKey.String(), err)
	}

	// Get provider by name.
	provider, err := ProviderByName[auth.RESTConfigProvider](cm.Data[meta.KubeConfigKeyProvider])
	if err != nil {
		return nil, err
	}

	// Configure options.
	if c, ok := cm.Data[meta.KubeConfigKeyCluster]; ok {
		opts = append(opts, auth.WithClusterResource(c))
	}
	if a, ok := cm.Data[meta.KubeConfigKeyAddress]; ok {
		opts = append(opts, auth.WithClusterAddress(a))
	}
	if ca, ok := cm.Data[meta.KubeConfigKeyCACert]; ok {
		opts = append(opts, auth.WithCAData(ca))
	}
	if name, ok := cm.Data[meta.KubeConfigKeyServiceAccountName]; ok {
		saKey := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		opts = append(opts, auth.WithServiceAccount(saKey, ctrlClient))
	} else {
		opts = append(opts, auth.WithClient(ctrlClient))
	}
	if a, ok := cm.Data[meta.KubeConfigKeyAudiences]; ok {
		var audiences []string
		for aud := range strings.SplitSeq(a, "\n") {
			aud = strings.TrimSpace(aud)
			if aud == "" {
				continue
			}
			audiences = append(audiences, aud)
		}
		opts = append(opts, auth.WithAudiences(audiences...))
	}

	conf, err := auth.GetRESTConfig(ctx, provider, opts...)
	if err != nil {
		return nil, err
	}

	// Build wrapped *rest.Config that will call
	// auth.GetRESTConfig for every HTTP request.
	restConfig := &rest.Config{
		Host:            conf.Host,
		TLSClientConfig: rest.TLSClientConfig{CAData: conf.CAData},
	}
	restConfig.Wrap(func(base http.RoundTripper) http.RoundTripper {
		return &restConfigRoundTripper{
			base:     base,
			provider: provider,
			opts:     opts,
		}
	})

	return restConfig, nil
}

// restConfigRoundTripper is an http.RoundTripper that wraps the base
// RoundTripper and retrieves a bearer token for the remote cluster
// using auth.GetRESTConfig before each HTTP request.
type restConfigRoundTripper struct {
	base     http.RoundTripper
	provider auth.RESTConfigProvider
	opts     []auth.Option
}

// RoundTrip implements http.RoundTripper.
func (r *restConfigRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	details, err := auth.GetRESTConfig(req.Context(), r.provider, r.opts...)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+details.BearerToken)
	return r.base.RoundTrip(req)
}
