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

package kubernetes_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/gomega"
	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/crypto"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/kubernetes"
)

type mockImplementation struct {
	readFile func(name string) ([]byte, error)
}

func (m mockImplementation) ReadFile(name string) ([]byte, error) {
	return m.readFile(name)
}

func TestProvider_NewJWT_objectLevel(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	envClient, _ := newTestEnv(t, ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	provider := kubernetes.Provider{}
	expirationSeconds := int64(3600)
	token, err := provider.NewJWT(ctx,
		auth.WithClient(envClient),
		auth.WithServiceAccount(*serviceAccount),
		auth.WithAudiences("audience1", "audience2"),
		auth.WithCredential(&crypto.Credential{
			Provider:          crypto.CredentialProviderKubernetes,
			Type:              crypto.CredentialTypeJWT,
			ExpirationSeconds: &expirationSeconds,
		}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeNil())
	g.Expect(token.Type()).To(Equal(crypto.CredentialTypeJWT))
	g.Expect(time.Until(token.ExpiresAt)).To(BeNumerically("~", time.Hour, 5*time.Minute))

	// Validate the token subject and audiences.
	jwtToken, _, err := jwt.NewParser().ParseUnverified(token.Token, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())
	sub, err := jwtToken.Claims.GetSubject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub).To(Equal("system:serviceaccount:default:tenant"))
	aud, err := jwtToken.Claims.GetAudience()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aud).To(ConsistOf("audience1", "audience2"))
}

func TestProvider_NewJWT_controllerLevel(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	envClient, _ := newTestEnv(t, ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	// Craft a token whose subject points to the controller's ServiceAccount.
	raw := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "system:serviceaccount:default:controller",
	})
	signed, err := raw.SignedString([]byte("test-secret"))
	g.Expect(err).NotTo(HaveOccurred())

	provider := kubernetes.Provider{mockImplementation{
		readFile: func(string) ([]byte, error) {
			return []byte(signed), nil
		},
	}}

	token, err := provider.NewJWT(ctx,
		auth.WithClient(envClient),
		auth.WithAudiences("audience"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).NotTo(BeNil())

	jwtToken, _, err := jwt.NewParser().ParseUnverified(token.Token, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())
	sub, err := jwtToken.Claims.GetSubject()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub).To(Equal("system:serviceaccount:default:controller"))
}

func TestProvider_NewX509_objectLevel(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	envClient, _ := newTestEnv(t, ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant",
			Namespace: "default",
		},
	}
	g.Expect(envClient.Create(ctx, serviceAccount)).To(Succeed())

	// Generate a CA to sign certificate signing requests with.
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	g.Expect(err).NotTo(HaveOccurred())
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	g.Expect(err).NotTo(HaveOccurred())
	caCert, err := x509.ParseCertificate(caDER)
	g.Expect(err).NotTo(HaveOccurred())

	// Start a signer for the CSRs, since envtest does not run the
	// kube-controller-manager that implements the built-in signers.
	type signedResult struct {
		csr *certv1.CertificateSigningRequest
		err error
	}
	signedCh := make(chan signedResult, 1)
	go func() {
		csr, err := signCertificateSigningRequest(ctx, envClient, caCert, caKey)
		signedCh <- signedResult{csr, err}
	}()

	// Issue an X.509 credential for the ServiceAccount.
	type credentialResult struct {
		credential *auth.X509
		err        error
	}
	credentialCh := make(chan credentialResult, 1)
	go func() {
		expirationSeconds := int64(3600)
		credential, err := kubernetes.Provider{}.NewX509(ctx,
			auth.WithClient(envClient),
			auth.WithServiceAccount(*serviceAccount),
			auth.WithCredential(&crypto.Credential{
				Provider:          crypto.CredentialProviderKubernetes,
				Type:              crypto.CredentialTypeX509,
				ExpirationSeconds: &expirationSeconds,
			}))
		credentialCh <- credentialResult{credential, err}
	}()

	var credentialRes credentialResult
	select {
	case credentialRes = <-credentialCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the X.509 credential")
	}
	g.Expect(credentialRes.err).NotTo(HaveOccurred())
	g.Expect(credentialRes.credential).NotTo(BeNil())

	var signedRes signedResult
	select {
	case signedRes = <-signedCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the signer")
	}
	g.Expect(signedRes.err).NotTo(HaveOccurred())

	// Verify the CertificateSigningRequest.
	g.Expect(signedRes.csr.Spec.SignerName).To(Equal(certv1.KubeAPIServerClientSignerName))
	g.Expect(signedRes.csr.Spec.Usages).To(ConsistOf(
		certv1.UsageDigitalSignature,
		certv1.UsageKeyEncipherment,
		certv1.UsageClientAuth,
	))
	g.Expect(signedRes.csr.Spec.ExpirationSeconds).NotTo(BeNil())
	g.Expect(*signedRes.csr.Spec.ExpirationSeconds).To(Equal(int32(3600)))
	g.Expect(signedRes.csr.Status.Conditions).To(ContainElement(And(
		HaveField("Type", Equal(certv1.CertificateApproved)),
		HaveField("Status", Equal(corev1.ConditionTrue)),
	)))

	// Verify the issued certificate.
	g.Expect(credentialRes.credential.Type()).To(Equal(crypto.CredentialTypeX509))
	block, _ := pem.Decode(credentialRes.credential.Certificate)
	g.Expect(block).NotTo(BeNil())
	cert, err := x509.ParseCertificate(block.Bytes)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cert.Subject.CommonName).To(Equal("system:serviceaccount:default:tenant"))
	g.Expect(cert.Subject.Organization).To(ConsistOf(
		"system:serviceaccounts",
		"system:serviceaccounts:default",
	))
	g.Expect(cert.ExtKeyUsage).To(ConsistOf(x509.ExtKeyUsageClientAuth))
	g.Expect(cert.CheckSignatureFrom(caCert)).To(Succeed())
	g.Expect(time.Until(cert.NotAfter)).To(BeNumerically("~", time.Hour, 5*time.Minute))
	g.Expect(credentialRes.credential.ExpiresAt).To(Equal(cert.NotAfter))

	// Verify that the private key matches the certificate.
	_, err = tls.X509KeyPair(credentialRes.credential.Certificate, credentialRes.credential.PrivateKey)
	g.Expect(err).NotTo(HaveOccurred())
}

// signCertificateSigningRequest waits for a certificate signing request to be
// created and approved, signs it with the given CA and updates its status,
// simulating the built-in signer that envtest does not run.
func signCertificateSigningRequest(ctx context.Context, c client.Client,
	caCert *x509.Certificate, caKey *rsa.PrivateKey) (*certv1.CertificateSigningRequest, error) {

	var signed *certv1.CertificateSigningRequest
	err := wait.PollUntilContextCancel(ctx, 100*time.Millisecond, true,
		func(ctx context.Context) (bool, error) {
			var csrList certv1.CertificateSigningRequestList
			if err := c.List(ctx, &csrList); err != nil {
				return false, err
			}
			for i := range csrList.Items {
				csr := &csrList.Items[i]
				if csr.Spec.SignerName != certv1.KubeAPIServerClientSignerName ||
					len(csr.Status.Certificate) > 0 ||
					!isCertificateRequestApproved(csr) {
					continue
				}
				certReq, err := parseCertificateRequest(csr.Spec.Request)
				if err != nil {
					return false, err
				}
				expirationSeconds := int32(3600)
				if csr.Spec.ExpirationSeconds != nil {
					expirationSeconds = *csr.Spec.ExpirationSeconds
				}
				template := &x509.Certificate{
					SerialNumber:          big.NewInt(time.Now().UnixNano()),
					Subject:               certReq.Subject,
					DNSNames:              certReq.DNSNames,
					IPAddresses:           certReq.IPAddresses,
					EmailAddresses:        certReq.EmailAddresses,
					URIs:                  certReq.URIs,
					NotBefore:             time.Now().Add(-time.Minute),
					NotAfter:              time.Now().Add(time.Duration(expirationSeconds) * time.Second),
					KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
					BasicConstraintsValid: true,
				}
				der, err := x509.CreateCertificate(rand.Reader, template, caCert, certReq.PublicKey, caKey)
				if err != nil {
					return false, err
				}
				csr.Status.Certificate = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
				if err := c.SubResource("status").Update(ctx, csr); err != nil {
					return false, err
				}
				signed = csr.DeepCopy()
				return true, nil
			}
			return false, nil
		})
	if err != nil {
		return nil, err
	}
	return signed, nil
}

// isCertificateRequestApproved returns true if the certificate signing request
// has been approved.
func isCertificateRequestApproved(csr *certv1.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certv1.CertificateApproved && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// parseCertificateRequest parses and verifies a PEM-encoded PKCS#10
// certificate signing request.
func parseCertificateRequest(data []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in certificate signing request")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate signing request: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("failed to verify certificate signing request signature: %w", err)
	}
	return csr, nil
}
