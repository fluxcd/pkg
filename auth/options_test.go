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

package auth_test

import (
	"net/url"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/google/go-cmp/cmp"
)

func TestOptions_GetHTTPClient(t *testing.T) {
	tests := []struct {
		name        string
		options     auth.Options
		expectProxy bool
	}{
		{
			name:        "no proxy configured",
			options:     auth.Options{},
			expectProxy: false,
		},
		{
			name: "proxy configured",
			options: auth.Options{
				ProxyURL: &url.URL{Scheme: "http", Host: "proxy.example.com:8080"},
			},
			expectProxy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.options.GetHTTPClient()

			if client == nil {
				t.Error("GetHTTPClient() returned nil, expected non-nil client")
				return // fix linter error
			}

			expectedTimeout := 10 * time.Second
			if client.Timeout != expectedTimeout {
				t.Errorf("GetHTTPClient() timeout = %v, want %v", client.Timeout, expectedTimeout)
			}

			if client.Transport == nil {
				t.Error("GetHTTPClient() transport is nil")
				return
			}

			if tt.expectProxy && tt.options.ProxyURL == nil {
				t.Error("Expected proxy but ProxyURL is nil")
			}
		})
	}
}

func TestOptions_ServiceAccountRef(t *testing.T) {
	tests := []struct {
		name     string
		opts     []auth.Option
		expected bool
	}{
		{
			name: "service account name provided",
			opts: []auth.Option{
				auth.WithServiceAccountName("test-sa"),
			},
			expected: true,
		},
		{
			name:     "service account name not provided",
			opts:     []auth.Option{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var o auth.Options
			o.Apply(tt.opts...)

			result := o.ServiceAccountRef() != nil
			if result != tt.expected {
				t.Errorf("ServiceAccountRef() != nil = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestOptionsFromSpec(t *testing.T) {
	tests := []struct {
		name string
		spec *cryptov1.AuthSpec
		want auth.Options
	}{
		{
			name: "nil spec",
		},
		{
			name: "empty spec",
			spec: &cryptov1.AuthSpec{},
		},
		{
			name: "credential opted in",
			spec: &cryptov1.AuthSpec{
				Credential: &cryptov1.CredentialSpec{
					Provider:   cryptov1.CredentialProviderSPIFFE,
					Type:       cryptov1.CredentialTypeX509,
					Expiration: 5 * time.Minute,
					Audiences:  []string{"audience"},
					Username:   "user",
				},
			},
			want: auth.Options{
				CredentialProvider:   cryptov1.CredentialProviderSPIFFE,
				CredentialType:       cryptov1.CredentialTypeX509,
				CredentialExpiration: 5 * time.Minute,
				CredentialAudiences:  []string{"audience"},
				CredentialUsername:   "user",
			},
		},
		{
			name: "credential not opted in with legacy audiences",
			spec: &cryptov1.AuthSpec{
				Credential: &cryptov1.CredentialSpec{
					Audiences: []string{"audience"},
				},
			},
			want: auth.Options{
				CredentialAudiences: []string{"audience"},
			},
		},
		{
			name: "service-provider specs",
			spec: &cryptov1.AuthSpec{
				AWS: &cryptov1.AWSSpec{
					RoleARN:     "arn:aws:iam::123456789012:role/role",
					STSEndpoint: "https://sts.us-east-1.amazonaws.com",
					Cluster: &cryptov1.AWSClusterSpec{
						Region: "us-east-1",
						Name:   "test-cluster",
					},
				},
				Azure: &cryptov1.AzureSpec{
					TenantID: "tenant-id",
					ClientID: "client-id",
					Cluster: &cryptov1.AzureClusterSpec{
						SubscriptionID: "subscription-id",
						ResourceGroup:  "resource-group",
						Name:           "test-cluster",
					},
				},
				GCP: &cryptov1.GCPSpec{
					WorkloadIdentityProvider: "projects/123/locations/global/workloadIdentityPools/pool/providers/provider",
					ServiceAccount:           "sa@project.iam.gserviceaccount.com",
				},
				DockerHub: &cryptov1.DockerHubSpec{
					OIDCConnectionID: "connection-id",
				},
			},
			want: auth.Options{
				AWSRoleARN:                  "arn:aws:iam::123456789012:role/role",
				AWSSTSEndpoint:              "https://sts.us-east-1.amazonaws.com",
				AWSRegion:                   "us-east-1",
				AWSClusterName:              "test-cluster",
				AzureTenantID:               "tenant-id",
				AzureClientID:               "client-id",
				AzureSubscriptionID:         "subscription-id",
				AzureResourceGroup:          "resource-group",
				AzureClusterName:            "test-cluster",
				GCPWorkloadIdentityProvider: "projects/123/locations/global/workloadIdentityPools/pool/providers/provider",
				GCPServiceAccount:           "sa@project.iam.gserviceaccount.com",
				DockerHubOIDCConnectionID:   "connection-id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got auth.Options
			got.Apply(auth.OptionsFromSpec(tt.spec)...)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("OptionsFromSpec() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestOptions_ObjectReferences(t *testing.T) {
	entitled := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "entitled",
			Namespace: "namespace",
			UID:       "entitled-uid",
		},
	}
	involved := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "involved",
			Namespace: "namespace",
			UID:       "involved-uid",
		},
	}
	wantEntitled := auth.ObjectReference{
		APIGroup:  "source.toolkit.fluxcd.io",
		Kind:      "GitRepository",
		Name:      "entitled",
		Namespace: "namespace",
		UID:       "entitled-uid",
	}
	wantInvolved := auth.ObjectReference{
		APIGroup:  "image.toolkit.fluxcd.io",
		Kind:      "ImagePolicy",
		Name:      "involved",
		Namespace: "namespace",
		UID:       "involved-uid",
	}

	var o auth.Options
	o.Apply(
		auth.WithObject(wantEntitled.APIGroup, wantEntitled.Kind, entitled),
		auth.WithCacheOperation("reconcile"),
	)

	if o.ObjectReference != wantEntitled {
		t.Errorf("ObjectReference = %+v, want %+v", o.ObjectReference, wantEntitled)
	}
	if o.InvolvedObjectReference != wantEntitled {
		t.Errorf("InvolvedObjectReference = %+v, want %+v", o.InvolvedObjectReference, wantEntitled)
	}

	o.Apply(auth.WithInvolvedObject(wantInvolved.APIGroup, wantInvolved.Kind, involved))

	if o.ObjectReference != wantEntitled {
		t.Errorf("ObjectReference = %+v, want %+v", o.ObjectReference, wantEntitled)
	}
	if o.InvolvedObjectReference != wantInvolved {
		t.Errorf("InvolvedObjectReference = %+v, want %+v", o.InvolvedObjectReference, wantInvolved)
	}
}
