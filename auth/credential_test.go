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

package auth_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/crypto"
	"github.com/fluxcd/pkg/auth"
)

func TestJWT(t *testing.T) {
	g := NewWithT(t)

	jwt := &auth.JWT{
		Token:     "token",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	g.Expect(jwt.Type()).To(Equal(crypto.CredentialTypeJWT))
	g.Expect(jwt.GetDuration()).To(BeNumerically("~", time.Hour, time.Minute))
}

func TestX509(t *testing.T) {
	g := NewWithT(t)

	x509 := &auth.X509{
		Certificate: []byte("certificate"),
		PrivateKey:  []byte("private-key"),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	g.Expect(x509.Type()).To(Equal(crypto.CredentialTypeX509))
	g.Expect(x509.GetDuration()).To(BeNumerically("~", time.Hour, time.Minute))
}

func TestOptions_WithCredential(t *testing.T) {
	g := NewWithT(t)

	credential := &crypto.Credential{
		Provider:  crypto.CredentialProviderKubernetes,
		Type:      crypto.CredentialTypeJWT,
		Audiences: []string{"audience"},
		Annotations: map[string]string{
			"auth.fluxcd.io/username": "username",
		},
	}

	var o auth.Options
	o.Apply(auth.WithCredential(credential))
	g.Expect(o.Credential).To(Equal(credential))
}

func TestOptions_WithServiceAccount(t *testing.T) {
	g := NewWithT(t)

	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
	}

	var o auth.Options
	o.Apply(auth.WithServiceAccount(serviceAccount))
	g.Expect(o.ServiceAccount).To(Equal(&serviceAccount))
}

type mockCredential struct {
	token string
}

func (m *mockCredential) GetDuration() time.Duration {
	return time.Hour
}
