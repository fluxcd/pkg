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
	name      string
	namespace string
	dependsOn []meta.NamespacedObjectReference
}

func (in object) GetName() string {
	return in.name
}

func (in object) GetNamespace() string {
	return in.namespace
}

func (in object) GetDependsOn() []meta.NamespacedObjectReference {
	return in.dependsOn
}

func TestSort(t *testing.T) {
	for _, tt := range []struct {
		name    string
		objects []dependency.Dependent
		want    []meta.NamespacedObjectReference
		err     string
	}{
		{
			name: "simple",
			objects: []dependency.Dependent{
				&object{
					name:      "frontend",
					namespace: "default",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "linkerd", Name: "linkerd"},
						{Namespace: "default", Name: "backend"},
					},
				},
				&object{
					name:      "linkerd",
					namespace: "linkerd",
				},
				&object{
					namespace: "default",
					name:      "backend",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "linkerd", Name: "linkerd"},
					},
				},
			},
			want: []meta.NamespacedObjectReference{
				{Namespace: "linkerd", Name: "linkerd"},
				{Namespace: "default", Name: "backend"},
				{Namespace: "default", Name: "frontend"},
			},
		},
		{
			name: "circular dependency",
			objects: []dependency.Dependent{
				&object{
					name:      "dependency",
					namespace: "default",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "default", Name: "endless"},
					},
				},
				&object{
					name:      "endless",
					namespace: "default",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "default", Name: "circular"},
					},
				},
				&object{
					name:      "circular",
					namespace: "default",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "default", Name: "dependency"},
					},
				},
			},
			err: "circular dependency detected: default/dependency -> default/endless -> default/circular -> default/dependency",
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
					dependsOn: []meta.NamespacedObjectReference{
						{Name: "backend"},
					},
				},
			},
			want: []meta.NamespacedObjectReference{
				{Namespace: "application", Name: "backend"},
				{Namespace: "application", Name: "frontend"},
			},
		},
		{
			name: "dead end",
			objects: []dependency.Dependent{
				&object{
					name:      "backend",
					namespace: "default",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "default", Name: "common"},
					},
				},
				&object{
					name:      "frontend",
					namespace: "default",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "default", Name: "infra"},
					},
				},
				&object{
					name:      "common",
					namespace: "default",
				},
			},
			want: []meta.NamespacedObjectReference{
				{Namespace: "default", Name: "common"},
				{Namespace: "default", Name: "backend"},
				{Namespace: "default", Name: "infra"},
				{Namespace: "default", Name: "frontend"},
			},
		},
		{
			name: "vertices not in the input list",
			objects: []dependency.Dependent{
				&object{
					name:      "frontend",
					namespace: "default",
					dependsOn: []meta.NamespacedObjectReference{
						{Namespace: "linkerd", Name: "linkerd"},
						{Namespace: "default", Name: "backend"},
					},
				},
			},
			want: []meta.NamespacedObjectReference{
				{Namespace: "linkerd", Name: "linkerd"},
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
