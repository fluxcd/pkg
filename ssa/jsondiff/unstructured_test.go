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

package jsondiff

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/wI2L/jsondiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/ssa"
)

const dummyFieldOwner = "dummy"

func TestUnstructuredList(t *testing.T) {
	tests := []struct {
		name          string
		paths         []string
		mutateCluster func(*unstructured.Unstructured)
		mutateDesired func(*unstructured.Unstructured)
		opts          []ListOption
		want          func(ns string) DiffSet
		wantErr       bool
	}{
		{
			name: "resources do not exist",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/service.yaml",
			},
			mutateCluster: func(obj *unstructured.Unstructured) {
				obj.Object = nil
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeCreate,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
						},
						Namespace: ns,
						Name:      "podinfo",
					},
					&Diff{
						Type: DiffTypeCreate,
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Kind:    "Service",
						},
						Namespace: ns,
						Name:      "podinfo",
					},
				}
			},
		},
		{
			name: "resources with multiple changes",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/service.yaml",
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				if obj.GetKind() == "Deployment" {
					_ = unstructured.SetNestedField(obj.Object, float64(2), "spec", "replicas")
				}
				if obj.GetKind() == "Service" {
					_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "annotations", "annotated")
				}
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
						},
						Namespace: ns,
						Name:      "podinfo",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationReplace, Path: "/spec/replicas", Value: float64(2), OldValue: float64(1)},
						},
					},
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Kind:    "Service",
						},
						Namespace: ns,
						Name:      "podinfo",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationAdd, Path: "/metadata", Value: map[string]interface{}{
								"annotations": map[string]interface{}{
									"annotated": "yes",
								},
							}},
						},
					},
				}
			},
		},
		{
			name: "excludes resources with matching label",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/service.yaml",
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				if obj.GetKind() != "Deployment" {
					return
				}

				labels := obj.GetLabels()
				labels["exclude"] = "enabled"
				obj.SetLabels(labels)

				_ = unstructured.SetNestedField(obj.Object, float64(2), "spec", "replicas")
			},
			opts: []ListOption{
				ExclusionSelector{"exclude": "enabled"},
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeExclude,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
						},
						Namespace: ns,
						Name:      "podinfo",
					},
					&Diff{
						Type: DiffTypeNone,
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Kind:    "Service",
						},
						Namespace: ns,
						Name:      "podinfo",
					},
				}
			},
		},
		{
			name: "excludes resources with matching annotation",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/service.yaml",
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				if obj.GetKind() != "Service" {
					return
				}

				annotations := obj.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations["exclude"] = "enabled"
				obj.SetAnnotations(annotations)

				_ = unstructured.SetNestedField(obj.Object, "NodePort", "spec", "type")
			},
			opts: []ListOption{
				ExclusionSelector{"exclude": "enabled"},
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeNone,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
						},
						Namespace: ns,
						Name:      "podinfo",
					},
					&Diff{
						Type: DiffTypeExclude,
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Kind:    "Service",
						},
						Namespace: ns,
						Name:      "podinfo",
					},
				}
			},
		},
		{
			name: "ignores JSON pointers for resources matching selector",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/service.yaml",
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "change", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "change", "metadata", "labels", "labeled")
			},
			opts: []ListOption{
				IgnoreRules{
					{
						Paths: []string{
							"/metadata/annotations",
						},
						Selector: &Selector{
							Kind: "Service",
						},
					},
				},
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
						},
						Namespace: ns,
						Name:      "podinfo",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationAdd, Path: "/metadata/annotations/annotated", Value: "change"},
							{Type: jsondiff.OperationAdd, Path: "/metadata/labels/labeled", Value: "change"},
						},
					},
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Kind:    "Service",
						},
						Namespace: ns,
						Name:      "podinfo",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationAdd, Path: "/metadata", Value: map[string]interface{}{
								"labels": map[string]interface{}{
									"labeled": "change",
								},
							}},
						},
					},
				}
			},
		},
		{
			name: "ignores paths for all resources without selector",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/service.yaml",
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "change", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "change", "metadata", "labels", "labeled")
			},
			opts: []ListOption{
				IgnoreRules{
					{
						Paths: []string{
							"/metadata/annotations",
						},
					},
				},
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
						},
						Namespace: ns,
						Name:      "podinfo",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationAdd, Path: "/metadata/labels/labeled", Value: "change"},
						},
					},
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Kind:    "Service",
						},
						Namespace: ns,
						Name:      "podinfo",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationAdd, Path: "/metadata", Value: map[string]interface{}{
								"labels": map[string]interface{}{
									"labeled": "change",
								},
							}},
						},
					},
				}
			},
		},
		{
			name: "masks Secret data",
			paths: []string{
				"testdata/empty-secret.yaml",
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "bar", "stringData", "foo")
				_ = ssa.NormalizeUnstructured(obj)
			},
			opts: []ListOption{
				MaskSecrets(true),
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "",
							Version: "v1",
							Kind:    "Secret",
						},
						Namespace: ns,
						Name:      "secret-data",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationAdd, Path: "/data", Value: map[string]interface{}{
								"foo": sensitiveMaskDefault,
							}},
						},
					},
				}
			},
		},
		{
			name: "rationalizes data",
			paths: []string{
				"testdata/empty-configmap.yaml",
			},
			mutateCluster: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedMap(obj.Object, map[string]interface{}{
					"a": "2",
					"b": "1",
				}, "data")
				_ = ssa.NormalizeUnstructured(obj)
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedMap(obj.Object, map[string]interface{}{
					"a": "1",
					"b": "2",
				}, "data")
				_ = ssa.NormalizeUnstructured(obj)
			},
			opts: []ListOption{
				Rationalize(true),
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Group:   "",
							Version: "v1",
							Kind:    "ConfigMap",
						},
						Namespace: ns,
						Name:      "configmap-data",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationReplace, Path: "/data", Value: map[string]interface{}{
								"a": "1",
								"b": "2",
							}, OldValue: map[string]interface{}{
								"a": "2",
								"b": "1",
							}},
						},
					},
				}
			},
		},
		{
			name: "handles errors gracefully when instructed",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/empty-configmap.yaml",
				"testdata/service.yaml",
			},
			mutateDesired: func(u *unstructured.Unstructured) {
				switch u.GetKind() {
				case "ConfigMap":
					_ = unstructured.SetNestedField(u.Object, "value", "data", "key")
				default:
					_ = unstructured.SetNestedField(u.Object, "invalid", "spec")
				}
			},
			opts: []ListOption{
				Graceful(true),
			},
			want: func(ns string) DiffSet {
				return DiffSet{
					&Diff{
						Type: DiffTypeUpdate,
						GroupVersionKind: schema.GroupVersionKind{
							Version: "v1",
							Kind:    "ConfigMap",
						},
						Namespace: ns,
						Name:      "configmap-data",
						Patch: jsondiff.Patch{
							{Type: jsondiff.OperationAdd, Path: "/data", Value: map[string]interface{}{"key": "value"}},
						},
					},
				}
			},
			wantErr: true,
		},
		{
			name: "returns error without graceful option",
			paths: []string{
				"testdata/deployment.yaml",
				"testdata/empty-configmap.yaml",
				"testdata/service.yaml",
			},
			mutateDesired: func(u *unstructured.Unstructured) {
				switch u.GetKind() {
				case "ConfigMap":
					_ = unstructured.SetNestedField(u.Object, "value", "data", "key")
				default:
					_ = unstructured.SetNestedField(u.Object, "invalid", "spec")
				}
			},
			want:    func(ns string) DiffSet { return nil },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)

			ns, err := CreateNamespace(ctx, "test-unstructured-list")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = testClient.Delete(ctx, ns) })

			var desired []*unstructured.Unstructured
			for _, path := range tt.paths {
				res, err := LoadResource(path)
				if err != nil {
					t.Fatal(err)
				}

				cObj, dObj := res.DeepCopy(), res.DeepCopy()
				cObj.SetNamespace(ns.Name)
				if tt.mutateCluster != nil {
					tt.mutateCluster(cObj)
				}
				if cObj.Object != nil {
					if err := testClient.Patch(ctx, cObj, client.Apply, client.FieldOwner(dummyFieldOwner)); err != nil {
						t.Fatal(err)
					}
				}

				dObj.SetNamespace(ns.Name)
				if tt.mutateDesired != nil {
					tt.mutateDesired(dObj)
				}
				if dObj != nil {
					desired = append(desired, dObj)
				}
			}

			opts := []ListOption{
				FieldOwner(dummyFieldOwner),
			}
			opts = append(opts, tt.opts...)
			change, err := UnstructuredList(ctx, testClient, desired, opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnstructuredList() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want(ns.Name), change, cmpopts.IgnoreUnexported(jsondiff.Operation{})); diff != "" {
				t.Errorf("UnstructuredList() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnstructured(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		mutateCluster func(*unstructured.Unstructured)
		mutateDesired func(*unstructured.Unstructured)
		opts          []ResourceOption
		want          func(ns string) *Diff
		wantErr       bool
	}{
		{
			name: "Deployment with added label and annotation",
			path: "testdata/deployment.yaml",
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "labels", "labeled")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationAdd, Path: "/metadata/annotations/annotated", Value: "yes"},
						{Type: jsondiff.OperationAdd, Path: "/metadata/labels/labeled", Value: "yes"},
					},
				}
			},
		},
		{
			name: "Deployment with missing label and annotation",
			path: "testdata/deployment.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "labels", "labeled")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeNone,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
				}
			},
		},
		{
			name: "Deployment with changed label and annotation",
			path: "testdata/deployment.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "no", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "no", "metadata", "labels", "labeled")
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "labels", "labeled")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationReplace, Path: "/metadata/annotations/annotated", Value: "yes", OldValue: "no"},
						{Type: jsondiff.OperationReplace, Path: "/metadata/labels/labeled", Value: "yes", OldValue: "no"},
					},
				}
			},
		},
		{
			name: "Deployment with ignored change path",
			path: "testdata/deployment.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "no", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "no", "metadata", "labels", "labeled")
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "annotations", "annotated")
				_ = unstructured.SetNestedField(obj.Object, "yes", "metadata", "labels", "labeled")
			},
			opts: []ResourceOption{
				IgnorePaths{"/metadata/annotations/annotated"},
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationReplace, Path: "/metadata/labels/labeled", Value: "yes", OldValue: "no"},
					},
				}
			},
		},
		{
			name: "Deployment with ignored root path",
			path: "testdata/deployment.yaml",
			opts: []ResourceOption{
				IgnorePaths{IgnorePathRoot},
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeExclude,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
				}
			},
		},
		{
			name: "Deployment with annotation matching exclusion selector",
			path: "testdata/deployment.yaml",
			opts: []ResourceOption{
				ExclusionSelector{
					"ignore": "enabled",
				},
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "enabled", "metadata", "annotations", "ignore")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeExclude,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
				}
			},
		},
		{
			name: "Deployment with label matching exclusion selector",
			path: "testdata/deployment.yaml",
			opts: []ResourceOption{
				ExclusionSelector{
					"ignore": "enabled",
				},
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "enabled", "metadata", "labels", "ignore")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeExclude,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
				}
			},
		},
		{
			name: "Deployment with added container",
			path: "testdata/deployment.yaml",
			mutateDesired: func(obj *unstructured.Unstructured) {
				containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
				containers = append(containers, map[string]interface{}{
					"name":  "nginx",
					"image": "nginx:latest",
				})
				_ = unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationAdd, Path: "/spec/template/spec/containers/-", Value: map[string]interface{}{
							"name":                     "nginx",
							"image":                    "nginx:latest",
							"imagePullPolicy":          "Always",
							"terminationMessagePath":   "/dev/termination-log",
							"terminationMessagePolicy": "File",
							"resources":                map[string]interface{}{},
						}},
					},
				}
			},
		},
		{
			name: "Deployment with removed container",
			path: "testdata/deployment.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
				containers = append(containers, map[string]interface{}{
					"name":  "nginx",
					"image": "nginx:latest",
				})
				_ = unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationRemove, Path: "/spec/template/spec/containers/1", OldValue: map[string]interface{}{
							"name":                     "nginx",
							"image":                    "nginx:latest",
							"imagePullPolicy":          "Always",
							"terminationMessagePath":   "/dev/termination-log",
							"terminationMessagePolicy": "File",
							"resources":                map[string]interface{}{},
						}},
					},
				}
			},
		},
		{
			name: "Deployment with changed container value",
			path: "testdata/deployment.yaml",
			mutateDesired: func(obj *unstructured.Unstructured) {
				containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
				containers[0].(map[string]interface{})["image"] = "nginx:latest"
				_ = unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers")
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationReplace, Path: "/spec/template/spec/containers/0/image", Value: "nginx:latest", OldValue: "ghcr.io/stefanprodan/podinfo:6.0.3"},
					},
				}
			},
		},
		{
			name: "Deployment with changed container value and ignored path",
			path: "testdata/deployment.yaml",
			mutateDesired: func(obj *unstructured.Unstructured) {
				containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
				containers[0].(map[string]interface{})["image"] = "nginx:latest"
				_ = unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers")
			},
			opts: []ResourceOption{
				IgnorePaths{"/spec/template/spec/containers/0/image"},
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeNone,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
				}
			},
		},
		{
			name: "Deployment without changes",
			path: "testdata/deployment.yaml",
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeNone,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
				}
			},
		},
		{
			name: "Deployment does not exist",
			path: "testdata/deployment.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				obj.Object = nil
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeCreate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
					Namespace: ns,
					Name:      "podinfo",
				}
			},
		},
		{
			name: "Secret without changes",
			path: "testdata/empty-secret.yaml",
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeNone,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Secret",
					},
					Namespace: ns,
					Name:      "secret-data",
				}
			},
		},
		{
			name: "Secret with added key and unmasked value",
			path: "testdata/empty-secret.yaml",
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "bar", "stringData", "foo")
				_ = ssa.NormalizeUnstructured(obj)
			},
			opts: []ResourceOption{
				MaskSecrets(false),
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Secret",
					},
					Namespace: ns,
					Name:      "secret-data",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationAdd, Path: "/data", Value: map[string]interface{}{
							"foo": "YmFy",
						}},
					},
				}
			},
		},
		{
			name: "Secret with changed and deleted key and masked value",
			path: "testdata/empty-secret.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "bar", "stringData", "foo")
				_ = unstructured.SetNestedField(obj.Object, "bar", "stringData", "bar")
				_ = ssa.NormalizeUnstructured(obj)
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "baz", "stringData", "foo")
				_ = ssa.NormalizeUnstructured(obj)
			},
			opts: []ResourceOption{
				MaskSecrets(true),
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Secret",
					},
					Namespace: ns,
					Name:      "secret-data",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationRemove, Path: "/data/bar", OldValue: sensitiveMaskDefault},
						{Type: jsondiff.OperationReplace, Path: "/data/foo", OldValue: sensitiveMaskBefore, Value: sensitiveMaskAfter},
					},
				}
			},
		},
		{
			name: "Secret with changed and deleted key, and rationalization enabled",
			path: "testdata/empty-secret.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "bar", "stringData", "foo")
				_ = unstructured.SetNestedField(obj.Object, "bar", "stringData", "bar")
				_ = ssa.NormalizeUnstructured(obj)
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "baz", "stringData", "foo")
				_ = ssa.NormalizeUnstructured(obj)
			},
			opts: []ResourceOption{
				MaskSecrets(true),
				Rationalize(true),
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Secret",
					},
					Namespace: ns,
					Name:      "secret-data",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationReplace, Path: "/data", OldValue: map[string]interface{}{
							"bar": sensitiveMaskDefault,
							"foo": sensitiveMaskBefore,
						}, Value: map[string]interface{}{
							"foo": sensitiveMaskAfter,
						}},
					},
				}
			},
		},
		{
			name: "ConfigMap is not masked",
			path: "testdata/empty-configmap.yaml",
			mutateCluster: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "bar", "data", "foo")
			},
			mutateDesired: func(obj *unstructured.Unstructured) {
				_ = unstructured.SetNestedField(obj.Object, "baz", "data", "foo")
			},
			opts: []ResourceOption{
				MaskSecrets(true),
			},
			want: func(ns string) *Diff {
				return &Diff{
					Type: DiffTypeUpdate,
					GroupVersionKind: schema.GroupVersionKind{
						Version: "v1",
						Kind:    "ConfigMap",
					},
					Namespace: ns,
					Name:      "configmap-data",
					Patch: jsondiff.Patch{
						{Type: jsondiff.OperationReplace, Path: "/data/foo", OldValue: "bar", Value: "baz"},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)

			ns, err := CreateNamespace(ctx, "test-resource")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = testClient.Delete(ctx, ns) })

			res, err := LoadResource(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			cluster, desired := res.DeepCopy(), res.DeepCopy()

			cluster.SetNamespace(ns.Name)
			if tt.mutateCluster != nil {
				tt.mutateCluster(cluster)
			}
			if cluster.Object != nil {
				if err := testClient.Patch(ctx, cluster, client.Apply, client.FieldOwner(dummyFieldOwner)); err != nil {
					t.Fatal(err)
				}
			}

			desired.SetNamespace(ns.Name)
			if tt.mutateDesired != nil {
				tt.mutateDesired(desired)
			}

			opts := []ResourceOption{
				FieldOwner(dummyFieldOwner),
			}
			opts = append(opts, tt.opts...)
			change, err := Unstructured(ctx, testClient, desired, opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unstructured() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(tt.want(ns.Name), change, cmpopts.IgnoreUnexported(jsondiff.Operation{})); diff != "" {
				t.Errorf("Unstructured() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_diffUnstructuredMetadata(t *testing.T) {
	tests := []struct {
		name    string
		x       *unstructured.Unstructured
		y       *unstructured.Unstructured
		opts    []jsondiff.Option
		want    jsondiff.Patch
		wantErr bool
	}{
		{
			name: "label added",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
							"bar": "foo",
						},
					},
				},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/metadata/labels/bar", Value: "foo"},
			},
		},
		{
			name: "label removed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
							"bar": "foo",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "label changed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "baz",
						},
					},
				},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/metadata/labels/foo", OldValue: "bar", Value: "baz"},
			},
		},
		{
			name: "annotation added",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"foo": "bar",
							"bar": "foo",
						},
					},
				},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationAdd, Path: "/metadata/annotations/bar", Value: "foo"},
			},
		},
		{
			name: "annotation removed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"foo": "bar",
							"bar": "foo",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "annotation changed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"foo": "bar",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"foo": "baz",
						},
					},
				},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/metadata/annotations/foo", OldValue: "bar", Value: "baz"},
			},
		},
		{
			name: "label and annotation changed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
						},
						"annotations": map[string]interface{}{
							"bar": "foo",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "baz",
						},
						"annotations": map[string]interface{}{
							"bar": "baz",
						},
					},
				},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/metadata/annotations/bar", OldValue: "foo", Value: "baz"},
				{Type: jsondiff.OperationReplace, Path: "/metadata/labels/foo", OldValue: "bar", Value: "baz"},
			},
		},
		{
			name: "label and annotation changed with ignore path",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "bar",
						},
						"annotations": map[string]interface{}{
							"bar": "foo",
						},
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"foo": "baz",
						},
						"annotations": map[string]interface{}{
							"bar": "baz",
						},
					},
				},
			},
			opts: []jsondiff.Option{
				jsondiff.Ignores("/metadata/annotations/bar"),
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/metadata/labels/foo", Value: "baz", OldValue: "bar"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := diffUnstructuredMetadata(tt.x, tt.y, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("diffResourceMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreUnexported(jsondiff.Operation{})); diff != "" {
				t.Errorf("diffResourceMetadata() got = %v", diff)
			}
		})
	}
}

func Test_diffUnstructured(t *testing.T) {
	tests := []struct {
		name    string
		x       *unstructured.Unstructured
		y       *unstructured.Unstructured
		opts    []jsondiff.Option
		want    jsondiff.Patch
		wantErr bool
	}{
		{
			name: "no diff",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": float64(1),
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": float64(1),
					},
				},
			},
			want: nil,
		},
		{
			name: "spec changed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": float64(1),
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": float64(2),
					},
				},
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/spec/replicas", OldValue: float64(1), Value: float64(2)},
			},
		},
		{
			name: "data change with rationalization",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"data": map[string]interface{}{
						"a": "1",
						"b": "2",
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"data": map[string]interface{}{
						"a": "2",
						"b": "1",
					},
				},
			},
			opts: []jsondiff.Option{
				jsondiff.Rationalize(),
			},
			want: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/data", OldValue: map[string]interface{}{
					"a": "1",
					"b": "2",
				}, Value: map[string]interface{}{
					"a": "2",
					"b": "1",
				}},
			},
		},
		{
			name: "metadata changed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "foo",
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "bar",
					},
				},
			},
			want: nil,
		},
		{
			name: "status changed",
			x: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"observedGeneration": int64(1),
					},
				},
			},
			y: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"observedGeneration": int64(2),
					},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := diffUnstructured(tt.x, tt.y, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("diffResourceMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreUnexported(jsondiff.Operation{})); diff != "" {
				t.Errorf("diffResourceMetadata() got = %v", diff)
			}
		})
	}
}

func Test_copyAnnotationsAndLabels(t *testing.T) {
	tests := []struct {
		name string
		u    *unstructured.Unstructured
		want *unstructured.Unstructured
	}{
		{
			name: "copy annotations and labels",
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"annotation1": true,
							"annotation2": "value",
						},
						"labels": map[string]interface{}{
							"label1": false,
							"label2": "value",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 1,
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"annotation1": true,
							"annotation2": "value",
						},
						"labels": map[string]interface{}{
							"label1": false,
							"label2": "value",
						},
					},
				},
			},
		},
		{
			name: "copy annotations and labels with empty metadata",
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": 1,
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
		},
		{
			name: "copy annotations and labels with empty annotations and labels",
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{},
						"labels":      map[string]interface{}{},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{},
						"labels":      map[string]interface{}{},
					},
				},
			},
		},
		{
			name: "copy annotations and labels with nil annotations and labels",
			u: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": nil,
						"labels":      nil,
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": nil,
						"labels":      nil,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := copyAnnotationsAndLabels(tt.u); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("copyAnnotationsAndLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
