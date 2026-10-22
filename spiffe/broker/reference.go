/*
Copyright 2026 The Flux authors

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

package broker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	brokerapi "github.com/spiffe/go-spiffe/v2/exp/proto/spiffe/broker"
	"google.golang.org/protobuf/types/known/anypb"
)

// KubernetesObjectReference identifies a Kubernetes object for the SPIFFE
// Broker API. Group is the API group of the object, with an empty value
// normalized to "core" for core resources.
//
// Kind is the singular CRD kind, e.g. "OCIRepository". The Broker API resource
// plural is derived from Kind using the same library and rules as controller-gen.
// At least one of Name or UID must be set.
type KubernetesObjectReference struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
	UID       string
}

// resourcePlural returns the plural resource name for the reference.
func (r KubernetesObjectReference) resourcePlural() (string, error) {
	if r.Kind == "" {
		return "", errors.New("Kubernetes object kind is required")
	}
	// This is the same transformation controller-gen applies to the default
	// CRD plural (sigs.k8s.io/controller-tools/pkg/crd/spec.go).
	return strings.ToLower(flect.Pluralize(r.Kind)), nil
}

// workloadReference returns the Broker API workload reference for the
// Kubernetes object.
func (r KubernetesObjectReference) workloadReference() (*brokerapi.WorkloadReference, error) {
	group := r.Group
	if group == "" {
		group = "core"
	}
	plural, err := r.resourcePlural()
	if err != nil {
		return nil, err
	}
	if r.Name == "" && r.UID == "" {
		return nil, errors.New("Kubernetes object name or UID is required")
	}

	obj := &brokerapi.KubernetesObjectReference{
		Type: &brokerapi.KubernetesObjectType{
			Group:  group,
			Plural: plural,
		},
		Uid: r.UID,
	}
	if r.Name != "" {
		obj.Key = &brokerapi.KubernetesObjectKey{
			Namespace: r.Namespace,
			Name:      r.Name,
		}
	}

	ref, err := anypb.New(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Kubernetes object reference: %w", err)
	}
	return &brokerapi.WorkloadReference{Reference: ref}, nil
}
