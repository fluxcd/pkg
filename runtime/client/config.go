package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	UserType           string = "User"
	ServiceAccountType string = "ServiceAccount"
	DefaultUser               = "reconciler"
)

type KubeConfig struct {
	SecretRef meta.LocalObjectReference
}

type ImpersonationConfig struct {
	Name       string
	Kind       string
	KubeConfig *KubeConfig
	Namespace  string
	Enabled    bool
}

func GetServiceAccountToken(ctx context.Context, client client.Client, impConfig ImpersonationConfig) (string, error) {
	namespacedName := types.NamespacedName{
		Namespace: impConfig.Namespace,
		Name:      impConfig.Name,
	}

	var serviceAccount corev1.ServiceAccount
	err := client.Get(ctx, namespacedName, &serviceAccount)
	if err != nil {
		return "", err
	}

	secretName := types.NamespacedName{
		Namespace: impConfig.Namespace,
		Name:      impConfig.Name,
	}

	for _, secret := range serviceAccount.Secrets {
		if strings.HasPrefix(secret.Name, fmt.Sprintf("%s-token", serviceAccount.Name)) {
			secretName.Name = secret.Name
			break
		}
	}

	var secret corev1.Secret
	err = client.Get(ctx, secretName, &secret)
	if err != nil {
		return "", err
	}

	var token string
	if data, ok := secret.Data["token"]; ok {
		token = string(data)
	} else {
		return "", fmt.Errorf("the service account secret '%s' does not containt a token", secretName.String())
	}

	return token, nil
}

func GetImpersonationConfig(config *rest.Config, username string, namespace string) *rest.Config {
	config.Impersonate = rest.ImpersonationConfig{
		UserName: username,
		Groups:   []string{"flux:users", "system:authenticated", "flux:users:" + namespace},
	}

	return config
}

func GetKubeConfigFromSecret(ctx context.Context, client client.Client, secretName types.NamespacedName) ([]byte, error) {
	var secret corev1.Secret
	if err := client.Get(ctx, secretName, &secret); err != nil {
		return nil, fmt.Errorf("unable to read KubeConfig secret '%s' error: %w", secretName.String(), err)
	}

	kubeConfig, ok := secret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("KubeConfig secret '%s' doesn't contain a 'value' key ", secretName.String())
	}

	return kubeConfig, nil
}

func GetConfigForAccount(ctx context.Context, client client.Client, config *rest.Config, impConfig ImpersonationConfig) (*rest.Config, error) {
	if !impConfig.Enabled {
		if impConfig.Kind == ServiceAccountType {
			token, err := GetServiceAccountToken(ctx, client, impConfig)
			if err != nil {
				return nil, err
			}
			config.BearerToken = token
			config.BearerTokenFile = ""

			return config, nil
		}

		if impConfig.Kind == UserType {
			return nil, errors.New("cannot impersonate user if --user-impersonation is not set")
		}

		return config, nil
	}

	var username string
	namespace := impConfig.Namespace

	if impConfig.Kind == ServiceAccountType {
		username = fmt.Sprintf("system:serviceaccount:%s:%s", namespace, impConfig.Name)
		config.Impersonate = rest.ImpersonationConfig{UserName: username}
		return config, nil
	}

	if impConfig.Kind == UserType {
		username = fmt.Sprintf("flux:user:%s:%s", namespace, impConfig.Name)
	}

	// Sets default username if both service account and user name is unset
	if username == "" {
		username = fmt.Sprintf("flux:user:%s:%s", namespace, DefaultUser)
	}

	return GetImpersonationConfig(config, username, namespace), nil
}
