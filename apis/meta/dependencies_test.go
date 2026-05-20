/*
Copyright 2025 The Flux authors

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

package meta_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/apis/meta"
)

func TestMakeDependsOn(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name string
		deps []string
		want []meta.DependencyReference
	}{
		{
			name: "empty",
			deps: []string{},
			want: []meta.DependencyReference{},
		},
		{
			name: "single name only (Flux Applier API)",
			deps: []string{"redis"},
			want: []meta.DependencyReference{
				{Name: "redis"},
			},
		},
		{
			name: "single name (Flux Applier API) with readiness check",
			deps: []string{"redis:true"},
			want: []meta.DependencyReference{
				{Name: "redis", Ready: new(true)},
			},
		},
		{
			name: "single (Flux Applier API) with namespace",
			deps: []string{"default/redis"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis"},
			},
		},
		{
			name: "single (Flux Applier API) with namespace and readiness check",
			deps: []string{"default/redis:true"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis", Ready: new(true)},
			},
		},
		{
			name: "single (Flux Applier API) with CEL expression",
			deps: []string{"redis@status.ready==true"},
			want: []meta.DependencyReference{
				{Name: "redis", ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single (Flux Applier API) with CEL expression (disabled)",
			deps: []string{"redis:false@status.ready==true"},
			want: []meta.DependencyReference{
				{Name: "redis", Ready: new(false), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single (Flux Applier API) with CEL expression (enabled)",
			deps: []string{"redis:true@status.ready==true"},
			want: []meta.DependencyReference{
				{Name: "redis", Ready: new(true), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single (Flux Applier API) with namespace and CEL expression",
			deps: []string{"default/redis@status.ready==true"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis", ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single (Flux Applier API) with namespace and CEL expression (disabled)",
			deps: []string{"default/redis:false@status.ready==true"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis", Ready: new(false), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single (Flux Applier API) with namespace and CEL expression (enabled)",
			deps: []string{"default/redis:true@status.ready==true"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis", Ready: new(true), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single Kubernetes object",
			deps: []string{"v1/Pod/redis-abc"},
			want: []meta.DependencyReference{
				{APIVersion: "v1", Kind: "Pod", Name: "redis-abc"},
			},
		},
		{
			name: "single Kubernetes object with readiness check",
			deps: []string{"apps/v1/Deployment/redis:true"},
			want: []meta.DependencyReference{
				{APIVersion: "apps/v1", Kind: "Deployment", Name: "redis", Ready: new(true)},
			},
		},
		{
			name: "single Kubernetes object with namespace",
			deps: []string{"v1/Secret/default/redis"},
			want: []meta.DependencyReference{
				{APIVersion: "v1", Kind: "Secret", Namespace: "default", Name: "redis"},
			},
		},
		{
			name: "single Kubernetes object with namespace and readiness check",
			deps: []string{"v1/ConfigMap/default/redis:true"},
			want: []meta.DependencyReference{
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "redis", Ready: new(true)},
			},
		},
		{
			name: "single Kubernetes object with CEL expression",
			deps: []string{"apps/v1/DaemonSet/redis@status.ready==true"},
			want: []meta.DependencyReference{
				{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "redis", ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single Kubernetes object with CEL expression (disabled)",
			deps: []string{"apiextensions.k8s.io/v1/CustomResourceDefinition/kustomizations.kustomize.toolkit.fluxcd.io:false@status.ready==true"},
			want: []meta.DependencyReference{
				{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "kustomizations.kustomize.toolkit.fluxcd.io", Ready: new(false), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single Kubernetes object with CEL expression (enabled)",
			deps: []string{"apiextensions.k8s.io/v1/CustomResourceDefinition/helmreleases.helm.toolkit.fluxcd.io:true@status.ready==true"},
			want: []meta.DependencyReference{
				{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "helmreleases.helm.toolkit.fluxcd.io", Ready: new(true), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single Kubernetes object with namespace and CEL expression",
			deps: []string{"apps/v1/StatefulSet/default/redis@status.ready==true"},
			want: []meta.DependencyReference{
				{APIVersion: "apps/v1", Kind: "StatefulSet", Namespace: "default", Name: "redis", ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single Kubernetes object with namespace and CEL expression (disabled)",
			deps: []string{"source.toolkit.fluxcd.io/v1/OCIRepository/default/redis:false@status.ready==true"},
			want: []meta.DependencyReference{
				{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "OCIRepository", Namespace: "default", Name: "redis", Ready: new(false), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single Kubernetes object with namespace and CEL expression (enabled)",
			deps: []string{"source.toolkit.fluxcd.io/v1/GitRepository/default/redis:true@status.ready==true"},
			want: []meta.DependencyReference{
				{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "GitRepository", Namespace: "default", Name: "redis", Ready: new(true), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "multiple dependencies",
			deps: []string{
				"redis",
				"v1/Pod/podinfo:true",
				"v1/PersistentVolumeClaim/backend:true@status.phase==Bound",
				"default/postgres@status.ready==true",
				"infra/cert-manager:true",
				"source.toolkit.fluxcd.io/v1/OCIRepository/flux-system/flux-operator:false",
				"apiextensions.k8s.io/v1/CustomResourceDefinition/resourcesets.fluxcd.controlplane.io:true@status.ready==true",
			},
			want: []meta.DependencyReference{
				{Name: "redis"},
				{APIVersion: "v1", Kind: "Pod", Name: "podinfo", Ready: new(true)},
				{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: "backend", Ready: new(true), ReadyExpr: "status.phase==Bound"},
				{Namespace: "default", Name: "postgres", ReadyExpr: "status.ready==true"},
				{Namespace: "infra", Name: "cert-manager", Ready: new(true)},
				{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "OCIRepository", Namespace: "flux-system", Name: "flux-operator", Ready: new(false)},
				{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "resourcesets.fluxcd.controlplane.io", Ready: new(true), ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "CEL expression with multiple operators",
			deps: []string{
				"default/app@status.ready==true && status.observed==1",
				"v1/Pod/kube-system/kube-apiserver:true@status.phase==Running && status.observedGeneration==metadata.generation",
			},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "app", ReadyExpr: "status.ready==true && status.observed==1"},
				{APIVersion: "v1", Kind: "Pod", Namespace: "kube-system", Name: "kube-apiserver", Ready: new(true), ReadyExpr: "status.phase==Running && status.observedGeneration==metadata.generation"},
			},
		},
		{
			name: "CEL expression with function calls",
			deps: []string{
				"infra/ingress@has(status.loadBalancer.ingress)",
				"v1/ConfigMap/podinfo:true@!has(data)",
			},
			want: []meta.DependencyReference{
				{Namespace: "infra", Name: "ingress", ReadyExpr: "has(status.loadBalancer.ingress)"},
				{APIVersion: "v1", Kind: "ConfigMap", Name: "podinfo", Ready: new(true), ReadyExpr: "!has(data)"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := meta.MakeDependsOn(tt.deps)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestDependencyReferenceString(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name string
		ref  meta.DependencyReference
		want string
	}{
		{
			name: "name only (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "redis"},
			want: "redis",
		},
		{
			name: "namespace and name (Flux Applier API)",
			ref:  meta.DependencyReference{Namespace: "default", Name: "redis"},
			want: "default/redis",
		},
		{
			name: "name and readiness check (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "redis", Ready: new(false)},
			want: "redis:false",
		},
		{
			name: "namespace, name, and readiness check (Flux Applier API)",
			ref:  meta.DependencyReference{Namespace: "default", Name: "redis", Ready: new(true)},
			want: "default/redis:true",
		},
		{
			name: "name and CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "redis", ReadyExpr: "status.ready==true"},
			want: "redis@status.ready==true",
		},
		{
			name: "namespace, name, and CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Namespace: "default", Name: "redis", ReadyExpr: "status.ready==true"},
			want: "default/redis@status.ready==true",
		},
		{
			name: "name, readiness check and CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "redis", Ready: new(false), ReadyExpr: "status.ready==true"},
			want: "redis:false@status.ready==true",
		},
		{
			name: "namespace, name, readiness check and CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Namespace: "default", Name: "redis", Ready: new(true), ReadyExpr: "status.ready==true"},
			want: "default/redis:true@status.ready==true",
		},
		{
			name: "name with complex CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "app", ReadyExpr: "status.ready==true && status.observed==1"},
			want: "app@status.ready==true && status.observed==1",
		},
		{
			name: "name with readiness check and complex CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "app", Ready: new(true), ReadyExpr: "status.ready==true && status.observed==1"},
			want: "app:true@status.ready==true && status.observed==1",
		},
		{
			name: "name only (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "Pod", Name: "redis"},
			want: "v1/Pod/redis",
		},
		{
			name: "namespace and name (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "apps/v1", Kind: "DaemonSet", Namespace: "default", Name: "redis"},
			want: "apps/v1/DaemonSet/default/redis",
		},
		{
			name: "name and readiness check (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "Secret", Name: "redis", Ready: new(false)},
			want: "v1/Secret/redis:false",
		},
		{
			name: "namespace, name and readiness check (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "redis", Ready: new(true)},
			want: "v1/ConfigMap/default/redis:true",
		},
		{
			name: "name and CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "PersistentVolume", Name: "redis", ReadyExpr: "status.ready==true"},
			want: "v1/PersistentVolume/redis@status.ready==true",
		},
		{
			name: "namespace, name and CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "PersistentVolumeClaim", Namespace: "default", Name: "redis", ReadyExpr: "status.ready==true"},
			want: "v1/PersistentVolumeClaim/default/redis@status.ready==true",
		},
		{
			name: "name, readiness check and CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "Node", Name: "control-plane-2", Ready: new(false), ReadyExpr: "status.ready==true"},
			want: "v1/Node/control-plane-2:false@status.ready==true",
		},
		{
			name: "namespace, name, readiness check and CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "Ingress", Namespace: "default", Name: "redis", Ready: new(true), ReadyExpr: "status.ready==true"},
			want: "v1/Ingress/default/redis:true@status.ready==true",
		},
		{
			name: "name and complex CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "GitRepository", Name: "infra-controllers", ReadyExpr: "status.ready==true && status.observed==1"},
			want: "source.toolkit.fluxcd.io/v1/GitRepository/infra-controllers@status.ready==true && status.observed==1",
		},
		{
			name: "namespace, name and complex CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "OCIRepository", Namespace: "flux-system", Name: "flux-operator", ReadyExpr: "status.ready==true && status.observed==1"},
			want: "source.toolkit.fluxcd.io/v1/OCIRepository/flux-system/flux-operator@status.ready==true && status.observed==1",
		},
		{
			name: "name, readiness check and complex CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "helmreleases.helm.toolkit.fluxcd.io", Ready: new(true), ReadyExpr: "status.ready==true && status.observed==1"},
			want: "apiextensions.k8s.io/v1/CustomResourceDefinition/helmreleases.helm.toolkit.fluxcd.io:true@status.ready==true && status.observed==1",
		},
		{
			name: "namespace, name, readiness check and complex CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Name: "app", Ready: new(true), ReadyExpr: "status.ready==true && status.observed==1"},
			want: "kustomize.toolkit.fluxcd.io/v1/Kustomization/app:true@status.ready==true && status.observed==1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestDependencyReferenceRoundTrip(t *testing.T) {
	g := NewWithT(t)

	tests := []meta.DependencyReference{
		{Name: "redis"},
		{Namespace: "default", Name: "postgres"},
		{Name: "cache", ReadyExpr: "status.ready==true"},
		{Namespace: "infra", Name: "ingress", ReadyExpr: "has(status.loadBalancer.ingress)"},
		{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "helmreleases.helm.toolkit.fluxcd.io"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Name: "cert-manager", Ready: new(false), ReadyExpr: "status.ready==true"},
		{APIVersion: "v1", Kind: "Node", Name: "control-plane-2", Ready: new(true)},
	}

	for _, original := range tests {
		t.Run(original.String(), func(t *testing.T) {
			// Serialize to string
			str := original.String()

			// Parse back from string
			parsed := meta.MakeDependsOn([]string{str})
			g.Expect(parsed).To(HaveLen(1))

			// Should match original
			g.Expect(parsed[0]).To(Equal(original))
		})
	}
}
