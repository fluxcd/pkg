package gittestserver

import (
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
