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

package conditions

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const fakeGroupName = "fake"

// fakeSchemeGroupVersion is group version used to register the fake object
var fakeSchemeGroupVersion = schema.GroupVersion{Group: fakeGroupName, Version: "v1"}

// fake is a mock struct that adheres to the minimal requirements to
// work with the condition helpers, by implementing client.Object.
type fake struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status fakeStatus `json:"status,omitempty"`
}

type fakeStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (f fake) GetConditions() []metav1.Condition {
	return f.Status.Conditions
}

func (f *fake) SetConditions(conditions []metav1.Condition) {
	f.Status.Conditions = conditions
}

func (f *fake) DeepCopyInto(out *fake) {
	*out = *f
	out.TypeMeta = f.TypeMeta
	f.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	f.Status.DeepCopyInto(&out.Status)
}

func (f *fake) DeepCopy() *fake {
	if f == nil {
		return nil
	}
	out := new(fake)
	f.DeepCopyInto(out)
	return out
}

func (f *fake) DeepCopyObject() runtime.Object {
	if c := f.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (f *fakeStatus) DeepCopyInto(out *fakeStatus) {
	*out = *f
	if f.Conditions != nil {
		in, out := &f.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (f *fakeStatus) DeepCopy() *fakeStatus {
	if f == nil {
		return nil
	}
	out := new(fakeStatus)
	f.DeepCopyInto(out)
	return out
}
