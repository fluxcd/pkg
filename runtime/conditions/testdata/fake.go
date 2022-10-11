/*
Copyright 2021 The Flux authors

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

package testdata

import (
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// +kubebuilder:object:generate=true
// +groupName=fake.toolkit.fluxcd.io
// +versionName=v1

const (
	FakeGroupName = "fake.toolkit.fluxcd.io"
	FakeVersion   = "v1"
)

var (
	// FakeGroupVersion is group version used to register the fake object.
	FakeGroupVersion = schema.GroupVersion{Group: FakeGroupName, Version: FakeVersion}

	// FakeSchemeBuilder is used to add go types to the FakeGroupVersionKind scheme.
	FakeSchemeBuilder = &scheme.Builder{GroupVersion: FakeGroupVersion}

	// AddFakeToScheme adds the types in this group-version to the given scheme.
	AddFakeToScheme = FakeSchemeBuilder.AddToScheme
)

type FakeSpec struct {
	Suspend  bool            `json:"suspend,omitempty"`
	Value    string          `json:"value,omitempty"`
	Interval metav1.Duration `json:"interval"`
}

type FakeStatus struct {
	ObservedGeneration          int64              `json:"observedGeneration,omitempty"`
	Conditions                  []metav1.Condition `json:"conditions,omitempty"`
	ObservedValue               string             `json:"observedValue,omitempty"`
	meta.ReconcileRequestStatus `json:",inline"`
}

func (f Fake) GetConditions() []metav1.Condition {
	return f.Status.Conditions
}

func (f *Fake) SetConditions(conditions []metav1.Condition) {
	f.Status.Conditions = conditions
}

func (f *Fake) DeepCopyInto(out *Fake) {
	*out = *f
	out.TypeMeta = f.TypeMeta
	f.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	f.Status.DeepCopyInto(&out.Status)
}

func (f *Fake) DeepCopy() *Fake {
	if f == nil {
		return nil
	}
	out := new(Fake)
	f.DeepCopyInto(out)
	return out
}

func (f *Fake) DeepCopyObject() runtime.Object {
	if c := f.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FakeList) DeepCopyInto(out *FakeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Fake, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *FakeList) DeepCopy() *FakeList {
	if in == nil {
		return nil
	}
	out := new(FakeList)
	in.DeepCopyInto(out)
	return out
}

func (in *FakeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (f *FakeSpec) DeepCopyInto(out *FakeSpec) {
	*out = *f
}

func (f *FakeSpec) DeepCopy() *FakeSpec {
	if f == nil {
		return nil
	}
	out := new(FakeSpec)
	f.DeepCopyInto(out)
	return out
}

func (f *FakeStatus) DeepCopyInto(out *FakeStatus) {
	*out = *f
	if f.Conditions != nil {
		in, out := &f.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (f *FakeStatus) DeepCopy() *FakeStatus {
	if f == nil {
		return nil
	}
	out := new(FakeStatus)
	f.DeepCopyInto(out)
	return out
}

// Fake is a mock struct that adheres to the minimal requirements to
// work with the condition helpers, by implementing client.Object.
// +genclient
// +genclient:Namespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Fake struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FakeSpec   `json:"spec,omitempty"`
	Status FakeStatus `json:"status,omitempty"`
}

// FakeList is a mock list struct.
// +kubebuilder:object:root=true
type FakeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Fake `json:"items"`
}

func init() {
	FakeSchemeBuilder.Register(&Fake{}, &FakeList{})
}
