/*
Copyright 2025 The Flux authors

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

package gcp_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func startGKEMetadataServer(t *testing.T) {
	t.Helper()
	g := NewWithT(t)

	lis, err := net.Listen("tcp", ":0")
	g.Expect(err).NotTo(HaveOccurred())

	gkeMetadataServer := &http.Server{
		Addr: lis.Addr().String(),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/computeMetadata/v1/project/project-id":
				fmt.Fprintf(w, "%s", "project-id")
			case "/computeMetadata/v1/instance/attributes/cluster-location":
				fmt.Fprintf(w, "%s", "cluster-location")
			case "/computeMetadata/v1/instance/attributes/cluster-name":
				fmt.Fprintf(w, "%s", "cluster-name")
			}
		}),
	}

	go func() {
		err := gkeMetadataServer.Serve(lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			g.Expect(err).NotTo(HaveOccurred())
		}
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := gkeMetadataServer.Shutdown(ctx)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Setenv("GCE_METADATA_HOST", lis.Addr().String())
}
