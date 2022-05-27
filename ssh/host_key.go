/*
Copyright 2020 The Flux authors

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

import (
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ScanHostKey collects the given host's preferred public key for the
// host. Any errors (e.g. authentication  failures) are ignored, except
// if no key could be collected from the host.
//
// clientHostKeyAlgos defines what HostKey algorithms to be
// used by the ssh client when using `ssh.Dial`. The default is
// empty, which defaults to Golang's preferred HostKey algorithms.
func ScanHostKey(host string, timeout time.Duration, clientHostKeyAlgos []string, hashKeys bool) ([]byte, error) {
	col := &HostKeyCollector{hashKeys: hashKeys}
	config := &ssh.ClientConfig{
		HostKeyCallback: col.StoreKey(),
		Timeout:         timeout,
	}
	config.SetDefaults()

	if len(clientHostKeyAlgos) > 0 {
		config.HostKeyAlgorithms = clientHostKeyAlgos
	}

	client, err := ssh.Dial("tcp", host, config)
	if err == nil {
		defer client.Close()
	}
	if len(col.knownKeys) > 0 {
		return col.knownKeys, nil
	}
	return col.knownKeys, err
}

// HostKeyCollector offers a StoreKey method which provides an
// HostKeyCallBack to collect public keys from an SSH server.
type HostKeyCollector struct {
	knownKeys []byte
	hashKeys  bool
}

// StoreKey stores the public key in bytes as returned by the host.
// To collect multiple public key types from the host, multiple
// SSH dials need with the ClientConfig HostKeyAlgorithms set to
// the algorithm you want to collect.
func (c *HostKeyCollector) StoreKey() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		h := knownhosts.Normalize(hostname)
		if c.hashKeys {
			h = knownhosts.HashHostname(h)
		}
		c.knownKeys = append(
			c.knownKeys,
			fmt.Sprintf("%s %s %s\n", h, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))...,
		)
		return nil
	}
}

// GetKnownKeys returns the collected public keys in bytes.
func (c *HostKeyCollector) GetKnownKeys() []byte {
	return c.knownKeys
}
