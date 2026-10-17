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

package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/fluxcd/pkg/auth/internal/tokenfile"
)

const (
	controllerTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	clusterAudience     = "https://kubernetes.default.svc.cluster.local"
)

// signToken returns a signed token for the given claims. The code under test
// parses controller tokens without verifying their signature.
func signToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

// newControllerToken returns a controller token with the given subject and
// audiences.
func newControllerToken(t *testing.T, sub string, audiences ...string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": sub}
	if len(audiences) > 0 {
		claims["aud"] = audiences
	}
	return signToken(t, claims)
}

// withReadFile replaces the function used to read the controller token for the
// duration of the test.
func withReadFile(t *testing.T, fn func(string) ([]byte, error)) {
	t.Helper()
	old := tokenfile.ReadFile
	tokenfile.ReadFile = fn
	t.Cleanup(func() { tokenfile.ReadFile = old })
}

func TestLoadControllerToken(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		g := NewWithT(t)

		token := newControllerToken(t, "system:serviceaccount:flux-system:source-controller", "audience1", "audience2")
		withReadFile(t, func(name string) ([]byte, error) {
			g.Expect(name).To(Equal(controllerTokenFile))
			return []byte(token), nil
		})

		sa, audiences, err := loadControllerToken()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sa).To(Equal(&ServiceAccountReference{
			Name:      "source-controller",
			Namespace: "flux-system",
		}))
		g.Expect(audiences).To(ConsistOf("audience1", "audience2"))
	})

	t.Run("read error", func(t *testing.T) {
		g := NewWithT(t)

		withReadFile(t, func(string) ([]byte, error) {
			return nil, errors.New("boom")
		})

		sa, audiences, err := loadControllerToken()
		g.Expect(err).To(MatchError(ContainSubstring("failed to read service account token file " +
			controllerTokenFile + ": boom")))
		g.Expect(sa).To(BeNil())
		g.Expect(audiences).To(BeNil())
	})

	t.Run("invalid token", func(t *testing.T) {
		g := NewWithT(t)

		withReadFile(t, func(string) ([]byte, error) {
			return []byte("not-a-token"), nil
		})

		sa, audiences, err := loadControllerToken()
		g.Expect(err).To(MatchError(ContainSubstring("failed to parse service account token")))
		g.Expect(sa).To(BeNil())
		g.Expect(audiences).To(BeNil())
	})

	t.Run("missing subject", func(t *testing.T) {
		g := NewWithT(t)

		withReadFile(t, func(string) ([]byte, error) {
			return []byte(signToken(t, jwt.MapClaims{"aud": []string{"audience1"}})), nil
		})

		sa, audiences, err := loadControllerToken()
		g.Expect(err).To(MatchError(ContainSubstring(
			"invalid subject format in service account token: ")))
		g.Expect(sa).To(BeNil())
		g.Expect(audiences).To(BeNil())
	})

	t.Run("invalid subject type", func(t *testing.T) {
		g := NewWithT(t)

		withReadFile(t, func(string) ([]byte, error) {
			return []byte(signToken(t, jwt.MapClaims{
				"sub": 123,
				"aud": []string{"audience1"},
			})), nil
		})

		sa, audiences, err := loadControllerToken()
		g.Expect(err).To(MatchError(ContainSubstring("failed to get subject from service account token")))
		g.Expect(sa).To(BeNil())
		g.Expect(audiences).To(BeNil())
	})

	t.Run("invalid subject", func(t *testing.T) {
		g := NewWithT(t)

		withReadFile(t, func(string) ([]byte, error) {
			return []byte(newControllerToken(t, "system:serviceaccount:flux-system")), nil
		})

		sa, audiences, err := loadControllerToken()
		g.Expect(err).To(MatchError(ContainSubstring(
			"invalid subject format in service account token: system:serviceaccount:flux-system")))
		g.Expect(sa).To(BeNil())
		g.Expect(audiences).To(BeNil())
	})

	t.Run("invalid audiences", func(t *testing.T) {
		g := NewWithT(t)

		withReadFile(t, func(string) ([]byte, error) {
			return []byte(signToken(t, jwt.MapClaims{
				"sub": "system:serviceaccount:flux-system:source-controller",
				"aud": []any{"audience1", 123},
			})), nil
		})

		sa, audiences, err := loadControllerToken()
		g.Expect(err).To(MatchError(ContainSubstring("failed to get audiences from service account token")))
		g.Expect(sa).To(BeNil())
		g.Expect(audiences).To(BeNil())
	})
}

func TestCreateJWT_Validation(t *testing.T) {
	validControllerToken := newControllerToken(t, "system:serviceaccount:flux-system:source-controller", clusterAudience)

	for _, tt := range []struct {
		name     string
		saRef    *ServiceAccountReference
		aud      []string
		readFile func(string) ([]byte, error)
		err      string
	}{
		{
			name:  "missing service account name",
			saRef: &ServiceAccountReference{Namespace: "default"},
			aud:   []string{"audience1"},
			err:   "service account and namespace must be specified",
		},
		{
			name:  "missing namespace",
			saRef: &ServiceAccountReference{Name: "tenant"},
			aud:   []string{"audience1"},
			err:   "service account and namespace must be specified",
		},
		{
			name: "missing audiences",
			err:  "at least one audience must be specified",
		},
		{
			name:  "controller token read error",
			saRef: &ServiceAccountReference{Name: "tenant", Namespace: "default"},
			aud:   []string{"audience1"},
			readFile: func(string) ([]byte, error) {
				return nil, errors.New("boom")
			},
			err: "failed to load controller token",
		},
		{
			name:  "cluster audience rejected",
			saRef: &ServiceAccountReference{Name: "tenant", Namespace: "default"},
			aud:   []string{clusterAudience},
			err:   "audience '" + clusterAudience + "' is not allowed as it is intended for the local kubernetes cluster",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if tt.readFile != nil {
				withReadFile(t, tt.readFile)
			} else {
				withReadFile(t, func(string) ([]byte, error) {
					return []byte(validControllerToken), nil
				})
			}

			token, err := CreateJWT(context.Background(), nil, tt.saRef, tt.aud, 0)
			g.Expect(err).To(MatchError(ContainSubstring(tt.err)))
			g.Expect(token).To(BeEmpty())
		})
	}
}

func TestCreateJWT_ControllerServiceAccount(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	_, envClient, oidcClient := newTestEnv(t, ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	withReadFile(t, func(string) ([]byte, error) {
		return []byte(newControllerToken(t,
			"system:serviceaccount:default:controller", clusterAudience)), nil
	})

	// A nil ServiceAccount reference creates the token for the controller
	// ServiceAccount found in the controller token.
	token, err := CreateJWT(ctx, envClient, nil, []string{"audience1", "audience2"}, 0)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeEmpty())

	expectToken(t, ctx, oidcClient, token,
		"system:serviceaccount:default:controller",
		[]string{"audience1", "audience2"}, time.Hour)
}

func TestCreateJWT_ServiceAccount(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	_, envClient, oidcClient := newTestEnv(t, ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	withReadFile(t, func(string) ([]byte, error) {
		return []byte(newControllerToken(t,
			"system:serviceaccount:default:controller", clusterAudience)), nil
	})

	token, err := CreateJWT(ctx, envClient,
		&ServiceAccountReference{Name: "tenant", Namespace: "default"},
		[]string{"audience1"}, 15*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeEmpty())

	expectToken(t, ctx, oidcClient, token,
		"system:serviceaccount:default:tenant", []string{"audience1"}, 15*time.Minute)
}

func TestCreateJWT_ServiceAccountNotFound(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	_, envClient, _ := newTestEnv(t, ctx)

	withReadFile(t, func(string) ([]byte, error) {
		return []byte(newControllerToken(t,
			"system:serviceaccount:default:controller", clusterAudience)), nil
	})

	token, err := CreateJWT(ctx, envClient,
		&ServiceAccountReference{Name: "missing", Namespace: "default"},
		[]string{"audience1"}, 0)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found"))
	g.Expect(token).To(BeEmpty())
}

func TestCreateJWT_Authenticates(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	envConfig, envClient, _ := newTestEnv(t, ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	withReadFile(t, func(string) ([]byte, error) {
		return []byte(newControllerToken(t,
			"system:serviceaccount:default:controller", clusterAudience)), nil
	})

	// Use the API server address as the audience so the token is accepted
	// when authenticating against the local cluster.
	token, err := CreateJWT(ctx, envClient,
		&ServiceAccountReference{Name: "tenant", Namespace: "default"},
		[]string{envConfig.Host}, 0)
	g.Expect(err).NotTo(HaveOccurred())

	conf := &rest.Config{
		Host:        envConfig.Host,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: envConfig.TLSClientConfig.CAData,
		},
	}
	client, err := kubernetes.NewForConfig(conf)
	g.Expect(err).NotTo(HaveOccurred())

	// The ServiceAccount should be able to authenticate.
	version, err := client.Discovery().ServerVersion()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(version).NotTo(BeNil())

	// The ServiceAccount has no RBAC permissions.
	_, err = client.CoreV1().Namespaces().Get(ctx, "default", metav1.GetOptions{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(
		`forbidden: User "system:serviceaccount:default:tenant" cannot get resource "namespaces"`))
}

// expectToken verifies the token's subject and audiences against the API
// server's OIDC discovery endpoint, and checks its remaining lifetime.
func expectToken(t *testing.T, ctx context.Context, oidcClient *http.Client,
	raw, subject string, audiences []string, lifetime time.Duration) {

	t.Helper()
	g := NewWithT(t)

	jwtToken, _, err := jwt.NewParser().ParseUnverified(raw, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())

	sub, err := jwtToken.Claims.GetSubject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub).To(Equal(subject))

	aud, err := jwtToken.Claims.GetAudience()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aud).To(ConsistOf(audiences))

	exp, err := jwtToken.Claims.GetExpirationTime()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(time.Until(exp.Time)).To(BeNumerically("~", lifetime, 10*time.Second))

	iss, err := jwtToken.Claims.GetIssuer()
	g.Expect(err).NotTo(HaveOccurred())
	ctx = oidc.ClientContext(ctx, oidcClient)
	jwks := oidc.NewRemoteKeySet(ctx, iss+"openid/v1/jwks")
	for _, audience := range audiences {
		_, err = oidc.NewVerifier(iss, jwks, &oidc.Config{
			ClientID:             audience,
			SupportedSigningAlgs: []string{jwtToken.Method.Alg()},
		}).Verify(ctx, raw)
		g.Expect(err).NotTo(HaveOccurred())
	}
}
