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

package knownhosts

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
)

type KnownKey struct {
	hosts []string
	key   ssh.PublicKey
}

// ParseKnownHosts takes a string with the contents of known_hosts, parses it
// and returns a slice of KnownKey
func ParseKnownHosts(s string) ([]KnownKey, error) {
	var knownHosts []KnownKey
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		_, hosts, pubKey, _, _, err := ssh.ParseKnownHosts(scanner.Bytes())
		if err != nil {
			// Lines that aren't host public key result in EOF, like a comment
			// line. Continue parsing the other lines.
			if err == io.EOF {
				continue
			}
			return []KnownKey{}, err
		}

		knownHost := KnownKey{
			hosts: hosts,
			key:   pubKey,
		}
		knownHosts = append(knownHosts, knownHost)
	}

	if err := scanner.Err(); err != nil {
		return []KnownKey{}, err
	}

	return knownHosts, nil
}

// Matches checks if the specified host is present and if the fingerprint matches
// the present public key key.
func (k KnownKey) Matches(host string, fingerprint []byte) bool {
	if !containsHost(k.hosts, host) {
		return false
	}
	hasher := sha256.New()
	hasher.Write(k.key.Marshal())
	return bytes.Equal(hasher.Sum(nil), fingerprint)
}

func containsHost(hosts []string, host string) bool {
	for _, kh := range hosts {
		// hashed host must start with a pipe
		if kh[0] == '|' {
			match, _ := matchHashedHost(kh, host)
			if match {
				return true
			}

		} else if kh == host { // unhashed host check
			return true
		}
	}
	return false
}

// matchHashedHost tries to match a hashed known host (kh) to
// host.
//
// Note that host is not hashed, but it is rather hashed during
// the matching process using the same salt used when hashing
// the known host.
func matchHashedHost(kh, host string) (bool, error) {
	if kh == "" || kh[0] != '|' {
		return false, fmt.Errorf("hashed known host must begin with '|': '%s'", kh)
	}

	components := strings.Split(kh, "|")
	if len(components) != 4 {
		return false, fmt.Errorf("invalid format for hashed known host: '%s'", kh)
	}

	if components[1] != "1" {
		return false, fmt.Errorf("unsupported hash type '%s'", components[1])
	}

	hkSalt, err := base64.StdEncoding.DecodeString(components[2])
	if err != nil {
		return false, fmt.Errorf("cannot decode hashed known host: '%w'", err)
	}

	hkHash, err := base64.StdEncoding.DecodeString(components[3])
	if err != nil {
		return false, fmt.Errorf("cannot decode hashed known host: '%w'", err)
	}

	mac := hmac.New(sha1.New, hkSalt)
	mac.Write([]byte(host))
	hostHash := mac.Sum(nil)

	return bytes.Equal(hostHash, hkHash), nil
}
