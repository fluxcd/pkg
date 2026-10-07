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

package kubernetes

import (
	"context"
	"fmt"

	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the Kubernetes credential provider.
const ProviderName = "kubernetes"

// Provider implements the auth.CredentialProvider, auth.JWTProvider and
// auth.X509Provider interfaces using Kubernetes ServiceAccount tokens and
// certificates.
type Provider struct{ Implementation }

// GetName implements auth.CredentialProvider.
func (Provider) GetName() string {
	return ProviderName
}

// NewJWT implements auth.JWTProvider. It issues a Kubernetes ServiceAccount
// token either for the object-level ServiceAccount (when one is configured
// through the options) or for the controller's own ServiceAccount otherwise.
func (p Provider) NewJWT(ctx context.Context, opts ...auth.Option) (*auth.JWT, error) {

	var o auth.Options
	o.Apply(opts...)

	if o.Client == nil {
		return nil, fmt.Errorf("client is required to create a Kubernetes token")
	}

	// Determine the ServiceAccount to issue the token for.
	serviceAccount, err := p.resolveServiceAccount(o)
	if err != nil {
		return nil, err
	}

	// Issue the token.
	tokenReq := &authnv1.TokenRequest{
		Spec: authnv1.TokenRequestSpec{
			Audiences: o.Audiences,
		},
	}
	if o.Credential != nil {
		tokenReq.Spec.ExpirationSeconds = o.Credential.ExpirationSeconds
	}
	if err := o.Client.SubResource("token").Create(ctx, &serviceAccount, tokenReq); err != nil {
		return nil, fmt.Errorf("failed to create kubernetes token for service account '%s': %w",
			client.ObjectKeyFromObject(&serviceAccount), err)
	}

	return &auth.JWT{
		Token:     tokenReq.Status.Token,
		ExpiresAt: tokenReq.Status.ExpirationTimestamp.Time,
	}, nil
}

// resolveServiceAccount determines the ServiceAccount to issue a credential
// for: the one configured through the options, or the controller's own
// ServiceAccount read from the token mounted in the pod when no object-level
// ServiceAccount is configured.
func (p Provider) resolveServiceAccount(o auth.Options) (corev1.ServiceAccount, error) {
	switch {
	case o.ServiceAccount != nil:
		return *o.ServiceAccount, nil
	case o.ServiceAccountName != "" || o.ServiceAccountNamespace != "":
		return corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.ServiceAccountName,
				Namespace: o.ServiceAccountNamespace,
			},
		}, nil
	default:
		// No object-level ServiceAccount configured, discover the
		// controller's own ServiceAccount from the mounted token.
		ref, err := findPodServiceAccount(p.impl().ReadFile)
		if err != nil {
			return corev1.ServiceAccount{}, err
		}
		return corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: ref.Namespace,
			},
		}, nil
	}
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
