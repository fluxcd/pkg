package gittestserver

import (
	"testing"
	"time"
)

func TestCreateSSHServer(t *testing.T) {
	srv, err := NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
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
