/*
Copyright 2024 The Flux authors

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

package chartutil

import (
	"github.com/matheuscscp/helm/pkg/chartutil"
	"github.com/opencontainers/go-digest"
)

// DigestValues calculates the digest of the values using the provided algorithm.
// The caller is responsible for ensuring that the algorithm is supported.
func DigestValues(algo digest.Algorithm, values chartutil.Values) digest.Digest {
	digester := algo.Digester()
	if values = valuesOrNil(values); values != nil {
		if err := Encode(digester.Hash(), values, SortMapSlice); err != nil {
			return ""
		}
	}
	return digester.Digest()
}

// VerifyValues verifies the digest of the values against the provided digest.
func VerifyValues(digest digest.Digest, values chartutil.Values) bool {
	if digest.Validate() != nil {
		return false
	}

	verifier := digest.Verifier()
	if values = valuesOrNil(values); values != nil {
		if err := Encode(verifier, values, SortMapSlice); err != nil {
			return false
		}
	}
	return verifier.Verified()
}

// valuesOrNil returns nil if the values are empty, otherwise the values are
// returned. This is used to ensure that the digest is calculated against nil
// opposed to an empty object.
func valuesOrNil(values chartutil.Values) chartutil.Values {
	if values != nil && len(values) == 0 {
		return nil
	}
	return values
}
