/*
Copyright 2022 The Flux authors

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
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	rc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/statusreaders"
)

// Impersonator holds the state for impersonating a Kubernetes account.
type Impersonator struct {
	client                  rc.Client
	kubeConfigRef           *meta.KubeConfigReference
	kubeConfigOpts          KubeConfigOptions
	kubeConfigNamespace     string
	kubeConfigProvider      ProviderRESTConfigFetcher
	defaultServiceAccount   string
	serviceAccountName      string
	serviceAccountNamespace string
	scheme                  *runtime.Scheme
	pollingOpts             *polling.Options
	pollingReaders          []func(apimeta.RESTMapper) engine.StatusReader
}

// NewImpersonator creates an Impersonator from the given arguments.
func NewImpersonator(kubeClient rc.Client, opts ...ImpersonatorOption) *Impersonator {
	i := &Impersonator{client: kubeClient}
	for _, o := range opts {
		o(i)
	}
	return i
}

// GetClient creates a controller-runtime client for talking to a Kubernetes API server.
// If spec.KubeConfig is set, use the kubeconfig bytes from the Kubernetes secret.
// Otherwise, will assume running in cluster and use the cluster provided kubeconfig.
// If a --default-service-account is set and no spec.ServiceAccountName, use the provided kubeconfig and impersonate the default SA.
// If spec.ServiceAccountName is set, use the provided kubeconfig and impersonate the specified SA.
func (i *Impersonator) GetClient(ctx context.Context) (rc.Client, *polling.StatusPoller, error) {
	switch {
	case i.kubeConfigRef != nil:
		return i.clientForKubeConfig(ctx)
	case i.defaultServiceAccount != "" || i.serviceAccountName != "":
		return i.clientForServiceAccountOrDefault()
	default:
		return i.defaultClient()
	}
}

// CanImpersonate checks if the given Kubernetes account can be impersonated.
func (i *Impersonator) CanImpersonate(ctx context.Context) bool {
	name := i.defaultServiceAccount
	if sa := i.serviceAccountName; sa != "" {
		name = sa
	}
	if name == "" {
		return true
	}

	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.serviceAccountNamespace,
		},
	}
	if err := i.client.Get(ctx, rc.ObjectKeyFromObject(sa), sa); err != nil {
		return false
	}

	return true
}

func (i *Impersonator) clientForServiceAccountOrDefault() (rc.Client, *polling.StatusPoller, error) {
	restConfig, err := config.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	i.setImpersonationConfig(restConfig)

	restMapper, err := NewDynamicRESTMapper(restConfig)
	if err != nil {
		return nil, nil, err
	}

	client, err := rc.New(restConfig, rc.Options{
		Scheme: i.scheme,
		Mapper: restMapper,
	})
	if err != nil {
		return nil, nil, err
	}

	statusPoller := i.newStatusPoller(client, restMapper)
	return client, statusPoller, err
}

func (i *Impersonator) defaultClient() (rc.Client, *polling.StatusPoller, error) {
	restConfig, err := config.GetConfig()
	if err != nil {
		return nil, nil, err
	}

	restMapper, err := NewDynamicRESTMapper(restConfig)
	if err != nil {
		return nil, nil, err
	}

	client, err := rc.New(restConfig, rc.Options{
		Scheme: i.scheme,
		Mapper: restMapper,
	})
	if err != nil {
		return nil, nil, err
	}

	statusPoller := i.newStatusPoller(client, restMapper)
	return client, statusPoller, err
}

func (i *Impersonator) clientForKubeConfig(ctx context.Context) (rc.Client, *polling.StatusPoller, error) {
	var restConfig *rest.Config
	var err error

	switch {
	case i.kubeConfigProvider != nil:
		getRESTConfigFromProvider := i.kubeConfigProvider
		restConfig, err = getRESTConfigFromProvider(ctx, *i.kubeConfigRef, i.kubeConfigNamespace, i.client)
	case i.kubeConfigRef.SecretRef != nil:
		restConfig, err = i.getRESTConfigFromSecret(ctx)
	default:
		return nil, nil, errors.New("invalid .spec.kubeConfig, neither .spec.kubeConfig.provider nor .spec.kubeConfig.secretRef is set")
	}
	if err != nil {
		return nil, nil, err
	}

	restConfig = KubeConfig(restConfig, i.kubeConfigOpts)
	i.setImpersonationConfig(restConfig)

	restMapper, err := NewDynamicRESTMapper(restConfig)
	if err != nil {
		return nil, nil, err
	}

	client, err := rc.New(restConfig, rc.Options{
		Scheme: i.scheme,
		Mapper: restMapper,
	})
	if err != nil {
		return nil, nil, err
	}

	statusPoller := i.newStatusPoller(client, restMapper)

	return client, statusPoller, err
}

func (i *Impersonator) getRESTConfigFromSecret(ctx context.Context) (*rest.Config, error) {
	secretName := types.NamespacedName{
		Namespace: i.kubeConfigNamespace,
		Name:      i.kubeConfigRef.SecretRef.Name,
	}

	var secret corev1.Secret
	if err := i.client.Get(ctx, secretName, &secret); err != nil {
		return nil, fmt.Errorf("unable to read KubeConfig secret '%s' error: %w", secretName.String(), err)
	}

	var kubeConfig []byte
	switch {
	case i.kubeConfigRef.SecretRef.Key != "":
		key := i.kubeConfigRef.SecretRef.Key
		kubeConfig = secret.Data[key]
		if kubeConfig == nil {
			return nil, fmt.Errorf("KubeConfig secret '%s' does not contain a '%s' key with a kubeconfig", secretName, key)
		}
	case secret.Data["value"] != nil:
		kubeConfig = secret.Data["value"]
	case secret.Data["value.yaml"] != nil:
		kubeConfig = secret.Data["value.yaml"]
	default:
		// User did not specify a key, and the 'value' key was not defined.
		return nil, fmt.Errorf("KubeConfig secret '%s' does not contain a 'value' key with a kubeconfig", secretName)
	}

	return clientcmd.RESTConfigFromKubeConfig(kubeConfig)
}

func (i *Impersonator) setImpersonationConfig(restConfig *rest.Config) {
	name := i.defaultServiceAccount
	if sa := i.serviceAccountName; sa != "" {
		name = sa
	}
	if name != "" {
		username := fmt.Sprintf("system:serviceaccount:%s:%s", i.serviceAccountNamespace, name)
		restConfig.Impersonate = rest.ImpersonationConfig{UserName: username}
	}
}

func (i *Impersonator) newStatusPoller(reader rc.Reader, restMapper apimeta.RESTMapper) *polling.StatusPoller {
	var opts polling.Options

	if i.pollingOpts != nil {
		opts = *i.pollingOpts
	}

	pollingReaders := append(i.pollingReaders, statusreaders.NewCustomJobStatusReader)
	for _, ctor := range pollingReaders {
		sr := ctor(restMapper)
		opts.CustomStatusReaders = append(opts.CustomStatusReaders, sr)
	}

	return polling.NewStatusPoller(reader, restMapper, opts)
}
