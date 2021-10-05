package gittestserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
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

func TestInitRepo(t *testing.T) {
	srv, err := NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	srv.Auth("test-user", "test-pswd")
	defer os.RemoveAll(srv.Root())
	srv.KeyDir(srv.Root())
	if err = srv.StartHTTP(); err != nil {
		t.Fatal(err)
	}
	defer srv.StopHTTP()

	repoPath := "bar/test-reponame"
	err = srv.InitRepo("testdata/git/repo1", "main", repoPath)
	if err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}

	// Clone and verify the repo.
	cloneDir, err := os.MkdirTemp("", "test-clone-")
	if err != nil {
		t.Fatalf("failed to create clone dir: %v", err)
	}
	defer os.RemoveAll(cloneDir)

	repoURL := srv.HTTPAddressWithCredentials() + "/" + repoPath
	_, err = gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{
		URL: repoURL,
	})
	if err != nil {
		t.Fatalf("failed to clone repo: %v", err)
	}

	// Check file from clone.
	if _, err := os.Stat(filepath.Join(cloneDir, "foo.txt")); os.IsNotExist(err) {
		t.Error("expected foo.txt to exist")
	}
}
