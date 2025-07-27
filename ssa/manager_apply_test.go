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
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
