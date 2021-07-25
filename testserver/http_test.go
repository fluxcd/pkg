/*
Copyright 2021 The Flux authors

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

package testserver

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
)

const testResponseContent = "foo-bar-content"

// testMiddleware writes some test content in the response.
func testMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testResponseContent))
		next.ServeHTTP(w, r)
	})
}

// testMiddlewareResult fetches content from a given address and verifies that
// the response body is as expected.
func testMiddlewareResult(t *testing.T, client *http.Client, addr string, want string) {
	resp, err := client.Get(addr)
	if err != nil {
		t.Errorf("failed to GET %s: %v", addr, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(string(body), want) {
		t.Errorf("expected the response body to contain %q, got: %q", want, string(body))
	}
}

func TestHTTPServer(t *testing.T) {
	srv, err := NewTempHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srv.Root())
	srv.WithMiddleware(testMiddleware).Start()
	defer srv.Stop()

	addr := srv.URL()
	// Check it's got the right protocol.
	if !strings.HasPrefix(addr, "http://") {
		t.Errorf("URL given for HTTP server doesn't start with http://, got: %s", addr)
	}

	// Check if the middleware worked.
	testMiddlewareResult(t, &http.Client{}, addr, testResponseContent)
}

func TestHTTPSServer(t *testing.T) {
	srv, err := NewTempHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srv.Root())

	examplePublicKey, err := ioutil.ReadFile("../testdata/certs/server.pem")
	if err != nil {
		t.Fatal(err)
	}
	examplePrivateKey, err := ioutil.ReadFile("../testdata/certs/server-key.pem")
	if err != nil {
		t.Fatal(err)
	}
	exampleCA, err := ioutil.ReadFile("../testdata/certs/ca.pem")
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := ioutil.ReadFile("../testdata/certs/ca.pem")
	if err != nil {
		t.Fatal(err)
	}

	err = srv.WithMiddleware(testMiddleware).StartTLS(examplePublicKey, examplePrivateKey, exampleCA, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	addr := srv.URL()
	// Check it's got the right protocol.
	if !strings.HasPrefix(addr, "https://") {
		t.Errorf("URL given for HTTP server doesn't start with http://, got: %s", addr)
	}

	// Configure an http client with the CA cert.
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}
	// Check if the middleware worked.
	testMiddlewareResult(t, client, addr, testResponseContent)
}
