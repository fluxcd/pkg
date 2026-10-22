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

func TestGetAWSRoleSessionName(t *testing.T) {
	if got, want := GetAWSRoleSessionName("my-sa", "my-ns", "us-east-1"), "my-sa.my-ns.us-east-1.fluxcd.io"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGetDefaultAudienceGCP(t *testing.T) {
	const wip = "projects/1/locations/global/workloadIdentityPools/p/providers/x"
	if got, want := GetDefaultAudienceGCP(wip), "https://iam.googleapis.com/"+wip; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindNewAnnotation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		ann     map[string]string
		wantKey string
		want    bool
	}{
		{
			name:    "auth annotation",
			ann:     ann(AnnotationCredentialProvider, CredentialProviderKubernetes),
			wantKey: AnnotationCredentialProvider,
			want:    true,
		},
		{
			name:    "aws auth annotation",
			ann:     ann(AnnotationAWSSTSEndpoint, "https://sts.amazonaws.com"),
			wantKey: AnnotationAWSSTSEndpoint,
			want:    true,
		},
		{
			name:    "mtls annotation",
			ann:     ann(AnnotationMTLSSPIFFEServerIDs, "spiffe://example.org"),
			wantKey: AnnotationMTLSSPIFFEServerIDs,
			want:    true,
		},
		{
			name: "gcp annotation borrowed from RFC-0010",
			ann:  ann(AnnotationGCPWorkloadIdentityProvider, "projects/1/locations/global/workloadIdentityPools/p/providers/x"),
		},
		{
			name: "unrelated toolkit annotation",
			ann:  ann("kustomize.toolkit.fluxcd.io/prune", "enabled"),
		},
		{
			name: "unrelated source annotation",
			ann:  ann("source.toolkit.fluxcd.io/ignore", "true"),
		},
		{
			name: "unrelated core annotation",
			ann:  ann("fluxcd.io/foo", "bar"),
		},
		{
			name: "annotation without slash",
			ann:  ann("justakey", "x"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, got := findNewAnnotation(tt.ann)
			if got != tt.want || gotKey != tt.wantKey {
				t.Fatalf("findNewAnnotation() = (%q, %t), want (%q, %t)",
					gotKey, got, tt.wantKey, tt.want)
			}
		})
	}
}

func TestValidateSTSEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name     string
		endpoint string
		valid    bool
	}{
		{
			name:     "global endpoint",
			endpoint: "https://sts.amazonaws.com",
			valid:    true,
		},
		{
			name:     "trailing slash",
			endpoint: "https://sts.amazonaws.com/",
			valid:    true,
		},
		{
			name:     "regional endpoint",
			endpoint: "https://sts.us-east-1.amazonaws.com",
			valid:    true,
		},
		{
			name:     "fips endpoint",
			endpoint: "https://sts-fips.us-east-1.amazonaws.com",
			valid:    true,
		},
		{
			name:     "vpce endpoint",
			endpoint: "https://vpce-002b7cc8966426bc6-njisq19r.sts.us-east-1.vpce.amazonaws.com",
			valid:    true,
		},
		{
			name:     "vpce zonal endpoint",
			endpoint: "https://vpce-002b7cc8966426bc6-njisq19r-us-east-1a.sts.us-east-1.vpce.amazonaws.com",
			valid:    true,
		},
		{
			name:     "http endpoint",
			endpoint: "http://sts.amazonaws.com",
			valid:    false,
		},
		{
			name:     "no scheme",
			endpoint: "sts.amazonaws.com",
			valid:    false,
		},
		{
			name:     "non sts host",
			endpoint: "https://stss.amazonaws.com",
			valid:    false,
		},
		{
			name:     "non aws host",
			endpoint: "https://sts.amazonaws.example.com",
			valid:    false,
		},
		{
			name:     "uppercase host",
			endpoint: "https://STS.AMAZONAWS.COM",
			valid:    false,
		},
		{
			name:     "path",
			endpoint: "https://sts.amazonaws.com/foo",
			valid:    false,
		},
		{
			name:     "query",
			endpoint: "https://sts.amazonaws.com?foo=bar",
			valid:    false,
		},
		{
			name:     "fragment",
			endpoint: "https://sts.amazonaws.com#foo",
			valid:    false,
		},
		{
			name:     "credentials",
			endpoint: "https://user:pass@sts.amazonaws.com",
			valid:    false,
		},
		{
			name:     "port",
			endpoint: "https://sts.amazonaws.com:443",
			valid:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSTSEndpoint(tt.endpoint)
			if (err == nil) != tt.valid {
				t.Fatalf("validateSTSEndpoint(%q) error = %v, valid = %t", tt.endpoint, err, tt.valid)
			}
		})
	}
}
