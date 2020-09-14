/*
Copyright 2020 The Flux CD contributors.

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

	"k8s.io/apimachinery/pkg/types"
)

type MockDependent struct {
	NamespacedName types.NamespacedName
	DependsOn      []CrossNamespaceDependencyReference
}

func (d MockDependent) GetDependsOn() (types.NamespacedName, []CrossNamespaceDependencyReference) {
	return d.NamespacedName, d.DependsOn
}

func TestDependencySort(t *testing.T) {
	tests := []struct {
		name    string
		d       []Dependent
		want    []CrossNamespaceDependencyReference
		wantErr bool
	}{
		{
			"simple",
			[]Dependent{
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "frontend",
					},
					DependsOn: []CrossNamespaceDependencyReference{
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
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "linkerd",
						Name:      "linkerd",
					},
				},
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "backend",
					},
					DependsOn: []CrossNamespaceDependencyReference{
						{
							Namespace: "linkerd",
							Name:      "linkerd",
						},
					},
				},
			},
			[]CrossNamespaceDependencyReference{
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
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "dependency",
					},
					DependsOn: []CrossNamespaceDependencyReference{
						{
							Namespace: "default",
							Name:      "endless",
						},
					},
				},
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "endless",
					},
					DependsOn: []CrossNamespaceDependencyReference{
						{
							Namespace: "default",
							Name:      "circular",
						},
					},
				},
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      "circular",
					},
					DependsOn: []CrossNamespaceDependencyReference{
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
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "application",
						Name:      "backend",
					},
				},
				MockDependent{
					NamespacedName: types.NamespacedName{
						Namespace: "application",
						Name:      "frontend",
					},
					DependsOn: []CrossNamespaceDependencyReference{
						{
							Name: "backend",
						},
					},
				},
			},
			[]CrossNamespaceDependencyReference{
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
		MockDependent{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "backend",
			},
			DependsOn: []CrossNamespaceDependencyReference{
				{
					Namespace: "default",
					Name:      "common",
				},
			},
		},
		MockDependent{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "frontend",
			},
			DependsOn: []CrossNamespaceDependencyReference{
				{
					Namespace: "default",
					Name:      "infra",
				},
			},
		},
		MockDependent{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "common",
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
