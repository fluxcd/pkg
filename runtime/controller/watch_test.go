/*
Copyright 2023 The Flux authors

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

package controller_test

import (
	"fmt"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/controller"
)

func Test_WatchOptions_BindFlags(t *testing.T) {
	objects := []client.Object{
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "t0",
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "t1",
				Labels: map[string]string{
					"sharding.fluxcd.io/shard": "shard1",
				},
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "t2",
				Labels: map[string]string{
					"sharding.fluxcd.io/shard": "shard2",
				},
			},
		},
	}

	tests := []struct {
		name          string
		commandLine   []string
		objects       []client.Object
		expectedMatch []string
	}{
		{
			name:          "empty flag selects objects",
			commandLine:   []string{""},
			objects:       objects,
			expectedMatch: []string{"t0", "t1", "t2"},
		},
		{
			name:          "flag selects objects by label key val",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard=shard1"},
			objects:       objects,
			expectedMatch: []string{"t1"},
		},
		{
			name:          "flag selects objects by label exclusion expression",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard, sharding.fluxcd.io/shard notin (shard1)"},
			objects:       objects,
			expectedMatch: []string{"t2"},
		},
		{
			name:          "flag selects objects by label inclusion expression",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard in (shard1, shard2)"},
			objects:       objects,
			expectedMatch: []string{"t1", "t2"},
		},
		{
			name:          "flag selects objects with no matching labels",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard notin (shard1, shard2)"},
			objects:       objects,
			expectedMatch: []string{"t0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			opts := controller.WatchOptions{}
			opts.BindFlags(f)

			err := f.Parse(tt.commandLine)
			g.Expect(err).NotTo(HaveOccurred())

			sel, err := controller.GetWatchSelector(opts)
			g.Expect(err).NotTo(HaveOccurred())

			for _, object := range tt.objects {
				if sel.Matches(labels.Set(object.GetLabels())) {
					var found bool
					for _, match := range tt.expectedMatch {
						if object.GetName() == match {
							found = true
						}
					}
					g.Expect(found).To(BeTrue())
				} else {
					var found bool
					for _, match := range tt.expectedMatch {
						if object.GetName() == match {
							found = true
						}
					}
					g.Expect(found).ToNot(BeTrue())
				}
			}
		})
	}
}

func TestGetWatchConfigsPredicate(t *testing.T) {
	for _, tt := range []struct {
		name               string
		arguments          []string
		shouldMatchDefault bool
		shouldMatchCustom  bool
		shouldMatchEmpty   bool
		shouldMatchHelm    bool
	}{
		{
			name:               "default selector",
			arguments:          []string{},
			shouldMatchDefault: true,
			shouldMatchCustom:  false,
			shouldMatchEmpty:   false,
			shouldMatchHelm:    false,
		},
		{
			name:               "custom selector",
			arguments:          []string{"--watch-configs-label-selector=app=my-app"},
			shouldMatchDefault: false,
			shouldMatchCustom:  true,
			shouldMatchEmpty:   false,
			shouldMatchHelm:    false,
		},
		{
			name:               "empty selector",
			arguments:          []string{"--watch-configs-label-selector="},
			shouldMatchDefault: true,
			shouldMatchCustom:  true,
			shouldMatchEmpty:   true,
			shouldMatchHelm:    true,
		},
		{
			name:               "not helm",
			arguments:          []string{"--watch-configs-label-selector=owner!=helm"},
			shouldMatchDefault: true,
			shouldMatchCustom:  true,
			shouldMatchEmpty:   true,
			shouldMatchHelm:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Build predicate.
			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			var opts controller.WatchOptions
			opts.BindFlags(f)
			err := f.Parse(tt.arguments)
			g.Expect(err).NotTo(HaveOccurred())
			pred, err := controller.GetWatchConfigsPredicate(opts)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(pred).NotTo(BeNil())

			ev := event.CreateEvent{
				Object: &corev1.ConfigMap{},
			}

			// Test default label.
			ev.Object.SetLabels(map[string]string{
				meta.LabelKeyWatch: meta.LabelValueWatchEnabled,
			})
			ok := pred.Create(ev)
			g.Expect(ok).To(Equal(tt.shouldMatchDefault))

			// Test custom label.
			ev.Object.SetLabels(map[string]string{
				"app": "my-app",
			})
			ok = pred.Create(ev)
			g.Expect(ok).To(Equal(tt.shouldMatchCustom))

			// Test empty labels.
			ev.Object.SetLabels(map[string]string{})
			ok = pred.Create(ev)
			g.Expect(ok).To(Equal(tt.shouldMatchEmpty))

			// Test Helm label.
			ev.Object.SetLabels(map[string]string{
				"owner": "helm",
			})
			ok = pred.Create(ev)
			g.Expect(ok).To(Equal(tt.shouldMatchHelm))
		})
	}
}

func TestGetWatchLabelSelector(t *testing.T) {
	t.Run("empty selector", func(t *testing.T) {
		g := NewWithT(t)

		opts := controller.WatchOptions{}
		ls, err := controller.GetWatchLabelSelector(opts)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ls).To(BeNil())

		sel, err := controller.GetWatchSelector(opts)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sel).To(Equal(labels.Everything()))
	})

	t.Run("valid selector", func(t *testing.T) {
		g := NewWithT(t)

		opts := controller.WatchOptions{
			LabelSelector: "sharding.fluxcd.io/shard in (shard1,shard2)",
		}
		ls, err := controller.GetWatchLabelSelector(opts)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ls).NotTo(BeNil())

		sel, err := controller.GetWatchSelector(opts)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sel.Matches(labels.Set{
			"sharding.fluxcd.io/shard": "shard2",
		})).To(BeTrue())
		g.Expect(sel.Matches(labels.Set{
			"sharding.fluxcd.io/shard": "shard3",
		})).To(BeFalse())
	})

	t.Run("invalid selector", func(t *testing.T) {
		g := NewWithT(t)

		opts := controller.WatchOptions{
			LabelSelector: "sharding.fluxcd.io/shard in (shard1",
		}
		_, err := controller.GetWatchLabelSelector(opts)
		g.Expect(err).To(HaveOccurred())

		_, err = controller.GetWatchSelector(opts)
		g.Expect(err).To(HaveOccurred())
	})
}

func TestGetWatchConfigsPredicateInvalidSelector(t *testing.T) {
	g := NewWithT(t)

	opts := controller.WatchOptions{
		ConfigsLabelSelector: "owner in (helm",
	}
	pred, err := controller.GetWatchConfigsPredicate(opts)
	g.Expect(err).To(HaveOccurred())
	g.Expect(pred).To(BeNil())
}

// expectPredicateMatches asserts that the predicate allows or filters out the
// given object for every event type.
func expectPredicateMatches(g *WithT, pred predicate.Predicate, obj client.Object, expected bool) {
	g.ExpectWithOffset(1, pred.Create(event.CreateEvent{Object: obj})).To(Equal(expected))
	g.ExpectWithOffset(1, pred.Update(event.UpdateEvent{ObjectOld: obj, ObjectNew: obj})).To(Equal(expected))
	g.ExpectWithOffset(1, pred.Delete(event.DeleteEvent{Object: obj})).To(Equal(expected))
	g.ExpectWithOffset(1, pred.Generic(event.GenericEvent{Object: obj})).To(Equal(expected))
}

func TestWatchedObjects(t *testing.T) {
	newConfigMap := func(namespace, name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		}
	}

	var (
		objA      = newConfigMap("ns1", "a")
		objB      = newConfigMap("ns2", "a")
		watcher1  = newConfigMap("ns1", "watcher1")
		watcher2  = newConfigMap("ns1", "watcher2")
		watcher1K = client.ObjectKeyFromObject(watcher1)
		watcher2K = client.ObjectKeyFromObject(watcher2)
	)

	t.Run("nothing is watched initially", func(t *testing.T) {
		g := NewWithT(t)

		w, pred := controller.NewWatchedObjects()
		g.Expect(w).NotTo(BeNil())
		g.Expect(pred).NotTo(BeNil())

		expectPredicateMatches(g, pred, objA, false)
		expectPredicateMatches(g, pred, objB, false)
		g.Expect(w.ReconcileRequests(objA)).To(BeEmpty())
	})

	t.Run("watch adds object and requests", func(t *testing.T) {
		g := NewWithT(t)

		w, pred := controller.NewWatchedObjects()
		w.Watch(objA, watcher1)

		expectPredicateMatches(g, pred, objA, true)
		// Objects are tracked by namespace and name.
		expectPredicateMatches(g, pred, objB, false)
		g.Expect(w.ReconcileRequests(objA)).To(ConsistOf(
			reconcile.Request{NamespacedName: watcher1K},
		))
		g.Expect(w.ReconcileRequests(objB)).To(BeEmpty())
	})

	t.Run("multiple watchers for the same object", func(t *testing.T) {
		g := NewWithT(t)

		w, _ := controller.NewWatchedObjects()
		w.Watch(objA, watcher1)
		w.Watch(objA, watcher2)
		// Registering the same watcher twice is a no-op.
		w.Watch(objA, watcher1)

		g.Expect(w.ReconcileRequests(objA)).To(ConsistOf(
			reconcile.Request{NamespacedName: watcher1K},
			reconcile.Request{NamespacedName: watcher2K},
		))
	})

	t.Run("unwatch removes watcher and object", func(t *testing.T) {
		g := NewWithT(t)

		w, pred := controller.NewWatchedObjects()
		w.Watch(objA, watcher1)
		w.Watch(objA, watcher2)

		w.Unwatch(objA, watcher1)
		expectPredicateMatches(g, pred, objA, true)
		g.Expect(w.ReconcileRequests(objA)).To(ConsistOf(
			reconcile.Request{NamespacedName: watcher2K},
		))

		w.Unwatch(objA, watcher2)
		expectPredicateMatches(g, pred, objA, false)
		g.Expect(w.ReconcileRequests(objA)).To(BeEmpty())

		// Unwatching unknown objects or watchers is a no-op.
		w.Unwatch(objA, watcher2)
		w.Unwatch(objB, watcher1)
		g.Expect(w.ReconcileRequests(objB)).To(BeEmpty())
	})

	t.Run("remove watcher from all objects", func(t *testing.T) {
		g := NewWithT(t)

		w, pred := controller.NewWatchedObjects()
		w.Watch(objA, watcher1)
		w.Watch(objA, watcher2)
		w.Watch(objB, watcher1)

		w.RemoveWatcher(watcher1)
		expectPredicateMatches(g, pred, objA, true)
		expectPredicateMatches(g, pred, objB, false)
		g.Expect(w.ReconcileRequests(objA)).To(ConsistOf(
			reconcile.Request{NamespacedName: watcher2K},
		))
		g.Expect(w.ReconcileRequests(objB)).To(BeEmpty())

		w.RemoveWatcher(watcher2)
		expectPredicateMatches(g, pred, objA, false)
		g.Expect(w.ReconcileRequests(objA)).To(BeEmpty())

		// Removing an unknown watcher is a no-op.
		w.RemoveWatcher(watcher1)
	})

	t.Run("concurrent access", func(t *testing.T) {
		g := NewWithT(t)

		w, _ := controller.NewWatchedObjects()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			watcher := newConfigMap("ns1", fmt.Sprintf("watcher%d", i))
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					w.Watch(objA, watcher)
					_ = w.ReconcileRequests(objA)
					w.Unwatch(objA, watcher)
					w.RemoveWatcher(watcher)
				}
			}()
		}
		wg.Wait()

		g.Expect(w.ReconcileRequests(objA)).To(BeEmpty())
	})
}
