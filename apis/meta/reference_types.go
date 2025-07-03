/*
Copyright 2020, 2024 The Flux authors

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

package meta

// LocalObjectReference contains enough information to locate the referenced Kubernetes resource object.
type LocalObjectReference struct {
	// Name of the referent.
	// +required
	Name string `json:"name"`
}

// NamespacedObjectReference contains enough information to locate the referenced Kubernetes resource object in any
// namespace.
type NamespacedObjectReference struct {
	// Name of the referent.
	// +required
	Name string `json:"name"`

	// Namespace of the referent, when not specified it acts as LocalObjectReference.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// NamespacedObjectKindReference contains enough information to locate the typed referenced Kubernetes resource object
// in any namespace.
type NamespacedObjectKindReference struct {
	// API version of the referent, if not specified the Kubernetes preferred version will be used.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent.
	// +required
	Kind string `json:"kind"`

	// Name of the referent.
	// +required
	Name string `json:"name"`

	// Namespace of the referent, when not specified it acts as LocalObjectReference.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// SecretKeyReference contains enough information to locate the referenced Kubernetes Secret object in the same
// namespace. Optionally a key can be specified.
// Use this type instead of core/v1 SecretKeySelector when the Key is optional and the Optional field is not
// applicable.
type SecretKeyReference struct {
	// Name of the Secret.
	// +required
	Name string `json:"name"`

	// Key in the Secret, when not specified an implementation-specific default key is used.
	// +optional
	Key string `json:"key,omitempty"`
}

// KubeConfigReference contains enough information build a kubeconfig
// in memory for connecting to remote Kubernetes clusters.
// +kubebuilder:validation:XValidation:rule="has(self.secretRef) || (has(self.provider) && self.provider != 'generic')", message=".secretRef is required for the 'generic' .provider"
// +kubebuilder:validation:XValidation:rule="!has(self.secretRef) || !has(self.provider) || self.provider == 'generic'", message=".secretRef is not supported for the specified .provider"
// +kubebuilder:validation:XValidation:rule="!has(self.serviceAccountName) || (has(self.provider) && self.provider != 'generic')", message=".serviceAccountName is not supported when .provider is empty or 'generic'"
// +kubebuilder:validation:XValidation:rule="!has(self.cluster) || (has(self.provider) && self.provider != 'generic')", message=".cluster is not supported when .provider is empty or 'generic'"
// +kubebuilder:validation:XValidation:rule="!has(self.address) || (has(self.provider) && self.provider != 'generic')", message=".address is not supported when .provider is empty or 'generic'"
type KubeConfigReference struct {
	// Provider is the optional name of the cloud provider that should be used
	// to authenticate to the Kubernetes API server. Can be one of "aws",
	// "azure", "gcp", or "generic". Defaults to "generic".
	// +kubebuilder:validation:Enum=aws;azure;gcp;generic
	// +kubebuilder:default=generic
	// +optional
	Provider string `json:"provider,omitempty"`

	// Cluster is the optional fully qualified resource name of the
	// Kubernetes cluster in the cloud provider to connect to.
	// Not supported for the generic provider, required for the
	// other providers.
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// Address is the optional address of the Kubernetes API server.
	// Not supported for the generic provider, optional for the
	// other providers. The address is used to select among a list
	// of endpoints in the cluster resource. If not set, the first
	// endpoint on the list is used. If none of the addresses in the
	// cluster resource match a provided address, the controller will
	// error out and the reconciliation will fail. Must be a valid
	// HTTPS endpoint, e.g. "https://api.example.com:6443".
	// +kubebuilder:validation:Pattern=`^https://.*`
	// +optional
	Address string `json:"address,omitempty"`

	// ServiceAccountName is the optional name of the Kubernetes
	// ServiceAccount in the same namespace that should be used
	// to authenticate to the Kubernetes API server. If not set,
	// the controller ServiceAccount will be used. Not supported
	// for the generic provider.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// SecretRef holds an optional name of a secret that contains a key with
	// the kubeconfig file as the value. If no key is set, the key will default
	// to 'value'.
	// It is recommended that the kubeconfig is self-contained, and the secret
	// is regularly updated if credentials such as a cloud-access-token expire.
	// Cloud specific `cmd-path` auth helpers will not function without adding
	// binaries and credentials to the Pod that is responsible for reconciling
	// Kubernetes resources. Supported only for the generic provider.
	// +optional
	SecretRef *SecretKeyReference `json:"secretRef,omitempty"`
}

// ValuesReference contains a reference to a resource containing Helm values,
// and optionally the key they can be found at.
type ValuesReference struct {
	// Kind of the values referent, valid values are ('Secret', 'ConfigMap').
	// +kubebuilder:validation:Enum=Secret;ConfigMap
	// +required
	Kind string `json:"kind"`

	// Name of the values referent. Should reside in the same namespace as the
	// referring resource.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`

	// ValuesKey is the data key where the values.yaml or a specific value can be
	// found at. Defaults to 'values.yaml'.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[\-._a-zA-Z0-9]+$`
	// +optional
	ValuesKey string `json:"valuesKey,omitempty"`

	// TargetPath is the YAML dot notation path the value should be merged at. When
	// set, the ValuesKey is expected to be a single flat value. Defaults to 'None',
	// which results in the values getting merged at the root.
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9_\-.\\\/]|\[[0-9]{1,5}\])+$`
	// +optional
	TargetPath string `json:"targetPath,omitempty"`

	// Optional marks this ValuesReference as optional. When set, a not found error
	// for the values reference is ignored, but any ValuesKey, TargetPath or
	// transient error will still result in a reconciliation failure.
	// +optional
	Optional bool `json:"optional,omitempty"`
}

// GetValuesKey returns the defined ValuesKey, or the default ('values.yaml').
func (in ValuesReference) GetValuesKey() string {
	if in.ValuesKey == "" {
		return "values.yaml"
	}
	return in.ValuesKey
}
