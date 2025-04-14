/*
Copyright 2025 The Flux authors

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

package aws_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth/aws"
)

func TestParseRegistry(t *testing.T) {
	tests := []struct {
		registry      string
		wantAccountID string
		wantRegion    string
		wantOK        bool
	}{
		{
			registry:      "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1",
			wantAccountID: "012345678901",
			wantRegion:    "us-east-1",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo",
			wantAccountID: "012345678901",
			wantRegion:    "us-east-1",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.us-east-1.amazonaws.com",
			wantAccountID: "012345678901",
			wantRegion:    "us-east-1",
			wantOK:        true,
		},
		{
			registry:      "https://012345678901.dkr.ecr.us-east-1.amazonaws.com/v2/part/part",
			wantAccountID: "012345678901",
			wantRegion:    "us-east-1",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.cn-north-1.amazonaws.com.cn/foo",
			wantAccountID: "012345678901",
			wantRegion:    "cn-north-1",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr-fips.us-gov-west-1.amazonaws.com",
			wantAccountID: "012345678901",
			wantRegion:    "us-gov-west-1",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.us-secret-region.sc2s.sgov.gov",
			wantAccountID: "012345678901",
			wantRegion:    "us-secret-region",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr-fips.us-ts-region.c2s.ic.gov",
			wantAccountID: "012345678901",
			wantRegion:    "us-ts-region",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.uk-region.cloud.adc-e.uk",
			wantAccountID: "012345678901",
			wantRegion:    "uk-region",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.us-ts-region.csp.hci.ic.gov",
			wantAccountID: "012345678901",
			wantRegion:    "us-ts-region",
			wantOK:        true,
		},
		// TODO: Fix: this invalid registry is allowed by the regex.
		// {
		// 	registry: ".dkr.ecr.error.amazonaws.com",
		// 	wantOK:   false,
		// },
		{
			registry: "gcr.io/foo/bar:baz",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.registry, func(t *testing.T) {
			g := NewWithT(t)

			accId, region, ok := aws.ParseRegistry(tt.registry)
			g.Expect(ok).To(Equal(tt.wantOK), "unexpected OK")
			g.Expect(accId).To(Equal(tt.wantAccountID), "unexpected account IDs")
			g.Expect(region).To(Equal(tt.wantRegion), "unexpected regions")
		})
	}
}
