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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"time"

	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
)

const (
	// rsaKeyBits is the size of the RSA keys generated for certificate
	// signing requests. RSA-2048 is used because it is broadly supported
	// for authentication, e.g. GCP Workload Identity Federation supports
	// RSA keys from 2048 to 4096 bits and ECDSA keys only from the P-256
	// and P-384 curves.
	rsaKeyBits = 2048

	// certPollInterval is the interval between certificate signing request
	// polls while waiting for a certificate to be issued.
	certPollInterval = 1 * time.Second

	// certPollTimeout is the maximum time to wait for a certificate to be
	// issued for a certificate signing request.
	certPollTimeout = 5 * time.Minute
)

// NewX509 implements auth.X509Provider. It issues an X.509 client certificate
// for a Kubernetes ServiceAccount using the CertificateSigningRequest API and
// the built-in kubernetes.io/kube-apiserver-client signer.
//
// The issued certificate represents the ServiceAccount identity: its common
// name is the ServiceAccount username (system:serviceaccount:<namespace>:<name>)
// and its organization is the set of ServiceAccount groups
// (system:serviceaccounts and system:serviceaccounts:<namespace>), which makes
// the certificate usable both for authenticating with the Kubernetes API
// server and for exchanging with external services. No subject alternative
// names are set, as they are not required by the supported use cases.
//
// The certificate is meant for TLS client authentication. The
// kubernetes.io/kube-apiserver-client signer only issues client certificates,
// so it cannot be used for TLS server authentication.
func (p Provider) NewX509(ctx context.Context, opts ...auth.Option) (*auth.X509, error) {

	var o auth.Options
	o.Apply(opts...)

	if o.Client == nil {
		return nil, fmt.Errorf("client is required to create a Kubernetes certificate")
	}

	// Determine the ServiceAccount the certificate represents.
	serviceAccount, err := p.resolveServiceAccount(o)
	if err != nil {
		return nil, err
	}
	saRef := client.ObjectKeyFromObject(&serviceAccount)

	// Generate a private key and a certificate signing request for the
	// ServiceAccount identity.
	privateKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	csrData, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   serviceAccountUsername(&serviceAccount),
			Organization: serviceAccountGroups(&serviceAccount),
		},
	}, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate signing request: %w", err)
	}

	// Create the CertificateSigningRequest.
	csr := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "flux-",
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Request:    pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrData}),
			SignerName: certv1.KubeAPIServerClientSignerName,
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageClientAuth,
			},
		},
	}
	if o.Credential != nil && o.Credential.ExpirationSeconds != nil {
		exp := *o.Credential.ExpirationSeconds
		if exp < math.MinInt32 || exp > math.MaxInt32 {
			return nil, fmt.Errorf("credential expirationSeconds %d is out of range", exp)
		}
		expirationSeconds := int32(exp)
		csr.Spec.ExpirationSeconds = &expirationSeconds
	}
	if err := o.Client.Create(ctx, csr); err != nil {
		return nil, fmt.Errorf("failed to create certificate signing request for service account '%s': %w",
			saRef, err)
	}

	// Approve the CertificateSigningRequest.
	csr.Status.Conditions = append(csr.Status.Conditions, certv1.CertificateSigningRequestCondition{
		Type:    certv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  "AutoApproved",
		Message: fmt.Sprintf("Automatically approved by Flux for service account '%s'", saRef),
	})
	if err := o.Client.SubResource("approval").Update(ctx, csr); err != nil {
		return nil, fmt.Errorf("failed to approve certificate signing request '%s': %w", csr.Name, err)
	}

	// Wait for the certificate to be issued.
	certData, err := waitForCertificate(ctx, o.Client, csr.Name, csr.UID)
	if err != nil {
		return nil, err
	}

	// Parse the issued certificate to get its expiration time.
	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("issued certificate for certificate signing request '%s' is not PEM-encoded", csr.Name)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issued certificate for certificate signing request '%s': %w", csr.Name, err)
	}

	// Encode the private key.
	keyData, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	return &auth.X509{
		Certificate: certData,
		PrivateKey:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyData}),
		ExpiresAt:   cert.NotAfter,
	}, nil
}

// waitForCertificate polls the CertificateSigningRequest until a certificate
// is issued, the request is denied or failed, or the timeout is reached.
func waitForCertificate(ctx context.Context, c client.Client, name string, uid types.UID) ([]byte, error) {
	var certData []byte
	err := wait.PollUntilContextTimeout(ctx, certPollInterval, certPollTimeout, true,
		func(ctx context.Context) (bool, error) {
			var csr certv1.CertificateSigningRequest
			if err := c.Get(ctx, client.ObjectKey{Name: name}, &csr); err != nil {
				if apierrors.IsNotFound(err) {
					return false, fmt.Errorf("certificate signing request '%s' was deleted", name)
				}
				return false, err
			}
			if csr.UID != uid {
				return false, fmt.Errorf("certificate signing request '%s' changed UIDs", name)
			}
			for _, condition := range csr.Status.Conditions {
				switch condition.Type {
				case certv1.CertificateDenied:
					return false, fmt.Errorf("certificate signing request '%s' was denied: %s: %s",
						name, condition.Reason, condition.Message)
				case certv1.CertificateFailed:
					return false, fmt.Errorf("certificate signing request '%s' failed: %s: %s",
						name, condition.Reason, condition.Message)
				}
			}
			if len(csr.Status.Certificate) == 0 {
				return false, nil
			}
			certData = csr.Status.Certificate
			return true, nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed waiting for certificate for certificate signing request '%s': %w", name, err)
	}
	return certData, nil
}

// serviceAccountUsername returns the Kubernetes username of the ServiceAccount.
func serviceAccountUsername(serviceAccount *corev1.ServiceAccount) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", serviceAccount.Namespace, serviceAccount.Name)
}

// serviceAccountGroups returns the Kubernetes groups of the ServiceAccount.
func serviceAccountGroups(serviceAccount *corev1.ServiceAccount) []string {
	return []string{
		"system:serviceaccounts",
		fmt.Sprintf("system:serviceaccounts:%s", serviceAccount.Namespace),
	}
}
