package client

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetConfigForAccount(t *testing.T) {
	ctx := context.Background()
	fakeClient := setupFakeClient()
	if err := fakeClient.Get(ctx, types.NamespacedName{
		Name: "test-sa",
	}, &corev1.ServiceAccount{}); err != nil {
		t.Fatalf("fake client not set up correctly, %s", err)
	}

	tests := []struct {
		config     ImpersonationConfig
		testString string
	}{
		{
			config: ImpersonationConfig{
				Name:      "dev",
				Kind:      UserType,
				Enabled:   true,
				Namespace: "test",
			},
			testString: "flux:user:test:dev",
		},
		{
			config: ImpersonationConfig{
				Enabled:   true,
				Namespace: "test",
			},
			testString: "flux:user:test:reconciler",
		},
		{
			config: ImpersonationConfig{
				Enabled:   true,
				Namespace: "test",
				Kind:      ServiceAccountType,
				Name:      "sa",
			},
			testString: "system:serviceaccount:test:sa",
		},
		{
			config: ImpersonationConfig{
				Enabled: false,
				Kind:    ServiceAccountType,
				Name:    "test-sa",
			},
			testString: "random-token",
		},
		{
			config: ImpersonationConfig{
				Enabled: false,
			},
			testString: "",
		},
	}

	for _, tt := range tests {
		config := &rest.Config{}
		impConfig, err := GetConfigForAccount(ctx, fakeClient, config, tt.config)
		if err != nil {
			t.Fatalf("error getting impersonation config: %s", err)
		}

		if !tt.config.Enabled {
			// check that user impersonation isn't set when it isn't enabled
			if impConfig.Impersonate.UserName != "" {
				t.Fatalf("username set when user impersonation isn't enabled, username=%s",
					impConfig.Impersonate.UserName)
			}

			if impConfig.BearerToken != tt.testString {
				t.Errorf("token not set correctly on config, expected=%s, got=%s",
					tt.testString, impConfig.BearerToken)
			}

			return
		}

		if impConfig.Impersonate.UserName != tt.testString {
			t.Errorf("wrong username, expected=%s, got=%s",
				tt.testString, impConfig.Impersonate.UserName)
		}
	}
}

func TestErrorHandling(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()

	tests := []struct {
		config ImpersonationConfig
		errMsg string
	}{
		{
			config: ImpersonationConfig{
				Enabled: false,
				Kind:    UserType,
				Name:    "dev",
			},
			errMsg: "cannot impersonate user if --user-impersonation is not set",
		},
	}

	for _, tt := range tests {
		_, err := GetConfigForAccount(context.Background(), fakeClient, &rest.Config{}, tt.config)
		if err == nil {
			t.Fatalf("error expected: %s", tt.errMsg)
		}

		if !strings.Contains(err.Error(), tt.errMsg) {
			t.Fatalf("wrong error message, error should contain %q, but got %q",
				tt.errMsg, err.Error())
		}
	}
}

func setupFakeClient() client.Client {
	clientBuilder := fake.NewClientBuilder()
	serviceaccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sa",
		},
		Secrets: []corev1.ObjectReference{
			{
				Name: "test-sa-token",
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sa-token",
		},
		Data: map[string][]byte{
			"token": []byte("random-token"),
		},
	}
	//clientBuilder.WithObjects(serviceaccount, secret)
	clientBuilder.WithRuntimeObjects(serviceaccount, secret)

	return clientBuilder.Build()
}
