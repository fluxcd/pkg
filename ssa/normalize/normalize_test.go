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
			name: "valid Pod",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test-container",
								"image": "test-image",
							},
						},
					},
				},
			},
			want: &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
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
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-namespace",
				},
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
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test-container",
								"image": "test-image",
							},
						},
					},
				},
			},
			scheme: defaultScheme,
			want: &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"ports": []interface{}{
									map[string]interface{}{
										"containerPort": 80,
									},
									map[string]interface{}{
										"containerPort": 8080,
									},
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name": "",
								"ports": []interface{}{
									map[string]interface{}{
										"containerPort": int64(80),
										"protocol":      "TCP",
									},
									map[string]interface{}{
										"containerPort": int64(8080),
										"protocol":      "TCP",
									},
								},
								"resources": map[string]interface{}{},
							},
						},
					},
				},
			},
		},
		{
			name: "adds default port protocol to Deployment",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"selector": nil,
						"strategy": map[string]interface{}{},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name": "",
										"ports": []interface{}{
											map[string]interface{}{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]interface{}{},
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"selector":    nil,
						"serviceName": "",
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name": "",
										"ports": []interface{}{
											map[string]interface{}{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]interface{}{},
									},
								},
							},
						},
						"updateStrategy": map[string]interface{}{},
					},
				},
			},
		},
		{
			name: "adds default port protocol to DaemonSet",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"selector": nil,
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name": "",
										"ports": []interface{}{
											map[string]interface{}{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]interface{}{},
									},
								},
							},
						},
						"updateStrategy": map[string]interface{}{},
					},
				},
			},
		},
		{
			name: "adds default port protocol to ReplicaSet",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"selector": nil,
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name": "",
										"ports": []interface{}{
											map[string]interface{}{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]interface{}{},
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name": "",
										"ports": []interface{}{
											map[string]interface{}{
												"containerPort": int64(80),
												"protocol":      "TCP",
											},
										},
										"resources": map[string]interface{}{},
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "CronJob",
					"spec": map[string]interface{}{
						"jobTemplate": map[string]interface{}{
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"containers": []interface{}{
											map[string]interface{}{
												"ports": []interface{}{
													map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "CronJob",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"jobTemplate": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
									"metadata": map[string]interface{}{
										"creationTimestamp": nil,
									},
									"spec": map[string]interface{}{
										"containers": []interface{}{
											map[string]interface{}{
												"name": "",
												"ports": []interface{}{
													map[string]interface{}{
														"containerPort": int64(80),
														"protocol":      "TCP",
													},
												},
												"resources": map[string]interface{}{},
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"spec": map[string]interface{}{
						"ports": []interface{}{
							map[string]interface{}{
								"port": 80,
							},
							map[string]interface{}{
								"port": 443,
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"ports": []interface{}{
							map[string]interface{}{
								"port":       int64(80),
								"protocol":   "TCP",
								"targetPort": int64(0),
							},
							map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"stringData": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"data": map[string]interface{}{
						"foo": "YmFy",
					},
				},
			},
		},
		{
			name: "removes status from any object",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"status": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
				},
			},
		},
		{
			name: "nil writes creationTimestamp on any object",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]interface{}{
						"creationTimestamp": "2020-01-01T00:00:00Z",
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "test/v1",
					"kind":       "Test",
					"metadata": map[string]interface{}{
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
			name: "removes duplicated metrics from v2beta2 HorizontalPodAutoscaler",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "autoscaling/v2beta2",
					"kind":       "HorizontalPodAutoscaler",
					"spec": map[string]interface{}{
						"metrics": []interface{}{
							map[string]interface{}{
								"type": "Resource",
								"resource": map[string]interface{}{
									"name": "cpu",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{
								"type": "ContainerResource",
								"containerResource": map[string]interface{}{
									"name":      "cpu",
									"container": "application",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{
								"type": "Resource",
								"resource": map[string]interface{}{
									"name": "cpu",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "autoscaling/v2beta2",
					"kind":       "HorizontalPodAutoscaler",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"maxReplicas": int64(0),
						"metrics": []interface{}{
							map[string]interface{}{
								"type": "Resource",
								"resource": map[string]interface{}{
									"name": "cpu",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{
								"type": "ContainerResource",
								"containerResource": map[string]interface{}{
									"name":      "cpu",
									"container": "application",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
						},
						"scaleTargetRef": map[string]interface{}{
							"kind": "",
							"name": "",
						},
					},
					"status": map[string]interface{}{
						"conditions":      nil,
						"currentMetrics":  nil,
						"currentReplicas": int64(0),
						"desiredReplicas": int64(0),
					},
				},
			},
		},
		{
			name: "removes duplicated metrics from v2 HorizontalPodAutoscaler",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "autoscaling/v2",
					"kind":       "HorizontalPodAutoscaler",
					"spec": map[string]interface{}{
						"metrics": []interface{}{
							map[string]interface{}{
								"type": "Resource",
								"resource": map[string]interface{}{
									"name": "cpu",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{
								"type": "ContainerResource",
								"containerResource": map[string]interface{}{
									"name":      "cpu",
									"container": "application",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{
								"type": "Resource",
								"resource": map[string]interface{}{
									"name": "cpu",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "autoscaling/v2",
					"kind":       "HorizontalPodAutoscaler",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"maxReplicas": int64(0),
						"metrics": []interface{}{
							map[string]interface{}{
								"type": "Resource",
								"resource": map[string]interface{}{
									"name": "cpu",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
							map[string]interface{}{
								"type": "ContainerResource",
								"containerResource": map[string]interface{}{
									"name":      "cpu",
									"container": "application",
									"target": map[string]interface{}{
										"type":               "Utilization",
										"averageUtilization": int64(60),
									},
								},
							},
						},
						"scaleTargetRef": map[string]interface{}{
							"kind": "",
							"name": "",
						},
					},
					"status": map[string]interface{}{
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
