/*
Copyright 2026 The Flux authors

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

package dependency_test

import (
	"context"
	"testing"

	celtypes "github.com/google/cel-go/common/types"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/cel"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/dependency"
)

var (
	cmGVK            = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	podGVK           = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
	kustomizationGVK = schema.GroupVersionKind{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"}

	testScheme = func() *runtime.Scheme {
		s := runtime.NewScheme()
		s.AddKnownTypeWithName(cmGVK, &unstructured.Unstructured{})
		s.AddKnownTypeWithName(podGVK, &unstructured.Unstructured{})
		s.AddKnownTypeWithName(kustomizationGVK, &unstructured.Unstructured{})
		return s
	}()
)

func newDep(gvk schema.GroupVersionKind, ns, name string, status map[string]any, conds ...*metav1.Condition) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(name)
	u.SetNamespace(ns)
	for k, v := range status {
		_ = unstructured.SetNestedField(u.Object, v, "status", k)
	}
	if len(conds) > 0 {
		list := make([]metav1.Condition, len(conds))
		for i, c := range conds {
			list[i] = *c
		}
		conditions.UnstructuredSetter(u).SetConditions(list)
	}
	return u
}

func applier(refs ...meta.DependencyReference) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(kustomizationGVK)
	obj.SetName("applier")
	obj.SetNamespace("default")
	if len(refs) > 0 {
		depMaps := make([]any, len(refs))
		for i, ref := range refs {
			depMaps[i] = map[string]any{
				"apiVersion": ref.APIVersion,
				"kind":       ref.Kind,
				"name":       ref.Name,
				"namespace":  ref.Namespace,
				"readyExpr":  ref.ReadyExpr,
			}
		}
		_ = unstructured.SetNestedSlice(obj.Object, depMaps, "spec", "dependsOn")
	}
	return obj
}

func condition(condType string, status metav1.ConditionStatus) *metav1.Condition {
	return &metav1.Condition{Type: condType, Status: status, LastTransitionTime: metav1.Now()}
}

func TestBuildDependencyExpressions(t *testing.T) {
	for _, tt := range []struct {
		name string
		deps []meta.DependencyReference
		nils []bool
		err  string
	}{
		{
			name: "all empty ReadyExpr",
			deps: []meta.DependencyReference{
				{Kind: "Pod", Name: "pod1"},
				{Kind: "Pod", Name: "pod2"},
			},
			nils: []bool{true, true},
		},
		{
			name: "all valid ReadyExpr",
			deps: []meta.DependencyReference{
				{Kind: "Pod", Name: "pod1", ReadyExpr: "true"},
				{Kind: "Pod", Name: "pod2", ReadyExpr: "dep.status.phase == 'Running'"},
			},
			nils: []bool{false, false},
		},
		{
			name: "mixed empty and valid ReadyExpr",
			deps: []meta.DependencyReference{
				{Kind: "Pod", Name: "pod1"},
				{Kind: "Pod", Name: "pod2", ReadyExpr: "true"},
				{Kind: "Pod", Name: "pod3", ReadyExpr: "dep.status.phase == 'Running'"},
			},
			nils: []bool{true, false, false},
		},
		{
			name: "invalid ReadyExpr syntax",
			deps: []meta.DependencyReference{
				{Kind: "Pod", Name: "pod1", ReadyExpr: "foo."},
			},
			err: "failed to parse expression for dependency Pod/pod1",
		},
		{
			name: "invalid ReadyExpr undeclared variable",
			deps: []meta.DependencyReference{
				{Kind: "Pod", Name: "pod1", ReadyExpr: "deps.metadata.generation == self.metadata.generation"},
			},
			err: "failed to parse expression for dependency Pod/pod1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &object{kind: "Kustomization", dependsOn: tt.deps}
			exprs, err := dependency.BuildDependencyExpressions(obj)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(exprs).To(BeNil())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exprs).To(HaveLen(len(tt.deps)))
			for i, wantNil := range tt.nils {
				if wantNil {
					g.Expect(exprs[i]).To(BeNil())
				} else {
					g.Expect(exprs[i]).NotTo(BeNil())
				}
			}
		})
	}
}

func TestApplyDependencyDefaults(t *testing.T) {
	p := &object{
		apiVersion: "kustomize.toolkit.fluxcd.io/v1",
		kind:       "Kustomization",
		name:       "self",
		namespace:  "default",
	}
	for _, tt := range []struct {
		name string
		dep  meta.DependencyReference
		want meta.DependencyReference
	}{
		{
			name: "Kind empty defaults to parent",
			dep:  meta.DependencyReference{Name: "ks1", Namespace: "other"},
			want: meta.DependencyReference{APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Name: "ks1", Namespace: "other"},
		},
		{
			name: "Kind different from parent leaves APIVersion/Namespace alone",
			dep:  meta.DependencyReference{Kind: "ConfigMap", Name: "cm1"},
			want: meta.DependencyReference{Kind: "ConfigMap", Name: "cm1"},
		},
		{
			name: "Kind matches, APIVersion empty defaults to parent",
			dep:  meta.DependencyReference{Kind: "Kustomization", Name: "ks1"},
			want: meta.DependencyReference{APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Name: "ks1", Namespace: "default"},
		},
		{
			name: "all set, no changes",
			dep:  meta.DependencyReference{APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Name: "ks1", Namespace: "default"},
			want: meta.DependencyReference{APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Name: "ks1", Namespace: "default"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(dependency.ApplyDependencyDefaults(p, tt.dep)).To(Equal(tt.want))
		})
	}
}

func TestFetchDependency(t *testing.T) {
	for _, tt := range []struct {
		name   string
		dep    meta.DependencyReference
		depObj *unstructured.Unstructured
		err    string
	}{
		{
			name:   "object exists",
			dep:    meta.DependencyReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1"},
			depObj: newDep(podGVK, "default", "pod1", nil),
		},
		{
			name: "object not found",
			dep:  meta.DependencyReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "missing"},
			err:  "not found",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			builder := fake.NewClientBuilder().WithScheme(testScheme)
			if tt.depObj != nil {
				builder = builder.WithObjects(tt.depObj)
			}
			got, err := dependency.FetchDependency(context.Background(), builder.Build(), tt.dep)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).NotTo(BeNil())
			g.Expect(got.GetName()).To(Equal(tt.dep.Name))
		})
	}
}

func TestEvaluateCEL(t *testing.T) {
	for _, tt := range []struct {
		name string
		expr string
		obj  *unstructured.Unstructured
		dep  *unstructured.Unstructured
		err  string
	}{
		{
			name: "true",
			expr: "dep.status.phase == 'Running'",
			obj:  applier(),
			dep:  newDep(podGVK, "default", "pod1", map[string]any{"phase": "Running"}),
		},
		{
			name: "false",
			expr: "dep.status.phase == 'Running'",
			obj:  applier(),
			dep:  newDep(podGVK, "default", "pod1", map[string]any{"phase": "Pending"}),
			err:  "not ready according to readyExpr",
		},
		{
			name: "evaluation error",
			expr: "dep.status.missing.field == 'x'",
			obj:  applier(),
			dep:  newDep(podGVK, "default", "pod1", nil),
			err:  "failed to evaluate dependency",
		},
		{
			name: "true with self property access",
			expr: "self.metadata.annotations['key'] == dep.metadata.annotations['key']",
			obj: func() *unstructured.Unstructured {
				obj := applier()
				obj.SetAnnotations(map[string]string{"key": "val"})
				return obj
			}(),
			dep: func() *unstructured.Unstructured {
				d := newDep(podGVK, "default", "pod1", nil)
				d.SetAnnotations(map[string]string{"key": "val"})
				return d
			}(),
		},
		{
			name: "false with self property access",
			expr: "self.metadata.annotations['key'] == dep.metadata.annotations['key']",
			obj: func() *unstructured.Unstructured {
				obj := applier()
				obj.SetAnnotations(map[string]string{"key": "val1"})
				return obj
			}(),
			dep: func() *unstructured.Unstructured {
				d := newDep(podGVK, "default", "pod1", nil)
				d.SetAnnotations(map[string]string{"key": "val2"})
				return d
			}(),
			err: "not ready according to readyExpr",
		},
		{
			name: "has() function true",
			expr: "has(dep.data)",
			obj:  applier(),
			dep: func() *unstructured.Unstructured {
				u := &unstructured.Unstructured{}
				u.SetGroupVersionKind(cmGVK)
				u.SetName("cm1")
				u.SetNamespace("default")
				_ = unstructured.SetNestedField(u.Object, map[string]any{"key": "val"}, "data")
				return u
			}(),
		},
		{
			name: "has() function false",
			expr: "has(dep.data)",
			obj:  applier(),
			dep: func() *unstructured.Unstructured {
				u := &unstructured.Unstructured{}
				u.SetGroupVersionKind(cmGVK)
				u.SetName("cm1")
				u.SetNamespace("default")
				return u
			}(),
			err: "not ready according to readyExpr",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			expr, err := cel.NewExpression(tt.expr,
				cel.WithCompile(),
				cel.WithOutputType(celtypes.BoolType),
				cel.WithStructVariables("self", "dep"),
			)
			g.Expect(err).NotTo(HaveOccurred())
			err = dependency.EvaluateCEL(context.Background(), tt.obj, tt.dep, expr)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestCheckDependencies(t *testing.T) {
	for _, tt := range []struct {
		name string
		obj  *unstructured.Unstructured
		deps []*unstructured.Unstructured
		opts []dependency.CheckOption
		err  string
	}{
		{
			name: "Ready=True passes",
			obj:  applier(meta.DependencyReference{Kind: "Kustomization", Name: "r1"}),
			deps: []*unstructured.Unstructured{
				newDep(kustomizationGVK, "default", "r1", nil,
					condition(meta.ReadyCondition, metav1.ConditionTrue)),
			},
		},
		{
			name: "no conditions fails (same-kind check)",
			obj:  applier(meta.DependencyReference{Kind: "Kustomization", Name: "r1"}),
			deps: []*unstructured.Unstructured{
				newDep(kustomizationGVK, "default", "r1", nil),
			},
			err: "is not ready",
		},
		{
			name: "non-Ready condition only fails",
			obj:  applier(meta.DependencyReference{Kind: "Kustomization", Name: "r1"}),
			deps: []*unstructured.Unstructured{
				newDep(kustomizationGVK, "default", "r1", nil,
					condition("Available", metav1.ConditionTrue)),
			},
			err: "is not ready",
		},
		{
			name: "Ready=Unknown caught by kstatus",
			obj:  applier(meta.DependencyReference{Kind: "Kustomization", Name: "r1"}),
			deps: []*unstructured.Unstructured{
				newDep(kustomizationGVK, "default", "r1", nil,
					condition(meta.ReadyCondition, metav1.ConditionUnknown)),
			},
			err: "not ready: status InProgress",
		},
		{
			name: "Ready=False with matching ObsGen/Gen fails (same-kind check)",
			obj:  applier(meta.DependencyReference{Kind: "Kustomization", Name: "r1"}),
			deps: []*unstructured.Unstructured{
				newDep(kustomizationGVK, "default", "r1",
					map[string]any{"observedGeneration": int64(1)},
					condition(meta.ReadyCondition, metav1.ConditionFalse)),
			},
			err: "is not ready",
		},
		{
			name: "Ready=True but ObsGen < Gen caught by kstatus (same-kind)",
			obj:  applier(meta.DependencyReference{Kind: "Kustomization", Name: "r1"}),
			deps: []*unstructured.Unstructured{
				func() *unstructured.Unstructured {
					d := newDep(kustomizationGVK, "default", "r1",
						map[string]any{"observedGeneration": int64(1)},
						condition(meta.ReadyCondition, metav1.ConditionTrue))
					d.SetGeneration(2)
					return d
				}(),
			},
			err: "not ready: status InProgress",
		},
		{
			name: "Ready=True but ObsGen < Gen caught by kstatus (cross-kind Pod)",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1",
				ReadyExpr: "dep.status.phase == 'Running'",
			}),
			deps: []*unstructured.Unstructured{
				func() *unstructured.Unstructured {
					d := newDep(podGVK, "default", "pod1",
						map[string]any{"phase": "Running", "observedGeneration": int64(1)})
					d.SetGeneration(2)
					return d
				}(),
			},
			opts: []dependency.CheckOption{dependency.WithAdditiveCEL()},
			err:  "not ready",
		},
		{
			name: "CEL true",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1",
				ReadyExpr: "dep.status.phase == 'Running'",
			}),
			deps: []*unstructured.Unstructured{
				newDep(podGVK, "default", "pod1", map[string]any{"phase": "Running"}),
			},
		},
		{
			name: "CEL false",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1",
				ReadyExpr: "dep.status.phase == 'Running'",
			}),
			deps: []*unstructured.Unstructured{
				newDep(podGVK, "default", "pod1", map[string]any{"phase": "Pending"}),
			},
			err: "not ready according to readyExpr",
		},
		{
			name: "CEL evaluation error",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1",
				ReadyExpr: "dep.status.missing.field == 'x'",
			}),
			deps: []*unstructured.Unstructured{
				newDep(podGVK, "default", "pod1", nil),
			},
			err: "failed to evaluate dependency",
		},
		{
			name: "CEL true additive still runs kstatus check",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1",
				ReadyExpr: "dep.status.phase == 'Running'",
			}),
			deps: []*unstructured.Unstructured{
				newDep(podGVK, "default", "pod1", map[string]any{"phase": "Running"}),
			},
			opts: []dependency.CheckOption{dependency.WithAdditiveCEL()},
			err:  "not ready",
		},
		{
			name: "CEL with self property access match",
			obj: func() *unstructured.Unstructured {
				obj := applier(meta.DependencyReference{
					APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1",
					ReadyExpr: "self.metadata.annotations['app/version'] == dep.metadata.annotations['app/version']",
				})
				obj.SetAnnotations(map[string]string{"app/version": "v1.2.3"})
				return obj
			}(),
			deps: func() []*unstructured.Unstructured {
				d := newDep(podGVK, "default", "pod1", nil)
				d.SetAnnotations(map[string]string{"app/version": "v1.2.3"})
				return []*unstructured.Unstructured{d}
			}(),
		},
		{
			name: "CEL with self property access mismatch",
			obj: func() *unstructured.Unstructured {
				obj := applier(meta.DependencyReference{
					APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1",
					ReadyExpr: "self.metadata.annotations['app/version'] == dep.metadata.annotations['app/version']",
				})
				obj.SetAnnotations(map[string]string{"app/version": "v1.2.4"})
				return obj
			}(),
			deps: []*unstructured.Unstructured{
				func() *unstructured.Unstructured {
					d := newDep(podGVK, "default", "pod1", nil)
					d.SetAnnotations(map[string]string{"app/version": "v1.2.3"})
					return d
				}(),
			},
			err: "not ready according to readyExpr",
		},
		{
			name: "CEL with has() function true",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cm1",
				ReadyExpr: "has(dep.data)",
			}),
			deps: []*unstructured.Unstructured{
				func() *unstructured.Unstructured {
					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(cmGVK)
					u.SetName("cm1")
					u.SetNamespace("default")
					_ = unstructured.SetNestedField(u.Object, map[string]any{"key": "val"}, "data")
					return u
				}(),
			},
		},
		{
			name: "CEL with has() function false",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cm1",
				ReadyExpr: "has(dep.data)",
			}),
			deps: []*unstructured.Unstructured{
				func() *unstructured.Unstructured {
					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(cmGVK)
					u.SetName("cm1")
					u.SetNamespace("default")
					return u
				}(),
			},
			err: "not ready according to readyExpr",
		},
		{
			name: "CEL with !has() function",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cm1",
				ReadyExpr: "!has(dep.data)",
			}),
			deps: []*unstructured.Unstructured{
				func() *unstructured.Unstructured {
					u := &unstructured.Unstructured{}
					u.SetGroupVersionKind(cmGVK)
					u.SetName("cm1")
					u.SetNamespace("default")
					_ = unstructured.SetNestedField(u.Object, map[string]any{"key": "val"}, "data")
					return u
				}(),
			},
			err: "not ready according to readyExpr",
		},
		{
			name: "cross-kind ConfigMap skips check",
			obj:  applier(meta.DependencyReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "config1"}),
			deps: []*unstructured.Unstructured{
				newDep(cmGVK, "default", "config1", nil),
			},
		},
		{
			name: "missing object returns not found",
			obj: applier(meta.DependencyReference{
				APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "missing",
				ReadyExpr: "true",
			}),
			err: "not found",
		},
		{
			name: "no dependsOn returns nil",
			obj:  applier(),
			deps: []*unstructured.Unstructured{},
		},
		{
			name: "multiple deps first fails",
			obj: applier(
				meta.DependencyReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1", ReadyExpr: "dep.status.phase == 'Running'"},
				meta.DependencyReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cm1"},
			),
			deps: []*unstructured.Unstructured{
				newDep(cmGVK, "default", "cm1", nil),
			},
			err: "not found",
		},
		{
			name: "multiple deps second fails first passes",
			obj: applier(
				meta.DependencyReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1", ReadyExpr: "dep.status.phase == 'Running'"},
				meta.DependencyReference{Kind: "Kustomization", Name: "r1"},
			),
			deps: []*unstructured.Unstructured{
				newDep(podGVK, "default", "pod1", map[string]any{"phase": "Running"}),
				newDep(kustomizationGVK, "default", "r1", nil),
			},
			err: "is not ready",
		},
		{
			name: "multiple deps all pass",
			obj: applier(
				meta.DependencyReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod1", ReadyExpr: "dep.status.phase == 'Running'"},
				meta.DependencyReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cm1"},
			),
			deps: []*unstructured.Unstructured{
				newDep(podGVK, "default", "pod1", map[string]any{"phase": "Running"}),
				newDep(cmGVK, "default", "cm1", nil),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			builder := fake.NewClientBuilder().WithScheme(testScheme)
			if len(tt.deps) > 0 {
				objs := make([]client.Object, len(tt.deps))
				for i, d := range tt.deps {
					objs[i] = d
				}
				builder = builder.WithObjects(objs...)
			}
			err := dependency.CheckDependencies(context.Background(), builder.Build(), tt.obj, tt.opts...)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}
