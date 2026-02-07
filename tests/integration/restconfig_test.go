//go:build integration
// +build integration

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

package integration

import (
	"testing"
)

func TestRESTConfig(t *testing.T) {
	if !testRESTConfig {
		t.Skip(skippedMessage)
	}

	for _, tt := range []struct {
		name string
		skip bool
		opts []jobOption
	}{
		{
			name: "controller-level workload identity",
			skip: false,
		},
		{
			name: "object-level workload identity (impersonation)",
			skip: false,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeImpersonation)},
		},
		{
			name: "object-level workload identity (direct access)",
			skip: !testWIDirectAccess,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeDirectAccess)},
		},
		{
			name: "object-level workload identity (impersonation, federation)",
			skip: !testWIFederation,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeImpersonationFederation)},
		},
		{
			name: "object-level workload identity (direct access, federation)",
			skip: !testWIDirectAccess || !testWIFederation,
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeDirectAccessFederation)},
		},
		{
			name: "impersonation: controller IRSA -> AssumeRole (AWS)",
			skip: !testImpersonation || *targetProvider != "aws",
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeAssumeRoleIRSA)},
		},
		{
			name: "impersonation: controller Pod Identity -> AssumeRole (AWS)",
			skip: !testImpersonation || *targetProvider != "aws",
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeAssumeRolePodIdentity)},
		},
		{
			name: "impersonation: object-level IRSA -> AssumeRole (AWS)",
			skip: !testImpersonation || *targetProvider != "aws",
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeAssumeRoleObjectLevel)},
		},
		{
			name: "impersonation: controller WIF -> Impersonate (GCP)",
			skip: !testImpersonation || *targetProvider != "gcp",
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeGCPImpersonateCtrl)},
		},
		{
			name: "impersonation: controller WIF+SA -> Impersonate (GCP)",
			skip: !testImpersonation || *targetProvider != "gcp",
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeGCPImpersonateCtrlSA)},
		},
		{
			name: "impersonation: object-level WIF+SA -> Impersonate (GCP)",
			skip: !testImpersonation || *targetProvider != "gcp",
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeGCPImpersonateObj)},
		},
		{
			name: "impersonation: object-level WIF direct access -> Impersonate (GCP)",
			skip: !testImpersonation || *targetProvider != "gcp",
			opts: []jobOption{withObjectLevelWI(objectLevelWIModeGCPImpersonateObjDA)},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip(skippedMessage)
			}

			testjobExecutionWithArgs(t, []string{
				"-category=restconfig",
				"-provider=" + *targetProvider,
			}, tt.opts...)
		})
	}
}
