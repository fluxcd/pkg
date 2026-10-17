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
	"fmt"
	"net/url"
	"strings"
)

// ParseGitRepository validates the given Git repository URL against the rules
// of the given service provider and returns the provider-specific input
// required to issue credentials for it. The input is also used as part of the
// credentials cache key.
//
// For the `aws` service provider the input is the Git repository URL, as AWS
// CodeCommit credentials are a SigV4 signature over the request URL. For the
// `azure` service provider the input is a constant, as Azure DevOps access
// tokens are not bound to the Git repository URL.
func ParseGitRepository(serviceProvider, gitURL string) (string, error) {
	switch serviceProvider {
	case ProviderAWS:
		if _, err := GetAWSRegionFromCodeCommitURL(gitURL); err != nil {
			return "", err
		}
		return gitURL, nil
	case ProviderAzure:
		return "azure-devops", nil
	}
	return "", fmt.Errorf("service provider '%s' does not support Git repositories", serviceProvider)
}

// GetAWSRegionFromCodeCommitURL returns the AWS region of the given AWS
// CodeCommit HTTPS Git repository URL, e.g. `us-east-1` for
// `https://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo`.
//
// https://docs.aws.amazon.com/codecommit/latest/userguide/regions.html#regions-git
func GetAWSRegionFromCodeCommitURL(gitURL string) (string, error) {
	if gitURL == "" {
		return "", fmt.Errorf("Git URL is required for AWS CodeCommit authentication")
	}
	u, err := url.Parse(gitURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Git URL '%s': %w", gitURL, err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", fmt.Errorf("AWS CodeCommit authentication requires an HTTPS Git URL")
	}
	host := u.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) < 4 ||
		!(strings.HasPrefix(host, "git-codecommit.") || strings.HasPrefix(host, "git-codecommit-fips.")) ||
		!(strings.HasSuffix(host, ".amazonaws.com") || strings.HasSuffix(host, ".amazonaws.com.cn")) {
		return "", fmt.Errorf("invalid AWS CodeCommit Git URL: %s", u.Host)
	}
	return parts[1], nil
}
