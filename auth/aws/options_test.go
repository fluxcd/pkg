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

func TestValidateSTSEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name        string
		stsEndpoint string
		valid       bool
	}{
		// valid endpoints
		{
			name:        "global endpoint",
			stsEndpoint: "https://sts.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.us-east-2.amazonaws.com",
			stsEndpoint: "https://sts.us-east-2.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts-fips.us-east-2.amazonaws.com",
			stsEndpoint: "https://sts-fips.us-east-2.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.us-east-1.amazonaws.com",
			stsEndpoint: "https://sts.us-east-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts-fips.us-east-1.amazonaws.com",
			stsEndpoint: "https://sts-fips.us-east-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.us-west-1.amazonaws.com",
			stsEndpoint: "https://sts.us-west-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts-fips.us-west-1.amazonaws.com",
			stsEndpoint: "https://sts-fips.us-west-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.us-west-2.amazonaws.com",
			stsEndpoint: "https://sts.us-west-2.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts-fips.us-west-2.amazonaws.com",
			stsEndpoint: "https://sts-fips.us-west-2.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.il-central-1.amazonaws.com",
			stsEndpoint: "https://sts.il-central-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.mx-central-1.amazonaws.com",
			stsEndpoint: "https://sts.mx-central-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.me-south-1.amazonaws.com",
			stsEndpoint: "https://sts.me-south-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.me-central-1.amazonaws.com",
			stsEndpoint: "https://sts.me-central-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.sa-east-1.amazonaws.com",
			stsEndpoint: "https://sts.sa-east-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.us-gov-east-1.amazonaws.com",
			stsEndpoint: "https://sts.us-gov-east-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "sts.us-gov-west-1.amazonaws.com",
			stsEndpoint: "https://sts.us-gov-west-1.amazonaws.com",
			valid:       true,
		},
		{
			name:        "vpce-002b7cc8966426bc6-njisq19r.sts.us-east-1.vpce.amazonaws.com",
			stsEndpoint: "https://vpce-002b7cc8966426bc6-njisq19r.sts.us-east-1.vpce.amazonaws.com",
			valid:       true,
		},
		{
			name:        "vpce-002b7cc8966426bc6-njisq19r-us-east-1a.sts.us-east-1.vpce.amazonaws.com",
			stsEndpoint: "https://vpce-002b7cc8966426bc6-njisq19r-us-east-1a.sts.us-east-1.vpce.amazonaws.com",
			valid:       true,
		},
		// invalid endpoints
		{
			name:        "non sts endpoint",
			stsEndpoint: "https://stss.amazonaws.com",
			valid:       false,
		},
		{
			name:        "non aws endpoint",
			stsEndpoint: "https://sts.amazonaws.example.com",
			valid:       false,
		},
		{
			name:        "http endpoint",
			stsEndpoint: "http://sts.amazonaws.com",
			valid:       false,
		},
		{
			name:        "no scheme",
			stsEndpoint: "sts.amazonaws.com",
			valid:       false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			err := aws.ValidateSTSEndpoint(tt.stsEndpoint)

			g.Expect(err == nil).To(Equal(tt.valid))
		})
	}
}
