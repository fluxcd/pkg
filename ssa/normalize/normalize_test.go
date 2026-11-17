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

package normalize

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func TestFromUnstructured(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    metav1.Object
		wantErr bool
	}{
		{
			name: "no rules ClusterRole",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"kind":       "ClusterRole",
					"metadata": map[string]any{
						"name": "test-clusterrole",
					},
				},
			},
			want: &rbacv1.ClusterRole{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
				Name:       "test-clusterrole",
			},
		},
		{
			name: "valid Pod",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "test-pod",
						"namespace": "test-namespace",
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name":  "test-container",
								"image": "test-image",
							},
						},
					},
				},
			},
			want: &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "test-pod",
				Namespace:  "test-namespace",
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
		},
		{
			name: "valid Deployment",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "test-namespace",
					},
					"spec": map[string]any{
						"replicas": int64(1),
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name":  "test-container",
										"image": "test-image",
									},
								},
							},
						},
					},
				},
			},
			want: &appsv1.Deployment{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test-deployment",
				Namespace:  "test-namespace",
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-image",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "unrecognized GroupVersionKind",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]any{
						"name":      "test-name",
						"namespace": "test-namespace",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromUnstructured(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromUnstructured() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("FromUnstructured() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFromUnstructuredWithScheme(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		scheme  *runtime.Scheme
		want    metav1.Object
		wantErr bool
	}{
		{
			name: "valid Pod",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "test-pod",
						"namespace": "test-namespace",
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name":  "test-container",
								"image": "test-image",
							},
						},
					},
				},
			},
			scheme: defaultScheme,
			want: &corev1.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "test-pod",
				Namespace:  "test-namespace",
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
		},
		{
			name: "unrecognized Deployment",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "test-namespace",
					},
					"spec": map[string]any{
						"replicas": int64(1),
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name":  "test-container",
										"image": "test-image",
									},
								},
							},
						},
					},
				},
			},
			scheme:  runtime.NewScheme(),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromUnstructuredWithScheme(tt.object, tt.scheme)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromUnstructuredWithScheme() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("FromUnstructuredWithScheme() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNormalizeUnstructured(t *testing.T) {
	tests := []struct {
		name    string
		scheme  *runtime.Scheme
		object  *unstructured.Unstructured
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "adds default port protocol to Pod",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"ports": []any{
									map[string]any{
										"containerPort": 80,
									},
									map[string]any{
										"containerPort": 8080,
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "",
								"ports": []any{
									map[string]any{
										"containerPort": int64(80),
										"protocol":      "TCP",
									},
									map[string]any{
										"containerPort": int64(8080),
										"protocol":      "TCP",
									},
								},
								"resources": map[string]any{},
							},
						},
					},
				},
			},
		},
		{
			name: "adds default port protocol to Deployment",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"ports": []any{
											map[string]any{
												"containerPort": 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"selector": nil,
						"strategy": map[string]any{},
						"template": map[string]any{
							"metadata": map[string]any{},
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name": "",
										"ports": []any{
											map[string]any{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]any{},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "adds default port protocol to StatefulSet",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"ports": []any{
											map[string]any{
												"containerPort": 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"selector":    nil,
						"serviceName": "",
						"template": map[string]any{
							"metadata": map[string]any{},
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name": "",
										"ports": []any{
											map[string]any{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]any{},
									},
								},
							},
						},
						"updateStrategy": map[string]any{},
					},
				},
			},
		},
		{
			name: "adds default port protocol to DaemonSet",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"ports": []any{
											map[string]any{
												"containerPort": 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"selector": nil,
						"template": map[string]any{
							"metadata": map[string]any{},
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name": "",
										"ports": []any{
											map[string]any{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]any{},
									},
								},
							},
						},
						"updateStrategy": map[string]any{},
					},
				},
			},
		},
		{
			name: "adds default port protocol to ReplicaSet",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"ports": []any{
											map[string]any{
												"containerPort": 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"selector": nil,
						"template": map[string]any{
							"metadata": map[string]any{},
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name": "",
										"ports": []any{
											map[string]any{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]any{},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "adds default port protocol to Job",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"ports": []any{
											map[string]any{
												"containerPort": 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"template": map[string]any{
							"metadata": map[string]any{},
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name": "",
										"ports": []any{
											map[string]any{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]any{},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "adds default port protocol to CronJob",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "CronJob",
					"spec": map[string]any{
						"jobTemplate": map[string]any{
							"spec": map[string]any{
								"template": map[string]any{
									"spec": map[string]any{
										"containers": []any{
											map[string]any{
												"ports": []any{
													map[string]any{
														"containerPort": 80,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "CronJob",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"jobTemplate": map[string]any{
							"metadata": map[string]any{},
							"spec": map[string]any{
								"template": map[string]any{
									"metadata": map[string]any{},
									"spec": map[string]any{
										"containers": []any{
											map[string]any{
												"name": "",
												"ports": []any{
													map[string]any{
														"containerPort": int64(80),
														"protocol":      "TCP",
													},
												},
												"resources": map[string]any{},
											},
										},
									},
								},
							},
						},
						"schedule": "",
					},
				},
			},
		},
		{
			name: "adds default port protocol to Service",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Service",
					"spec": map[string]any{
						"ports": []any{
							map[string]any{
								"port": 80,
							},
							map[string]any{
								"port": 443,
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"spec": map[string]any{
						"ports": []any{
							map[string]any{
								"port":       int64(80),
								"protocol":   "TCP",
								"targetPort": int64(0),
							},
							map[string]any{
								"port":       int64(443),
								"protocol":   "TCP",
								"targetPort": int64(0),
							},
						},
					},
				},
			},
		},
		{
			name: "moves stringData to data in Secret",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"stringData": map[string]any{
						"foo": "bar",
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
					"data": map[string]any{
						"foo": "YmFy",
					},
				},
			},
		},
		{
			name: "removes status from any object",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"status": map[string]any{
						"foo": "bar",
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
				},
			},
		},
		{
			name: "nil writes creationTimestamp on any object",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]any{
						"creationTimestamp": "2020-01-01T00:00:00Z",
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]any{
						"creationTimestamp": nil,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Unstructured(tt.object); (err != nil) != tt.wantErr {
				t.Errorf("Unstructured() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, tt.object); diff != "" {
				t.Errorf("Unstructured() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNormalizeDryRunUnstructured(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "removes duplicated metrics from v2 HorizontalPodAutoscaler",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "autoscaling/v2",
					"kind":       "HorizontalPodAutoscaler",
					"spec": map[string]any{
						"metrics": []any{
							map[string]any{
								"type": "Resource",
								"resource": map[string]any{
									"name": "cpu",
									"target": map[string]any{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]any{
								"type": "ContainerResource",
								"containerResource": map[string]any{
									"name":      "cpu",
									"container": "application",
									"target": map[string]any{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]any{
								"type": "Resource",
								"resource": map[string]any{
									"name": "cpu",
									"target": map[string]any{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]any{},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "autoscaling/v2",
					"kind":       "HorizontalPodAutoscaler",
					"metadata":   map[string]any{},
					"spec": map[string]any{
						"maxReplicas": int64(0),
						"metrics": []any{
							map[string]any{
								"type": "Resource",
								"resource": map[string]any{
									"name": "cpu",
									"target": map[string]any{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]any{
								"type": "ContainerResource",
								"containerResource": map[string]any{
									"name":      "cpu",
									"container": "application",
									"target": map[string]any{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
						},
						"scaleTargetRef": map[string]any{
							"kind": "",
							"name": "",
						},
					},
					"status": map[string]any{
						"currentMetrics":  nil,
						"desiredReplicas": int64(0),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DryRunUnstructured(tt.object); (err != nil) != tt.wantErr {
				t.Errorf("DryRunUnstructured() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, tt.object); diff != "" {
				t.Errorf("DryRunUnstructured() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
