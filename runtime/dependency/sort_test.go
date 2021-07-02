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

package dependency

import (
	"reflect"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockDependent struct {
	corev1.Node
	DependsOn      []meta.NamespacedObjectReference
}

func (d MockDependent) GetDependsOn() []meta.NamespacedObjectReference {
	return d.DependsOn
}

func TestDependencySort(t *testing.T) {
	tests := []struct {
		name    string
		d       []Dependent
		want    []meta.NamespacedObjectReference
		wantErr bool
	}{
		{
			"simple",
			[]Dependent{
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "default",
							Name:      "frontend",
						},
					},
					DependsOn: []meta.NamespacedObjectReference{
						{
							Namespace: "linkerd",
							Name:      "linkerd",
						},
						{
							Namespace: "default",
							Name:      "backend",
						},
					},
				},
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "linkerd",
							Name:      "linkerd",
						},
					},
				},
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "default",
							Name:      "backend",
						},
					},
					DependsOn: []meta.NamespacedObjectReference{
						{
							Namespace: "linkerd",
							Name:      "linkerd",
						},
					},
				},
			},
			[]meta.NamespacedObjectReference{
				{
					Namespace: "linkerd",
					Name:      "linkerd",
				},
				{
					Namespace: "default",
					Name:      "backend",
				},
				{
					Namespace: "default",
					Name:      "frontend",
				},
			},
			false,
		},
		{
			"circular dependency",
			[]Dependent{
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "default",
							Name:      "dependency",
						},
					},
					DependsOn: []meta.NamespacedObjectReference{
						{
							Namespace: "default",
							Name:      "endless",
						},
					},
				},
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "default",
							Name:      "endless",
						},
					},
					DependsOn: []meta.NamespacedObjectReference{
						{
							Namespace: "default",
							Name:      "circular",
						},
					},
				},
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "default",
							Name:      "circular",
						},
					},
					DependsOn: []meta.NamespacedObjectReference{
						{
							Namespace: "default",
							Name:      "dependency",
						},
					},
				},
			},
			nil,
			true,
		},
		{
			"missing namespace",
			[]Dependent{
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "application",
							Name:      "backend",
						},
					},
				},
				&MockDependent{
					Node: corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Namespace: "application",
							Name:      "frontend",
						},
					},
					DependsOn: []meta.NamespacedObjectReference{
						{
							Name: "backend",
						},
					},
				},
			},
			[]meta.NamespacedObjectReference{
				{
					Namespace: "application",
					Name:      "backend",
				},
				{
					Namespace: "application",
					Name:      "frontend",
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sort(tt.d)
			if (err != nil) != tt.wantErr {
				t.Errorf("Sort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sort() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencySort_DeadEnd(t *testing.T) {
	d := []Dependent{
		&MockDependent{
			Node: corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "backend",
				},
			},
			DependsOn: []meta.NamespacedObjectReference{
				{
					Namespace: "default",
					Name:      "common",
				},
			},
		},
		&MockDependent{
			Node: corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "frontend",
				},
			},
			DependsOn: []meta.NamespacedObjectReference{
				{
					Namespace: "default",
					Name:      "infra",
				},
			},
		},
		&MockDependent{
			Node: corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      "common",
				},
			},
		},
	}
	got, err := Sort(d)
	if err != nil {
		t.Errorf("Sort() error = %v", err)
		return
	}
	if len(got) != len(d) {
		t.Errorf("Sort() len = %v, want %v", len(got), len(d))
	}
}
