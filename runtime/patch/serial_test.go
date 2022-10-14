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

package patch

import (
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func TestSerialPatcher(t *testing.T) {
	t.Run("should be able to patch object consecutively", func(t *testing.T) {
		g := NewWithT(t)

		testFinalizer := "test.finalizer.flux.io"
		obj := &testdata.Fake{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    "default",
			},
		}

		ownedConditions := []string{
			meta.ReadyCondition,
			meta.ReconcilingCondition,
			meta.StalledCondition,
		}

		t.Log("Creating the object")
		g.Expect(env.Create(ctx, obj)).To(Succeed())
		defer func() {
			g.Expect(env.Delete(ctx, obj)).To(Succeed())
		}()
		key := client.ObjectKeyFromObject(obj)

		t.Log("Checking that the object has been created")
		g.Eventually(func() error {
			objAfter := obj.DeepCopy()
			if err := env.Get(ctx, key, objAfter); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		t.Log("Creating a new serial patcher")
		patcher := NewSerialPatcher(obj, env.Client)

		t.Log("Add a finalizer")
		controllerutil.AddFinalizer(obj, testFinalizer)

		t.Log("Patching the object")
		g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

		t.Log("Validating that the finalizer is added")
		g.Eventually(func() bool {
			objAfter := obj.DeepCopy()
			if err := env.Get(ctx, key, objAfter); err != nil {
				return false
			}
			return reflect.DeepEqual(obj.Finalizers, objAfter.Finalizers)
		}, timeout).Should(BeTrue())

		t.Log("Add status condition")
		conditions.MarkReconciling(obj, "reason", "")
		conditions.MarkFalse(obj, meta.ReadyCondition, "reason", "")

		t.Log("Patch the object")
		patchOpts := []Option{
			WithOwnedConditions{ownedConditions},
		}
		g.Expect(patcher.Patch(ctx, obj, patchOpts...))

		t.Log("Validating that the conditions are added")
		g.Eventually(func() bool {
			objAfter := obj.DeepCopy()
			if err := env.Get(ctx, key, objAfter); err != nil {
				return false
			}
			return !conditions.IsReady(objAfter) && conditions.IsReconciling(objAfter)
		}, timeout).Should(BeTrue())

		t.Log("Remove and update conditions")
		conditions.Delete(obj, meta.ReconcilingCondition)
		conditions.MarkTrue(obj, meta.ReadyCondition, "reason", "")

		t.Log("Patch the object")
		g.Expect(patcher.Patch(ctx, obj, patchOpts...))

		t.Log("Validating that the conditions are updated")
		g.Eventually(func() bool {
			objAfter := obj.DeepCopy()
			if err := env.Get(ctx, key, objAfter); err != nil {
				return false
			}
			return conditions.IsReady(objAfter) && !conditions.IsReconciling(objAfter)
		})

		t.Log("Remove finalizer")
		controllerutil.RemoveFinalizer(obj, testFinalizer)

		t.Log("Patch the object")
		g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

		t.Log("Validating that the finalizer is removed")
		g.Eventually(func() bool {
			objAfter := obj.DeepCopy()
			if err := env.Get(ctx, key, objAfter); err != nil {
				return false
			}
			return len(objAfter.Finalizers) == 0
		}, timeout).Should(BeTrue())
	})
}
