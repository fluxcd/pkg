package gittestserver

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCreateSSHServer(t *testing.T) {
	srv, err := NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srv.Root())
	// without setting the key dir, the SSH server will fail to start
	srv.KeyDir(srv.Root())
	errc := make(chan error)
	go func() {
		errc <- srv.StartSSH()
	}()
	select {
	case err := <-errc:
		t.Fatal(err)
	case <-time.After(time.Second):
		break
	}
	srv.StopSSH()
}

func TestListenSSH(t *testing.T) {
	srv, err := NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srv.Root())
	srv.KeyDir(srv.Root())
	if err = srv.ListenSSH(); err != nil {
		t.Fatal(err)
	}
	defer srv.StopSSH()
	addr := srv.SSHAddress()
	// check it's got the right protocol, at least
	if !strings.HasPrefix(addr, "ssh://") {
		t.Fatal("URL given for SSH server doesn't start with ssh://")
	}
}

func TestHTTPServer(t *testing.T) {
	testUsername := "foo"
	testPassword := "bar"

	srv, err := NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srv.Root())
	srv = srv.Auth(testUsername, testPassword)
	if err = srv.StartHTTP(); err != nil {
		t.Fatal(err)
	}
	defer srv.StopHTTP()

	// Check if the address with credentials are right.
	addr := srv.HTTPAddressWithCredentials()
	wantCreds := fmt.Sprintf("%s:%s", testUsername, testPassword)
	if !strings.Contains(addr, wantCreds) {
		t.Errorf("Unexpected credentials in the address %q, want: %s", addr, wantCreds)
	}

	// Check it's got the right protocol.
	if !strings.HasPrefix(addr, "http://") {
		t.Errorf("URL given for HTTP server doesn't start with http://, got: %s", addr)
	}
}

func TestHTTPSServer(t *testing.T) {
	srv, err := NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srv.Root())

	examplePublicKey, err := os.ReadFile("../testdata/certs/server.pem")
	if err != nil {
		t.Fatal(err)
	}
	examplePrivateKey, err := os.ReadFile("../testdata/certs/server-key.pem")
	if err != nil {
		t.Fatal(err)
	}
	exampleCA, err := os.ReadFile("../testdata/certs/ca.pem")
	if err != nil {
		t.Fatal(err)
	}

	if err := srv.StartHTTPS(examplePublicKey, examplePrivateKey, exampleCA, "example.com"); err != nil {
		t.Fatal(err)
	}
	defer srv.StopHTTP()

	// Check it's got the right protocol.
	addr := srv.HTTPAddress()
	if !strings.HasPrefix(addr, "https://") {
		t.Errorf("URL given for HTTPS server doesn't start with https://, got: %s", addr)
	}
}
