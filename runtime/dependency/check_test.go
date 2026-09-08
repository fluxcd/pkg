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
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/dependency"
)

var (
	kustomizationGVK = schema.GroupVersionKind{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"}
	helmReleaseGVK   = schema.GroupVersionKind{Group: "helm.toolkit.fluxcd.io", Version: "v2", Kind: "HelmRelease"}
	foreignGVK       = schema.GroupVersionKind{Group: "other.example.com", Version: "v1", Kind: "Kustomization"}
	crdGVK           = schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}
	podGVK           = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
	serviceGVK       = schema.GroupVersionKind{Version: "v1", Kind: "Service"}
	secretGVK        = schema.GroupVersionKind{Version: "v1", Kind: "Secret"}
)

// Cross-kind and Kubernetes-native dependency references shared across tests.
var (
	certManagerRef = meta.DependencyReference{
		APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "default", Name: "cert-manager",
	}
	secretRef = meta.DependencyReference{APIVersion: "v1", Kind: "Secret", Namespace: "default", Name: "credentials"}
	podRef    = meta.DependencyReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "workload"}
	crdRef    = meta.DependencyReference{
		APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition",
		Name: "kustomizations.kustomize.toolkit.fluxcd.io",
	}
)

// newObject returns an unstructured object of the given kind, with an
// optional status subresource. An empty namespace makes the object
// cluster-scoped.
func newObject(gvk schema.GroupVersionKind, namespace, name string, status ...map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	if len(status) > 0 {
		obj.Object["status"] = status[0]
	}
	return obj
}

// newDependent returns an unstructured object named default/applier of the
// given kind, declaring the given dependencies in spec.dependsOn.
func newDependent(gvk schema.GroupVersionKind, deps ...meta.DependencyReference) *unstructured.Unstructured {
	obj := newObject(gvk, "default", "applier")
	if len(deps) == 0 {
		return obj
	}

	list := make([]any, 0, len(deps))
	for _, dep := range deps {
		ref := map[string]any{"name": dep.Name}
		if dep.APIVersion != "" {
			ref["apiVersion"] = dep.APIVersion
		}
		if dep.Kind != "" {
			ref["kind"] = dep.Kind
		}
		if dep.Namespace != "" {
			ref["namespace"] = dep.Namespace
		}
		if dep.ReadyExpr != "" {
			ref["readyExpr"] = dep.ReadyExpr
		}
		list = append(list, ref)
	}

	obj.Object["spec"] = map[string]any{"dependsOn": list}
	return obj
}

// withReadyExpr returns a copy of ref with the given CEL readiness expression.
func withReadyExpr(ref meta.DependencyReference, expr string) meta.DependencyReference {
	ref.ReadyExpr = expr
	return ref
}

// condition returns a status condition of the given type and status.
func condition(condType, condStatus string) map[string]any {
	return map[string]any{
		"type":               condType,
		"status":             condStatus,
		"reason":             "TestReason",
		"message":            "test message",
		"lastTransitionTime": "2026-01-01T00:00:00Z",
	}
}

// readyStatus returns a status subresource with the Ready condition set to
// the given status.
func readyStatus(condStatus string) map[string]any {
	return map[string]any{
		"conditions": []any{condition(meta.ReadyCondition, condStatus)},
	}
}

// checkCase is a single CheckDependencies scenario run by runCheckCases.
type checkCase struct {
	name     string
	obj      *unstructured.Unstructured
	objects  []ctrlclient.Object
	err      string
	terminal bool
}

// runCheckCases runs CheckDependencies with the given options for each
// case, against a fake client seeded with the case objects.
func runCheckCases(t *testing.T, cases []checkCase, opts ...dependency.CheckOption) {
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := fake.NewClientBuilder().WithObjects(tt.objects...).Build()
			err := dependency.CheckDependencies(context.Background(), c, tt.obj, opts...)
			if tt.err == "" {
				g.Expect(err).NotTo(HaveOccurred())
				return
			}

			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.err))
			g.Expect(errors.Is(err, reconcile.TerminalError(nil))).To(Equal(tt.terminal), "terminal error mismatch")
		})
	}
}

func TestBuildDependencyExpressions(t *testing.T) {
	for _, tt := range []struct {
		name string
		deps []meta.DependencyReference
		nils []bool
		err  string
	}{
		{
			name: "no expressions",
			deps: []meta.DependencyReference{
				{Name: "infra"},
				{Kind: "HelmRelease", Name: "cert-manager"},
			},
			nils: []bool{true, true},
		},
		{
			name: "expressions aligned to dependencies",
			deps: []meta.DependencyReference{
				{Name: "infra"},
				{Kind: "Pod", Name: "workload", ReadyExpr: `dep.status.phase == 'Running'`},
				{Name: "backend", ReadyExpr: `self.metadata.namespace == dep.metadata.namespace`},
			},
			nils: []bool{true, false, false},
		},
		{
			name: "invalid expression syntax",
			deps: []meta.DependencyReference{
				{Name: "infra", ReadyExpr: `dep.metadata.name ==`},
			},
			err: "failed to parse expression for dependency infra@dep.metadata.name ==",
		},
		{
			name: "expression not evaluating to a boolean",
			deps: []meta.DependencyReference{
				{Name: "infra", ReadyExpr: `dep.metadata.name`},
			},
			err: "failed to parse expression for dependency infra@dep.metadata.name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			exprs, err := dependency.BuildDependencyExpressions(&object{
				apiVersion: kustomizationGVK.GroupVersion().String(),
				kind:       kustomizationGVK.Kind,
				name:       "applier",
				namespace:  "default",
				dependsOn:  tt.deps,
			})

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue(), "expected a terminal error")
				g.Expect(exprs).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exprs).To(HaveLen(len(tt.deps)))
			for i, isNil := range tt.nils {
				if isNil {
					g.Expect(exprs[i]).To(BeNil(), "expression %d", i)
				} else {
					g.Expect(exprs[i]).NotTo(BeNil(), "expression %d", i)
				}
			}
		})
	}
}

func TestApplyDependencyDefaults(t *testing.T) {
	parent := &object{
		apiVersion: kustomizationGVK.GroupVersion().String(),
		kind:       kustomizationGVK.Kind,
		name:       "applier",
		namespace:  "default",
	}

	for _, tt := range []struct {
		name string
		dep  meta.DependencyReference
		want meta.DependencyReference
	}{
		{
			name: "name only defaults to the parent kind, API version and namespace",
			dep:  meta.DependencyReference{Name: "infra"},
			want: meta.DependencyReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1",
				Kind:       "Kustomization",
				Namespace:  "default",
				Name:       "infra",
			},
		},
		{
			name: "same kind keeps the given namespace",
			dep:  meta.DependencyReference{Namespace: "infra-ns", Name: "infra"},
			want: meta.DependencyReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1",
				Kind:       "Kustomization",
				Namespace:  "infra-ns",
				Name:       "infra",
			},
		},
		{
			name: "same kind keeps the given API version",
			dep:  meta.DependencyReference{APIVersion: "kustomize.toolkit.fluxcd.io/v1beta2", Kind: "Kustomization", Name: "infra"},
			want: meta.DependencyReference{
				APIVersion: "kustomize.toolkit.fluxcd.io/v1beta2",
				Kind:       "Kustomization",
				Namespace:  "default",
				Name:       "infra",
			},
		},
		{
			name: "same kind matching ignores the API group",
			dep:  meta.DependencyReference{APIVersion: "other.example.com/v1", Kind: "Kustomization", Name: "infra"},
			want: meta.DependencyReference{
				APIVersion: "other.example.com/v1",
				Kind:       "Kustomization",
				Namespace:  "default",
				Name:       "infra",
			},
		},
		{
			name: "cross-kind gets no defaults",
			dep:  meta.DependencyReference{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Name: "infra"},
			want: meta.DependencyReference{
				APIVersion: "helm.toolkit.fluxcd.io/v2",
				Kind:       "HelmRelease",
				Name:       "infra",
			},
		},
		{
			name: "fully specified reference is unchanged",
			dep: meta.DependencyReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "apps",
				Name:       "credentials",
			},
			want: meta.DependencyReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "apps",
				Name:       "credentials",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(dependency.ApplyDependencyDefaults(parent, tt.dep)).To(Equal(tt.want))
		})
	}
}

func TestEvaluateCEL(t *testing.T) {
	for _, tt := range []struct {
		name string
		expr string
		err  string
	}{
		{
			name: "expression evaluates to true",
			expr: `self.metadata.namespace == dep.metadata.namespace`,
		},
		{
			name: "expression evaluates to false",
			expr: `dep.metadata.name == 'other'`,
			err:  "dependency v1/Pod/default/workload is not ready according to readyExpr eval",
		},
		{
			name: "expression fails to evaluate on a missing field",
			expr: `dep.status.phase == 'Running'`,
			err:  "failed to evaluate dependency v1/Pod/default/workload",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			exprs, err := dependency.BuildDependencyExpressions(&object{
				dependsOn: []meta.DependencyReference{{Kind: "Pod", Name: "workload", ReadyExpr: tt.expr}},
			})
			g.Expect(err).NotTo(HaveOccurred())

			self := newObject(kustomizationGVK, "default", "applier")
			depObj := newObject(podGVK, "default", "workload")
			err = dependency.EvaluateCEL(context.Background(), self, depObj, exprs[0])

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

// TestCheckDependencies covers the parsing of spec.dependsOn from the
// object declaring the dependencies.
func TestCheckDependencies(t *testing.T) {
	noDependsOn := newObject(kustomizationGVK, "default", "applier")
	noDependsOn.Object["spec"] = map[string]any{"interval": "5m"}

	malformedSpec := newObject(kustomizationGVK, "default", "applier")
	malformedSpec.Object["spec"] = "invalid"

	malformedDependsOn := newObject(kustomizationGVK, "default", "applier")
	malformedDependsOn.Object["spec"] = map[string]any{"dependsOn": "invalid"}

	runCheckCases(t, []checkCase{
		{
			name: "no dependencies",
			obj:  newDependent(kustomizationGVK),
		},
		{
			name: "spec without dependsOn",
			obj:  noDependsOn,
		},
		{
			name: "malformed spec",
			obj:  malformedSpec,
			err:  "failed to read spec",
		},
		{
			name: "malformed dependsOn",
			obj:  malformedDependsOn,
			err:  "failed to parse spec",
		},
	})
}

// TestCheckDependencies_SameKind covers dependencies between objects of
// the same kind, where the namespace and API version default to the
// parent's, and the Ready condition is verified in addition to kstatus.
func TestCheckDependencies_SameKind(t *testing.T) {
	staleKustomization := newObject(kustomizationGVK, "default", "infra",
		map[string]any{"observedGeneration": int64(1)})
	staleKustomization.SetGeneration(2)

	terminatingKustomization := newObject(kustomizationGVK, "default", "infra", readyStatus("True"))
	terminatingKustomization.SetDeletionTimestamp(new(metav1.Now()))
	terminatingKustomization.SetFinalizers([]string{"finalizers.fluxcd.io"})

	runCheckCases(t, []checkCase{
		{
			name:    "Kustomization depending on a ready Kustomization with defaults applied",
			obj:     newDependent(kustomizationGVK, meta.DependencyReference{Name: "infra"}),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "default", "infra", readyStatus("True"))},
		},
		{
			name:    "HelmRelease depending on a ready HelmRelease with defaults applied",
			obj:     newDependent(helmReleaseGVK, meta.DependencyReference{Name: "infra"}),
			objects: []ctrlclient.Object{newObject(helmReleaseGVK, "default", "infra", readyStatus("True"))},
		},
		{
			name:    "dependency in another namespace",
			obj:     newDependent(kustomizationGVK, meta.DependencyReference{Namespace: "infra-ns", Name: "infra"}),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "infra-ns", "infra", readyStatus("True"))},
		},
		{
			name: "dependency not found",
			obj:  newDependent(kustomizationGVK, meta.DependencyReference{Name: "infra"}),
			err:  "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/infra not found",
		},
		{
			name:    "dependency without Ready condition",
			obj:     newDependent(kustomizationGVK, meta.DependencyReference{Name: "infra"}),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "default", "infra")},
			err:     "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/infra is not ready",
		},
		{
			name:    "dependency with Ready condition False",
			obj:     newDependent(kustomizationGVK, meta.DependencyReference{Name: "infra"}),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "default", "infra", readyStatus("False"))},
			err:     "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/infra is not ready: status InProgress",
		},
		{
			name:    "dependency with stale observed generation",
			obj:     newDependent(kustomizationGVK, meta.DependencyReference{Name: "infra"}),
			objects: []ctrlclient.Object{staleKustomization},
			err:     "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/infra is not ready: status InProgress",
		},
		{
			name:    "dependency being deleted",
			obj:     newDependent(kustomizationGVK, meta.DependencyReference{Name: "infra"}),
			objects: []ctrlclient.Object{terminatingKustomization},
			err:     "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/infra is not ready: status Terminating",
		},
		{
			name: "dependency of same kind but in a different API group doesn't require the Ready condition",
			obj: newDependent(kustomizationGVK, meta.DependencyReference{
				APIVersion: "other.example.com/v1", Kind: "Kustomization", Name: "infra",
			}),
			objects: []ctrlclient.Object{newObject(foreignGVK, "default", "infra")},
		},
		{
			name: "multiple dependencies with one not ready",
			obj: newDependent(kustomizationGVK,
				meta.DependencyReference{Name: "infra"},
				meta.DependencyReference{Name: "backend"},
			),
			objects: []ctrlclient.Object{
				newObject(kustomizationGVK, "default", "infra", readyStatus("True")),
				newObject(kustomizationGVK, "default", "backend", readyStatus("False")),
			},
			err: "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/backend is not ready",
		},
	})
}

// TestCheckDependencies_CrossKind covers dependencies on a kind other than
// the parent's: Flux Applier APIs depending on each other and on
// Kubernetes-native resources, checked with kstatus only.
func TestCheckDependencies_CrossKind(t *testing.T) {
	readyCertManager := newObject(helmReleaseGVK, "default", "cert-manager", readyStatus("True"))
	establishedCRD := newObject(crdGVK, "", crdRef.Name, map[string]any{
		"conditions": []any{condition("Established", "True")},
	})

	infraKustomizationRef := meta.DependencyReference{
		APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Namespace: "default", Name: "infra",
	}

	lbService := newObject(serviceGVK, "default", "app")
	lbService.Object["spec"] = map[string]any{"type": "LoadBalancer"}

	runCheckCases(t, []checkCase{
		{
			name:    "Kustomization depending on a ready HelmRelease",
			obj:     newDependent(kustomizationGVK, certManagerRef),
			objects: []ctrlclient.Object{readyCertManager},
		},
		{
			name:    "Kustomization depending on a failed HelmRelease",
			obj:     newDependent(kustomizationGVK, certManagerRef),
			objects: []ctrlclient.Object{newObject(helmReleaseGVK, "default", "cert-manager", readyStatus("False"))},
			err:     "dependency helm.toolkit.fluxcd.io/v2/HelmRelease/default/cert-manager is not ready: status InProgress",
		},
		{
			name: "Kustomization depending on a stalled HelmRelease",
			obj:  newDependent(kustomizationGVK, certManagerRef),
			objects: []ctrlclient.Object{
				newObject(helmReleaseGVK, "default", "cert-manager", map[string]any{
					"conditions": []any{condition("Stalled", "True")},
				}),
			},
			err: "dependency helm.toolkit.fluxcd.io/v2/HelmRelease/default/cert-manager is not ready: status Failed",
		},
		{
			name:    "HelmRelease depending on a ready Kustomization",
			obj:     newDependent(helmReleaseGVK, infraKustomizationRef),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "default", "infra", readyStatus("True"))},
		},
		{
			name:    "HelmRelease depending on a not ready Kustomization",
			obj:     newDependent(helmReleaseGVK, infraKustomizationRef),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "default", "infra", readyStatus("False"))},
			err:     "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/infra is not ready: status InProgress",
		},
		{
			name:    "HelmRelease depending on an existing Secret",
			obj:     newDependent(helmReleaseGVK, secretRef),
			objects: []ctrlclient.Object{newObject(secretGVK, "default", "credentials")},
		},
		{
			name: "Pod dependency running and ready",
			obj:  newDependent(kustomizationGVK, podRef),
			objects: []ctrlclient.Object{
				newObject(podGVK, "default", "workload", map[string]any{
					"phase":      "Running",
					"conditions": []any{condition("Ready", "True")},
				}),
			},
		},
		{
			name:    "Pod dependency pending",
			obj:     newDependent(kustomizationGVK, podRef),
			objects: []ctrlclient.Object{newObject(podGVK, "default", "workload", map[string]any{"phase": "Pending"})},
			err:     "dependency v1/Pod/default/workload is not ready: status InProgress",
		},
		{
			name:    "Pod dependency in Unknown phase",
			obj:     newDependent(kustomizationGVK, podRef),
			objects: []ctrlclient.Object{newObject(podGVK, "default", "workload", map[string]any{"phase": "Unknown"})},
			err:     "dependency v1/Pod/default/workload is not ready: unknown phase Unknown",
		},
		{
			name: "Service dependency of type LoadBalancer without cluster IP",
			obj: newDependent(kustomizationGVK, meta.DependencyReference{
				APIVersion: "v1", Kind: "Service", Namespace: "default", Name: "app",
			}),
			objects: []ctrlclient.Object{lbService},
			err:     "dependency v1/Service/default/app is not ready: status InProgress",
		},
		{
			name:    "cluster-scoped CRD dependency established",
			obj:     newDependent(kustomizationGVK, crdRef),
			objects: []ctrlclient.Object{establishedCRD},
		},
		{
			name:    "cluster-scoped CRD dependency not established",
			obj:     newDependent(kustomizationGVK, crdRef),
			objects: []ctrlclient.Object{newObject(crdGVK, "", crdRef.Name)},
			err:     "dependency apiextensions.k8s.io/v1/CustomResourceDefinition/kustomizations.kustomize.toolkit.fluxcd.io is not ready: status InProgress",
		},
		{
			name:    "namespaced dependency without namespace is not found",
			obj:     newDependent(kustomizationGVK, meta.DependencyReference{APIVersion: "v1", Kind: "Secret", Name: "credentials"}),
			objects: []ctrlclient.Object{newObject(secretGVK, "default", "credentials")},
			err:     "dependency v1/Secret/credentials not found",
		},
		{
			// The readyExpr of the second dependency must be evaluated
			// against the second dependency only, verifying the expression
			// slice stays aligned in a list of mixed dependencies.
			name: "multiple dependencies of mixed kinds and readyExpr all ready",
			obj: newDependent(kustomizationGVK,
				meta.DependencyReference{Name: "infra"},
				withReadyExpr(certManagerRef, `dep.metadata.name == 'cert-manager'`),
				crdRef,
			),
			objects: []ctrlclient.Object{
				newObject(kustomizationGVK, "default", "infra", readyStatus("True")),
				readyCertManager,
				establishedCRD,
			},
		},
	})
}

// TestCheckDependencies_ReadyExpr covers dependencies with a CEL readiness
// expression, which by default replaces the built-in kstatus check.
func TestCheckDependencies_ReadyExpr(t *testing.T) {
	runCheckCases(t, []checkCase{
		{
			// The dependency has no Ready condition, so the built-in
			// same-kind check would fail: the readyExpr replaces it.
			name:    "readyExpr replaces the built-in readiness check",
			obj:     newDependent(kustomizationGVK, withReadyExpr(meta.DependencyReference{Name: "infra"}, `dep.metadata.name == 'infra'`)),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "default", "infra")},
		},
		{
			name: "readyExpr on the dependency conditions",
			obj: newDependent(kustomizationGVK, withReadyExpr(certManagerRef,
				`dep.status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')`)),
			objects: []ctrlclient.Object{newObject(helmReleaseGVK, "default", "cert-manager", readyStatus("True"))},
		},
		{
			name:    "readyExpr referencing only self",
			obj:     newDependent(kustomizationGVK, withReadyExpr(secretRef, `self.metadata.name == 'applier'`)),
			objects: []ctrlclient.Object{newObject(secretGVK, "default", "credentials")},
		},
		{
			name:    "readyExpr not met",
			obj:     newDependent(kustomizationGVK, withReadyExpr(secretRef, `has(dep.data)`)),
			objects: []ctrlclient.Object{newObject(secretGVK, "default", "credentials")},
			err:     "dependency v1/Secret/default/credentials is not ready according to readyExpr eval",
		},
		{
			name:    "readyExpr evaluation error on a missing field is transient",
			obj:     newDependent(kustomizationGVK, withReadyExpr(podRef, `dep.status.phase == 'Running'`)),
			objects: []ctrlclient.Object{newObject(podGVK, "default", "workload")},
			err:     "failed to evaluate dependency v1/Pod/default/workload",
		},
		{
			name:     "invalid readyExpr fails terminally before any dependency is fetched",
			obj:      newDependent(kustomizationGVK, withReadyExpr(meta.DependencyReference{Name: "infra"}, `dep.metadata.name ==`)),
			err:      "failed to parse expression for dependency infra@dep.metadata.name ==",
			terminal: true,
		},
	})
}

// TestCheckDependencies_AdditiveCEL covers the WithAdditiveCEL option,
// which combines the readyExpr with the built-in check instead of
// replacing it.
func TestCheckDependencies_AdditiveCEL(t *testing.T) {
	readyCertManager := newObject(helmReleaseGVK, "default", "cert-manager", readyStatus("True"))

	runCheckCases(t, []checkCase{
		{
			name:    "requires the built-in check to pass",
			obj:     newDependent(kustomizationGVK, withReadyExpr(certManagerRef, `dep.metadata.name == 'cert-manager'`)),
			objects: []ctrlclient.Object{newObject(helmReleaseGVK, "default", "cert-manager", readyStatus("False"))},
			err:     "dependency helm.toolkit.fluxcd.io/v2/HelmRelease/default/cert-manager@dep.metadata.name == 'cert-manager' is not ready: status InProgress",
		},
		{
			name:    "requires the readyExpr to pass",
			obj:     newDependent(kustomizationGVK, withReadyExpr(certManagerRef, `dep.metadata.name == 'other'`)),
			objects: []ctrlclient.Object{readyCertManager},
			err:     "dependency helm.toolkit.fluxcd.io/v2/HelmRelease/default/cert-manager is not ready according to readyExpr eval",
		},
		{
			name:    "passes when both checks pass",
			obj:     newDependent(kustomizationGVK, withReadyExpr(certManagerRef, `dep.metadata.name == 'cert-manager'`)),
			objects: []ctrlclient.Object{readyCertManager},
		},
		{
			name:    "without readyExpr falls back to the built-in check",
			obj:     newDependent(kustomizationGVK, certManagerRef),
			objects: []ctrlclient.Object{readyCertManager},
		},
		{
			// The readyExpr passes and kstatus reports Current, but the
			// same-kind dependency has no Ready condition.
			name:    "still requires the same-kind Ready condition",
			obj:     newDependent(kustomizationGVK, withReadyExpr(meta.DependencyReference{Name: "infra"}, `dep.metadata.name == 'infra'`)),
			objects: []ctrlclient.Object{newObject(kustomizationGVK, "default", "infra")},
			err:     "dependency kustomize.toolkit.fluxcd.io/v1/Kustomization/default/infra@dep.metadata.name == 'infra' is not ready",
		},
	}, dependency.WithAdditiveCEL())
}
