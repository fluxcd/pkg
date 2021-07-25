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
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"testing"
)

func TestArtifactServer(t *testing.T) {
	srv, err := NewTempArtifactServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srv.Root())
	srv.Start()
	defer srv.Stop()

	files := []File{
		{"fileA", "foo"},
		{"fileB", "bar"},
		{"fileC", "baz"},
	}
	filename, err := srv.ArtifactFromFiles(files)
	if err != nil {
		t.Fatalf("failed to get artifact from files: %v", err)
	}

	// Get the artifact from the server.
	url, err := srv.URLForFile(filename)
	if err != nil {
		t.Errorf("failed to get URL from file: %v", err)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("failed to download the artifact: %v", err)
	}
	defer resp.Body.Close()

	gzRead, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Error(err)
	}
	tarRead := tar.NewReader(gzRead)

	// Check if the artifact contains all the expected files.
	count := 0
	for {
		cur, err := tarRead.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Error(err)
		}
		if cur.Typeflag != tar.TypeReg {
			continue
		}
		count++
		found := false
		for _, f := range files {
			if cur.Name == f.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("found unexpected file in the artifact: %s", cur.Name)
		}
	}
	if count != len(files) {
		t.Errorf("expected file count: %d, got: %d", len(files), count)
	}
}
