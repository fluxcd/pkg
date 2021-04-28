/*
Copyright 2021 The Flux authors

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

package ssh

import "golang.org/x/crypto/ssh"

// These string constants are copied from golang/crypto
// https://github.com/golang/crypto/blob/0c34fe9e7dc2486962ef9867e3edb3503537209f/ssh/kex.go#L23-L34
const (
	kexAlgoDH1SHA1          = "diffie-hellman-group1-sha1"
	kexAlgoDH14SHA1         = "diffie-hellman-group14-sha1"
	kexAlgoECDH256          = "ecdh-sha2-nistp256"
	kexAlgoECDH384          = "ecdh-sha2-nistp384"
	kexAlgoECDH521          = "ecdh-sha2-nistp521"
	kexAlgoCurve25519SHA256 = "curve25519-sha256@libssh.org"

	// For the following kex only the client half contains a production
	// ready implementation. The server half only consists of a minimal
	// implementation to satisfy the automated tests.
	kexAlgoDHGEXSHA1   = "diffie-hellman-group-exchange-sha1"
	kexAlgoDHGEXSHA256 = "diffie-hellman-group-exchange-sha256"
)

// PreferredKeyAlgos is aligned with the preferredKeyAlgos from golang/crypto
// but includes kexAlgoDHGEXSHA256 as the least preferred option.
var PreferredKexAlgos = []string{
	kexAlgoCurve25519SHA256,
	kexAlgoECDH256, kexAlgoECDH384, kexAlgoECDH521,
	kexAlgoDH14SHA1,
	kexAlgoDHGEXSHA256,
}

// SetPreferredKeyAlgos sets the PreferredKexAlgos on a given ClientConfig.
func SetPreferredKeyAlgos(config *ssh.ClientConfig) {
	if config != nil {
		config.KeyExchanges = PreferredKexAlgos
	}
}
