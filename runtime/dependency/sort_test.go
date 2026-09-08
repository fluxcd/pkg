/*
Copyright 2020 The Flux authors

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
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/dependency"
)

type object struct {
	apiVersion string
	kind       string
	name       string
	namespace  string
	dependsOn  []meta.DependencyReference
}

func (in *object) GetAPIVersion() string {
	return in.apiVersion
}

func (in *object) GetKind() string {
	return in.kind
}

func (in *object) GetName() string {
	return in.name
}

func (in *object) GetNamespace() string {
	return in.namespace
}

func (in *object) GetDependsOn() []meta.DependencyReference {
	return in.dependsOn
}

func TestSort(t *testing.T) {
	for _, tt := range []struct {
		name    string
		objects []dependency.Dependent
		want    []meta.TypedNamespacedObjectReference
		err     string
	}{
		{
			name: "simple",
			objects: []dependency.Dependent{
				&object{
					name:      "frontend",
					namespace: "default",
					dependsOn: []meta.DependencyReference{
						{Namespace: "linkerd", Name: "linkerd"},
						{Namespace: "default", Name: "backend"},
						{APIVersion: "cert-manager.io/v1", Kind: "Certificate", Namespace: "default", Name: "frontend"},
					},
				},
				&object{
					name:      "linkerd",
					namespace: "linkerd",
				},
				&object{
					namespace: "default",
					name:      "backend",
					dependsOn: []meta.DependencyReference{
						{Namespace: "linkerd", Name: "linkerd"},
					},
				},
				&object{
					apiVersion: "cert-manager.io/v1",
					kind:       "Certificate",
					name:       "frontend",
					namespace:  "default",
				},
			},
			want: []meta.TypedNamespacedObjectReference{
				{Namespace: "linkerd", Name: "linkerd"},
				{Namespace: "default", Name: "backend"},
				{APIVersion: "cert-manager.io/v1", Kind: "Certificate", Namespace: "default", Name: "frontend"},
				{Namespace: "default", Name: "frontend"},
			},
		},
		{
			name: "circular dependency",
			objects: []dependency.Dependent{
				&object{
					name:      "dependency",
					namespace: "default",
					dependsOn: []meta.DependencyReference{
						{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "default", Name: "endless"},
					},
				},
				&object{
					apiVersion: "helm.toolkit.fluxcd.io/v2",
					kind:       "HelmRelease",
					name:       "endless",
					namespace:  "default",
					dependsOn: []meta.DependencyReference{
						{Namespace: "default", Name: "circular"},
					},
				},
				&object{
					name:      "circular",
					namespace: "default",
					dependsOn: []meta.DependencyReference{
						{Namespace: "default", Name: "dependency"},
					},
				},
			},
			err: "circular dependency detected: default/dependency -> helm.toolkit.fluxcd.io/v2/HelmRelease/default/endless -> default/circular -> default/dependency",
		},
		{
			name: "missing namespace",
			objects: []dependency.Dependent{
				&object{
					name:      "backend",
					namespace: "application",
				},
				&object{
					name:      "frontend",
					namespace: "application",
					dependsOn: []meta.DependencyReference{
						{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "resourcesets.fluxcd.controlplane.io"},
						{Name: "backend"},
					},
				},
				&object{
					apiVersion: "apiextensions.k8s.io/v1",
					kind:       "CustomResourceDefinition",
					name:       "resourcesets.fluxcd.controlplane.io",
				},
			},
			want: []meta.TypedNamespacedObjectReference{
				{Namespace: "application", Name: "backend"},
				{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "resourcesets.fluxcd.controlplane.io"},
				{Namespace: "application", Name: "frontend"},
			},
		},
		{
			name: "dead end",
			objects: []dependency.Dependent{
				&object{
					name:      "backend",
					namespace: "default",
					dependsOn: []meta.DependencyReference{
						{APIVersion: "cert-manager.io/v1", Kind: "ClusterIssuer", Namespace: "default", Name: "acme-issuer"},
						{Namespace: "default", Name: "common"},
					},
				},
				&object{
					name:      "frontend",
					namespace: "default",
					dependsOn: []meta.DependencyReference{
						{Namespace: "default", Name: "infra"},
						{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "default", Name: "podinfo"},
					},
				},
				&object{
					name:      "common",
					namespace: "default",
					dependsOn: []meta.DependencyReference{
						{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "default", Name: "crds"},
					},
				},
			},
			want: []meta.TypedNamespacedObjectReference{
				{APIVersion: "cert-manager.io/v1", Kind: "ClusterIssuer", Namespace: "default", Name: "acme-issuer"},
				{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "default", Name: "crds"},
				{Namespace: "default", Name: "common"},
				{Namespace: "default", Name: "backend"},
				{Namespace: "default", Name: "infra"},
				{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "default", Name: "podinfo"},
				{Namespace: "default", Name: "frontend"},
			},
		},
		{
			name: "vertices not in the input list",
			objects: []dependency.Dependent{
				&object{
					name:      "frontend",
					namespace: "default",
					dependsOn: []meta.DependencyReference{
						{APIVersion: "cert-manager.io/v1", Kind: "ClusterIssuer", Namespace: "default", Name: "acme-issuer"},
						{Namespace: "linkerd", Name: "linkerd"},
						{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: "backend-data"},
						{Namespace: "default", Name: "backend"},
					},
				},
			},
			want: []meta.TypedNamespacedObjectReference{
				{APIVersion: "cert-manager.io/v1", Kind: "ClusterIssuer", Namespace: "default", Name: "acme-issuer"},
				{Namespace: "linkerd", Name: "linkerd"},
				{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: "backend-data"},
				{Namespace: "default", Name: "backend"},
				{Namespace: "default", Name: "frontend"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := dependency.Sort(tt.objects)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.err))
				g.Expect(got).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(got).To(Equal(tt.want))
			}
		})
	}
}
