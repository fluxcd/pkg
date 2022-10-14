/*
Copyright 2022 The Flux authors

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

package client

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/tar"
	. "github.com/onsi/gomega"
)

func TestBuild(t *testing.T) {
	c := NewLocalClient()

	tests := []struct {
		name       string
		path       string
		ignorePath []string
		expectErr  bool
		checkPaths []string
	}{
		{
			name:      "non-existent path",
			path:      "testdata/non-existent",
			expectErr: true,
		},
		{
			name:       "existing path",
			path:       "testdata/artifact",
			ignorePath: []string{"ignore.txt", "ignore-dir/", "!/deploy", "somedir/git"},
			checkPaths: []string{"ignore.txt", "ignore-dir/", "!/deploy", "somedir/git"},
		},
		{
			name:       "existing path with leading slash",
			path:       "./testdata/artifact",
			ignorePath: []string{"ignore.txt", "ignore-dir/", "!/deploy", "somedir/git"},
			checkPaths: []string{"ignore.txt", "ignore-dir/", "!/deploy", "somedir/git"},
		},
		{
			name:       "current directory",
			path:       ".",
			ignorePath: []string{"/*", "!/internal"},
			checkPaths: []string{"/testdata", "!internal/", "build.go", "meta.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tmpDir := t.TempDir()
			artifactPath := filepath.Join(tmpDir, "files.tar.gz")

			err := c.Build(artifactPath, tt.path, tt.ignorePath)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).To(Not(HaveOccurred()))

			_, err = os.Stat(artifactPath)
			g.Expect(err).ToNot(HaveOccurred())

			b, err := os.ReadFile(artifactPath)
			g.Expect(err).ToNot(HaveOccurred())

			untarDir := t.TempDir()
			err = tar.Untar(bytes.NewReader(b), untarDir, tar.WithMaxUntarSize(-1))
			g.Expect(err).To(BeNil())

			checkPathExists(t, untarDir, tt.path, tt.checkPaths)
		})
	}
}

// test only one file exists
func TestBuildOneFile(t *testing.T) {
	c := NewLocalClient()
	g := NewWithT(t)

	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "files.tar.gz")

	sourceDir := "testdata/artifact"
	sourceFile := filepath.Join(sourceDir, "/deployment.yaml")

	err := c.Build(artifactPath, sourceFile, []string{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(artifactPath)
	g.Expect(err).ToNot(HaveOccurred())

	b, err := os.ReadFile(artifactPath)
	g.Expect(err).ToNot(HaveOccurred())

	untarDir := t.TempDir()
	err = tar.Untar(bytes.NewReader(b), untarDir, tar.WithMaxUntarSize(-1))
	g.Expect(err).ToNot(HaveOccurred())

	_, err = os.Stat(filepath.Join(untarDir, sourceFile))
	g.Expect(err).ToNot(HaveOccurred())

	files, err := os.ReadDir(filepath.Join(untarDir, sourceDir))
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(len(files)).To(Equal(1))
}

func checkPathExists(t *testing.T, dir, testDir string, paths []string) {
	g := NewWithT(t)

	for _, path := range paths {
		var shouldExist bool
		if strings.HasPrefix(path, "!") {
			shouldExist = true
			path = path[1:]
		}

		fullPath := filepath.Join(dir, testDir, path)
		_, err := os.Stat(fullPath)
		if shouldExist {
			g.Expect(err).To(BeNil())
			continue
		}
		g.Expect(err).ToNot(BeNil())
		g.Expect(os.IsNotExist(err)).To(BeTrue())
	}
}
