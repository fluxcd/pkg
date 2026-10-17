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
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crypto "github.com/fluxcd/pkg/apis/crypto/v1"
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
// needing to know about the option types. Usage example:
//
//	provider := authutils.GetRESTConfigFetcher(opts...)
//	impersonatorOpts = append(impersonatorOpts,
//		runtimeclient.WithKubeConfig(ref, kubeConfOpts, namespace, provider))
//
// Controllers that don't use the runtime/client.(*Impersonator) can simply call
// GetRESTConfig directly, passing the options as variadic arguments.
func GetRESTConfigFetcher(opts ...Option) RESTConfigFetcher {
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
// It reads the kubeconfig ConfigMap and validates the configuration with the
// github.com/fluxcd/pkg/apis/crypto package. When a ServiceAccount is
// configured (either in the ConfigMap or via WithDefaultServiceAccount), it is
// fetched and validated with its annotations; otherwise only the ConfigMap
// inputs (e.g. the cluster resource) are validated.
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
	opts ...Option) (*rest.Config, error) {

	var o options
	for _, opt := range opts {
		opt(&o)
	}

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
	providerName := cm.Data[meta.KubeConfigKeyProvider]
	provider, err := ProviderByName[auth.RESTConfigProvider](providerName)
	if err != nil {
		return nil, err
	}

	// Resolve the object-level ServiceAccount, if any.
	saName := cm.Data[meta.KubeConfigKeyServiceAccountName]
	if saName == "" {
		saName = o.defaultServiceAccount
	}
	var sa *corev1.ServiceAccount
	if saName != "" {
		if !o.objectLevelWorkloadIdentity {
			return nil, auth.ErrObjectLevelWorkloadIdentityNotEnabled
		}
		sa = &corev1.ServiceAccount{}
		if err := ctrlClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: namespace}, sa); err != nil {
			return nil, fmt.Errorf("failed to get service account '%s/%s': %w", namespace, saName, err)
		}
	}

	// Validate the configuration. This is done even when no ServiceAccount is
	// configured, as the kubeconfig ConfigMap cluster resource is validated and
	// parsed regardless.
	var annotations map[string]string
	if sa != nil {
		annotations = sa.Annotations
	}
	cfg, err := crypto.ValidateConfig(
		append([]crypto.Option{
			crypto.WithAnnotations(annotations),
			crypto.WithServiceProvider(providerName),
			crypto.WithKubeConfigCluster(cm.Data[meta.KubeConfigKeyCluster]),
			crypto.WithKubeConfigAudiences(cm.Data[meta.KubeConfigKeyAudiences]),
		}, o.cryptoOpts...)...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	// Configure options.
	authOpts := slices.Clone(o.authOpts)
	authOpts = append(authOpts, auth.WithClient(ctrlClient))
	authOpts = append(authOpts, auth.OptionsFromConfig(cfg)...)
	if sa != nil {
		authOpts = append(authOpts, auth.WithServiceAccount(sa))
	}
	if c := cm.Data[meta.KubeConfigKeyCluster]; c != "" {
		authOpts = append(authOpts, auth.WithClusterResource(c))
	}
	if a := cm.Data[meta.KubeConfigKeyAddress]; a != "" {
		authOpts = append(authOpts, auth.WithClusterAddress(a))
	}
	if ca := cm.Data[meta.KubeConfigKeyCACert]; ca != "" {
		authOpts = append(authOpts, auth.WithCAData(ca))
	}

	// Ensure the object reference carries the namespace.
	var current auth.Options
	current.Apply(authOpts...)
	ref := current.ObjectReference
	ref.Namespace = namespace
	authOpts = append(authOpts, auth.WithObjectReference(ref))

	conf, err := auth.GetRESTConfig(ctx, provider, authOpts...)
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
			opts:     authOpts,
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
