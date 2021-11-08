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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	secretName, secret := getFirstObject(objects, "Secret", id)
	crbName, crb := getFirstObject(objects, "ClusterRoleBinding", id)

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
			expected = append(expected, FmtUnstructured(object))
		}

		// verify the change set contains only created actions
		var output []string
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(entry.Action, string(CreatedAction)); diff != "" {
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
		var output []string
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(string(UnchangedAction), entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s\n%v", diff, changeSet)
			}
			output = append(output, entry.Subject)
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
				if diff := cmp.Diff(string(ConfiguredAction), entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(string(UnchangedAction), entry.Action); diff != "" {
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
		expectedErr := fmt.Sprintf("%s invalid, error: secret is immutable", FmtUnstructured(secret))
		if diff := cmp.Diff(expectedErr, err.Error()); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("force applies immutable secret", func(t *testing.T) {
		// force apply
		changeSet, err := manager.ApplyAllStaged(ctx, objects, ApplyOptions{Force: true, WaitTimeout: timeout})
		if err != nil {
			t.Fatal(err)
		}

		// verify the secret was recreated
		for _, entry := range changeSet.Entries {
			if entry.Subject == secretName {
				if diff := cmp.Diff(string(CreatedAction), entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(string(UnchangedAction), entry.Action); diff != "" {
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

	t.Run("recreates immutable RBAC", func(t *testing.T) {
		// update roleRef
		err = unstructured.SetNestedField(crb.Object, "test", "roleRef", "name")
		if err != nil {
			t.Fatal(err)
		}

		// force apply
		changeSet, err := manager.ApplyAllStaged(ctx, objects, ApplyOptions{Force: true, WaitTimeout: timeout})
		if err != nil {
			t.Fatal(err)
		}

		// verify the binding was recreated
		for _, entry := range changeSet.Entries {
			if entry.Subject == crbName {
				if diff := cmp.Diff(string(CreatedAction), entry.Action); diff != "" {
					t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
				}
				break
			}
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

	if err := SetNativeKindsDefaults(objects); err != nil {
		t.Fatal(err)
	}

	t.Run("creates objects", func(t *testing.T) {
		// create objects
		_, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
	})
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

	if err := SetNativeKindsDefaults(objects); err != nil {
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
			if entry.Action != string(UnchangedAction) {
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

		// apply with exclusions
		changeSet, err := manager.ApplyAll(ctx, objects, ApplyOptions{
			Force:       false,
			Exclusions:  meta,
			WaitTimeout: time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if entry.Action != string(UnchangedAction) {
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
			if entry.Action != string(ConfiguredAction) && entry.Subject == FmtUnstructured(configMap) {
				t.Errorf("Expected %s, got %s", ConfiguredAction, entry.Action)
			}
		}
	})

}
