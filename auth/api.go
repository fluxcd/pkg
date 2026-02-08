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
	"encoding/json"
	"fmt"
)

const (
	// APIGroup is the API group for auth-related APIs, e.g. annotations on ServiceAccounts.
	APIGroup = "auth.fluxcd.io"
)

// ImpersonationAnnotation is the annotation key that should be used in the
// ServiceAccount to specify the impersonation configuration.
func ImpersonationAnnotation(provider ProviderWithImpersonation) string {
	return fmt.Sprintf("%s.%s/%s",
		provider.GetName(),
		APIGroup,
		provider.GetImpersonationAnnotationKey())
}

// Impersonation contains the cloud provider identity that should be impersonated
// after acquiring an initial cloud provider access token.
type Impersonation struct {
	// Identity contains the marshaled JSON text that was unmarshaled for creating
	// an instance of this struct. The content of this field is then unmarshaled by
	// a provider into an Identity value for expressing an identity. This allows
	// the provider-specific fields extracted by the provider to be included in the
	// same JSON text object as the provider-agnostic fields defined in this struct.
	Identity json.RawMessage `json:"-"`

	// UseServiceAccount indicates whether OIDC exchange using a token issued for the
	// ServiceAccount should be used to get the initial cloud provider access token,
	// before impersonating Identity. This field exists to support providers that do
	// not require an initial identity to get the initial access token, e.g. GCP does
	// not require the Kubernetes ServiceAccount to be associated with a GCP service
	// account. Because of this property, it's not possible to decide whether to use
	// the ServiceAccount for the initial token exchange or not by just looking at
	// the presence of provider-specific annotations on the ServiceAccount indicating
	// the initial identity. In other words, any combinations of this field and the
	// ServiceAccount annotation for the initial identity are valid for GCP. Setting
	// this field to false on multi-tenancy lockdown is not a valid configuration and
	// will result in an error. When multi-tenancy lockdown is not enabled, the default
	// value of this field is false, which means that the initial cloud provider access
	// token will be retrieved from the environment of the controller pod, e.g. from
	// files mounted in the pod, environment variables, local metadata services, etc.
	// When multi-tenancy lockdown is enabled, the default value of this field is true,
	// which means that the ServiceAccount token will be used for the initial token
	// exchange.
	UseServiceAccount *bool `json:"useServiceAccount,omitempty"`
}

// UnmarshalJSON implements the json.Unmarshaler interface to allow the custom unmarshaling
// logic for the Impersonation struct as described in the field comments.
func (i *Impersonation) UnmarshalJSON(data []byte) error {
	var aux struct {
		UseServiceAccount *bool `json:"useServiceAccount,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	i.Identity = json.RawMessage(data)
	i.UseServiceAccount = aux.UseServiceAccount
	return nil
}
