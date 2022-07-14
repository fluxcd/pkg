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

package filesys

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func TestMakeFsOnDiskSecure(t *testing.T) {
	t.Run("error on root prefixed with allowed prefix", func(t *testing.T) {
		g := NewWithT(t)

		tmpDir, err := testTempDir(t)
		g.Expect(err).ToNot(HaveOccurred())

		matchingDir := filepath.Join(tmpDir, "subdir")
		g.Expect(os.Mkdir(matchingDir, 0o644)).To(Succeed())

		got, err := MakeFsOnDiskSecure(filepath.Join(tmpDir, "subdir"), tmpDir)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("cannot be prefixed with"))
		g.Expect(got).To(BeNil())
	})
}

func Test_fsSecure_Create(t *testing.T) {
	g := NewWithT(t)

	root, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure create", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "file.txt")
		got, err := fs.Create(path)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.Close()).To(Succeed())
		g.Expect(fs.Exists(path)).To(BeTrue())
	})

	t.Run("illegal create", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "../file.txt")
		got, err := fs.Create("/file.txt")
		g.Expect(err).To(HaveOccurred())
		g.Expect(got).To(BeNil())
		g.Expect(fs.Exists(path)).To(BeFalse())
	})
}

func Test_fsSecure_Mkdir(t *testing.T) {
	g := NewWithT(t)

	root, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure mkdir", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "secure")
		g.Expect(fs.Mkdir(path)).To(Succeed())
		g.Expect(path).To(BeADirectory())
	})

	t.Run("illegal mkdir", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(os.TempDir(), "illegal")
		g.Expect(fs.Mkdir(path)).To(HaveOccurred())
		g.Expect(path).ToNot(BeADirectory())
		g.Expect(path).ToNot(BeAnExistingFile())
	})
}

func Test_fsSecure_MkdirAll(t *testing.T) {
	g := NewWithT(t)

	root, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure mkdir all", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "secure", "subdir")
		g.Expect(fs.MkdirAll(path)).To(Succeed())
		g.Expect(path).To(BeADirectory())
	})

	t.Run("illegal mkdir all", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "..", "..", "subdir")
		g.Expect(fs.MkdirAll(path)).To(HaveOccurred())
		g.Expect(path).ToNot(BeADirectory())
		g.Expect(path).ToNot(BeAnExistingFile())
	})
}

func Test_fsSecure_RemoveAll(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	root := filepath.Join(tmpDir, "workdir")

	g.Expect(os.MkdirAll(filepath.Join(root, "subdir"), 0o700)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(root, "subdir", "file.txt"), []byte(""), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte(""), 0o644)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure remove all", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "subdir")
		g.Expect(fs.RemoveAll(path)).To(Succeed())
		g.Expect(path).NotTo(BeADirectory())
	})

	t.Run("illegal remove all", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(tmpDir, "file.txt")
		g.Expect(fs.RemoveAll(path)).To(HaveOccurred())
		g.Expect(path).To(BeAnExistingFile())
	})
}

func Test_fsSecure_Open(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(root, "file.txt"), []byte("secure"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("illegal"), 0o644)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure open", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "file.txt")
		f, err := fs.Open(path)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(f).ToNot(BeNil())
		var b bytes.Buffer
		_, err = io.Copy(&b, f)
		g.Expect(err).To(Succeed())
		g.Expect(b.String()).To(Equal("secure"))
	})

	t.Run("illegal open", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(tmpDir, "file.txt")
		f, err := fs.Open(path)
		g.Expect(err).To(HaveOccurred())
		g.Expect(f).To(BeNil())
	})
}

func Test_fsSecure_IsDir(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())
	g.Expect(os.Mkdir(filepath.Join(tmpDir, "illegal"), 0o700)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure is dir", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "")
		g.Expect(fs.IsDir(path)).To(BeTrue())
	})

	t.Run("illegal is dir", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(tmpDir, "illegal")
		g.Expect(fs.IsDir(path)).To(BeFalse())
	})
}

func Test_fsSecure_ReadDir(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(root, "file.txt"), []byte("secure"), 0o644)).To(Succeed())
	g.Expect(os.Mkdir(filepath.Join(tmpDir, "illegal"), 0o700)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(tmpDir, "illegal", "file.txt"), []byte("illegal"), 0o644)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure read dir", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "")
		files, err := fs.ReadDir(path)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(files).To(HaveLen(1))
		g.Expect(files).To(ContainElement("file.txt"))
	})

	t.Run("illegal is dir", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(tmpDir, "illegal")
		files, err := fs.ReadDir(path)
		g.Expect(err).To(HaveOccurred())
		g.Expect(files).To(HaveLen(0))
	})
}

func Test_fsSecure_CleanedAbs(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure cleaned abs", func(t *testing.T) {
		g := NewWithT(t)

		d, f, err := fs.CleanedAbs(filepath.Join(root, "../workdir"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(d).To(Equal(filesys.ConfirmedDir(root)))
		g.Expect(f).To(BeEmpty())
	})

	t.Run("illegal cleaned abs", func(t *testing.T) {
		g := NewWithT(t)

		d, f, err := fs.CleanedAbs(filepath.Join(root, "../../workdir"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(d).To(BeEmpty())
		g.Expect(f).To(BeEmpty())
	})

	t.Run("prefix allowed cleaned abs", func(t *testing.T) {
		g := NewWithT(t)

		allowedPrefix, err := TmpConfirmedDirPrefix()
		g.Expect(err).ToNot(HaveOccurred())

		fs, err := MakeFsOnDiskSecureBuild(root, allowedPrefix)
		g.Expect(err).ToNot(HaveOccurred())

		prefixedDir, err := os.MkdirTemp("", tmpConfirmedDirPrefix)
		g.Expect(err).ToNot(HaveOccurred())
		prefixedDir, err = filepath.EvalSymlinks(prefixedDir)
		g.Expect(err).ToNot(HaveOccurred())
		t.Cleanup(func() { _ = os.RemoveAll(prefixedDir) })

		d, f, err := fs.CleanedAbs(prefixedDir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(d).To(Equal(filesys.ConfirmedDir(prefixedDir)))
		g.Expect(f).To(BeEmpty())
	})
}

func Test_fsSecure_Exists(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure exists", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(fs.Exists(root)).To(BeTrue())
	})

	t.Run("illegal exists", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(fs.Exists(tmpDir)).To(BeFalse())
	})
}

func Test_fsSecure_Glob(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())

	g.Expect(os.WriteFile(filepath.Join(root, "file.txt"), []byte("secure"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("illegal"), 0o644)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	files, err := fs.Glob(filepath.Join(tmpDir, "*/*.txt"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(files).To(ContainElement(filepath.Join(root, "file.txt")))
	g.Expect(files).ToNot(ContainElement(filepath.Join(tmpDir, "file.txt")))
}

func Test_fsSecure_ReadFile(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(root, "file.txt"), []byte("secure"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("illegal"), 0o644)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure read file", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "file.txt")
		b, err := fs.ReadFile(path)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(b).To(Equal([]byte("secure")))
	})

	t.Run("illegal read file", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(tmpDir, "file.txt")
		b, err := fs.ReadFile(path)
		g.Expect(err).To(HaveOccurred())
		g.Expect(b).To(BeNil())
	})
}

func Test_fsSecure_WriteFile(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure write file", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(root, "file.txt")
		data := []byte("secure")
		err := fs.WriteFile(path, data)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(path).To(BeAnExistingFile())
		b, err := fs.ReadFile(path)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(b).To(Equal(data))
	})

	t.Run("illegal write file", func(t *testing.T) {
		g := NewWithT(t)

		path := filepath.Join(tmpDir, "file.txt")
		err := fs.WriteFile(path, []byte("illegal"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(path).ToNot(BeAnExistingFile())
	})
}

func Test_fsSecure_Walk(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	root := filepath.Join(tmpDir, "workdir")
	g.Expect(os.Mkdir(root, 0o700)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(root, "file.txt"), []byte("secure"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("illegal"), 0o644)).To(Succeed())

	fs, err := MakeFsOnDiskSecure(root)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("secure walk", func(t *testing.T) {
		g := NewWithT(t)

		var walkedPaths []string
		walk := func(path string, info os.FileInfo, err error) error {
			walkedPaths = append(walkedPaths, path)
			return nil
		}
		g.Expect(fs.Walk(root, walk)).To(Succeed())
		g.Expect(walkedPaths).To(Equal([]string{root, filepath.Join(root, "file.txt")}))
	})

	t.Run("illegal walk", func(t *testing.T) {
		g := NewWithT(t)

		var walkedPaths []string
		walk := func(path string, info os.FileInfo, err error) error {
			walkedPaths = append(walkedPaths, path)
			return nil
		}
		g.Expect(fs.Walk(tmpDir, walk)).To(HaveOccurred())
		g.Expect(walkedPaths).To(BeEmpty())
	})
}

func Test_isSecurePath(t *testing.T) {
	g := NewWithT(t)

	prefixedDir, err := os.MkdirTemp("", tmpConfirmedDirPrefix)
	g.Expect(err).ToNot(HaveOccurred())
	prefixedDir, err = filepath.EvalSymlinks(prefixedDir)
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() { _ = os.RemoveAll(prefixedDir) })

	allowPrefix, err := TmpConfirmedDirPrefix()
	g.Expect(err).ToNot(HaveOccurred())

	type file struct {
		name    string
		symlink string
	}
	tests := []struct {
		name            string
		fs              filesys.FileSystem
		rootSuffix      string
		files           []file
		path            string
		allowedPrefixes []string
		wantErr         types.GomegaMatcher
	}{
		{
			name:    "secure non existing path",
			fs:      filesys.MakeFsOnDisk(),
			path:    "<root>/filepath",
			wantErr: Succeed(),
		},
		{
			name:       "illegal relative path",
			fs:         filesys.MakeFsOnDisk(),
			rootSuffix: "subdir",
			path:       "../",
			wantErr:    HaveOccurred(),
		},
		{
			name:       "illegal absolute path",
			fs:         filesys.MakeFsOnDisk(),
			rootSuffix: "subdir",
			path:       "<root>",
			wantErr:    HaveOccurred(),
		},
		{
			name:       "relative symlink",
			fs:         filesys.MakeFsOnDisk(),
			rootSuffix: "subdir",
			files: []file{
				{name: "subdir/file.txt"},
				{name: "subdir/subsubdir/symlink", symlink: "../file.txt"},
			},
			path:    "<root>/subdir/subsubdir/symlink",
			wantErr: Succeed(),
		},
		{
			name:       "absolute symlink",
			fs:         filesys.MakeFsOnDisk(),
			rootSuffix: "subdir",
			files: []file{
				{name: "subdir/file.txt"},
				{name: "subdir/subsubdir/symlink", symlink: "<root>/subdir/file.txt"},
			},
			path:    "<root>/subdir/subsubdir/symlink",
			wantErr: Succeed(),
		},
		{
			name:       "illegal relative symlink",
			fs:         filesys.MakeFsOnDisk(),
			rootSuffix: "subdir",
			files: []file{
				{name: "file.txt"},
				{name: "subdir/symlink", symlink: "../file.txt"},
			},
			path:    "<root>/subdir/symlink",
			wantErr: HaveOccurred(),
		},
		{
			name:       "illegal absolute symlink",
			fs:         filesys.MakeFsOnDisk(),
			rootSuffix: "subdir",
			files: []file{
				{name: "file.txt"},
				{name: "subdir/symlink", symlink: "<root>/file.txt"},
			},
			path:    "<root>/subdir/symlink",
			wantErr: HaveOccurred(),
		},
		{
			name:            "allowed prefix",
			fs:              filesys.MakeFsOnDisk(),
			path:            prefixedDir,
			allowedPrefixes: []string{allowPrefix},
			wantErr:         Succeed(),
		},
		{
			name:            "illegal prefix",
			fs:              filesys.MakeFsOnDisk(),
			path:            filepath.Join(os.TempDir(), "illegal-path"),
			allowedPrefixes: []string{allowPrefix},
			wantErr:         HaveOccurred(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			root, err := newTemp()
			g.Expect(err).ToNot(HaveOccurred())
			realRoot := filesys.ConfirmedDir(filepath.Join(root, tt.rootSuffix))
			g.Expect(tt.fs.MkdirAll(realRoot.String())).To(Succeed())
			t.Cleanup(func() {
				g.Expect(tt.fs.RemoveAll(root)).To(Succeed())
			})

			for _, f := range tt.files {
				fPath := filepath.Join(root, f.name)
				dir, base := filepath.Split(fPath)
				g.Expect(tt.fs.MkdirAll(dir)).To(Succeed())

				if symlink := f.symlink; symlink != "" {
					if strings.HasPrefix(symlink, "<root>") {
						symlink = strings.Replace(symlink, "<root>", root, 1)
					}
					g.Expect(os.Symlink(symlink, fPath)).To(Succeed())
					continue
				}

				if base != "" {
					file, err := tt.fs.Create(fPath)
					g.Expect(err).ToNot(HaveOccurred())
					_, err = file.Write([]byte(f.name + " data"))
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(file.Close()).To(Succeed())
				}
			}

			path := tt.path
			if strings.HasPrefix(path, "<root>") {
				path = strings.Replace(path, "<root>", root, 1)
			}

			err = isSecurePath(tt.fs, realRoot, path, tt.allowedPrefixes...)
			g.Expect(err).To(tt.wantErr)
		})
	}
}

func Test_hasOneOfPrefixes(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		prefixes   []string
		want       bool
		wantPrefix string
	}{
		{name: "match", s: "/tmp/kustomize-3828348", prefixes: []string{"/tmp/kustomize-"}, want: true, wantPrefix: "/tmp/kustomize-"},
		{name: "not a match", s: "/tmp/workdir-6845913", prefixes: []string{"/tmp/kustomize-"}, want: false},
		{name: "match list", s: "/tmp/workdir-6845913", prefixes: []string{"/tmp/kustomize-", "/tmp/workdir-"}, want: true, wantPrefix: "/tmp/workdir-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			has, prefix := hasOneOfPrefixes(tt.s, tt.prefixes)
			g.Expect(has).To(Equal(tt.want))
			g.Expect(prefix).To(Equal(tt.wantPrefix))
		})
	}
}

func newTemp() (string, error) {
	tmpDir, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", err
	}
	return filepath.Join(tmpDir, "securefs-"+randStringBytes(5)), nil
}

func randStringBytes(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

func testTempDir(t *testing.T) (string, error) {
	tmpDir := t.TempDir()

	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return "", fmt.Errorf("error evaluating symlink: '%w'", err)
	}

	return tmpDir, err
}
