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
	"testing"

	brokerapi "github.com/spiffe/go-spiffe/v2/exp/proto/spiffe/broker"
)

func TestBrokerTarget(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		{endpoint: "spire-server.spire.svc:443", want: "dns:///spire-server.spire.svc:443"},
		{endpoint: "dns:///spire-server.spire.svc:443", want: "dns:///spire-server.spire.svc:443"},
		{endpoint: "passthrough:///127.0.0.1:1234", want: "passthrough:///127.0.0.1:1234"},
		{endpoint: "unix:///tmp/broker.sock", want: "unix:///tmp/broker.sock"},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			if got := brokerTarget(tt.endpoint); got != tt.want {
				t.Fatalf("brokerTarget(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestKubernetesObjectReference(t *testing.T) {
	ref, err := (KubernetesObjectReference{
		Kind:      "OCIRepository",
		Namespace: "flux-system",
		Name:      "my-repo",
		UID:       "b7b1b3b4-0000-0000-0000-000000000000",
	}).workloadReference()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj brokerapi.KubernetesObjectReference
	if err := ref.GetReference().UnmarshalTo(&obj); err != nil {
		t.Fatalf("failed to unmarshal reference: %v", err)
	}
	if obj.GetType().GetGroup() != "core" {
		t.Errorf("group = %q, want core", obj.GetType().GetGroup())
	}
	if obj.GetType().GetPlural() != "ocirepositories" {
		t.Errorf("plural = %q, want ocirepositories", obj.GetType().GetPlural())
	}
	if obj.GetKey().GetNamespace() != "flux-system" || obj.GetKey().GetName() != "my-repo" {
		t.Errorf("unexpected key: %+v", obj.GetKey())
	}
	if obj.GetUid() != "b7b1b3b4-0000-0000-0000-000000000000" {
		t.Errorf("unexpected UID: %q", obj.GetUid())
	}
}

func TestKubernetesObjectReferenceKind(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{kind: "OCIRepository", want: "ocirepositories"},
		{kind: "GitRepository", want: "gitrepositories"},
		{kind: "HelmRepository", want: "helmrepositories"},
		{kind: "HelmChart", want: "helmcharts"},
		{kind: "Bucket", want: "buckets"},
		{kind: "Kustomization", want: "kustomizations"},
		{kind: "HelmRelease", want: "helmreleases"},
		{kind: "ImagePolicy", want: "imagepolicies"},
		{kind: "ImageRepository", want: "imagerepositories"},
		{kind: "ImageUpdateAutomation", want: "imageupdateautomations"},
		{kind: "ExternalArtifact", want: "externalartifacts"},
		{kind: "Alert", want: "alerts"},
		{kind: "Provider", want: "providers"},
		{kind: "Receiver", want: "receivers"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			ref, err := (KubernetesObjectReference{
				Kind: tt.kind,
				Name: "my-object",
			}).workloadReference()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var obj brokerapi.KubernetesObjectReference
			if err := ref.GetReference().UnmarshalTo(&obj); err != nil {
				t.Fatalf("failed to unmarshal reference: %v", err)
			}
			if got := obj.GetType().GetPlural(); got != tt.want {
				t.Fatalf("plural for kind %q = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestKubernetesObjectReferenceValidation(t *testing.T) {
	tests := []struct {
		name string
		ref  KubernetesObjectReference
	}{
		{name: "missing kind", ref: KubernetesObjectReference{Name: "foo"}},
		{name: "missing name and UID", ref: KubernetesObjectReference{Kind: "OCIRepository"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.ref.workloadReference(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
