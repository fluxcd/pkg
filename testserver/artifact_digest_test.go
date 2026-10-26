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

package testserver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "crypto/sha512"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	digest "github.com/opencontainers/go-digest"
	_ "github.com/opencontainers/go-digest/blake3"
)

func TestArtifactFromDirWithDigest(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	wantFiles := map[string]string{"values.yaml": "replicas: 2\n", "nested/config": "hello"}
	for name, body := range wantFiles {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, ".hidden"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := &ArtifactServer{NewHTTPServer(t.TempDir())}
	srv.Start()
	t.Cleanup(srv.Stop)

	for _, algorithm := range []digest.Algorithm{digest.SHA256, digest.SHA384, digest.SHA512, "blake3"} {
		t.Run(algorithm.String(), func(t *testing.T) {
			name := algorithm.String() + ".tar.gz"
			got, err := srv.ArtifactFromDirWithDigest(source, name, algorithm)
			if err != nil {
				t.Fatal(err)
			}
			url, err := srv.URLForFile(name)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.Get(url)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Error(err)
				}
			})
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK || got != algorithm.FromBytes(body) {
				t.Fatalf("status %d, digest %s does not match downloaded archive", resp.StatusCode, got)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			gz, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := gz.Close(); err != nil {
					t.Error(err)
				}
			})
			tr := tar.NewReader(gz)
			files := make(map[string]string)
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				files[header.Name] = string(content)
			}
			if !reflect.DeepEqual(files, wantFiles) {
				t.Fatalf("archive contents: got %v, want %v", files, wantFiles)
			}
		})
	}

	checksum, err := srv.ArtifactFromDir(source, "legacy.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	archive, err := os.ReadFile(filepath.Join(srv.Root(), "legacy.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if want := digest.SHA256.FromBytes(archive).Encoded(); checksum != want {
		t.Fatalf("legacy checksum: got %s, want %s", checksum, want)
	}
}

func TestArtifactFromDirWithDigestErrors(t *testing.T) {
	source := t.TempDir()
	file := filepath.Join(source, "file")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		source      string
		destination string
		algorithm   digest.Algorithm
		wantError   error
	}{
		{"unknown algorithm", source, "out.tar.gz", "unknown", digest.ErrDigestUnsupported},
		{"empty algorithm", source, "out.tar.gz", "", digest.ErrDigestUnsupported},
		{"missing source", filepath.Join(source, "missing"), "out.tar.gz", digest.SHA256, os.ErrNotExist},
		{"source is a file", file, "out.tar.gz", digest.SHA256, nil},
		{"missing destination directory", source, "missing/out.tar.gz", digest.SHA256, os.ErrNotExist},
		{"destination escapes root", source, "../out.tar.gz", digest.SHA256, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := &ArtifactServer{NewHTTPServer(t.TempDir())}
			got, err := srv.ArtifactFromDirWithDigest(tc.source, tc.destination, tc.algorithm)
			if err == nil || got != "" {
				t.Fatalf("got digest %q and error %v, want empty digest and error", got, err)
			}
			if tc.wantError != nil && !errors.Is(err, tc.wantError) {
				t.Fatalf("got error %v, want %v", err, tc.wantError)
			}
			entries, err := os.ReadDir(srv.Root())
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed creation left artifacts behind: %v, error %v", entries, err)
			}
		})
	}
}
