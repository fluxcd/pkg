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

package signature

import "errors"

// Sentinel errors returned by the verification functions. Callers can use
// errors.Is to branch on these conditions; errors returned by the
// verification functions wrap one or more of these as appropriate.
//
// Errors from the underlying sshsig library (e.g. sshsig.ErrPublicKeyMismatch,
// sshsig.ErrNamespaceMismatch, sshsig.ErrUnsupportedHashAlgorithm) are
// preserved in the error chain when VerifySSHSignature exhausts all
// authorized keys without a match, so callers may also branch on those.
var (
	// ErrSignatureEmpty is returned when no signature was provided.
	ErrSignatureEmpty = errors.New("signature is empty")

	// ErrPayloadEmpty is returned when no payload was provided.
	ErrPayloadEmpty = errors.New("payload is empty")

	// ErrSignatureFormat is returned when the provided signature is not in
	// the format expected by the verification function, for example an SSH
	// signature handed to VerifyPGPSignature.
	ErrSignatureFormat = errors.New("signature format mismatch")

	// ErrNoMatchingKey is returned when verification was attempted against
	// at least one key ring or authorized_keys input and none of them could
	// verify the signature against the payload.
	ErrNoMatchingKey = errors.New("no matching key")
)
