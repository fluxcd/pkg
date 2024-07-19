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
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOciImageRepositoryListTags(t *testing.T) {
	if !enableOci {
		t.Skip("Skipping oci feature tests, specify -category oci to enable")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			args := []string{
				"-category=oci",
				fmt.Sprintf("-repo=%s", repo),
			}
			testjobExecutionWithArgs(t, args)
		})
	}
}

func TestOciRepositoryRootLoginListTags(t *testing.T) {
	if !enableOci {
		t.Skip("Skipping oci feature tests, specify -category oci to enable")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			parts := strings.SplitN(repo, "/", 2)
			args := []string{
				"-category=oci",
				fmt.Sprintf("-registry=%s", parts[0]),
				fmt.Sprintf("-repo=%s", parts[1]),
			}
			testjobExecutionWithArgs(t, args)
		})
	}
}

func TestOciOIDCLoginListTags(t *testing.T) {
	if !enableOci {
		t.Skip("Skipping oci feature tests, specify -category oci to enable")
	}

	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			// Registry only.
			parts := strings.SplitN(repo, "/", 2)
			args := []string{
				"-category=oci",
				"-oidc-login=true",
				fmt.Sprintf("-registry=%s", parts[0]),
				fmt.Sprintf("-repo=%s", parts[1]),
			}
			testjobExecutionWithArgs(t, args)

			// Registry + repo.
			args = []string{
				"-category=oci",
				"-oidc-login=true",
				fmt.Sprintf("-repo=%s", repo),
			}
			testjobExecutionWithArgs(t, args)
		})
	}
}

func TestGitCloneUsingProvider(t *testing.T) {
	if !enableGit {
		t.Skip("Skipping git feature tests, specify -category git to enable")
	}

	ctx := context.TODO()
	tmpDir := t.TempDir()

	setupGitRepository(ctx, tmpDir)
	t.Run("Git oidc credential test", func(t *testing.T) {
		args := []string{
			"-category=git",
			"-oidc-login=true",
			fmt.Sprintf("-provider=%s", *targetProvider),
			fmt.Sprintf("-repo=%s", cfg.applicationRepositoryWithoutUser),
		}
		testjobExecutionWithArgs(t, args)
	})
}

func testjobExecutionWithArgs(t *testing.T, args []string) {
	g := NewWithT(t)
	ctx := context.TODO()

	job := &batchv1.Job{}
	job.Name = "test-job-" + randStringRunes(5)
	job.Namespace = "default"
	job.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:            "test-app",
			Image:           testAppImage,
			Args:            args,
			ImagePullPolicy: corev1.PullAlways,
		},
	}
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	if enableWI {
		job.Spec.Template.Spec.ServiceAccountName = wiServiceAccount

		// azure requires this label on the pod for workload identity to work.
		if *targetProvider == "azure" {
			job.Spec.Template.Labels = map[string]string{
				"azure.workload.identity/use": "true",
			}
		}
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
