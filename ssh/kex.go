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
// https://github.com/golang/crypto/blob/5d542ad81a58c89581d596f49d0ba5d435481bcf/ssh/kex.go#L23-L36
const (
	kexAlgoDH14SHA1               = "diffie-hellman-group14-sha1"
	kexAlgoDH14SHA256             = "diffie-hellman-group14-sha256"
	kexAlgoECDH256                = "ecdh-sha2-nistp256"
	kexAlgoECDH384                = "ecdh-sha2-nistp384"
	kexAlgoECDH521                = "ecdh-sha2-nistp521"
	kexAlgoCurve25519SHA256       = "curve25519-sha256"
	kexAlgoCurve25519SHA256LibSSH = "curve25519-sha256@libssh.org"

	// For the following kex only the client half contains a production
	// ready implementation. The server half only consists of a minimal
	// implementation to satisfy the automated tests.
	kexAlgoDHGEXSHA256 = "diffie-hellman-group-exchange-sha256"
)

// PreferredKeyAlgos is aligned with the preferredKeyAlgos from golang/crypto
// with the exception of:
// - No support for diffie-hellman-group1-sha1.
// - Includes kexAlgoDHGEXSHA256 as the least preferred option.
var PreferredKexAlgos = []string{
	kexAlgoCurve25519SHA256, kexAlgoCurve25519SHA256LibSSH,
	kexAlgoECDH256, kexAlgoECDH384, kexAlgoECDH521,
	kexAlgoDH14SHA256, kexAlgoDH14SHA1,
	kexAlgoDHGEXSHA256,
}

// SetPreferredKeyAlgos sets the PreferredKexAlgos on a given ClientConfig.
func SetPreferredKeyAlgos(config *ssh.ClientConfig) {
	if config != nil {
		config.KeyExchanges = PreferredKexAlgos
	}
}
