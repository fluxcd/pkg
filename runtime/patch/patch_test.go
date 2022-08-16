/*
Copyright 2017 The Kubernetes Authors.
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


This file is modified from the source at
https://github.com/kubernetes-sigs/cluster-api/tree/d2faf482116114c4075da1390d905742e524ff89/util/patch/patch_test.go,
and initially adapted to work with the `conditions` and `testenv` packages, and `metav1.Condition` types.
*/

package patch

import (
	"reflect"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/conditions/testdata"
)

func TestPatchHelper(t *testing.T) {
	t.Run("should patch an unstructured object", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       "Fake",
				"apiVersion": testdata.FakeGroupVersion.Group + "/" + testdata.FakeGroupVersion.Version,
				"metadata": map[string]interface{}{
					"generateName": "test-",
					"namespace":    "default",
				},
			},
		}

		t.Run("adding an owner reference, preserving its status", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()

			t.Log("Creating the unstructured object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			obj.Object["status"] = map[string]interface{}{
				"observedValue": "arbitrary",
			}
			g.Expect(env.Status().Update(ctx, obj)).To(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Modifying the OwnerReferences")
			refs := []metav1.OwnerReference{
				{
					APIVersion: "fake.toolkit.fluxcd.io/v1",
					Kind:       "Fake",
					Name:       "test",
					UID:        types.UID("fake-uid"),
				},
			}
			obj.SetOwnerReferences(refs)

			t.Log("Patching the unstructured object")
			g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

			t.Log("Validating that the status has been preserved")
			g.Expect(err).NotTo(HaveOccurred())
			statusStrV, found, err := unstructured.NestedString(obj.Object, "status", "observedValue")
			g.Expect(err).To(BeNil())
			g.Expect(found).To(BeTrue())
			g.Expect(statusStrV).To(Equal("arbitrary"))

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}
				return reflect.DeepEqual(obj.GetOwnerReferences(), objAfter.GetOwnerReferences())
			}, timeout).Should(BeTrue())
		})
	})

	t.Run("Should patch conditions", func(t *testing.T) {
		t.Run("on a corev1.Node object", func(t *testing.T) {
			g := NewWithT(t)

			conditionTime := metav1.Date(2015, 1, 1, 12, 0, 0, 0, metav1.Now().Location())

			obj := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "node-patch-test-",
					Annotations: map[string]string{
						"test": "1",
					},
				},
			}

			t.Log("Creating a Node object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.GetName()}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Appending a new condition")
			condition := corev1.NodeCondition{
				Type:               "CustomCondition",
				Status:             corev1.ConditionTrue,
				LastHeartbeatTime:  conditionTime,
				LastTransitionTime: conditionTime,
				Reason:             "reason",
				Message:            "message",
			}
			obj.Status.Conditions = append(obj.Status.Conditions, condition)

			t.Log("Patching the Node")
			g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				g.Expect(env.Get(ctx, key, objAfter)).To(Succeed())

				ok, _ := ContainElement(condition).Match(objAfter.Status.Conditions)
				return ok
			}, timeout).Should(BeTrue())
		})

		t.Run("on a testdata.Fake object", func(t *testing.T) {
			obj := &testdata.Fake{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    "default",
				},
			}

			t.Run("should set field owner", func(t *testing.T) {
				g := NewWithT(t)

				obj := obj.DeepCopy()
				g.Expect(env.Create(ctx, obj)).To(Succeed())
				defer func() {
					g.Expect(env.Delete(ctx, obj)).To(Succeed())
				}()
				key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

				t.Log("Checking that the object has been created")
				g.Eventually(func() error {
					obj := obj.DeepCopy()
					if err := env.Get(ctx, key, obj); err != nil {
						return err
					}
					return nil
				}).Should(Succeed())

				t.Log("Creating a new patch helper")
				patcher, err := NewHelper(obj, env)
				g.Expect(err).NotTo(HaveOccurred())

				t.Log("Marking Ready=True")
				obj.Spec.Value = "foo"
				conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "")

				t.Log("Patching the object with field owner")
				fieldOwner := "test-owner"
				g.Expect(patcher.Patch(ctx, obj, WithFieldOwner(fieldOwner))).To(Succeed())

				t.Log("Validating the status subresource is managed")
				g.Eventually(func() bool {
					objAfter := obj.DeepCopy()
					if err := env.Get(ctx, key, objAfter); err != nil {
						return false
					}
					for _, v := range objAfter.ManagedFields {
						if v.Subresource == "status" {
							return v.Manager == fieldOwner
						}
					}
					return false
				}, timeout).Should(BeTrue())

			})

			t.Run("should mark it ready", func(t *testing.T) {
				g := NewWithT(t)

				obj := obj.DeepCopy()

				t.Log("Creating the object")
				g.Expect(env.Create(ctx, obj)).To(Succeed())
				defer func() {
					g.Expect(env.Delete(ctx, obj)).To(Succeed())
				}()
				key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

				t.Log("Checking that the object has been created")
				g.Eventually(func() error {
					obj := obj.DeepCopy()
					if err := env.Get(ctx, key, obj); err != nil {
						return err
					}
					return nil
				}).Should(Succeed())

				t.Log("Creating a new patch helper")
				patcher, err := NewHelper(obj, env)
				g.Expect(err).NotTo(HaveOccurred())

				t.Log("Marking Ready=True")
				conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "")

				t.Log("Patching the object")
				g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

				t.Log("Validating the object has been updated")
				g.Eventually(func() bool {
					objAfter := obj.DeepCopy()
					if err := env.Get(ctx, key, objAfter); err != nil {
						return false
					}
					return cmp.Equal(obj.Status.Conditions, objAfter.Status.Conditions)
				}, timeout).Should(BeTrue())
			})

			t.Run("should recover if there is a resolvable conflict", func(t *testing.T) {
				g := NewWithT(t)

				obj := obj.DeepCopy()

				t.Log("Creating the object")
				g.Expect(env.Create(ctx, obj)).To(Succeed())
				defer func() {
					g.Expect(env.Delete(ctx, obj)).To(Succeed())
				}()
				key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

				t.Log("Checking that the object has been created")
				g.Eventually(func() error {
					obj := obj.DeepCopy()
					if err := env.Get(ctx, key, obj); err != nil {
						return err
					}
					return nil
				}).Should(Succeed())

				objCopy := obj.DeepCopy()

				t.Log("Marking a custom condition to be false")
				conditions.MarkFalse(objCopy, "TestCondition", "reason", "message")
				g.Expect(env.Status().Update(ctx, objCopy)).To(Succeed())

				t.Log("Validating that the local object's resource version is behind")
				g.Expect(obj.ResourceVersion).NotTo(Equal(objCopy.ResourceVersion))

				t.Log("Creating a new patch helper")
				patcher, err := NewHelper(obj, env)
				g.Expect(err).NotTo(HaveOccurred())

				t.Log("Marking Ready=True")
				conditions.MarkTrue(obj, meta.ReadyCondition, meta.SucceededReason, "")

				t.Log("Patching the object")
				g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

				t.Log("Validating the object has been updated")
				g.Eventually(func() bool {
					objAfter := obj.DeepCopy()
					if err := env.Get(ctx, key, objAfter); err != nil {
						return false
					}

					testConditionCopy := conditions.Get(objCopy, "TestCondition")
					testConditionAfter := conditions.Get(objAfter, "TestCondition")

					readyBefore := conditions.Get(obj, meta.ReadyCondition)
					readyAfter := conditions.Get(objAfter, meta.ReadyCondition)

					return cmp.Equal(testConditionCopy, testConditionAfter) && cmp.Equal(readyBefore, readyAfter)
				}, timeout).Should(BeTrue())
			})

			t.Run("should recover if there is a resolvable conflict, incl. patch spec and status", func(t *testing.T) {
				g := NewWithT(t)

				obj := obj.DeepCopy()

				t.Log("Creating the object")
				g.Expect(env.Create(ctx, obj)).To(Succeed())
				defer func() {
					g.Expect(env.Delete(ctx, obj)).To(Succeed())
				}()
				key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

				t.Log("Checking that the object has been created")
				g.Eventually(func() error {
					obj := obj.DeepCopy()
					if err := env.Get(ctx, key, obj); err != nil {
						return err
					}
					return nil
				}).Should(Succeed())

				objCopy := obj.DeepCopy()

				t.Log("Marking a custom condition to be false")
				conditions.MarkFalse(objCopy, "TestCondition", "reason", "message")
				g.Expect(env.Status().Update(ctx, objCopy)).To(Succeed())

				t.Log("Validating that the local object's resource version is behind")
				g.Expect(obj.ResourceVersion).NotTo(Equal(objCopy.ResourceVersion))

				t.Log("Creating a new patch helper")
				patcher, err := NewHelper(obj, env)
				g.Expect(err).NotTo(HaveOccurred())

				t.Log("Changing the object spec, status, and adding Ready=True condition")
				obj.Spec.Suspend = true
				obj.Spec.Value = "arbitrary"
				obj.Status.ObservedValue = "arbitrary"
				conditions.MarkTrue(obj, meta.ReadyCondition, "reason", "")

				t.Log("Patching the object")
				g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

				t.Log("Validating the object has been updated")
				objAfter := obj.DeepCopy()
				g.Eventually(func() bool {
					if err := env.Get(ctx, key, objAfter); err != nil {
						return false
					}

					testConditionCopy := conditions.Get(objCopy, "TestCondition")
					testConditionAfter := conditions.Get(objAfter, "TestCondition")

					readyBefore := conditions.Get(obj, meta.ReadyCondition)
					readyAfter := conditions.Get(objAfter, meta.ReadyCondition)

					return cmp.Equal(testConditionCopy, testConditionAfter) && cmp.Equal(readyBefore, readyAfter) &&
						obj.Spec.Suspend == objAfter.Spec.Suspend &&
						obj.Spec.Value == objAfter.Spec.Value &&
						obj.Status.ObservedValue == objAfter.Status.ObservedValue
				}, timeout).Should(BeTrue(), cmp.Diff(obj, objAfter))
			})

			t.Run("should return an error if there is an unresolvable conflict", func(t *testing.T) {
				g := NewWithT(t)

				obj := obj.DeepCopy()

				t.Log("Creating the object")
				g.Expect(env.Create(ctx, obj)).To(Succeed())
				defer func() {
					g.Expect(env.Delete(ctx, obj)).To(Succeed())
				}()
				key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

				t.Log("Checking that the object has been created")
				g.Eventually(func() error {
					obj := obj.DeepCopy()
					if err := env.Get(ctx, key, obj); err != nil {
						return err
					}
					return nil
				}).Should(Succeed())

				objCopy := obj.DeepCopy()

				t.Log("Marking a custom condition to be false")
				conditions.MarkFalse(objCopy, meta.ReadyCondition, "reason", "message")
				g.Expect(env.Status().Update(ctx, objCopy)).To(Succeed())

				t.Log("Validating that the local object's resource version is behind")
				g.Expect(obj.ResourceVersion).NotTo(Equal(objCopy.ResourceVersion))

				t.Log("Creating a new patch helper")
				patcher, err := NewHelper(obj, env)
				g.Expect(err).NotTo(HaveOccurred())

				t.Log("Marking Ready=True")
				conditions.MarkTrue(obj, meta.ReadyCondition, "reason", "")

				t.Log("Patching the object")
				g.Expect(patcher.Patch(ctx, obj)).NotTo(Succeed())

				t.Log("Validating the object has not been updated")
				g.Eventually(func() bool {
					objAfter := obj.DeepCopy()
					if err := env.Get(ctx, key, objAfter); err != nil {
						return false
					}
					ok, _ := ContainElement(objCopy.Status.Conditions[0]).Match(objAfter.Status.Conditions)
					return ok
				}, timeout).Should(BeTrue())
			})

			t.Run("should not return an error if there is an unresolvable conflict but the conditions is owned by the controller", func(t *testing.T) {
				g := NewWithT(t)

				obj := obj.DeepCopy()

				t.Log("Creating the object")
				g.Expect(env.Create(ctx, obj)).To(Succeed())
				defer func() {
					g.Expect(env.Delete(ctx, obj)).To(Succeed())
				}()
				key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

				t.Log("Checking that the object has been created")
				g.Eventually(func() error {
					obj := obj.DeepCopy()
					if err := env.Get(ctx, key, obj); err != nil {
						return err
					}
					return nil
				}).Should(Succeed())

				objCopy := obj.DeepCopy()

				t.Log("Marking a custom condition to be false")
				conditions.MarkFalse(objCopy, meta.ReadyCondition, "reason", "message")
				g.Expect(env.Status().Update(ctx, objCopy)).To(Succeed())

				t.Log("Validating that the local object's resource version is behind")
				g.Expect(obj.ResourceVersion).NotTo(Equal(objCopy.ResourceVersion))

				t.Log("Creating a new patch helper")
				patcher, err := NewHelper(obj, env)
				g.Expect(err).NotTo(HaveOccurred())

				t.Log("Marking Ready=True")
				conditions.MarkTrue(obj, meta.ReadyCondition, "reason", "")

				t.Log("Patching the object")
				g.Expect(patcher.Patch(ctx, obj, WithOwnedConditions{Conditions: []string{meta.ReadyCondition}})).To(Succeed())

				t.Log("Validating the object has been updated")
				g.Eventually(func() bool {
					objAfter := obj.DeepCopy()
					if err := env.Get(ctx, key, objAfter); err != nil {
						return false
					}

					readyBefore := conditions.Get(obj, meta.ReadyCondition)
					readyAfter := conditions.Get(objAfter, meta.ReadyCondition)
					return cmp.Equal(readyBefore, readyAfter)
				}, extendedTimeout).Should(BeTrue())
			})

			t.Run("should not return an error if there is an unresolvable conflict when force overwrite is enabled", func(t *testing.T) {
				g := NewWithT(t)

				obj := obj.DeepCopy()

				t.Log("Creating the object")
				g.Expect(env.Create(ctx, obj)).To(Succeed())
				defer func() {
					g.Expect(env.Delete(ctx, obj)).To(Succeed())
				}()
				key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

				t.Log("Checking that the object has been created")
				g.Eventually(func() error {
					obj := obj.DeepCopy()
					if err := env.Get(ctx, key, obj); err != nil {
						return err
					}
					return nil
				}).Should(Succeed())

				objCopy := obj.DeepCopy()

				t.Log("Marking a custom condition to be false")
				conditions.MarkFalse(objCopy, meta.ReadyCondition, "reason", "message")
				g.Expect(env.Status().Update(ctx, objCopy)).To(Succeed())

				t.Log("Validating that the local object's resource version is behind")
				g.Expect(obj.ResourceVersion).NotTo(Equal(objCopy.ResourceVersion))

				t.Log("Creating a new patch helper")
				patcher, err := NewHelper(obj, env)
				g.Expect(err).NotTo(HaveOccurred())

				t.Log("Marking Ready=True")
				conditions.MarkTrue(obj, meta.ReadyCondition, "reason", "")

				t.Log("Patching the object")
				g.Expect(patcher.Patch(ctx, obj, WithForceOverwriteConditions{})).To(Succeed())

				t.Log("Validating the object has been updated")
				g.Eventually(func() bool {
					objAfter := obj.DeepCopy()
					if err := env.Get(ctx, key, objAfter); err != nil {
						return false
					}

					readyBefore := conditions.Get(obj, meta.ReadyCondition)
					readyAfter := conditions.Get(objAfter, meta.ReadyCondition)

					return cmp.Equal(readyBefore, readyAfter)
				}, timeout).Should(BeTrue())
			})
		})
	})

	t.Run("Should patch a testdata.Fake", func(t *testing.T) {
		obj := &testdata.Fake{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    "default",
			},
		}

		t.Run("add a finalizer", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()

			t.Log("Creating the object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Adding a finalizer")
			obj.Finalizers = append(obj.Finalizers, "test.finalizers.fluxcd.io")

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}

				return reflect.DeepEqual(obj.Finalizers, objAfter.Finalizers)
			}, timeout).Should(BeTrue())
		})

		t.Run("removing finalizers", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()
			obj.Finalizers = append(obj.Finalizers, "test.finalizers.fluxcd.io")

			t.Log("Creating the object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Removing the finalizers")
			obj.SetFinalizers(nil)

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}

				return len(objAfter.Finalizers) == 0
			}, timeout).Should(BeTrue())
		})

		t.Run("updating spec", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()
			obj.ObjectMeta.Namespace = "default"

			t.Log("Creating the object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Updating the object spec")
			obj.Spec.Suspend = true
			obj.Spec.Value = "arbitrary"

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}

				return objAfter.Spec.Suspend == true &&
					reflect.DeepEqual(obj.Spec.Value, objAfter.Spec.Value)
			}, timeout).Should(BeTrue())
		})

		t.Run("updating status", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()

			t.Log("Creating the object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Updating the object status")
			obj.Status.ObservedValue = "arbitrary"

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}
				return reflect.DeepEqual(objAfter.Status, obj.Status)
			}, timeout).Should(BeTrue())
		})

		t.Run("updating both spec, status, and adding a condition", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()
			obj.ObjectMeta.Namespace = "default"

			t.Log("Creating the object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Updating the object spec")
			obj.Spec.Suspend = true
			obj.Spec.Value = "arbitrary"

			t.Log("Updating the object status")
			obj.Status.ObservedValue = "arbitrary"

			t.Log("Setting Ready condition")
			conditions.MarkTrue(obj, meta.ReadyCondition, "reason", "")

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj)).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}

				return obj.Status.ObservedValue == objAfter.Status.ObservedValue &&
					conditions.IsTrue(objAfter, meta.ReadyCondition) &&
					reflect.DeepEqual(obj.Spec, objAfter.Spec)
			}, timeout).Should(BeTrue())
		})
	})

	t.Run("Should update Status.ObservedGeneration when using WithStatusObservedGeneration option", func(t *testing.T) {
		obj := &testdata.Fake{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-fake",
				Namespace:    "default",
			},
			Spec: testdata.FakeSpec{
				Value: "arbitrary",
			},
		}

		t.Run("when updating spec", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()

			t.Log("Creating the Fake object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Updating the object spec")
			obj.Spec.Value = "arbitrary2"

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj, WithStatusObservedGeneration{})).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}

				return reflect.DeepEqual(obj.Spec, objAfter.Spec) &&
					obj.GetGeneration() == objAfter.Status.ObservedGeneration
			}, timeout).Should(BeTrue())
		})

		t.Run("when updating spec, status, and metadata", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()

			t.Log("Creating the Fake object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Updating the object spec")
			obj.Spec.Value = "arbitrary3"

			t.Log("Updating the object status")
			obj.Status.ObservedValue = "arbitrary3"

			t.Log("Updating the object metadata")
			obj.ObjectMeta.Annotations = map[string]string{
				"test1": "annotation",
			}

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj, WithStatusObservedGeneration{})).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}

				return reflect.DeepEqual(obj.Spec, objAfter.Spec) &&
					reflect.DeepEqual(obj.Status, objAfter.Status) &&
					obj.GetGeneration() == objAfter.Status.ObservedGeneration
			}, timeout).Should(BeTrue())
		})

		t.Run("without any changes", func(t *testing.T) {
			g := NewWithT(t)

			obj := obj.DeepCopy()

			t.Log("Creating the Fake object")
			g.Expect(env.Create(ctx, obj)).To(Succeed())
			defer func() {
				g.Expect(env.Delete(ctx, obj)).To(Succeed())
			}()
			key := client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}

			t.Log("Checking that the object has been created")
			g.Eventually(func() error {
				obj := obj.DeepCopy()
				if err := env.Get(ctx, key, obj); err != nil {
					return err
				}
				return nil
			}).Should(Succeed())

			obj.Status.ObservedGeneration = obj.GetGeneration()
			lastGeneration := obj.GetGeneration()
			g.Expect(env.Status().Update(ctx, obj))

			t.Log("Creating a new patch helper")
			patcher, err := NewHelper(obj, env)
			g.Expect(err).NotTo(HaveOccurred())

			t.Log("Patching the object")
			g.Expect(patcher.Patch(ctx, obj, WithStatusObservedGeneration{})).To(Succeed())

			t.Log("Validating the object has been updated")
			g.Eventually(func() bool {
				objAfter := obj.DeepCopy()
				if err := env.Get(ctx, key, objAfter); err != nil {
					return false
				}

				return lastGeneration == objAfter.Status.ObservedGeneration
			}, timeout).Should(BeTrue())
		})
	})

	t.Run("Should error if the object isn't the same", func(t *testing.T) {
		g := NewWithT(t)

		fake := &testdata.Fake{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    "default",
			},
		}

		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-node",
				Namespace:    "default",
			},
		}

		g.Expect(env.Create(ctx, fake)).To(Succeed())
		defer func() {
			g.Expect(env.Delete(ctx, fake)).To(Succeed())
		}()
		g.Expect(env.Create(ctx, node)).To(Succeed())
		defer func() {
			g.Expect(env.Delete(ctx, node)).To(Succeed())
		}()

		patcher, err := NewHelper(fake, env)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(patcher.Patch(ctx, node)).NotTo(Succeed())
	})
}
