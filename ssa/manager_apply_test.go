/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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

package ssa

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssaerrors "github.com/fluxcd/pkg/ssa/errors"
	"github.com/fluxcd/pkg/ssa/normalize"
	"github.com/fluxcd/pkg/ssa/utils"
)

func TestApply(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("apply")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	configMapName, configMap := getFirstObject(objects, "ConfigMap", id)

	t.Run("creates objects in order", func(t *testing.T) {
		// create objects
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		// expected created order
		sort.Sort(SortableUnstructureds(objects))
		var expected []string
		for _, object := range objects {
			expected = append(expected, utils.FmtUnstructured(object))
		}

		// verify the change set contains only created actions
		var output []string
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(entry.Action, CreatedAction); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
			output = append(output, entry.Subject)
		}

		// verify the change set contains all objects in the right order
		if diff := cmp.Diff(expected, output); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("does not apply unchanged objects", func(t *testing.T) {
		// no-op apply
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		// verify the change set contains only unchanged actions
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(UnchangedAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s\n%v", diff, changeSet)
			}
		}
	})

	t.Run("applies only changed objects", func(t *testing.T) {
		// update a value in the configmap
		err = unstructured.SetNestedField(configMap.Object, "val", "data", "key")
		if err != nil {
			t.Fatal(err)
		}

		// apply changes
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		// verify the change set contains the configured action only for the configmap
		for _, entry := range changeSet.Entries {
			if entry.Subject == configMapName {
				if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(UnchangedAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			}
		}

		// get the configmap from cluster
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		// get data value from the in-cluster configmap
		val, _, err := unstructured.NestedFieldCopy(configMapClone.Object, "data", "key")
		if err != nil {
			t.Fatal(err)
		}

		// verify the configmap was updated in cluster with the right data value
		if diff := cmp.Diff(val, "val"); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})
}

func TestApplyAllStaged_PartialFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	id := generateName("test-staged-fail")
	objects, err := readManifest("testdata/test12.yaml", id)
	if err != nil {
		t.Fatal(err)
	}
	partialFailureID := fmt.Sprintf("%s-partial-failure", id)

	// This test requires a non-cluster-admin client to
	// make sure ClusterRoleBinding and ClusterRole objects can be
	// applied in the same call to ApplyAllStaged.
	partialFailureNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: partialFailureID,
		},
	}
	if err := manager.client.Create(ctx, partialFailureNS); err != nil {
		t.Fatal(err)
	}
	partialFailureSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      partialFailureID,
			Namespace: partialFailureID,
		},
	}
	if err := manager.client.Create(ctx, partialFailureSA); err != nil {
		t.Fatal(err)
	}
	partialFailureCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: partialFailureID,
		},
		Rules: []rbacv1.PolicyRule{
			// For applying the manifests from testdata/test12.yaml
			{
				APIGroups: []string{"", "rbac.authorization.k8s.io", "storage.k8s.io"},
				Resources: []string{"namespaces", "clusterroles", "clusterrolebindings", "storageclasses"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			// The ServiceAccount must have all the permissions it is indirectly granting
			// through the ClusterRole manifest from testdata/test12.yaml
			{
				APIGroups: []string{"apps"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	if err := manager.client.Create(ctx, partialFailureCR); err != nil {
		t.Fatal(err)
	}
	partialFailureCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: partialFailureID,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     partialFailureCR.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      partialFailureSA.Name,
			Namespace: partialFailureSA.Namespace,
		}},
	}
	if err := manager.client.Create(ctx, partialFailureCRB); err != nil {
		t.Fatal(err)
	}

	// Copy test suite manager and modify the client to one that isn't
	// cluster-admin.
	partialFailureManager := *manager
	cfg := rest.CopyConfig(cfg)
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:%s", partialFailureSA.Namespace, partialFailureSA.Name),
	}
	client, err := client.New(cfg, client.Options{
		Mapper: restMapper,
	})
	if err != nil {
		t.Fatal(err)
	}
	partialFailureManager.client = client

	partialFailureManager.SetOwnerLabels(objects, "app1", "default")

	_, crb := getFirstObject(objects, "ClusterRoleBinding", id)

	t.Run("creates objects in order", func(t *testing.T) {
		// create objects
		changeSet, err := partialFailureManager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		// expected created order
		expected := []string{
			fmt.Sprintf("Namespace/%s", id),
			fmt.Sprintf("ClusterRole/%s", id),
			fmt.Sprintf("StorageClass/%s", id),
			fmt.Sprintf("ClusterRoleBinding/%s", id),
		}

		var output []string
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(entry.Action, CreatedAction); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
			output = append(output, entry.Subject)
		}

		// verify the change set contains all objects in the right order
		if diff := cmp.Diff(expected, output); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("returns change set on failed apply", func(t *testing.T) {
		// update ClusterRoleBinding to trigger an immutable field error
		err = unstructured.SetNestedField(crb.Object, "test", "roleRef", "name")
		if err != nil {
			t.Fatal(err)
		}

		// apply and expect to fail
		changeSet, err := partialFailureManager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err == nil {
			t.Fatal("Expected error got none")
		}

		// expected change set after failed apply
		expected := []string{
			fmt.Sprintf("Namespace/%s", id),
			fmt.Sprintf("ClusterRole/%s", id),
			fmt.Sprintf("StorageClass/%s", id),
		}

		var output []string
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(entry.Action, UnchangedAction); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
			output = append(output, entry.Subject)
		}

		// verify the change set contains all applied objects in the right order
		if diff := cmp.Diff(expected, output); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})
}

func TestApply_Force(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("apply")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	secretName, secret := getFirstObject(objects, "Secret", id)
	crbName, crb := getFirstObject(objects, "ClusterRoleBinding", id)
	stName, st := getFirstObject(objects, "StorageClass", id)
	_, svc := getFirstObject(objects, "Service", id)

	// create objects
	if _, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	t.Run("fails to apply immutable secret", func(t *testing.T) {
		// update a value in the secret
		err = unstructured.SetNestedField(secret.Object, "val-secret", "stringData", "key")
		if err != nil {
			t.Fatal(err)
		}

		// apply and expect to fail
		_, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err == nil {
			t.Fatal("Expected error got none")
		}

		// verify that the error message does not contain sensitive information
		expectedErr := fmt.Sprintf(
			"%s dry-run failed (Invalid): Secret \"%s\" is invalid: data: Forbidden: field is immutable when `immutable` is set",
			utils.FmtUnstructured(secret), secret.GetName())
		if diff := cmp.Diff(expectedErr, err.Error()); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("force applies immutable secret", func(t *testing.T) {
		// force apply
		opts := DefaultApplyOptions()
		opts.Force = true

		changeSet, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		// verify the secret was recreated
		for _, entry := range changeSet.Entries {
			if entry.Subject == secretName {
				if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(UnchangedAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			}
		}

		// get the secret from cluster
		secretClone := secret.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(secretClone), secretClone)
		if err != nil {
			t.Fatal(err)
		}

		// get data value from the in-cluster secret
		val, _, err := unstructured.NestedFieldCopy(secretClone.Object, "data", "key")
		if err != nil {
			t.Fatal(err)
		}

		// verify the secret was updated in cluster with the right data value
		if diff := cmp.Diff(val, base64.StdEncoding.EncodeToString([]byte("val-secret"))); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("force apply waits for finalizer", func(t *testing.T) {
		secretClone := secret.DeepCopy()
		{
			secretWithFinalizer := secretClone.DeepCopy()

			unstructured.SetNestedStringSlice(secretWithFinalizer.Object, []string{"fluxcd.io/demo-finalizer"}, "metadata", "finalizers")
			if err := manager.client.Update(ctx, secretWithFinalizer); err != nil {
				t.Fatal(err)
			}
		}

		// remove finalizer after a delay, to ensure the controller handles a slow deletion
		go func() {
			time.Sleep(3 * time.Second)

			secretWithoutFinalizer := secretClone.DeepCopy()
			unstructured.SetNestedStringSlice(secretWithoutFinalizer.Object, []string{}, "metadata", "finalizers")
			if err := manager.client.Update(ctx, secretWithoutFinalizer); err != nil {
				panic(err)
			}
		}()

		// update a value in the secret
		err = unstructured.SetNestedField(secret.Object, "val-secret2", "stringData", "key")
		if err != nil {
			t.Fatal(err)
		}

		// force apply
		opts := DefaultApplyOptions()
		opts.Force = true

		changeSet, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		// verify the secret was recreated
		for _, entry := range changeSet.Entries {
			if entry.Subject == secretName {
				if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(UnchangedAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			}
		}
	})

	t.Run("recreates immutable RBAC", func(t *testing.T) {
		// update roleRef
		err = unstructured.SetNestedField(crb.Object, "test", "roleRef", "name")
		if err != nil {
			t.Fatal(err)
		}

		// force apply
		opts := DefaultApplyOptions()
		opts.Force = true

		changeSet, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		// verify the binding was recreated
		for _, entry := range changeSet.Entries {
			if entry.Subject == crbName {
				if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
				break
			}
		}
	})

	t.Run("recreates immutable StorageClass based on metadata", func(t *testing.T) {
		// update parameters
		err = unstructured.SetNestedField(st.Object, "true", "parameters", "encrypted")
		if err != nil {
			t.Fatal(err)
		}

		meta := map[string]string{
			"fluxcd.io/force": "true",
		}
		st.SetAnnotations(meta)

		// apply and expect to fail
		_, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err == nil {
			t.Fatal("Expected error got none")
		}

		// force apply selector
		opts := DefaultApplyOptions()
		opts.ForceSelector = meta

		changeSet, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		// verify the storage class was recreated
		for _, entry := range changeSet.Entries {
			if entry.Subject == stName {
				if diff := cmp.Diff(CreatedAction, entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
				break
			}
		}
	})

	t.Run("force apply returns validation error", func(t *testing.T) {
		// update to invalid yaml
		err = unstructured.SetNestedField(svc.Object, "ClusterIPSS", "spec", "type")
		if err != nil {
			t.Fatal(err)
		}

		// force apply objects
		opts := DefaultApplyOptions()
		opts.Force = true

		_, err := manager.ApplyAllStaged(ctx, objects, opts)
		if err == nil {
			t.Fatal("expected validation error but got none")
		}

		// should return validation error
		if !strings.Contains(err.Error(), "is invalid") {
			t.Errorf("expected error to contain invalid msg but got: %s", err)
		}
	})
}

func TestApply_SetNativeKindsDefaults(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("fix")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	t.Run("creates objects", func(t *testing.T) {
		// create objects
		_, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
	})

	// re-apply objects
	changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
	if err != nil {
		t.Fatal(err)
	}

	// verify that the change set contains no changed objects
	for _, entry := range changeSet.Entries {
		if diff := cmp.Diff(UnchangedAction, entry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	}

}

func TestApply_NoOp(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("fix")
	objects, err := readManifest("testdata/test3.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	t.Run("creates objects", func(t *testing.T) {
		// create objects
		_, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("skips apply", func(t *testing.T) {
		// apply changes
		changeSet, err := manager.ApplyAll(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if entry.Action != UnchangedAction {
				t.Errorf("Diff found for %s", entry.String())
			}
		}
	})
}

func TestApply_SkipsExcluded(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("fix")
	err := manager.client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	objects, err := readManifest("testdata/test13.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	if err := normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	opts := DefaultApplyOptions()
	opts.ExclusionSelector = map[string]string{
		"ssa.fluxcd.io/exclude": "true",
	}
	skippedSubject := fmt.Sprintf("Secret/%[1]s/data-%[1]s-excluded", id)

	t.Run("Apply", func(t *testing.T) {
		changeSetEntry, err := manager.Apply(ctx, objects[0], opts)
		if err != nil {
			t.Fatal(err)
		}

		if changeSetEntry.Subject != skippedSubject {
			t.Errorf("Expected %s, got %s", skippedSubject, changeSetEntry.Subject)
		}
	})

	t.Run("ApplyAll", func(t *testing.T) {
		changeSet, err := manager.ApplyAll(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		var found bool
		for _, entry := range changeSet.Entries {
			if entry.Action != SkippedAction {
				continue
			}
			found = true
			if entry.Subject != skippedSubject {
				t.Errorf("Expected %s, got %s", skippedSubject, entry.Subject)
			}
			break
		}
		if !found {
			t.Errorf("Expected to find skipped entry for %s", skippedSubject)
		}
	})
}

func TestApply_Exclusions(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("ignore")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)

	t.Run("creates objects", func(t *testing.T) {
		// create objects
		_, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("skips apply", func(t *testing.T) {
		// mutate in-cluster object
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		meta := map[string]string{
			"fluxcd.io/ignore": "true",
		}
		configMapClone.SetAnnotations(meta)

		if err := unstructured.SetNestedField(configMapClone.Object, "val", "data", "key"); err != nil {
			t.Fatal(err)
		}

		if err := manager.client.Update(ctx, configMapClone); err != nil {
			t.Fatal(err)
		}

		opts := DefaultApplyOptions()
		opts.ExclusionSelector = meta

		// apply with exclusions
		changeSet, err := manager.ApplyAll(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if entry.Action == ConfiguredAction {
				t.Errorf("Diff found for %s", entry.String())
			}
		}
	})

	t.Run("applies changes", func(t *testing.T) {
		// apply changes without exclusions
		changeSet, err := manager.ApplyAll(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if entry.Action != ConfiguredAction && entry.Subject == utils.FmtUnstructured(configMap) {
				t.Errorf("Expected %s, got %s", ConfiguredAction, entry.Action)
			}
		}
	})

	t.Run("skips apply when desired state is annotated", func(t *testing.T) {
		configMapClone := configMap.DeepCopy()
		meta := map[string]string{
			"fluxcd.io/ignore": "true",
		}
		configMapClone.SetAnnotations(meta)

		// apply changes without exclusions
		changeSet, err := manager.Apply(ctx, configMapClone, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}

		if changeSet.Action != UnchangedAction {
			t.Errorf("Diff found for %s", changeSet.String())
		}
	})
}

func TestApply_IfNotPresent(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	meta := map[string]string{
		"fluxcd.io/ssa": "IfNotPresent",
	}

	id := generateName("skip")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, ns := getFirstObject(objects, "Namespace", id)
	_, configMap := getFirstObject(objects, "ConfigMap", id)
	configMapClone := configMap.DeepCopy()
	configMapClone.SetAnnotations(meta)

	t.Run("creates objects", func(t *testing.T) {
		// create objects
		opts := DefaultApplyOptions()
		opts.IfNotPresentSelector = meta

		changeSet, err := manager.ApplyAllStaged(ctx, []*unstructured.Unstructured{ns, configMapClone}, opts)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if entry.Action != CreatedAction {
				t.Errorf("Expected %s, got %s for %s", CreatedAction, entry.Action, entry.Subject)
			}
		}
	})

	t.Run("skips apply when annotated IfNotPresent", func(t *testing.T) {
		opts := DefaultApplyOptions()
		opts.IfNotPresentSelector = meta

		changeSet, err := manager.Apply(ctx, configMapClone, opts)
		if err != nil {
			t.Fatal(err)
		}

		if changeSet.Action != SkippedAction {
			t.Errorf("Diff found for %s", changeSet.String())
		}
	})

	t.Run("resume apply when is annotated Override", func(t *testing.T) {
		override := map[string]string{
			"fluxcd.io/ssa": "Override",
		}
		configMapClone.SetAnnotations(override)

		opts := DefaultApplyOptions()
		opts.IfNotPresentSelector = meta

		changeSet, err := manager.Apply(ctx, configMapClone, opts)
		if err != nil {
			t.Fatal(err)
		}

		if changeSet.Action != ConfiguredAction {
			t.Errorf("Diff found for %s", changeSet.String())
		}
	})
}

func TestApply_Cleanup_ExactMatch(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("cleanup-exact")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}
	manager.SetOwnerLabels(objects, "app1", "default")

	_, deployObject := getFirstObject(objects, "Deployment", id)

	if err = normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	t.Run("creates objects as different managers", func(t *testing.T) {
		// Apply all with prefix manager
		for _, object := range objects {
			obj := object.DeepCopy()
			if err := manager.client.Patch(ctx, obj, client.Apply, client.FieldOwner("flux-apply-prefix")); err != nil {
				t.Fatal(err)
			}
		}

		// Apply deployment with exact match manager
		deploy := deployObject.DeepCopy()
		if err := manager.client.Patch(ctx, deploy, client.Apply, client.FieldOwner("flux")); err != nil {
			t.Fatal(err)
		}

		// Check that the deployment has both managers
		resultDeploy := deployObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(deploy), resultDeploy)
		if err != nil {
			t.Fatal(err)
		}

		managedFields := resultDeploy.GetManagedFields()
		foundExact := false
		foundPrefix := false

		for _, field := range managedFields {
			if field.Manager == "flux" && field.Operation == metav1.ManagedFieldsOperationApply {
				foundExact = true
			}
			if field.Manager == "flux-apply-prefix" && field.Operation == metav1.ManagedFieldsOperationApply {
				foundPrefix = true
			}
		}

		if !foundExact {
			t.Errorf("Expected to find exact match manager 'flux' with Apply operation")
		}
		if !foundPrefix {
			t.Errorf("Expected to find prefix manager 'flux-apply-prefix' with Apply operation")
		}
	})

	t.Run("cleanup removes only exact match", func(t *testing.T) {
		applyOpts := DefaultApplyOptions()
		applyOpts.Cleanup = ApplyCleanupOptions{
			FieldManagers: []FieldManager{
				{
					Name:          "flux",
					OperationType: metav1.ManagedFieldsOperationApply,
					ExactMatch:    true,
				},
			},
		}

		_, err := manager.ApplyAllStaged(ctx, objects, applyOpts)
		if err != nil {
			t.Fatal(err)
		}

		// Check that only exact match was removed
		resultDeploy := deployObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(resultDeploy), resultDeploy)
		if err != nil {
			t.Fatal(err)
		}

		managedFields := resultDeploy.GetManagedFields()
		foundExact := false
		foundPrefix := false
		foundManager := false

		for _, field := range managedFields {
			t.Logf("Found managed field: Manager=%s, Operation=%s", field.Manager, field.Operation)
			if field.Manager == "flux" {
				foundExact = true
			}
			if field.Manager == "flux-apply-prefix" {
				foundPrefix = true
			}
			if field.Manager == manager.owner.Field {
				foundManager = true
			}
		}

		if foundExact {
			t.Errorf("Expected exact match 'flux' to be removed, but it was still present")
		}
		if !foundPrefix {
			t.Errorf("Expected prefix match 'flux-apply-prefix' to remain, but it was not found")
		}
		if !foundManager {
			t.Errorf("Expected manager '%s' to be present, but it was not found", manager.owner.Field)
		}
	})
}

func TestApply_Cleanup(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	applyOpts := DefaultApplyOptions()
	applyOpts.Cleanup = ApplyCleanupOptions{
		Annotations: []string{corev1.LastAppliedConfigAnnotation},
		FieldManagers: []FieldManager{
			{
				Name:          "kubectl",
				OperationType: metav1.ManagedFieldsOperationApply,
			},
			{
				Name:          "kubectl",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
			{
				Name:          "before-first-apply",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
		},
	}

	id := generateName("cleanup")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}
	manager.SetOwnerLabels(objects, "app1", "default")

	_, deployObject := getFirstObject(objects, "Deployment", id)

	if err = normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	t.Run("creates objects as kubectl", func(t *testing.T) {
		for _, object := range objects {
			obj := object.DeepCopy()
			obj.SetAnnotations(map[string]string{corev1.LastAppliedConfigAnnotation: "test"})
			labels := obj.GetLabels()
			labels[corev1.LastAppliedConfigAnnotation] = "test"
			obj.SetLabels(labels)
			if err := manager.client.Create(ctx, obj, client.FieldOwner("kubectl-client-side-apply")); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("removes kubectl client-side-apply manager and annotation", func(t *testing.T) {
		applyOpts.Cleanup.Labels = []string{corev1.LastAppliedConfigAnnotation}
		changeSet, err := manager.ApplyAllStaged(ctx, objects, applyOpts)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}

		deploy := deployObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(deploy), deploy)
		if err != nil {
			t.Fatal(err)
		}

		if _, ok := deploy.GetAnnotations()[corev1.LastAppliedConfigAnnotation]; ok {
			t.Errorf("%s annotation not removed", corev1.LastAppliedConfigAnnotation)
		}

		if _, ok := deploy.GetLabels()[corev1.LastAppliedConfigAnnotation]; ok {
			t.Errorf("%s label not removed", corev1.LastAppliedConfigAnnotation)
		}

		expectedManagers := []string{"before-first-apply", manager.owner.Field}
		for _, entry := range deploy.GetManagedFields() {
			if !containsItemString(expectedManagers, entry.Manager) {
				t.Log(entry)
				t.Errorf("Mismatch from expected values, want %v got %s", expectedManagers, entry.Manager)
			}
		}
	})

	t.Run("replaces kubectl server-side-apply manager", func(t *testing.T) {
		for _, object := range objects {
			obj := object.DeepCopy()
			if err := manager.client.Patch(ctx, obj, client.Apply, client.FieldOwner("kubectl")); err != nil {
				t.Fatal(err)
			}
		}

		deploy := deployObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(deploy), deploy)
		if err != nil {
			t.Fatal(err)
		}

		changeSet, err := manager.ApplyAll(ctx, objects, applyOpts)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}

		deploy = deployObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(deploy), deploy)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range deploy.GetManagedFields() {
			if diff := cmp.Diff(manager.owner.Field, entry.Manager); diff != "" {
				t.Log(entry)
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}
	})
}

func TestApply_CleanupRemovals(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	applyOpts := DefaultApplyOptions()
	applyOpts.Cleanup = ApplyCleanupOptions{
		Annotations: []string{corev1.LastAppliedConfigAnnotation},
		FieldManagers: []FieldManager{
			{
				Name:          "kubectl",
				OperationType: metav1.ManagedFieldsOperationApply,
			},
			{
				Name:          "kubectl",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
			{
				Name:          "before-first-apply",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
		},
	}

	id := generateName("cleanup-removal")
	objects, err := readManifest("testdata/test8.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	editedObjects, err := readManifest("testdata/test9.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")
	manager.SetOwnerLabels(editedObjects, "app1", "default")

	_, ingressObject := getFirstObject(objects, "Ingress", id)

	t.Run("creates objects using manager apply", func(t *testing.T) {
		applyOpts.Cleanup.Labels = []string{corev1.LastAppliedConfigAnnotation}
		_, err := manager.ApplyAllStaged(ctx, objects, applyOpts)
		if err != nil {
			t.Fatal(err)
		}

		ingress := ingressObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
		if err != nil {
			t.Fatal(err)
		}

		expectedManagers := []string{manager.owner.Field}
		for _, entry := range ingress.GetManagedFields() {
			if !containsItemString(expectedManagers, entry.Manager) {
				t.Log(entry)
				t.Errorf("Mismatch from expected values, want %v got %s", expectedManagers, entry.Manager)
			}
		}
	})

	t.Run("applies edited objects as kubectl", func(t *testing.T) {
		for _, object := range editedObjects {
			obj := object.DeepCopy()
			obj.SetAnnotations(map[string]string{corev1.LastAppliedConfigAnnotation: "test"})
			labels := obj.GetLabels()
			labels[corev1.LastAppliedConfigAnnotation] = "test"
			obj.SetLabels(labels)
			if err := manager.client.Patch(ctx, obj, client.Merge, client.FieldOwner("kubectl-client-side-apply")); err != nil {
				t.Fatal(err)
			}
		}

		ingress := ingressObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
		if err != nil {
			t.Fatal(err)
		}

		expectedManagers := []string{"kubectl-client-side-apply", manager.owner.Field}
		for _, entry := range ingress.GetManagedFields() {
			if !containsItemString(expectedManagers, entry.Manager) {
				t.Log(entry)
				t.Errorf("Mismatch from expected values, want %v got %s", expectedManagers, entry.Manager)
			}
		}

		rules, _, err := unstructured.NestedSlice(ingress.Object, "spec", "rules")
		if err != nil {
			t.Fatal(err)
		}

		if len(rules) != 2 {
			t.Errorf("expected to two rules in Ingress, got %d", len(rules))
		}
	})

	t.Run("applies edited object using manager apply", func(t *testing.T) {
		applyOpts.Cleanup.Labels = []string{corev1.LastAppliedConfigAnnotation}
		_, err := manager.ApplyAllStaged(ctx, editedObjects, applyOpts)
		if err != nil {
			t.Fatal(err)
		}

		ingress := ingressObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
		if err != nil {
			t.Fatal(err)
		}

		expectedManagers := []string{manager.owner.Field}
		for _, entry := range ingress.GetManagedFields() {
			if !containsItemString(expectedManagers, entry.Manager) {
				t.Log(entry)
				t.Errorf("Mismatch from expected values, want %v got %s", expectedManagers, entry.Manager)
			}
		}

		tlsMap, exists, err := unstructured.NestedSlice(ingress.Object, "spec", "tls")
		if err != nil {
			t.Fatalf("unexpected error while getting field from object: %s", err)
		}

		if exists {
			t.Errorf("spec.tls shouldn't be present, got %s", tlsMap)
		}
	})
}

func TestApply_Cleanup_Exclusions(t *testing.T) {
	kubectlManager := "kubectl-client-side-apply"
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	applyOpts := DefaultApplyOptions()
	applyOpts.Cleanup = ApplyCleanupOptions{
		Annotations: []string{corev1.LastAppliedConfigAnnotation},
		FieldManagers: []FieldManager{
			{
				Name:          "kubectl",
				OperationType: metav1.ManagedFieldsOperationApply,
			},
			{
				Name:          "kubectl",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
			{
				Name:          "before-first-apply",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
		},
		Exclusions: map[string]string{"cleanup/exclusion": "true"},
	}

	id := generateName("cleanup")
	objects, err := readManifest("testdata/test2.yaml", id)
	if err != nil {
		t.Fatal(err)
	}
	manager.SetOwnerLabels(objects, "app1", "default")

	_, deployObject := getFirstObject(objects, "Deployment", id)

	if err = normalize.UnstructuredList(objects); err != nil {
		t.Fatal(err)
	}

	t.Run("creates objects as kubectl", func(t *testing.T) {
		for _, object := range objects {
			obj := object.DeepCopy()
			if err := manager.client.Create(ctx, obj, client.FieldOwner(kubectlManager)); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("does not not remove kubectl manager", func(t *testing.T) {
		for _, object := range objects {
			object.SetAnnotations(map[string]string{"cleanup/exclusion": "true"})
		}

		changeSet, err := manager.ApplyAllStaged(ctx, objects, applyOpts)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(ConfiguredAction, entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}

		deploy := deployObject.DeepCopy()
		err = manager.Client().Get(ctx, client.ObjectKeyFromObject(deploy), deploy)
		if err != nil {
			t.Fatal(err)
		}

		found := false
		for _, entry := range deploy.GetManagedFields() {
			if entry.Manager == kubectlManager {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Mismatch from expected values, want %v manager", kubectlManager)
		}
	})
}

func TestApply_MissingNamespaceErr(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("err")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)
	unstructured.RemoveNestedField(configMap.Object, "metadata", "namespace")

	_, err = manager.ApplyAllStaged(ctx, []*unstructured.Unstructured{configMap}, DefaultApplyOptions())
	if !strings.Contains(err.Error(), "namespace not specified") {
		t.Fatal("Expected namespace not specified error")
	}
}

func containsItemString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// exampleCert was generated from crypto/tls/generate_cert.go with the following command:
//
//	go run generate_cert.go  --rsa-bits 2048 --host example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h - from
//
// this example is from https://github.com/kubernetes/kubernetes/blob/04d2f336419b5a824cb96cb88462ef18a90d619d/staging/src/k8s.io/apiserver/pkg/util/webhook/validation_test.go
// Base64 encoded because caBundle field expects base64 string when stored in unstructured.Unstructured
var exampleCert = base64.StdEncoding.EncodeToString([]byte(`-----BEGIN CERTIFICATE-----
MIIDIDCCAgigAwIBAgIRALYg7UBIx7aeUpwohjIBhUEwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBANJuxq11hL2nB6nygf5/q7JRkPZCYuXwkaqZm7Bk8e9+WzEy9/EW
QtRP92IuKB8XysLY7a/vh9WOcUMw9zBICP754pBIUjgt2KveEYABDSkrAVWIGIO9
IN6crS3OvHiMKyShCvqMMho9wxyTbtnl3lrlcxVyLCmMahnoSyIwWiQ3TMT81eKt
FGEYXa8XEIJJFRX6wxtCgw0PqQy/NLM+G1QvYyKLSLm2cKUGH1A9RfAlMzsICOOf
Rx+/zCAgAfXnjg0SUXfgOjc/Y8EdVyMmBfCWMfovbpwCwULxlEDHHsjVZy5azZjm
E2AYW94BSdRd745M7fudchS6+9rGJi9lc5kCAwEAAaNvMG0wDgYDVR0PAQH/BAQD
AgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0O
BBYEFL/WGYyHD90dPKo8SswyPSydkwG/MBYGA1UdEQQPMA2CC2V4YW1wbGUuY29t
MA0GCSqGSIb3DQEBCwUAA4IBAQAS9qnl6mTF/HHRZSfQypxBj1lsDwYz99PsDAyw
hoXetTVmkejsPe9EcQ5eBRook6dFIevXN9bY5dxYSjWoSg/kdsihJ3FsJsmAQEtK
eM8ko9uvtZ+i0LUfg2l3kima1/oX0MCvnuePGgl7quyBhGUeg5tOudiX07hETWPW
Kt/FgMvfzK63pqcJpLj2+2pnmieV3ploJjw1sIAboR3W5LO/9XgRK3h1vr1BbplZ
dhv6TGB0Y1Zc9N64gh0A3xDOrBSllAWYw/XM6TodhvahFyE48fYSFBZVfZ3TZTfd
Bdcg8G2SMXDSZoMBltEIO7ogTjNAqNUJ8MWZFNZz6HnE8UJC
-----END CERTIFICATE-----`))

func TestResourceManager_ApplyAllStaged_CRDWebhookCABundle(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	t.Run("removes invalid CA bundle and applies successfully", func(t *testing.T) {
		id := generateName("remove-ca-bundle-invalid")
		objects, err := readManifest("testdata/test11.yaml", id)
		if err != nil {
			t.Fatal(err)
		}
		_, crd := getFirstObject(objects, "CustomResourceDefinition", "webhooks.example.com")
		uniqueGroup := fmt.Sprintf("%s.example.com", id)
		crdName := fmt.Sprintf("webhooks.%s", uniqueGroup)
		crd.SetName(crdName)
		err = unstructured.SetNestedField(crd.Object, uniqueGroup, "spec", "group")
		if err != nil {
			t.Fatal(err)
		}
		invalidCABundle := "invalid-cert-data"
		err = unstructured.SetNestedField(crd.Object, invalidCABundle, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}
		manager.SetOwnerLabels(objects, "test", "default")
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if entry.Action != CreatedAction {
				t.Errorf("Expected %s, got %s for %s", CreatedAction, entry.Action, entry.Subject)
			}
		}
		crdClone := crd.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(crdClone), crdClone)
		if err != nil {
			t.Fatal(err)
		}
		clusterCABundle, found, err := unstructured.NestedString(crdClone.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}
		if found && clusterCABundle != "" {
			t.Errorf("Expected invalid CA bundle to be removed, but found: %s", clusterCABundle)
		}
	})
	t.Run("removes valid CA bundle non base64 encoded and applies successfully", func(t *testing.T) {
		id := generateName("remove-ca-bundle-non-base64")
		objects, err := readManifest("testdata/test11.yaml", id)
		if err != nil {
			t.Fatal(err)
		}
		_, crd := getFirstObject(objects, "CustomResourceDefinition", "webhooks.example.com")
		uniqueGroup := fmt.Sprintf("%s.example.com", id)
		crdName := fmt.Sprintf("webhooks.%s", uniqueGroup)
		crd.SetName(crdName)
		err = unstructured.SetNestedField(crd.Object, uniqueGroup, "spec", "group")
		if err != nil {
			t.Fatal(err)
		}
		invalidCABundle, _ := base64.StdEncoding.DecodeString(exampleCert)
		err = unstructured.SetNestedField(crd.Object, string(invalidCABundle), "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}
		manager.SetOwnerLabels(objects, "test", "default")
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if entry.Action != CreatedAction {
				t.Errorf("Expected %s, got %s for %s", CreatedAction, entry.Action, entry.Subject)
			}
		}
		crdClone := crd.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(crdClone), crdClone)
		if err != nil {
			t.Fatal(err)
		}
		clusterCABundle, found, err := unstructured.NestedString(crdClone.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}
		if found && clusterCABundle != "" {
			t.Errorf("Expected invalid CA bundle to be removed, but found: %s", clusterCABundle)
		}
	})
	t.Run("preserves valid CA bundle and applies successfully", func(t *testing.T) {
		id := generateName("remove-ca-bundle-valid")
		objects, err := readManifest("testdata/test11.yaml", id)
		if err != nil {
			t.Fatal(err)
		}
		_, crd := getFirstObject(objects, "CustomResourceDefinition", "webhooks.example.com")
		uniqueGroup := fmt.Sprintf("%s.example.com", id)
		crdName := fmt.Sprintf("webhooks.%s", uniqueGroup)
		crd.SetName(crdName)
		err = unstructured.SetNestedField(crd.Object, uniqueGroup, "spec", "group")
		if err != nil {
			t.Fatal(err)
		}
		err = unstructured.SetNestedField(crd.Object, exampleCert, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}
		manager.SetOwnerLabels(objects, "test", "default")
		changeSet, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range changeSet.Entries {
			if entry.Action != CreatedAction {
				t.Errorf("Expected %s, got %s for %s", CreatedAction, entry.Action, entry.Subject)
			}
		}
		crdClone := crd.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(crdClone), crdClone)
		if err != nil {
			t.Fatal(err)
		}
		clusterCABundle, found, err := unstructured.NestedString(crdClone.Object, "spec", "conversion", "webhook", "clientConfig", "caBundle")
		if err != nil {
			t.Fatal(err)
		}
		if !found || clusterCABundle != exampleCert {
			t.Errorf("Expected valid CA bundle to be preserved, got: %s", clusterCABundle)
		}
	})
}

func TestApplyAllStaged_AppliesRoleAndRoleBinding(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("custom-stage")

	// Create a non-cluster-admin client to ensure dry-run checks are not bypassed.
	customStageNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
	}
	if err := manager.client.Create(ctx, customStageNS); err != nil {
		t.Fatal(err)
	}
	customStageSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id,
			Namespace: id,
		},
	}
	if err := manager.client.Create(ctx, customStageSA); err != nil {
		t.Fatal(err)
	}
	customStageCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{"roles", "rolebindings"},
				Verbs:     []string{"create", "update", "delete", "get", "list", "watch", "patch"},
			},
			// Grant the same permissions that the test Role will grant,
			// so RBAC escalation prevention allows creating the Role and
			// RoleBinding.
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	if err := manager.client.Create(ctx, customStageCR); err != nil {
		t.Fatal(err)
	}
	customStageCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     customStageCR.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      customStageSA.Name,
			Namespace: customStageSA.Namespace,
		}},
	}
	if err := manager.client.Create(ctx, customStageCRB); err != nil {
		t.Fatal(err)
	}

	// Create a manager with the non-cluster-admin client
	customStageManager := *manager
	customStageCfg := rest.CopyConfig(cfg)
	customStageCfg.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:%s", customStageSA.Namespace, customStageSA.Name),
	}
	customStageClient, err := client.New(customStageCfg, client.Options{
		Mapper: restMapper,
	})
	if err != nil {
		t.Fatal(err)
	}
	customStageManager.client = customStageClient

	// Create a Role and RoleBinding that references the Role.
	role := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "Role",
			"metadata": map[string]any{
				"name":      "role",
				"namespace": id,
			},
			"rules": []any{
				map[string]any{
					"apiGroups": []any{""},
					"resources": []any{"configmaps"},
					"verbs":     []any{"get", "list"},
				},
			},
		},
	}

	roleBinding := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata": map[string]any{
				"name":      "role-binding",
				"namespace": id,
			},
			"roleRef": map[string]any{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Role",
				"name":     "role",
			},
			"subjects": []any{
				map[string]any{
					"kind":      "ServiceAccount",
					"name":      "default",
					"namespace": id,
				},
			},
		},
	}

	objects := []*unstructured.Unstructured{roleBinding, role}

	t.Run("does not apply Role and RoleBinding together without custom stage", func(t *testing.T) {
		opts := DefaultApplyOptions()

		_, err := customStageManager.ApplyAllStaged(ctx, objects, opts)
		if err == nil {
			t.Fatal("Expected error when applying RoleBinding before Role, got none")
		}

		// Assert the error is a DryRunErr
		var dryRunErr *ssaerrors.DryRunErr
		if !errors.As(err, &dryRunErr) {
			t.Fatalf("Expected error to be *errors.DryRunErr, got %T", err)
		}

		// Assert the underlying error is NotFound
		if !apierrors.IsNotFound(dryRunErr.Unwrap()) {
			t.Errorf("Expected underlying error to be NotFound, got: %v", dryRunErr.Unwrap())
		}

		// Assert the NotFound is for the Role that the RoleBinding references
		var statusErr *apierrors.StatusError
		if !errors.As(dryRunErr.Unwrap(), &statusErr) {
			t.Fatalf("Expected underlying error to be *apierrors.StatusError, got %T", dryRunErr.Unwrap())
		}
		if statusErr.ErrStatus.Details == nil || statusErr.ErrStatus.Details.Name != "role" {
			t.Errorf("Expected NotFound to be for the Role named 'role', got: %+v", statusErr.ErrStatus.Details)
		}

		// Assert the involved object is the RoleBinding
		if dryRunErr.InvolvedObject().GetKind() != "RoleBinding" {
			t.Errorf("Expected involved object to be RoleBinding, got %s", dryRunErr.InvolvedObject().GetKind())
		}
	})

	t.Run("applies Role and RoleBinding together with Role in custom stage", func(t *testing.T) {
		opts := DefaultApplyOptions()
		opts.CustomStageKinds = map[schema.GroupKind]struct{}{
			{Group: "rbac.authorization.k8s.io", Kind: "Role"}: {},
		}

		changeSet, err := customStageManager.ApplyAllStaged(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		// Verify both objects were created
		if len(changeSet.Entries) != 2 {
			t.Errorf("Expected 2 entries, got %d", len(changeSet.Entries))
		}

		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(entry.Action, CreatedAction); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}
	})
}

// Tests for the ApplyOptions.MigrateAPIVersion feature.
//
// The feature rewrites the apiVersion label on every managed fields entry
// of an object before applying, so the API server doesn't try to validate
// a stale entry against an older version's schema. This is needed after a
// CRD introduces a new version that adds fields with default values: if
// any entry is still tagged with the old apiVersion, the server will try
// to locate those defaulted fields in the old schema and fail with
// "field not declared in schema".
//
// The scenario these tests build, at a high level:
//
//  1. Create a CRD with only v1beta1 (served, storage).
//  2. Create a CR at v1beta1. Our field manager now owns a v1beta1 entry.
//  3. Update the CRD to add v1 as the served/storage version.
//  4. Re-apply the CR at v1. This is a no-op, so our managed fields entry
//     stays tagged v1beta1 even though the object is applied at v1.
//  5. Update the CRD again to add a new field in the v1 schema with a
//     default value. This field does not exist in v1beta1.
//  6. Try to re-apply at v1. Without migration, the dry-run fails with
//     "field not declared in schema" because our entry is still at
//     v1beta1. With migration, it succeeds.
//
// The "real-world external-secrets" test layers one extra twist on top:
// a different field manager (external-secrets itself) owns a status
// subresource entry at v1beta1, and our own entry is already at v1. In
// that state, migrating only our own entries is not enough — we also
// need to rewrite the third-party entry, otherwise the server still fails
// the same way.

type applyFunc func(ctx context.Context, object *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error)

func applyOneViaApply(ctx context.Context, object *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error) {
	return manager.Apply(ctx, object, opts)
}

func applyOneViaApplyAll(ctx context.Context, object *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error) {
	changeSet, err := manager.ApplyAll(ctx, []*unstructured.Unstructured{object}, opts)
	if err != nil {
		return nil, err
	}
	if changeSet == nil || len(changeSet.Entries) != 1 {
		return nil, fmt.Errorf("expected 1 change set entry, got %d", len(changeSet.Entries))
	}
	return &changeSet.Entries[0], nil
}

// migrateScenarioOpts tweaks setupMigrateAPIVersionEnv for the real-world
// ExternalSecret reproduction. Both options default to false and are only
// set by TestApply_MigrateAPIVersion_RealWorldExternalSecrets.
type migrateScenarioOpts struct {
	// injectStaleThirdPartyStatus writes the CR's status subresource at
	// v1beta1 under a different field manager via a non-SSA Update.
	// This mirrors how external-secrets writes status: client-go's
	// UpdateStatus sends a PUT /status which records a managed fields
	// entry pinned to whatever apiVersion was used for the Update.
	injectStaleThirdPartyStatus bool
	// promoteOwnEntryToV1 triggers an extra drift-inducing SSA apply at
	// v1 after v1 becomes served but BEFORE the v1 schema gains the new
	// defaulted field. This moves our own field manager's entry from
	// v1beta1 to v1, matching the state of the real-world user object
	// where kustomize-controller had already re-applied at v1.
	promoteOwnEntryToV1 bool
}

const secondManagerName = "other-controller"

// setupMigrateAPIVersionEnv runs steps 1-5 of the scenario described at
// the top of this block and returns the CR ready to be re-applied in
// step 6. All apply operations go through the provided applyFunc so the
// caller can exercise ResourceManager.Apply or ResourceManager.ApplyAll.
func setupMigrateAPIVersionEnv(t *testing.T, ctx context.Context, id string, apply applyFunc, scenarioOpts migrateScenarioOpts) *unstructured.Unstructured {
	t.Helper()

	group := fmt.Sprintf("%s.example.com", id)
	crdName := fmt.Sprintf("widgets.%s", group)

	// Schema mirrors the ExternalSecret CRD shape that reproduces the
	// external-secrets 2.3 upgrade failure: spec.dataFrom is a list of
	// objects whose nested extract object gains a defaulted field in v1.
	specSchema := func(includeDefault bool) map[string]any {
		extractProps := map[string]any{
			"key": map[string]any{"type": "string"},
		}
		statusProps := map[string]any{
			"phase": map[string]any{"type": "string"},
		}
		if includeDefault {
			extractProps["nullBytePolicy"] = map[string]any{
				"type":    "string",
				"default": "Ignore",
			}
		}
		return map[string]any{
			"openAPIV3Schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"spec": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"refreshInterval": map[string]any{"type": "string"},
							"dataFrom": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"extract": map[string]any{
											"type":       "object",
											"properties": extractProps,
										},
									},
								},
							},
						},
					},
					"status": map[string]any{
						"type":       "object",
						"properties": statusProps,
					},
				},
			},
		}
	}

	v1beta1Schema := specSchema(false)
	v1Schema := specSchema(false)
	v1SchemaWithDefault := specSchema(true)

	makeCRD := func(versions []any) *unstructured.Unstructured {
		if scenarioOpts.injectStaleThirdPartyStatus {
			for _, v := range versions {
				v.(map[string]any)["subresources"] = map[string]any{
					"status": map[string]any{},
				}
			}
		}
		return &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": crdName,
				},
				"spec": map[string]any{
					"group": group,
					"names": map[string]any{
						"kind":     "Widget",
						"listKind": "WidgetList",
						"plural":   "widgets",
						"singular": "widget",
					},
					"scope":    "Namespaced",
					"versions": versions,
				},
			},
		}
	}

	waitForGVK := func(gvk schema.GroupVersionKind) error {
		return wait.PollUntilContextCancel(ctx, 200*time.Millisecond, true, func(ctx context.Context) (bool, error) {
			probe := &unstructured.Unstructured{}
			probe.SetGroupVersionKind(gvk)
			err := manager.client.List(ctx, probe, client.InNamespace("default"), client.Limit(1))
			if err == nil {
				return true, nil
			}
			if meta.IsNoMatchError(err) {
				return false, nil
			}
			return false, err
		})
	}

	opts := DefaultApplyOptions()
	opts.MigrateAPIVersion = true

	// Step 1: create the CRD with only v1beta1.
	crd := makeCRD([]any{
		map[string]any{
			"name":    "v1beta1",
			"served":  true,
			"storage": true,
			"schema":  v1beta1Schema,
		},
	})
	entry, err := apply(ctx, crd, opts)
	if err != nil {
		t.Fatalf("failed to create CRD: %v", err)
	}
	if entry.Action != CreatedAction {
		t.Errorf("expected CRD CreatedAction, got %s", entry.Action)
	}
	t.Cleanup(func() {
		_ = manager.client.Delete(context.Background(), crd)
	})
	gvkV1beta1 := schema.GroupVersionKind{Group: group, Version: "v1beta1", Kind: "Widget"}
	if err := waitForGVK(gvkV1beta1); err != nil {
		t.Fatalf("v1beta1 not available: %v", err)
	}

	// Step 2: create the CR at v1beta1.
	cr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fmt.Sprintf("%s/v1beta1", group),
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "test-widget",
				"namespace": "default",
			},
			"spec": map[string]any{
				"dataFrom": []any{
					map[string]any{
						"extract": map[string]any{
							"key": "bar",
						},
					},
				},
			},
		},
	}
	entry, err = apply(ctx, cr, opts)
	if err != nil {
		t.Fatalf("failed to create CR at v1beta1: %v", err)
	}
	if entry.Action != CreatedAction {
		t.Errorf("expected CR CreatedAction, got %s", entry.Action)
	}

	// Inject the third-party status entry at v1beta1, using a non-SSA
	// Update so the resulting managed fields entry has operation=Update,
	// matching how external-secrets writes its status.
	if scenarioOpts.injectStaleThirdPartyStatus {
		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(gvkV1beta1)
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cr), current); err != nil {
			t.Fatalf("failed to get CR before status update: %v", err)
		}
		if err := unstructured.SetNestedField(current.Object, "Ready", "status", "phase"); err != nil {
			t.Fatalf("failed to set status.phase: %v", err)
		}
		if err := manager.client.Status().Update(ctx, current, client.FieldOwner(secondManagerName)); err != nil {
			t.Fatalf("failed to write third-party status at v1beta1: %v", err)
		}
	}

	// Step 3: add v1 as the served/storage version. When a third-party
	// entry is present we keep v1beta1 served so the API server doesn't
	// drop that entry on the next apply, and we can observe the stale
	// entry surviving into step 6.
	crdV1 := makeCRD([]any{
		map[string]any{
			"name":    "v1beta1",
			"served":  scenarioOpts.injectStaleThirdPartyStatus,
			"storage": false,
			"schema":  v1beta1Schema,
		},
		map[string]any{
			"name":    "v1",
			"served":  true,
			"storage": true,
			"schema":  v1Schema,
		},
	})
	if _, err := apply(ctx, crdV1, opts); err != nil {
		t.Fatalf("failed to add v1 to CRD: %v", err)
	}
	gvkV1 := schema.GroupVersionKind{Group: group, Version: "v1", Kind: "Widget"}
	if err := waitForGVK(gvkV1); err != nil {
		t.Fatalf("v1 not available: %v", err)
	}

	// Step 4: re-apply the CR at v1. Since there's no drift and migration
	// is disabled, this is a no-op and our managed fields entry stays
	// tagged v1beta1. We also use this as an implicit assertion that the
	// object's apiVersion is reported as v1 while the managed entry is
	// still v1beta1 — that's the whole setup for step 5.
	crV1 := cr.DeepCopy()
	crV1.SetAPIVersion(fmt.Sprintf("%s/v1", group))
	entry, err = apply(ctx, crV1, DefaultApplyOptions())
	if err != nil {
		t.Fatalf("failed to apply CR at v1: %v", err)
	}
	if entry.Action != UnchangedAction {
		t.Errorf("expected UnchangedAction, got %s", entry.Action)
	}
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(gvkV1)
	if err := manager.client.Get(ctx, client.ObjectKeyFromObject(crV1), got); err != nil {
		t.Fatalf("failed to get CR: %v", err)
	}
	if expected := fmt.Sprintf("%s/v1", group); got.GetAPIVersion() != expected {
		t.Errorf("expected object apiVersion %s, got %s", expected, got.GetAPIVersion())
	}
	for _, mf := range got.GetManagedFields() {
		if mf.Manager != manager.owner.Field {
			continue
		}
		if expected := fmt.Sprintf("%s/v1beta1", group); mf.APIVersion != expected {
			t.Errorf("expected our managed field apiVersion %s, got %s", expected, mf.APIVersion)
		}
	}

	// Force the CR to be re-stored at v1 under a different field manager.
	// envtest doesn't run the storage-version migrator, so without this
	// the object would still be physically stored at v1beta1 and v1
	// defaulting wouldn't actually run on subsequent GETs. Using a
	// different manager means our own managed fields entry isn't touched
	// and stays at v1beta1.
	migratePatch := client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"annotations":{"test/rewritten":"true"}}}`))
	if err := manager.client.Patch(ctx, crV1.DeepCopy(), migratePatch, client.FieldOwner("storage-migrator")); err != nil {
		t.Fatalf("failed to re-store CR at v1: %v", err)
	}

	// Optionally promote our own entry from v1beta1 to v1 via a
	// drift-inducing SSA apply BEFORE the defaulted field lands in the
	// v1 schema. This matches the real-world user state.
	if scenarioOpts.promoteOwnEntryToV1 {
		promote := crV1.DeepCopy()
		labels := promote.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["test/promoted"] = "true"
		promote.SetLabels(labels)
		if err := manager.client.Patch(ctx, promote, client.Apply,
			client.FieldOwner(manager.owner.Field),
			client.ForceOwnership); err != nil {
			t.Fatalf("failed to promote own entry to v1: %v", err)
		}
	}

	// Step 5: add a new field with a default value to the v1 schema.
	crdV1WithDefault := makeCRD([]any{
		map[string]any{
			"name":    "v1beta1",
			"served":  scenarioOpts.injectStaleThirdPartyStatus,
			"storage": false,
			"schema":  v1beta1Schema,
		},
		map[string]any{
			"name":    "v1",
			"served":  true,
			"storage": true,
			"schema":  v1SchemaWithDefault,
		},
	})
	if _, err := apply(ctx, crdV1WithDefault, opts); err != nil {
		t.Fatalf("failed to add defaulted field to CRD v1: %v", err)
	}

	return crV1
}

func runMigrateAPIVersionScenario(t *testing.T, id string, apply applyFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	crV1 := setupMigrateAPIVersionEnv(t, ctx, id, apply, migrateScenarioOpts{})

	// With MigrateAPIVersion disabled, apply should fail with the
	// "field not declared in schema" dry-run error.
	_, err := apply(ctx, crV1.DeepCopy(), DefaultApplyOptions())
	if err == nil {
		t.Fatalf("expected apply to fail without MigrateAPIVersion, got nil")
	}
	var dryRunErr *ssaerrors.DryRunErr
	if !errors.As(err, &dryRunErr) {
		t.Errorf("expected *ssaerrors.DryRunErr, got %T: %v", err, err)
	} else if !strings.Contains(dryRunErr.Error(), "field not declared in schema") {
		t.Errorf("expected %q in error, got: %v", "field not declared in schema", dryRunErr)
	}

	// With MigrateAPIVersion enabled, apply should succeed and report
	// the object as configured.
	opts := DefaultApplyOptions()
	opts.MigrateAPIVersion = true
	entry, err := apply(ctx, crV1.DeepCopy(), opts)
	if err != nil {
		t.Fatalf("failed to apply CR with MigrateAPIVersion=true: %v", err)
	}
	if entry.Action != ConfiguredAction {
		t.Errorf("expected ConfiguredAction, got %s", entry.Action)
	}
}

func TestApply_MigrateAPIVersion(t *testing.T) {
	runMigrateAPIVersionScenario(t, generateName("migrate-api-version"), applyOneViaApply)
}

func TestApplyAll_MigrateAPIVersion(t *testing.T) {
	runMigrateAPIVersionScenario(t, generateName("migrate-api-version-all"), applyOneViaApplyAll)
}

// TestApply_MigrateAPIVersion_RealWorldExternalSecrets reproduces the
// exact shape of the external-secrets failure observed on a real cluster.
//
// The CR's managed fields on the user's object looked like this:
//
//   - kustomize-controller, apiVersion v1, Apply — owns spec fields.
//   - external-secrets, apiVersion v1beta1, Update, subresource=status —
//     owns status fields.
//   - external-secrets, apiVersion v1, Update — owns the finalizer.
//
// i.e. kustomize-controller's own entry was already at v1 (so migrating
// only our own entries is a no-op), but external-secrets still had a
// status entry pinned to v1beta1. That one stale entry is enough to make
// the next SSA apply at v1 fail with
// ".spec.dataFrom[0].extract.nullBytePolicy: field not declared in
// schema" once the CRD adds that defaulted field in v1.
//
// This test asserts two things:
//
//  1. Without migration, a plain SSA dry-run apply at v1 reproduces the
//     production error message. This confirms the reproduction is
//     correct and the setup matches the user's object.
//  2. With MigrateAPIVersion=true, ResourceManager.Apply fixes it by
//     rewriting every stale managed fields entry to v1 — including the
//     third-party status entry. If the fix regresses to only migrating
//     our own entries, this assertion fails.
func TestApply_MigrateAPIVersion_RealWorldExternalSecrets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id := generateName("migrate-api-version-real-world-eso")
	crV1 := setupMigrateAPIVersionEnv(t, ctx, id, applyOneViaApply,
		migrateScenarioOpts{
			injectStaleThirdPartyStatus: true,
			promoteOwnEntryToV1:         true,
		})

	bareErr := manager.client.Patch(ctx, crV1.DeepCopy(), client.Apply,
		client.DryRunAll,
		client.FieldOwner(manager.owner.Field),
		client.ForceOwnership)
	if bareErr == nil {
		t.Fatalf("expected bare dry-run apply to fail, got nil")
	}
	if !strings.Contains(bareErr.Error(), ".spec.dataFrom[0].extract.nullBytePolicy: field not declared in schema") {
		t.Errorf("expected the real-world external-secrets error, got: %v", bareErr)
	}

	opts := DefaultApplyOptions()
	opts.MigrateAPIVersion = true
	if _, err := manager.Apply(ctx, crV1.DeepCopy(), opts); err != nil {
		t.Errorf("expected Apply with MigrateAPIVersion=true to fix the real-world scenario, got: %v", err)
	}
}
