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
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth/internal/tokenfile"
)

// CreateJWT generates a JSON Web Token (JWT) for the specified service
// account in the given namespace with the provided audiences. If no
// audiences are provided, an error is returned. The function also loads
// the controller ServiceAccount token to know which audiences the
// controller token is intended for to also reject those. These rejections
// force created tokens to always target external services instead of the
// local Kubernetes cluster, which is the only use case for the auth library.
// A nil saRef means creating the token for the controller ServiceAccount.
func CreateJWT(ctx context.Context, c client.Client,
	saRef *ServiceAccountReference, aud []string, exp time.Duration) (string, error) {

	if saRef != nil && (saRef.Name == "" || saRef.Namespace == "") {
		return "", fmt.Errorf("service account and namespace must be specified")
	}
	if len(aud) == 0 {
		return "", fmt.Errorf("at least one audience must be specified")
	}

	// Load the controller token to check its audiences against the requested audiences.
	controllerSARef, clusterAudiences, err := loadControllerToken()
	if err != nil {
		return "", fmt.Errorf("failed to load controller token: %w", err)
	}
	for _, clusterAud := range clusterAudiences {
		if slices.Contains(aud, clusterAud) {
			return "", fmt.Errorf(
				"audience '%s' is not allowed as it is intended for the local kubernetes cluster", clusterAud)
		}
	}
	if saRef == nil { // As documented.
		saRef = controllerSARef
	}

	// Create token.
	sa := &corev1.ServiceAccount{
		Name:      saRef.Name,
		Namespace: saRef.Namespace,
	}
	tokenReq := &authnv1.TokenRequest{
		Spec: authnv1.TokenRequestSpec{
			Audiences: aud,
		},
	}
	if exp > 0 {
		expirationSeconds := int64(exp.Seconds())
		tokenReq.Spec.ExpirationSeconds = &expirationSeconds
	}
	if err := c.SubResource("token").Create(ctx, sa, tokenReq); err != nil {
		return "", err
	}
	return tokenReq.Status.Token, nil
}

// loadControllerToken loads and parses the controller ServiceAccount token from
// /var/run/secrets/kubernetes.io/serviceaccount/token. The token file can be
// replaced for testing through tokenfile.ReadFile.
func loadControllerToken() (*ServiceAccountReference, []string, error) {
	const tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	b, err := tokenfile.ReadFile(tokenFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read service account token file %s: %w", tokenFile, err)
	}

	// Get controller service account from token subject.
	tok, _, err := jwt.NewParser().ParseUnverified(string(b), jwt.MapClaims{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse service account token: %w", err)
	}
	sub, err := tok.Claims.GetSubject()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get subject from service account token: %w", err)
	}
	parts := strings.Split(sub, ":")
	if len(parts) != 4 {
		return nil, nil, fmt.Errorf("invalid subject format in service account token: %s", sub)
	}
	sa := ServiceAccountReference{
		Name:      parts[3],
		Namespace: parts[2],
	}

	// Get audiences.
	audiences, err := tok.Claims.GetAudience()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get audiences from service account token: %w", err)
	}

	return &sa, audiences, nil
}
