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

func TestGetAWSRegionFromCodeCommitURL(t *testing.T) {
	tests := []struct {
		name    string
		gitURL  string
		want    string
		wantErr string
	}{
		{
			name:   "valid url",
			gitURL: "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo",
			want:   "us-east-1",
		},
		{
			name:   "valid fips url",
			gitURL: "https://git-codecommit-fips.us-west-2.amazonaws.com/v1/repos/repo",
			want:   "us-west-2",
		},
		{
			name:   "valid china url",
			gitURL: "https://git-codecommit.cn-north-1.amazonaws.com.cn/v1/repos/repo",
			want:   "cn-north-1",
		},
		{
			name:    "empty url",
			gitURL:  "",
			wantErr: "Git URL is required",
		},
		{
			name:    "non-https url",
			gitURL:  "http://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo",
			wantErr: "requires an HTTPS Git URL",
		},
		{
			name:    "invalid host",
			gitURL:  "https://example.com/v1/repos/repo",
			wantErr: "invalid AWS CodeCommit Git URL",
		},
		{
			name:    "invalid url",
			gitURL:  "https://git-codecommit.us-east-1.amazonaws.com:invalid/v1/repos/repo",
			wantErr: "failed to parse Git URL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetAWSRegionFromCodeCommitURL(tt.gitURL)
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

func TestParseGitRepository(t *testing.T) {
	const codeCommitURL = "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo"

	tests := []struct {
		name            string
		serviceProvider string
		gitURL          string
		want            string
		wantErr         string
	}{
		{
			name:            "aws codecommit",
			serviceProvider: ProviderAWS,
			gitURL:          codeCommitURL,
			want:            codeCommitURL,
		},
		{
			name:            "aws invalid url",
			serviceProvider: ProviderAWS,
			gitURL:          "https://example.com/repo",
			wantErr:         "invalid AWS CodeCommit Git URL",
		},
		{
			name:            "azure devops",
			serviceProvider: ProviderAzure,
			gitURL:          "https://dev.azure.com/org/project/_git/repo",
			want:            "azure-devops",
		},
		{
			name:            "unsupported service provider",
			serviceProvider: ProviderGCP,
			gitURL:          "https://example.com/repo",
			wantErr:         "does not support Git repositories",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitRepository(tt.serviceProvider, tt.gitURL)
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
