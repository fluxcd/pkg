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
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ParseClusterAddress returns the canonical form
// https://<lowercase(host)>:<port> of the given Kubernetes API server address.
// The address is prefixed with https:// when no scheme is present, and the port
// defaults to 443.
func ParseClusterAddress(address string) (string, error) {
	if address == "" {
		return "", errors.New("empty address")
	}
	if !strings.HasPrefix(address, "http") {
		address = fmt.Sprintf("https://%s", address)
	}
	u, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("failed to parse Kubernetes API server address '%s': %w", address, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("the Kubernetes API server address '%s' must use https scheme", address)
	}
	host := u.Host
	if u.Port() == "" {
		host += ":443"
	}
	return fmt.Sprintf("https://%s", strings.ToLower(host)), nil
}

// parseCluster validates the given kubeconfig cluster resource name against
// regex and returns the captured groups.
func parseCluster(cluster string, regex *regexp.Regexp) ([]string, error) {
	matches := regex.FindStringSubmatch(cluster)
	if matches == nil {
		return nil, fmt.Errorf("invalid kubeconfig ConfigMap data key cluster: '%s'. must match %s",
			cluster, regex)
	}
	return matches, nil
}
