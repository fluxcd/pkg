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

package gcp

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"

	"github.com/fluxcd/pkg/auth"
)

const serviceAccountEmailPattern = `^[a-zA-Z0-9-]{1,100}@[a-zA-Z0-9-]{1,100}\.iam\.gserviceaccount\.com$`

var serviceAccountEmailRegex = regexp.MustCompile(serviceAccountEmailPattern)

func parseServiceAccountEmail(email string) error {
	if !serviceAccountEmailRegex.MatchString(email) {
		return fmt.Errorf("invalid GCP service account email: '%s'. must match %s",
			email, serviceAccountEmailPattern)
	}
	return nil
}

const workloadIdentityProviderPattern = `^projects/\d{1,30}/locations/global/workloadIdentityPools/[^/]{1,100}/providers/[^/]{1,100}$`

var workloadIdentityProviderRegex = regexp.MustCompile(workloadIdentityProviderPattern)

func getWorkloadIdentityProviderAudience(workloadIdentityProvider string) (string, error) {
	if !workloadIdentityProviderRegex.MatchString(workloadIdentityProvider) {
		return "", fmt.Errorf("invalid GCP workload identity provider: '%s'. must match %s",
			workloadIdentityProvider, workloadIdentityProviderPattern)
	}
	return fmt.Sprintf("//iam.googleapis.com/%s", workloadIdentityProvider), nil
}

const clusterPattern = `^projects/[^/]{1,200}/locations/[^/]{1,200}/clusters/[^/]{1,200}$`

var clusterRegex = regexp.MustCompile(clusterPattern)

func parseCluster(cluster string) error {
	if !clusterRegex.MatchString(cluster) {
		return fmt.Errorf("invalid GKE cluster ID: '%s'. must match %s",
			cluster, clusterPattern)
	}
	return nil
}

func newHTTPClient(ctx context.Context, token auth.Token, o *auth.Options) (*http.Client, error) {
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	if p := o.ProxyURL; p != nil {
		baseTransport.Proxy = http.ProxyURL(p)
	}
	transport, err := htransport.NewTransport(ctx, baseTransport, option.WithTokenSource(token.(*Token).source()))
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport}, nil
}
