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

package v1

import (
	"testing"
)

func TestGCPWorkloadIdentityProviderExtract(t *testing.T) {
	const valid = "projects/123456789/locations/global/workloadIdentityPools/my-pool/providers/my-provider"
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "valid",
			ann:    map[string]string{AnnotationGCPWorkloadIdentityProvider.String(): valid},
			want:   valid,
			wantOk: true,
		},
		{name: "missing", ann: map[string]string{}},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationGCPWorkloadIdentityProvider.String(): "nope"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationGCPWorkloadIdentityProvider.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestGCPServiceAccountExtract(t *testing.T) {
	const valid = "my-sa@my-project.iam.gserviceaccount.com"
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "valid",
			ann:    map[string]string{AnnotationGCPServiceAccount.String(): valid},
			want:   valid,
			wantOk: true,
		},
		{name: "missing", ann: map[string]string{}},
		{
			name:      "invalid",
			ann:       map[string]string{AnnotationGCPServiceAccount.String(): "nope"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationGCPServiceAccount.Extract(tt.ann)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestGCPClusterValidate(t *testing.T) {
	const cluster = "projects/my-project/locations/us-central1/clusters/my-cluster"
	tests := []struct {
		name      string
		cluster   string
		wantError bool
	}{
		{name: "valid", cluster: cluster},
		{name: "empty", wantError: true},
		{name: "invalid", cluster: "nope", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GCPCluster("").Validate(tt.cluster)
			if (err != nil) != tt.wantError {
				t.Fatalf("got error %v, want error=%v", err, tt.wantError)
			}
		})
	}
}
