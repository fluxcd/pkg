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

package utils

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crypto "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
)

// ServiceAccountOptions resolves the object-level workload identity
// ServiceAccount referenced by ref and returns the auth options derived from
// it. The ServiceAccount is fetched with ctrlClient and validated with the
// github.com/fluxcd/pkg/apis/crypto package for the given service provider.
//
// The ServiceAccount name in ref takes precedence; when empty, the default set
// with WithDefaultServiceAccount is used. If no ServiceAccount is resolved, it
// returns nil. If a ServiceAccount is resolved but object-level workload
// identity is disabled, auth.ErrObjectLevelWorkloadIdentityNotEnabled is
// returned.
//
// Use it to prepare auth options for functions that take auth.Option directly,
// like the cloud provider credential constructors:
//
//	saOpts, err := authutils.ServiceAccountOptions(ctx, ctrlClient, ref, aws.ProviderName,
//		authutils.WithDefaultServiceAccount(defaultSA),
//		authutils.WithObjectLevelWorkloadIdentity(gate))
//	opts = append(opts, saOpts...)
func ServiceAccountOptions(ctx context.Context, ctrlClient client.Client,
	ref client.ObjectKey, serviceProvider string, opts ...Option) ([]auth.Option, error) {

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	name := ref.Name
	if name == "" {
		name = o.defaultServiceAccount
	}
	if name == "" {
		return nil, nil
	}

	return getServiceAccountOptions(ctx, ctrlClient, ref.Namespace, name, serviceProvider,
		o.objectLevelWorkloadIdentity, o.cryptoOpts...)
}

// getServiceAccountOptions fetches the ServiceAccount identified by name in
// namespace, validates the configuration for the given service provider with
// the github.com/fluxcd/pkg/apis/crypto package and returns the auth options
// derived from the result, including auth.WithServiceAccount.
//
// It returns auth.ErrObjectLevelWorkloadIdentityNotEnabled if
// objectLevelWorkloadIdentity is false.
func getServiceAccountOptions(ctx context.Context, ctrlClient client.Client,
	namespace, name, serviceProvider string, objectLevelWorkloadIdentity bool,
	cryptoOpts ...crypto.Option) ([]auth.Option, error) {

	if !objectLevelWorkloadIdentity {
		return nil, auth.ErrObjectLevelWorkloadIdentityNotEnabled
	}

	var sa corev1.ServiceAccount
	if err := ctrlClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &sa); err != nil {
		return nil, fmt.Errorf("failed to get service account '%s/%s': %w", namespace, name, err)
	}

	validateOpts := append([]crypto.Option{
		crypto.WithAnnotations(sa.Annotations),
		crypto.WithServiceProvider(serviceProvider),
	}, cryptoOpts...)
	cfg, err := crypto.ValidateConfig(validateOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration for service account '%s/%s': %w",
			namespace, name, err)
	}

	opts := auth.OptionsFromConfig(cfg)
	opts = append(opts, auth.WithServiceAccount(&sa))
	return opts, nil
}

// resolveServiceAccount resolves the object-level workload identity
// ServiceAccount referenced by the auth options in o and returns the auth
// options derived from it. It returns nil if no ServiceAccount is configured,
// i.e. when the ServiceAccount name is empty and no default is set.
//
// The ServiceAccount name is taken from auth.WithServiceAccountName, falling
// back to the default ServiceAccount set with WithDefaultServiceAccount. The
// namespace is taken from auth.WithObjectReference and the client from
// auth.WithClient.
func resolveServiceAccount(ctx context.Context, serviceProvider string, o options,
	cryptoOpts ...crypto.Option) ([]auth.Option, error) {

	var ao auth.Options
	ao.Apply(o.authOpts...)

	name := ao.ServiceAccountName
	if name == "" {
		name = o.defaultServiceAccount
	}
	if name == "" {
		return nil, nil
	}

	if ao.Client == nil {
		return nil, errors.New("client is required to resolve the object-level ServiceAccount")
	}
	if ao.ObjectReference.Namespace == "" {
		return nil, errors.New("object reference namespace is required to resolve the object-level ServiceAccount")
	}

	return getServiceAccountOptions(ctx, ao.Client, ao.ObjectReference.Namespace, name,
		serviceProvider, o.objectLevelWorkloadIdentity, append(cryptoOpts, o.cryptoOpts...)...)
}
