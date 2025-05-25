//go:build integration
// +build integration

/*
Copyright 2022 The Flux authors

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
	"context"
	"testing"

	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	objectLevelWIModeDisabled objectLevelWIMode = iota
	objectLevelWIModeDirectAccess
	objectLevelWIModeImpersonation
	objectLevelWIModeDirectAccessFederation
	objectLevelWIModeImpersonationFederation
)

type objectLevelWIMode int

type jobOptions struct {
	objectLevelWIMode objectLevelWIMode
}

type jobOption func(*jobOptions)

func withObjectLevelWI(mode objectLevelWIMode) jobOption {
	return func(o *jobOptions) {
		o.objectLevelWIMode = mode
	}
}

func testjobExecutionWithArgs(t *testing.T, args []string, opts ...jobOption) {
	t.Helper()
	g := NewWithT(t)
	ctx := context.TODO()

	var o jobOptions
	for _, opt := range opts {
		opt(&o)
	}

	job := &batchv1.Job{}
	job.Name = "test-job-" + randStringRunes(5)
	job.Namespace = wiSANamespace
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	if enableWI {
		// Set pod SA.
		saName := wiServiceAccount
		if o.objectLevelWIMode != objectLevelWIModeDisabled {
			saName = controllerWIRBACName

			// Set impersonated SA.
			args = append(args, "-wisa-namespace="+wiSANamespace)
			switch o.objectLevelWIMode {
			case objectLevelWIModeImpersonation:
				args = append(args, "-wisa-name="+wiServiceAccount)
			case objectLevelWIModeDirectAccess:
				args = append(args, "-wisa-name="+wiServiceAccountDirectAccess)
			case objectLevelWIModeImpersonationFederation:
				args = append(args, "-wisa-name="+wiServiceAccountFederation)
			case objectLevelWIModeDirectAccessFederation:
				args = append(args, "-wisa-name="+wiServiceAccountFederationDirectAccess)
			}
		}
		job.Spec.Template.Spec.ServiceAccountName = saName

		// azure requires this label on the pod for workload identity to work.
		if *targetProvider == "azure" && o.objectLevelWIMode == objectLevelWIModeDisabled {
			job.Spec.Template.Labels = map[string]string{
				"azure.workload.identity/use": "true",
			}
		}
	}

	job.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:            "test-app",
			Image:           testAppImage,
			Args:            args,
			ImagePullPolicy: corev1.PullAlways,
		},
	}

	key := client.ObjectKeyFromObject(job)

	g.Expect(testEnv.Client.Create(ctx, job)).To(Succeed())

	defer func() {
		background := metav1.DeletePropagationBackground
		g.Expect(testEnv.Client.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &background})).To(Succeed())
	}()

	g.Eventually(func() bool {
		if err := testEnv.Client.Get(ctx, key, job); err != nil {
			return false
		}
		return job.Status.Succeeded == 1 && job.Status.Active == 0
	}, resultWaitTimeout).Should(BeTrue())
}
