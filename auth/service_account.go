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

package auth

import (
	"context"
	"fmt"

	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// CreateServiceAccountToken creates a ServiceAccount token for the
// given ServiceAccount reference and audiences.
func CreateServiceAccountToken(ctx context.Context, c client.Client,
	saRef client.ObjectKey, audiences ...string) (string, error) {
	obj := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saRef.Name,
			Namespace: saRef.Namespace,
		},
	}
	tokenReq := &authnv1.TokenRequest{
		Spec: authnv1.TokenRequestSpec{
			Audiences: audiences,
		},
	}
	if err := c.SubResource("token").Create(ctx, &obj, tokenReq); err != nil {
		return "", err
	}
	return tokenReq.Status.Token, nil
}

// serviceAccountInfo contains the parsed information of the ServiceAccount
// to be used for fetching the access token and generating the cache key when
// object-level workload identity is enabled.
type serviceAccountInfo struct {
	useServiceAccount                bool
	obj                              *corev1.ServiceAccount
	audiences                        []string
	providerIdentity                 string
	providerIdentityForImpersonation fmt.Stringer
}

// getServiceAccountInfo fetches the ServiceAccount and parses the necessary information for
// fetching the access token and generating the cache key when object-level workload identity
// is enabled.
func getServiceAccountInfo(ctx context.Context, provider Provider,
	client client.Client, opts ...Option) (*serviceAccountInfo, error) {

	var o Options
	o.Apply(opts...)

	key := types.NamespacedName{
		Name:      o.ServiceAccountName,
		Namespace: o.ServiceAccountNamespace,
	}

	// Apply multi-tenancy lockdown: use default service account when .serviceAccountName
	// is not explicitly specified in the object. This results in Object-Level Workload Identity.
	var setDefaultSA bool
	lockdownEnabled := o.DefaultServiceAccount != ""
	if key.Name == "" && lockdownEnabled {
		key.Name = o.DefaultServiceAccount
		setDefaultSA = true
	}

	// Get service account.
	var obj corev1.ServiceAccount
	if err := client.Get(ctx, key, &obj); err != nil {
		if errors.IsNotFound(err) && setDefaultSA {
			return nil, fmt.Errorf("failed to get service account '%s': %w",
				key, ErrDefaultServiceAccountNotFound)
		}
		return nil, fmt.Errorf("failed to get service account '%s': %w", key, err)
	}

	// If no impersonation configuration is found, default to using the ServiceAccount with the provider.
	useServiceAccount := true

	// Get provider identity for impersonation if supported by the provider.
	var providerIdentityForImpersonation fmt.Stringer
	if provider, ok := provider.(ProviderWithImpersonation); ok {
		annotationKey := ImpersonationAnnotation(provider)
		if impersonationYAML := obj.Annotations[annotationKey]; impersonationYAML != "" {
			// Parse the impersonation configuration from the annotation.
			var impersonation Impersonation
			if err := yaml.Unmarshal([]byte(impersonationYAML), &impersonation); err != nil {
				return nil, fmt.Errorf(
					"failed to parse impersonation annotation '%s' on service account '%s': %w",
					annotationKey, key, err)
			}
			var err error
			providerIdentityForImpersonation, err = provider.GetIdentityForImpersonation(impersonation.Identity)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to get provider identity for impersonation from service account '%s' annotation '%s': %w",
					key, annotationKey, err)
			}

			// If UseServiceAccount is not set, default to true if lockdown is enabled, and false otherwise.
			if impersonation.UseServiceAccount == nil {
				impersonation.UseServiceAccount = &lockdownEnabled
			}
			useServiceAccount = *impersonation.UseServiceAccount

			// If the user intention is to not use the ServiceAccount, but the cluster
			// administrator has enabled multi-tenancy lockdown, return an error.
			if !useServiceAccount && lockdownEnabled {
				return nil, fmt.Errorf("invalid impersonation configuration on service account '%s': "+
					"multi-tenancy lockdown is enabled, impersonation without service account is not allowed", key)
			}
		}
	}

	var audiences []string
	var providerIdentity string

	switch useServiceAccount {

	// If the user intention is to use the ServiceAccount,
	// get the required fields for the usage.
	case true:
		// Get provider audience.
		audiences = o.Audiences
		if len(audiences) == 0 {
			var err error
			audiences, err = provider.GetAudiences(ctx, obj)
			if err != nil {
				return nil, fmt.Errorf("failed to get provider audience: %w", err)
			}
		}

		// Get provider identity.
		var err error
		providerIdentity, err = provider.GetIdentity(obj)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get provider identity from service account '%s' annotations: %w", key, err)
		}

	// If the user does not want to use the ServiceAccount, error out if
	// unneeded annotations are present to avoid confusion.
	case false:
		// No need to check audiences, they may be needed for usage without a ServiceAccount.

		// Check provider identity.
		if id, err := provider.GetIdentity(obj); err == nil && id != "" {
			return nil, fmt.Errorf("invalid configuration on service account '%s': "+
				"identity annotation is present but the ServiceAccount is not used according "+
				"to the impersonation configuration", key)
		}
	}

	return &serviceAccountInfo{
		useServiceAccount:                useServiceAccount,
		obj:                              &obj,
		audiences:                        audiences,
		providerIdentity:                 providerIdentity,
		providerIdentityForImpersonation: providerIdentityForImpersonation,
	}, nil
}
