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

import "testing"

func TestEKSRoleARNExtract(t *testing.T) {
	const valid = "arn:aws:iam::123456789012:role/my-role"
	tests := []struct {
		name      string
		ann       map[string]string
		want      string
		wantError bool
	}{
		{name: "valid", ann: map[string]string{AnnotationEKSRoleARN.String(): valid}, want: valid},
		{name: "missing", ann: map[string]string{}, wantError: true},
		{name: "invalid", ann: map[string]string{AnnotationEKSRoleARN.String(): "not-an-arn"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnnotationEKSRoleARN.Extract(tt.ann)
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v), want (%q, err=%v)", got, err, tt.want, tt.wantError)
			}
		})
	}
}

func TestAWSSTSEndpointExtract(t *testing.T) {
	const endpoint = "https://sts.us-east-1.amazonaws.com"
	tests := []struct {
		name      string
		ann       map[string]string
		spec      string
		want      string
		wantOk    bool
		wantError bool
	}{
		{
			name:   "annotation valid",
			ann:    map[string]string{AnnotationAWSSTSEndpoint.String(): "https://sts.amazonaws.com"},
			want:   "https://sts.amazonaws.com",
			wantOk: true,
		},
		{name: "annotation missing", ann: map[string]string{}},
		{
			name:      "annotation invalid",
			ann:       map[string]string{AnnotationAWSSTSEndpoint.String(): "https://example.com"},
			wantError: true,
		},
		{name: "spec value", spec: endpoint, want: endpoint, wantOk: true},
		{
			name:      "spec value with rfc0014 annotation",
			ann:       map[string]string{AnnotationCredentialProvider.String(): CredentialProviderSPIFFE},
			spec:      endpoint,
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := AnnotationAWSSTSEndpoint.Extract(tt.ann, tt.spec)
			if got != tt.want || ok != tt.wantOk || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %v, %v), want (%q, %v, err=%v)",
					got, ok, err, tt.want, tt.wantOk, tt.wantError)
			}
		})
	}
}

func TestAWSClusterExtractRegionAndName(t *testing.T) {
	const cluster = "arn:aws:eks:us-east-1:123456789012:cluster/my-cluster"
	tests := []struct {
		name       string
		cluster    string
		wantRegion string
		wantName   string
		wantError  bool
	}{
		{
			name:       "valid",
			cluster:    cluster,
			wantRegion: "us-east-1",
			wantName:   "my-cluster",
		},
		{name: "empty", wantError: true},
		{name: "invalid", cluster: "not-an-arn", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region, name, err := AWSCluster("").ExtractRegionAndName(tt.cluster)
			if region != tt.wantRegion || name != tt.wantName || (err != nil) != tt.wantError {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, err=%v)",
					region, name, err, tt.wantRegion, tt.wantName, tt.wantError)
			}
		})
	}
}

func TestGetRoleSessionName(t *testing.T) {
	if got, want := GetRoleSessionName("my-sa", "my-ns", "us-east-1"), "my-sa.my-ns.us-east-1.fluxcd.io"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
