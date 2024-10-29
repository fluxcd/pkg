/*
Copyright 2024 The Flux authors

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

package github

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"time"
)

type FakeAccessToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func CreateTestPrivateKey() ([]byte, error) {
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pk := pem.EncodeToMemory(privateKeyBlock)
	return pk, nil
}
