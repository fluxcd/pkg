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
			name: "single (Flux Applier API) with namespace",
			deps: []string{"default/redis"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis"},
			},
		},
		{
			name: "single (Flux Applier API) with CEL expression",
			deps: []string{"redis@dep.status.ready==true"},
			want: []meta.DependencyReference{
				{Name: "redis", ReadyExpr: "dep.status.ready==true"},
			},
		},
		{
			name: "single (Flux Applier API) with namespace and CEL expression",
			deps: []string{"default/redis@dep.status.ready==true"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis", ReadyExpr: "dep.status.ready==true"},
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
			name: "single Kubernetes object with namespace",
			deps: []string{"v1/Secret/default/redis"},
			want: []meta.DependencyReference{
				{APIVersion: "v1", Kind: "Secret", Namespace: "default", Name: "redis"},
			},
		},
		{
			name: "single Kubernetes object with CEL expression",
			deps: []string{"apps/v1/DaemonSet/redis@dep.status.numberReady==dep.status.desiredNumberScheduled"},
			want: []meta.DependencyReference{
				{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "redis", ReadyExpr: "dep.status.numberReady==dep.status.desiredNumberScheduled"},
			},
		},
		{
			name: "single Kubernetes object with namespace and CEL expression",
			deps: []string{"apps/v1/StatefulSet/default/redis@dep.status.readyReplicas==dep.spec.replicas"},
			want: []meta.DependencyReference{
				{APIVersion: "apps/v1", Kind: "StatefulSet", Namespace: "default", Name: "redis", ReadyExpr: "dep.status.readyReplicas==dep.spec.replicas"},
			},
		},
		{
			name: "multiple dependencies",
			deps: []string{
				"redis",
				"v1/Pod/podinfo",
				"v1/PersistentVolumeClaim/backend@dep.status.phase==Bound",
				"default/postgres@dep.status.observedGeneration==dep.metadata.generation",
				"infra/cert-manager",
				"source.toolkit.fluxcd.io/v1/OCIRepository/flux-system/flux-operator@true",
				"apiextensions.k8s.io/v1/CustomResourceDefinition/resourcesets.fluxcd.controlplane.io@dep.status.storedVersions.size()==dep.spec.versions.size()",
			},
			want: []meta.DependencyReference{
				{Name: "redis"},
				{APIVersion: "v1", Kind: "Pod", Name: "podinfo"},
				{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: "backend", ReadyExpr: "dep.status.phase==Bound"},
				{Namespace: "default", Name: "postgres", ReadyExpr: "dep.status.observedGeneration==dep.metadata.generation"},
				{Namespace: "infra", Name: "cert-manager"},
				{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "OCIRepository", Namespace: "flux-system", Name: "flux-operator", ReadyExpr: "true"},
				{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "resourcesets.fluxcd.controlplane.io", ReadyExpr: "dep.status.storedVersions.size()==dep.spec.versions.size()"},
			},
		},
		{
			name: "CEL expression with function calls",
			deps: []string{
				"v1/ConfigMap/podinfo@!has(dep.data)",
				"networking.k8s.io/v1/Ingress/infra/ingress@has(dep.status.loadBalancer.ingress)",
			},
			want: []meta.DependencyReference{
				{APIVersion: "v1", Kind: "ConfigMap", Name: "podinfo", ReadyExpr: "!has(dep.data)"},
				{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Namespace: "infra", Name: "ingress", ReadyExpr: "has(dep.status.loadBalancer.ingress)"},
			},
		},
		{
			name: "CEL expression with multiple operators",
			deps: []string{
				"default/app@dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)",
				"v1/Pod/kube-system/kube-apiserver@dep.status.phase==Running && dep.status.observedGeneration==dep.metadata.generation",
			},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "app", ReadyExpr: "dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)"},
				{APIVersion: "v1", Kind: "Pod", Namespace: "kube-system", Name: "kube-apiserver", ReadyExpr: "dep.status.phase==Running && dep.status.observedGeneration==dep.metadata.generation"},
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
			name: "name and CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "redis", ReadyExpr: "dep.status.observedGeneration>=1"},
			want: "redis@dep.status.observedGeneration>=1",
		},
		{
			name: "namespace, name, and CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Namespace: "default", Name: "redis", ReadyExpr: "dep.status.observedGeneration>=1"},
			want: "default/redis@dep.status.observedGeneration>=1",
		},
		{
			name: "name with complex CEL expression (Flux Applier API)",
			ref:  meta.DependencyReference{Name: "app", ReadyExpr: "dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)"},
			want: "app@dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)",
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
			name: "name and CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "PersistentVolume", Name: "redis", ReadyExpr: "dep.status.phase==Bound && dep.status.observedGeneration==dep.metadata.generation"},
			want: "v1/PersistentVolume/redis@dep.status.phase==Bound && dep.status.observedGeneration==dep.metadata.generation",
		},
		{
			name: "namespace, name and CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "v1", Kind: "PersistentVolumeClaim", Namespace: "default", Name: "redis", ReadyExpr: "dep.status.phase==Bound && dep.status.observedGeneration==dep.metadata.generation"},
			want: "v1/PersistentVolumeClaim/default/redis@dep.status.phase==Bound && dep.status.observedGeneration==dep.metadata.generation",
		},
		{
			name: "name and complex CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "GitRepository", Name: "infra-controllers", ReadyExpr: "dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)"},
			want: "source.toolkit.fluxcd.io/v1/GitRepository/infra-controllers@dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)",
		},
		{
			name: "namespace, name and complex CEL expression (Kubernetes object)",
			ref:  meta.DependencyReference{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "OCIRepository", Namespace: "flux-system", Name: "flux-operator", ReadyExpr: "dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)"},
			want: "source.toolkit.fluxcd.io/v1/OCIRepository/flux-system/flux-operator@dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == dep.metadata.generation)",
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
		{Name: "cache", ReadyExpr: "dep.status.observedGeneration==dep.metadata.generation"},
		{Namespace: "infra", Name: "ingress", ReadyExpr: "has(dep.status.loadBalancer.ingress)"},
		{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "helmreleases.helm.toolkit.fluxcd.io", ReadyExpr: "dep.status.storedVersions.size()==dep.spec.versions.size()"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Name: "cert-manager", ReadyExpr: ""},
		{APIVersion: "v1", Kind: "Node", Name: "control-plane-2", ReadyExpr: "true"},
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
