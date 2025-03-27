/*
Copyright 2025 The Flux authors

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

package client

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"

	"github.com/fluxcd/pkg/apis/meta"
)

// ImpersonatorOption is a functional option for configuring the Impersonator.
type ImpersonatorOption func(*Impersonator)

// WithScheme sets the scheme for the Impersonator.
func WithScheme(scheme *runtime.Scheme) ImpersonatorOption {
	return func(i *Impersonator) {
		i.scheme = scheme
	}
}

// WithServiceAccount sets the service account options for the Impersonator.
func WithServiceAccount(defaultName, name, namespace string) ImpersonatorOption {
	return func(i *Impersonator) {
		i.defaultServiceAccount = defaultName
		i.serviceAccountName = name
		i.serviceAccountNamespace = namespace
	}
}

// WithKubeConfig sets the kubeconfig options for the Impersonator.
func WithKubeConfig(ref *meta.KubeConfigReference, opts KubeConfigOptions, namespace string) ImpersonatorOption {
	return func(i *Impersonator) {
		i.kubeConfigRef = ref
		i.kubeConfigOpts = opts
		i.kubeConfigNamespace = namespace
	}
}

// WithPolling sets the polling options for the Impersonator.
func WithPolling(clusterReader engine.ClusterReaderFactory,
	readers ...func(apimeta.RESTMapper) engine.StatusReader) ImpersonatorOption {

	return func(i *Impersonator) {
		i.pollingOpts = &polling.Options{ClusterReaderFactory: clusterReader}
		i.pollingReaders = readers
	}
}
