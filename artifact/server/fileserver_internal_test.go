/*
Copyright 2026 The Flux authors

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

package server

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/artifact/config"
)

func Test_newServer_Timeouts(t *testing.T) {
	g := NewWithT(t)

	srv := newServer(&config.Options{StoragePath: t.TempDir(), StorageAddress: ":0"})

	g.Expect(srv.ReadHeaderTimeout).To(Equal(readHeaderTimeout))
	g.Expect(srv.IdleTimeout).To(Equal(idleTimeout))
	g.Expect(srv.ReadTimeout).To(BeZero())
	g.Expect(srv.WriteTimeout).To(BeZero())
}

func Test_newServer_ClosesStalledConnections(t *testing.T) {
	g := NewWithT(t)

	srv := newServer(&config.Options{StoragePath: t.TempDir()})
	srv.ReadHeaderTimeout = 200 * time.Millisecond

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).NotTo(HaveOccurred())
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	// A connection that never sends a request is closed by the server.
	conn, err := net.Dial("tcp", ln.Addr().String())
	g.Expect(err).NotTo(HaveOccurred())
	defer conn.Close()
	g.Expect(conn.SetReadDeadline(time.Now().Add(5 * time.Second))).To(Succeed())

	_, err = conn.Read(make([]byte, 1))
	g.Expect(err).To(MatchError(io.EOF))

	// A request on a new connection is still served.
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
}
