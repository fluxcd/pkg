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

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/untar"
)

func TestBuild(t *testing.T) {
	g := NewWithT(t)
	testDir := "./testdata/artifact"
	c := NewLocalClient()

	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "files.tar.gz")

	// test with non-existent path
	err := c.Build(artifactPath, "testdata/non-existent", nil)
	g.Expect(err).To(HaveOccurred())

	ignorePaths := []string{"ignore.txt", "ignore-dir/", "!/deploy"}
	err = c.Build(artifactPath, testDir, ignorePaths)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = os.Stat(artifactPath)
	g.Expect(err).ToNot(HaveOccurred())

	b, err := os.ReadFile(artifactPath)
	g.Expect(err).ToNot(HaveOccurred())

	untarDir := t.TempDir()
	_, err = untar.Untar(bytes.NewReader(b), untarDir)
	g.Expect(err).To(BeNil())

	for _, path := range ignorePaths {
		var shouldExist bool
		if strings.HasPrefix(path, "!") {
			shouldExist = true
			path = path[1:]
		}

		fullPath := filepath.Join(untarDir, testDir, path)
		_, err := os.Stat(fullPath)
		if shouldExist {
			g.Expect(err).To(BeNil())
			return
		}
		g.Expect(err).ToNot(BeNil())
		g.Expect(os.IsNotExist(err)).To(BeTrue())
	}
}
