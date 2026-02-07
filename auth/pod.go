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

package auth

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FindPodServiceAccount attempts to determine the service account
// associated with the current pod by reading and parsing the
// service account token mounted in the pod.
func FindPodServiceAccount(readFile func(name string) ([]byte, error)) (*client.ObjectKey, error) {
	// tokenFile is the well-known path for the service account token mounted
	// in pods by Kubernetes.
	const tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	b, err := readFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account token file %s: %w", tokenFile, err)
	}
	tok, _, err := jwt.NewParser().ParseUnverified(string(b), jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account token: %w", err)
	}
	sub, err := tok.Claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("failed to get subject from service account token: %w", err)
	}
	parts := strings.Split(sub, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid subject format in service account token: %s", sub)
	}
	return &client.ObjectKey{
		Namespace: parts[2],
		Name:      parts[3],
	}, nil
}
