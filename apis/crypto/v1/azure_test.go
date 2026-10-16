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

const testUUID = "12345678-1234-1234-1234-123456789012"

func TestAzureTenantIDExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantError bool
	}{
		{name: "valid", ann: map[string]string{AnnotationAzureTenantID.String(): testUUID}, want: testUUID},
		{name: "missing", ann: map[string]string{}, wantError: true},
		{name: "invalid", ann: map[string]string{AnnotationAzureTenantID.String(): "nope"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnnotationAzureTenantID.Extract(tt.ann)
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v), want (%q, err=%v)", got, err, tt.want, tt.wantError)
			}
		})
	}
}

func TestAzureClientIDExtract(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantError bool
	}{
		{name: "valid", ann: map[string]string{AnnotationAzureClientID.String(): testUUID}, want: testUUID},
		{name: "missing", ann: map[string]string{}, wantError: true},
		{name: "invalid", ann: map[string]string{AnnotationAzureClientID.String(): "nope"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnnotationAzureClientID.Extract(tt.ann)
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v), want (%q, err=%v)", got, err, tt.want, tt.wantError)
			}
		})
	}
}

func TestAzureClusterExtract(t *testing.T) {
	const cluster = "/subscriptions/" + testUUID + "/resourceGroups/test-rg/" +
		"providers/Microsoft.ContainerService/managedClusters/test-cluster"
	tests := []struct {
		name      string
		cluster   string
		wantSub   string
		wantRG    string
		wantName  string
		wantError bool
	}{
		{
			name:     "valid",
			cluster:  cluster,
			wantSub:  testUUID,
			wantRG:   "test-rg",
			wantName: "test-cluster",
		},
		{name: "empty", wantError: true},
		{name: "invalid", cluster: "nope", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, rg, name, err := AzureCluster("").ExtractSubscriptionResourceGroupAndName(tt.cluster)
			if sub != tt.wantSub || rg != tt.wantRG || name != tt.wantName || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %q, %q, %v), want (%q, %q, %q, err=%v)",
					sub, rg, name, err, tt.wantSub, tt.wantRG, tt.wantName, tt.wantError)
			}
		})
	}
}

func TestExtractAzureIdentity(t *testing.T) {
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantError bool
	}{
		{
			name: "valid",
			ann: map[string]string{
				AnnotationAzureTenantID.String(): testUUID,
				AnnotationAzureClientID.String(): testUUID,
			},
			want: testUUID + "/" + testUUID,
		},
		{name: "missing tenant", ann: map[string]string{AnnotationAzureClientID.String(): testUUID}, wantError: true},
		{name: "missing client", ann: map[string]string{AnnotationAzureTenantID.String(): testUUID}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractAzureIdentity(tt.ann)
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v), want (%q, err=%v)", got, err, tt.want, tt.wantError)
			}
		})
	}
}
