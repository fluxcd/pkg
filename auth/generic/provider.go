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

package generic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/kubernetes"
)

// ProviderName is the name of the generic authentication provider.
const ProviderName = "generic"

// Provider implements the auth.Provider interface for generic authentication.
type Provider struct{}

// GetName implements auth.RESTConfigProvider.
func (Provider) GetName() string {
	return ProviderName
}

// NewControllerToken implements auth.RESTConfigProvider.
func (Provider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	if o.Client == nil {
		return nil, errors.New("client is required to create a controller token")
	}

	// For kubernetes.CreateJWT(), a nil saRef means creating
	// the token for the controller ServiceAccount.
	var controllerSARef *kubernetes.ServiceAccountReference
	var tokenDuration time.Duration // Default expiration (annotation not supported).
	token, err := kubernetes.CreateJWT(ctx, o.Client, controllerSARef,
		o.CredentialAudiences, tokenDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes token for controller service account: %w", err)
	}

	// Get the effective expiration from the token.
	exp, err := getExpirationFromToken(token)
	if err != nil {
		return nil, err
	}

	return &Token{
		Token:     token,
		ExpiresAt: *exp,
	}, nil
}

// GetAudiences implements auth.RESTConfigProvider.
func (Provider) GetAudiences(_ context.Context, opts ...auth.Option) ([]string, error) {
	var o auth.Options
	o.Apply(opts...)
	return o.CredentialAudiences, nil
}

// GetIdentity implements auth.RESTConfigProvider.
func (Provider) GetIdentity(opts ...auth.Option) (string, error) {
	var o auth.Options
	o.Apply(opts...)
	sa := o.ServiceAccountRef()
	return fmt.Sprintf("system:serviceaccount:%s:%s", sa.Namespace, sa.Name), nil
}

// NewTokenForServiceAccount implements auth.RESTConfigProvider.
func (Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	opts ...auth.Option) (auth.Token, error) {

	exp, err := getExpirationFromToken(oidcToken)
	if err != nil {
		return nil, err
	}

	return &Token{
		Token:     oidcToken,
		ExpiresAt: *exp,
	}, nil
}

// GetAccessTokenOptionsForCluster implements auth.RESTConfigProvider.
func (Provider) GetAccessTokenOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {

	var o auth.Options
	o.Apply(opts...)

	audiences := o.CredentialAudiences
	if len(audiences) == 0 {
		// Use cluster address as the default audience.
		audiences = []string{o.ClusterAddress}
	}

	return [][]auth.Option{{auth.WithCredentialAudiences(audiences...)}}, nil
}

// NewRESTConfig implements auth.RESTConfigProvider.
func (Provider) NewRESTConfig(ctx context.Context, accessTokens []auth.Token,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	token := accessTokens[0].(*Token)

	var o auth.Options
	o.Apply(opts...)

	// Parse the cluster address.
	host := o.ClusterAddress
	if host == "" {
		return nil, errors.New("cluster address is required to create a REST config")
	}
	var err error
	host, err = cryptov1.ParseClusterAddress(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster address %s: %w", o.ClusterAddress, err)
	}

	// Get CA if provided.
	var caData []byte
	if o.CAData != "" {
		caData = []byte(o.CAData)
	}

	return &auth.RESTConfig{
		Host:        host,
		CAData:      caData,
		BearerToken: token.Token,
		ExpiresAt:   token.ExpiresAt,
	}, nil
}

func getExpirationFromToken(token string) (*time.Time, error) {
	tok, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account token: %w", err)
	}
	exp, err := tok.Claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration time from service account token: %w", err)
	}
	return &exp.Time, nil
}
