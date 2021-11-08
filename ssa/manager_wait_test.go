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
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestWaitForSet(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("wait")
	objects, err := readManifest("testdata/test5.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "infra", "default")

	_, crd := getFirstObject(objects, "CustomResourceDefinition", "clustertests.testing.fluxcd.io")
	_, cr := getFirstObject(objects, "ClusterTest", id)

	t.Run("waits for CRD and CR", func(t *testing.T) {
		cs, err := manager.Apply(ctx, crd, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := manager.WaitForSet([]object.ObjMetadata{cs.ObjMetadata}, DefaultWaitOptions()); err != nil {
			t.Errorf("wait failed for CRD: %v", err)
		}

		changeSet, err := manager.ApplyAll(ctx, objects, false)
		if err != nil {
			t.Fatal(err)
		}

		if err := manager.WaitForSet(changeSet.ToObjMetadataSet(), WaitOptions{time.Second, 3 * time.Second}); err == nil {
			t.Error("wanted wait error due to observedGeneration < generation")
		}

		clusterCR := &unstructured.Unstructured{}
		clusterCR.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "testing.fluxcd.io",
			Kind:    "ClusterTest",
			Version: "v1",
		})
		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(cr), clusterCR); err != nil {
			t.Fatal(err)
		}

		var observedGeneration int64
		observedGeneration = 1
		clusterCR.SetManagedFields(nil)
		err = unstructured.SetNestedField(clusterCR.Object, observedGeneration, "status", "observedGeneration")
		if err != nil {
			t.Fatal(err)
		}

		opts := []client.PatchOption{
			client.ForceOwnership,
			client.FieldOwner(manager.owner.Field),
		}
		if err := manager.client.Status().Patch(ctx, clusterCR, client.Apply, opts...); err != nil {
			t.Fatal(err)
		}

		if err := manager.WaitForSet(changeSet.ToObjMetadataSet(), DefaultWaitOptions()); err != nil {
			t.Errorf("wait error: %v", err)
		}
	})
}
