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

package transport

import (
	"math/rand"
	"sync"
	"time"
)

var (
	once    sync.Once
	enabled bool
)

func init() {
	rand.Seed(time.Now().Unix())
}

// Enabled defines whether the use of Managed Transport is enabled which
// is only true if InitManagedTransport was called successfully at least
// once.
func Enabled() bool {
	return enabled
}

// InitManagedTransport initialises HTTP(S) and SSH managed transport
// for git2go, and therefore only impact git operations using the
// libgit2 implementation.
//
// This must run after git2go.init takes place, hence this is not executed
// within a init().
// Regardless of the state in libgit2/git2go, this will replace the
// built-in transports.
//
// This function will only register managed transports once, subsequent calls
// leads to no-op.
func InitManagedTransport() error {
	var err error

	once.Do(func() {
		if err = registerManagedHTTP(); err != nil {
			return
		}

		if err = registerManagedSSH(); err == nil {
			enabled = true
		}
	})

	return err
}
