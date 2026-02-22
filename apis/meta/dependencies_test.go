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
			name: "single name only",
			deps: []string{"redis"},
			want: []meta.DependencyReference{
				{Name: "redis"},
			},
		},
		{
			name: "single with namespace",
			deps: []string{"default/redis"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis"},
			},
		},
		{
			name: "single with CEL expression",
			deps: []string{"redis@status.ready==true"},
			want: []meta.DependencyReference{
				{Name: "redis", ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "single with namespace and CEL expression",
			deps: []string{"default/redis@status.ready==true"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "redis", ReadyExpr: "status.ready==true"},
			},
		},
		{
			name: "multiple dependencies",
			deps: []string{
				"redis",
				"default/postgres@status.ready==true",
				"infra/cert-manager",
			},
			want: []meta.DependencyReference{
				{Name: "redis"},
				{Namespace: "default", Name: "postgres", ReadyExpr: "status.ready==true"},
				{Namespace: "infra", Name: "cert-manager"},
			},
		},
		{
			name: "CEL expression with multiple operators",
			deps: []string{"default/app@status.ready==true && status.observed==1"},
			want: []meta.DependencyReference{
				{Namespace: "default", Name: "app", ReadyExpr: "status.ready==true && status.observed==1"},
			},
		},
		{
			name: "CEL expression with function calls",
			deps: []string{"infra/ingress@has(status.loadBalancer.ingress)"},
			want: []meta.DependencyReference{
				{Namespace: "infra", Name: "ingress", ReadyExpr: "has(status.loadBalancer.ingress)"},
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
			name: "name only",
			ref:  meta.DependencyReference{Name: "redis"},
			want: "redis",
		},
		{
			name: "namespace and name",
			ref:  meta.DependencyReference{Namespace: "default", Name: "redis"},
			want: "default/redis",
		},
		{
			name: "name and CEL expression",
			ref:  meta.DependencyReference{Name: "redis", ReadyExpr: "status.ready==true"},
			want: "redis@status.ready==true",
		},
		{
			name: "namespace, name, and CEL expression",
			ref:  meta.DependencyReference{Namespace: "default", Name: "redis", ReadyExpr: "status.ready==true"},
			want: "default/redis@status.ready==true",
		},
		{
			name: "name with complex CEL expression",
			ref:  meta.DependencyReference{Name: "app", ReadyExpr: "status.ready==true && status.observed==1"},
			want: "app@status.ready==true && status.observed==1",
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
