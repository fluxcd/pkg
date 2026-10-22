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
	"strings"
	"testing"
)

func TestGetRegistryFromArtifactRepository(t *testing.T) {
	tests := []struct {
		name               string
		artifactRepository string
		want               string
		wantErr            string
	}{
		{
			name:               "dot-less host with port",
			artifactRepository: "localhost:5000",
			want:               "localhost:5000",
		},
		{
			name:               "dot-less host without port",
			artifactRepository: "localhost",
			want:               "localhost",
		},
		{
			name:               "host with port",
			artifactRepository: "registry.io:5000",
			want:               "registry.io:5000",
		},
		{
			name:               "host without port",
			artifactRepository: "registry.io",
			want:               "registry.io",
		},
		{
			name:               "dot-less repo with port",
			artifactRepository: "localhost:5000/repo",
			want:               "localhost:5000",
		},
		{
			name:               "dot-less repo without port",
			artifactRepository: "localhost/repo",
			want:               "localhost",
		},
		{
			name:               "repo with port",
			artifactRepository: "registry.io:5000/repo",
			want:               "registry.io:5000",
		},
		{
			name:               "repo without port",
			artifactRepository: "registry.io/repo",
			want:               "registry.io",
		},
		{
			name:               "repo with tag",
			artifactRepository: "registry.io/repo:tag",
			want:               "registry.io",
		},
		{
			name:               "repo with trailing slash",
			artifactRepository: "foo.azurecr.io/bar/",
			want:               "foo.azurecr.io",
		},
		{
			name:               "invalid reference",
			artifactRepository: "foo.azurecr.io/BAR",
			wantErr:            "failed to parse artifact repository",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetRegistryFromArtifactRepository(tt.artifactRepository)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got error %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseArtifactRepository(t *testing.T) {
	tests := []struct {
		name               string
		serviceProvider    string
		artifactRepository string
		want               string
		wantErr            string
	}{
		{
			name:               "aws private ecr",
			serviceProvider:    ProviderAWS,
			artifactRepository: "012345678901.dkr.ecr.us-east-1.amazonaws.com/repo",
			want:               "us-east-1",
		},
		{
			name:               "aws public ecr",
			serviceProvider:    ProviderAWS,
			artifactRepository: "public.ecr.aws/nginx/nginx",
			want:               RegistryAWSPublicECR,
		},
		{
			name:               "aws public ecr dual stack",
			serviceProvider:    ProviderAWS,
			artifactRepository: "ecr-public.aws.com/nginx/nginx",
			want:               RegistryAWSPublicECR,
		},
		{
			name:               "aws invalid registry",
			serviceProvider:    ProviderAWS,
			artifactRepository: "foo.azurecr.io/bar",
			wantErr:            "invalid AWS registry",
		},
		{
			name:               "gcp artifact registry",
			serviceProvider:    ProviderGCP,
			artifactRepository: "us-central1-docker.pkg.dev/project/repo/image",
			want:               ProviderGCP,
		},
		{
			name:               "gcp container registry",
			serviceProvider:    ProviderGCP,
			artifactRepository: "gcr.io/project/image",
			want:               ProviderGCP,
		},
		{
			name:               "gcp invalid registry",
			serviceProvider:    ProviderGCP,
			artifactRepository: "foo.azurecr.io/bar",
			wantErr:            "invalid GCP registry",
		},
		{
			name:               "azure container registry",
			serviceProvider:    ProviderAzure,
			artifactRepository: "fluxtest.azurecr.io/repo/image",
			want:               "fluxtest.azurecr.io",
		},
		{
			name:               "azure invalid registry",
			serviceProvider:    ProviderAzure,
			artifactRepository: "us-central1-docker.pkg.dev/project/repo",
			wantErr:            "invalid Azure registry",
		},
		{
			name:               "unsupported service provider",
			serviceProvider:    ProviderGeneric,
			artifactRepository: "foo.azurecr.io/bar",
			wantErr:            "does not support artifact registries",
		},
		{
			name:               "invalid artifact repository",
			serviceProvider:    ProviderAWS,
			artifactRepository: "foo.azurecr.io/BAR",
			wantErr:            "failed to parse artifact repository",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArtifactRepository(tt.serviceProvider, tt.artifactRepository)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got error %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetAWSRegionFromArtifactRegistryInput(t *testing.T) {
	if got := GetAWSRegionFromArtifactRegistryInput(RegistryAWSPublicECR); got != "us-east-1" {
		t.Fatalf("got %q, want %q", got, "us-east-1")
	}
	if got := GetAWSRegionFromArtifactRegistryInput("us-west-2"); got != "us-west-2" {
		t.Fatalf("got %q, want %q", got, "us-west-2")
	}
}
