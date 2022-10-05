/*
Copyright 2022 The Flux authors

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

package libgit2

import (
	"fmt"
	"strings"

	git2go "github.com/libgit2/git2go/v34"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/fluxcd/pkg/git"
)

const (
	sshKeyWriteAccessError = "the SSH key may not have write access to the repository"
)

func getTransportOptsURL(transport git.TransportType) string {
	return string(transport) + "://" + string(uuid.NewUUID())
}

func pushError(err error, url string) error {
	if strings.Contains(err.Error(), "early EOF") && strings.HasPrefix(url, "ssh") {
		return fmt.Errorf("%w (%s)", err, sshKeyWriteAccessError)
	}
	return err
}

// RemoteCallbacks constructs git2go.RemoteCallbacks with dummy callbacks.
// Our smart transports don't require any callbacks but, passing nil to
// high level git2go functions like Push, Clone can result in panic, thus
// it's safer to use no-op functions.
func RemoteCallbacks() git2go.RemoteCallbacks {
	return git2go.RemoteCallbacks{
		CredentialsCallback:      credentialsCallback(),
		CertificateCheckCallback: certificateCallback(),
	}
}

// credentialsCallback constructs a dummy CredentialsCallback.
func credentialsCallback() git2go.CredentialsCallback {
	return func(url string, username string, allowedTypes git2go.CredentialType) (*git2go.Credential, error) {
		// If Credential is nil, panic will ensue. We fake it as managed transport does not
		// require it.
		return git2go.NewCredentialUserpassPlaintext("", "")
	}
}

// certificateCallback constructs a dummy CertificateCallback.
func certificateCallback() git2go.CertificateCheckCallback {
	// returning a nil func can cause git2go to panic.
	return func(cert *git2go.Certificate, valid bool, hostname string) error {
		return nil
	}
}
