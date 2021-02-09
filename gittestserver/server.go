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

package gittestserver

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http/httptest"
	"path/filepath"

	"github.com/sosedoff/gitkit"
)

// NewTempGitServer returns a GitServer with a newly created temp
// dir as repository docroot.
func NewTempGitServer() (*GitServer, error) {
	tmpDir, err := ioutil.TempDir("", "git-server-test-")
	if err != nil {
		return nil, err
	}
	srv := NewGitServer(tmpDir)
	return srv, nil
}

// NewGitServer returns a GitServer with the given repository docroot
// set.
func NewGitServer(docroot string) *GitServer {
	root, err := filepath.Abs(docroot)
	if err != nil {
		panic(err)
	}
	return &GitServer{
		config: gitkit.Config{
			Dir: root,
		},
	}
}

// GitServer is a git server for testing purposes.
// It can serve git repositories over HTTP and SSH.
type GitServer struct {
	config     gitkit.Config
	httpServer *httptest.Server
	sshServer  *gitkit.SSH
}

// AutoCreate enables the automatic creation of a non-existing Git
// repository on push.
func (s *GitServer) AutoCreate() *GitServer {
	s.config.AutoCreate = true
	return s
}

// KeyDir sets the SSH key directory in the config. Use before calling
// StartSSH.
func (s *GitServer) KeyDir(dir string) *GitServer {
	s.config.KeyDir = dir
	return s
}

// StartHTTP starts a new HTTP git server with the current configuration.
func (s *GitServer) StartHTTP() error {
	s.StopHTTP()
	service := gitkit.New(s.config)
	if err := service.Setup(); err != nil {
		return err
	}
	s.httpServer = httptest.NewServer(service)
	return nil
}

// StartHTTPS starts the TLS HTTPServer with the given TLS configuration.
func (s *GitServer) StartHTTPS(cert, key, ca []byte, serverName string) error {
	s.StopHTTP()
	service := gitkit.New(s.config)
	if err := service.Setup(); err != nil {
		return err
	}
	s.httpServer = httptest.NewUnstartedServer(service)

	config := tls.Config{}

	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return err
	}
	config.Certificates = []tls.Certificate{keyPair}

	cp := x509.NewCertPool()
	cp.AppendCertsFromPEM(ca)
	config.RootCAs = cp

	config.ServerName = serverName
	s.httpServer.TLS = &config

	s.httpServer.StartTLS()
	return nil
}

// StopHTTP stops the HTTP git server.
func (s *GitServer) StopHTTP() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	return
}

// ListenSSH creates an SSH server and a listener if not already
// created, but does not handle connections. This returns immediately,
// unlike StartSSH(), and the server URL is available with
// SSHAddress() after calling this.
func (s *GitServer) ListenSSH() error {
	if s.sshServer == nil {
		s.sshServer = gitkit.NewSSH(s.config)
		// This is where authentication would happen, when needed.
		s.sshServer.PublicKeyLookupFunc = func(string) (*gitkit.PublicKey, error) {
			return &gitkit.PublicKey{Id: "test-user"}, nil
		}
		// :0 should result in an OS assigned free port; 127.0.0.1
		// forces the lowest common denominator of TCPv4 on localhost.
		return s.sshServer.Listen("127.0.0.1:0")
	}
	return nil
}

// StartSSH creates a SSH git server and listener with the current
// configuration if necessary, and handles connections. Unless it
// returns an error immediately, this will block until the listener is
// stopped with `s.StopSSH()`. Usually you will want to use
// ListenSSH() first, so you can get the URL of the SSH git server
// before starting it.
func (s *GitServer) StartSSH() error {
	if err := s.ListenSSH(); err != nil {
		return err
	}
	return s.sshServer.Serve()
}

// StopSSH stops the SSH git server.
func (s *GitServer) StopSSH() error {
	if s.sshServer != nil {
		return s.sshServer.Stop()
	}
	return nil
}

// Root returns the repositories root directory.
func (s *GitServer) Root() string {
	return s.config.Dir
}

// HTTPAddress returns the address of the HTTP git server.
func (s *GitServer) HTTPAddress() string {
	if s.httpServer != nil {
		return s.httpServer.URL
	}
	return ""
}

// SSHAddress returns the address of the SSH git server as a URL.
func (s *GitServer) SSHAddress() string {
	if s.sshServer != nil {
		return "ssh://git@" + s.sshServer.Address()
	}
	return ""
}
