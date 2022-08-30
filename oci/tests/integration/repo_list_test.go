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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestImageRepositoryListTags(t *testing.T) {
	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			testImageRepositoryListTags(t, repo)
		})
	}
}

func testImageRepositoryListTags(t *testing.T, repoURL string) {
	g := NewWithT(t)
	ctx := context.TODO()

	job := &batchv1.Job{}
	job.Name = "test-job-" + randStringRunes(5)
	job.Namespace = "default"
	job.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:            "test-app",
			Image:           testAppImage,
			Args:            []string{fmt.Sprintf("-repo=%s", repoURL)},
			ImagePullPolicy: corev1.PullAlways,
		},
	}
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	key := client.ObjectKeyFromObject(job)

	g.Expect(testEnv.Client.Create(ctx, job)).To(Succeed())
	defer func() {
		g.Expect(testEnv.Client.Delete(ctx, job)).To(Succeed())
	}()
	g.Eventually(func() bool {
		if err := testEnv.Client.Get(ctx, key, job); err != nil {
			return false
		}
		return job.Status.Succeeded == 1 && job.Status.Active == 0
	}, resultWaitTimeout).Should(BeTrue())
}
