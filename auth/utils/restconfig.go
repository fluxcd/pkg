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

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
)

// GetRESTConfig retrieves a *rest.Config for the given meta.KubeConfigReference,
// namespace, controller-runtime client and options.
func GetRESTConfig(ctx context.Context,
	kubeConfigRef meta.KubeConfigReference,
	namespace string, ctrlClient client.Client,
	opts ...auth.Option) (*rest.Config, error) {

	provider, err := ProviderByName(kubeConfigRef.Provider)
	if err != nil {
		return nil, err
	}

	return auth.GetRESTConfig(ctx, provider, kubeConfigRef, namespace, ctrlClient, opts...)
}

// RESTConfigFetcher is a function that retrieves a *rest.Config for a given
// meta.KubeConfigReference, a namespace, and a controller-runtime client.
type RESTConfigFetcher func(ctx context.Context, ref meta.KubeConfigReference,
	namespace string, ctrlClient client.Client) (*rest.Config, error)

// GetRESTConfigFetcher is a convenience function for controllers that use the
// runtime.Impersonator to create controller-runtime clients. To keep runtime
// decoupled from auth, this function closes over the controller-provided
// options and returns a function that can be called by runtime without it
// needing to know about the auth package type auth.Option. Usage example:
//
//	provider := authutils.GetRESTConfigFetcher(opts...)
//	impersonatorOpts = append(impersonatorOpts,
//		runtimeclient.WithKubeConfig(ref, kubeConfOpts, namespace, provider))
//
// Controllers that don't use the runtime.Impersonator can simply call
// GetRESTConfig directly, passing the options as variadic arguments.
func GetRESTConfigFetcher(opts ...auth.Option) RESTConfigFetcher {
	return func(ctx context.Context, ref meta.KubeConfigReference,
		namespace string, ctrlClient client.Client) (*rest.Config, error) {
		return GetRESTConfig(ctx, ref, namespace, ctrlClient, opts...)
	}
}
