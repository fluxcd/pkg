package gittestserver

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/fluxcd/go-git/v5"
	"github.com/fluxcd/go-git/v5/plumbing"
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
	repoPath := "bar/test-reponame"
	initBranch := "test-branch"

	tests := []struct {
		name     string
		testFunc func(srv *GitServer, repoURL string)
	}{
		{
			name: "clone repo without any reference",
			testFunc: func(srv *GitServer, repoURL string) {
				cloneDir, err := os.MkdirTemp("", "test-clone-")
				if err != nil {
					t.Fatalf("failed to create clone dir: %v", err)
				}
				defer os.RemoveAll(cloneDir)

				// Clone the branch.
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
			},
		},
		{
			name: "clone initialized repo branch",
			testFunc: func(srv *GitServer, repoURL string) {
				cloneDir, err := os.MkdirTemp("", "test-clone-")
				if err != nil {
					t.Fatalf("failed to create clone dir: %v", err)
				}
				defer os.RemoveAll(cloneDir)

				// Clone the branch.
				_, err = gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{
					URL:           repoURL,
					ReferenceName: plumbing.NewBranchReferenceName("test-branch"),
				})
				if err != nil {
					t.Fatalf("failed to clone repo: %v", err)
				}

				// Check file from clone.
				if _, err := os.Stat(filepath.Join(cloneDir, "foo.txt")); os.IsNotExist(err) {
					t.Error("expected foo.txt to exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			// Initialize a repo.
			err = srv.InitRepo("testdata/git/repo1", initBranch, repoPath)
			if err != nil {
				t.Fatalf("failed to initialize repo: %v", err)
			}

			repoURL := srv.HTTPAddressWithCredentials() + "/" + repoPath

			tt.testFunc(srv, repoURL)
		})
	}
}

func TestGitServer_AddHTTPMiddlewares(t *testing.T) {
	repoPath := "bar/test-reponame"
	initBranch := "test-branch"

	srv, err := NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	srv.Auth("test-user", "test-pswd")
	defer os.RemoveAll(srv.Root())
	srv.KeyDir(srv.Root())

	// Add a middleware.
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "something failed", http.StatusInternalServerError)
		})
	}
	srv.AddHTTPMiddlewares(middleware)

	if err = srv.StartHTTP(); err != nil {
		t.Fatal(err)
	}
	defer srv.StopHTTP()

	// Initialize a repo.
	err = srv.InitRepo("testdata/git/repo1", initBranch, repoPath)
	if err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}
	repoURL := srv.HTTPAddressWithCredentials() + "/" + repoPath

	// Clone the branch.
	_, err = gogit.PlainClone("some-non-existing-dir", false, &gogit.CloneOptions{
		URL: repoURL,
	})
	if !strings.Contains(err.Error(), "status code: 500") {
		t.Errorf("expected error status code 500, got: %v", err)
	}
}
