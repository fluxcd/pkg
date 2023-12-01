/*
Copyright 2023 The Flux authors

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

package errors

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestIsImmutableError(t *testing.T) {
	testCases := []struct {
		name  string
		err   error
		match bool
	}{
		{
			name:  "CEL immutable error",
			err:   fmt.Errorf(`the ImmutableSinceFirstWrite "test1" is invalid: value: Invalid value: "string": Value is immutable`),
			match: true,
		},
		{
			name:  "Custom admission immutable error",
			err:   fmt.Errorf(`the IAMPolicyMember's spec is immutable: admission webhook "deny-immutable-field-updates.cnrm.cloud.google.com" denied the request: the IAMPolicyMember's spec is immutable`),
			match: true,
		},
		{
			name:  "Not immutable error",
			err:   fmt.Errorf(`is not immutable`),
			match: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(IsImmutableError(tc.err)).To(BeIdenticalTo(tc.match))
		})
	}
}
