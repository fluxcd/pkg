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

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/go-git/go-billy/v5"
	. "github.com/onsi/gomega"
)

func TestOpen(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "file: rel same dir",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "test-file",
		},
		{
			name: "file: rel path to above cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "file: rel path to below cwd",
			before: func(dir string) billy.Filesystem {
				os.Mkdir(filepath.Join(dir, "sub"), 0o700)
				os.WriteFile(filepath.Join(dir, "sub/rel-below-cwd"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "sub/rel-below-cwd",
		},
		{
			name: "file: abs inside cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "file: abs outside cwd",
			before: func(dir string) billy.Filesystem {
				return New(dir)
			},
			filename: "/some/path/outside/cwd",
			wantErr:  "/some/path/outside/cwd: no such file or directory",
		},
		{
			name: "symlink: same dir",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				os.WriteFile(target, []byte("anything"), 0o600)
				os.Symlink(target, filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
		},
		{
			name: "symlink: rel outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../outside/cwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  "/outside/cwd: no such file or directory",
		},
		{
			name: "symlink: abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  "/some/path/outside/cwd: no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()
			fs := New(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			fi, err := fs.Open(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(fi).To(BeNil())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(fi).ToNot(BeNil())
			}
		})
	}
}

func Test_Symlink(t *testing.T) {
	if runtime.GOOS == "linux" {
		// The umask value set at OS level can impact this test, so
		// it is set to 0 during the duration of this test and then
		// reverted back to the original value.
		defer syscall.Umask(syscall.Umask(0))
	}

	tests := []struct {
		name        string
		link        string
		target      string
		before      func(dir string) billy.Filesystem
		wantStatErr string
	}{
		{
			name:   "link to abs valid target",
			link:   "symlink",
			target: "/etc/passwd",
		},
		{
			name:   "link to abs inexistent target",
			link:   "symlink",
			target: "/some/random/path",
		},
		{
			name:   "link to rel valid target",
			link:   "symlink",
			target: "../../../../../../../../../etc/passwd",
		},
		{
			name:   "link to rel inexistent target",
			link:   "symlink",
			target: "../../../some/random/path",
		},
		{
			name:   "auto create dir",
			link:   "new-dir/symlink",
			target: "../../../some/random/path",
		},
		{
			name: "keep dir filemode if exists",
			link: "new-dir/symlink",
			before: func(dir string) billy.Filesystem {
				os.Mkdir(filepath.Join(dir, "new-dir"), 0o701)
				return New(dir)
			},
			target: "../../../some/random/path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()
			fs := New(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			// Even if CWD is changed outside of fs the instance,
			// the current working dir must still be observed.
			err := os.Chdir(os.TempDir())
			g.Expect(err).ToNot(HaveOccurred())

			link := filepath.Join(dir, tt.link)

			diBefore, _ := os.Lstat(filepath.Dir(link))

			err = fs.Symlink(tt.target, tt.link)
			g.Expect(err).ToNot(HaveOccurred())

			fi, err := os.Lstat(link)
			if tt.wantStatErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantStatErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(fi).ToNot(BeNil())
			}

			got, err := os.Readlink(link)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(got).To(Equal(tt.target))

			diAfter, err := os.Lstat(filepath.Dir(link))
			g.Expect(err).ToNot(HaveOccurred())

			if diBefore != nil {
				g.Expect(diAfter.Mode()).To(Equal(diBefore.Mode()))
			}
		})
	}
}

func TestTempFile(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	fs := New(dir)

	f, err := fs.TempFile("", "prefix")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(f).ToNot(BeNil())
	g.Expect(f.Name()).To(ContainSubstring(os.TempDir()))

	f, err = fs.TempFile("/above/cwd", "prefix")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(fmt.Sprint(dir, "/above/cwd/prefix")))
	g.Expect(f).To(BeNil())

	f, err = fs.TempFile(os.TempDir(), "prefix")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(filepath.Join(dir, os.TempDir(), "prefix")))
	g.Expect(f).To(BeNil())
}

func TestUnsupportedChroot(t *testing.T) {
	g := NewWithT(t)
	fs := New(t.TempDir())

	f, err := fs.Chroot("")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(Equal(billy.ErrNotSupported))
	g.Expect(f).To(BeNil())
}

func TestRoot(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	fs := New(dir)

	root := fs.Root()
	g.Expect(root).To(Equal(dir))
}

func TestReadLink(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		makeAbs         bool
		expected        string
		makeExpectedAbs bool
		before          func(dir string) billy.Filesystem
		wantErr         string
	}{
		{
			name: "symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			expected: "/etc/passwd",
		},
		{
			name:     "file: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  "path outside working dir",
		},
		{
			name: "symlink: abs symlink pointing outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			expected: "/etc/passwd",
		},
		{
			name: "symlink: dir pointing outside cwd",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "current-dir")
				outside := filepath.Join(dir, "outside-cwd")

				os.Mkdir(cwd, 0o700)
				os.Mkdir(outside, 0o700)

				os.Symlink(outside, filepath.Join(cwd, "symlink"))
				os.WriteFile(filepath.Join(outside, "file"), []byte("anything"), 0o600)

				return New(cwd)
			},
			filename: "current-dir/symlink/file",
			makeAbs:  true,
			wantErr:  "path outside working dir",
		},
		{
			name: "symlink: within cwd + workingDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				cwdAlt := filepath.Join(dir, "symlink-altdir")
				cwdTarget := filepath.Join(dir, "cwd-target")

				os.MkdirAll(cwdTarget, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(cwdTarget, cwdAlt)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(cwdAlt, "symlink-file"))
				return New(cwd)
			},
			filename:        "symlink-file",
			expected:        "cwd-target/file",
			makeExpectedAbs: true,
		},
		{
			name: "symlink: outside cwd + workingDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				outside := filepath.Join(cwd, "symlink-outside")
				cwdTarget := filepath.Join(dir, "cwd-target")
				outsideDir := filepath.Join(dir, "outside")

				os.Mkdir(cwdTarget, 0o700)
				os.Mkdir(outsideDir, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(outsideDir, outside)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(outside, "symlink-file"))
				return New(cwd)
			},
			filename: "symlink-outside/symlink-file",
			wantErr:  "path outside working dir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()
			fs := New(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			expected := tt.expected
			if tt.makeExpectedAbs {
				expected = filepath.Join(dir, expected)
			}

			got, err := fs.Readlink(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(got).To(BeEmpty())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(got).To(Equal(expected))
			}
		})
	}
}

func TestLstat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "rel symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
		},
		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  true,
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  false,
		},
		{
			name: "symlink: within cwd + workingDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				cwdAlt := filepath.Join(dir, "symlink-altdir")
				cwdTarget := filepath.Join(dir, "cwd-target")

				os.MkdirAll(cwdTarget, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(cwdTarget, cwdAlt)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(cwdAlt, "symlink-file"))
				return New(cwd)
			},
			filename: "symlink-file",
			makeAbs:  false,
		},
		{
			name: "symlink: outside cwd + workingDir symlink",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "symlink-dir")
				outside := filepath.Join(cwd, "symlink-outside")
				cwdTarget := filepath.Join(dir, "cwd-target")
				outsideDir := filepath.Join(dir, "outside")

				os.Mkdir(cwdTarget, 0o700)
				os.Mkdir(outsideDir, 0o700)

				os.WriteFile(filepath.Join(cwdTarget, "file"), []byte{}, 0o600)
				os.Symlink(cwdTarget, cwd)
				os.Symlink(outsideDir, outside)
				os.Symlink(filepath.Join(cwdTarget, "file"), filepath.Join(outside, "symlink-file"))
				return New(cwd)
			},
			filename: "symlink-outside/symlink-file",
			makeAbs:  false,
			wantErr:  "path outside working dir",
		},
		{
			name:     "path: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  "path outside working dir",
		},
		{
			name:     "path: abs pointing outside cwd",
			filename: "/etc/passwd",
			wantErr:  "path outside working dir",
		},
		{
			name: "file: rel",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "test-file",
		},
		{
			name: "file: abs",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()
			fs := New(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}
			fi, err := fs.Lstat(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(fi).To(BeNil())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(fi).ToNot(BeNil())
				g.Expect(fi.Name()).To(Equal(filepath.Base(tt.filename)))
			}
		})
	}
}

func TestStat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "rel symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			wantErr:  "/001/etc/passwd: no such file or directory",
		},
		{
			name: "rel symlink: pointing to rel path above cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			wantErr:  "/001/etc/passwd: no such file or directory",
		},

		{
			name: "abs symlink: pointing to abs outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  true,
			wantErr:  "/001/etc/passwd: no such file or directory",
		},
		{
			name: "abs symlink: pointing to rel outside cwd",
			before: func(dir string) billy.Filesystem {
				os.Symlink("../../../../../../../../etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  false,
			wantErr:  "/001/etc/passwd: no such file or directory",
		},
		{
			name:     "path: rel pointing to abs above cwd",
			filename: "../../file",
			wantErr:  "/001/file: no such file or directory",
		},
		{
			name:     "path: abs pointing outside cwd",
			filename: "/etc/passwd",
			wantErr:  "/001/etc/passwd: no such file or directory",
		},
		{
			name: "rel file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "test-file",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "test-file",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()
			fs := New(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			fi, err := fs.Stat(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(fi).To(BeNil())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(fi).ToNot(BeNil())
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name:     "path: rel pointing outside cwd w forward slash",
			filename: "/some/path/outside/cwd",
			wantErr:  "/001/some/path/outside/cwd: no such file or directory",
		},
		{
			name:     "path: rel pointing outside cwd",
			filename: "../../../../path/outside/cwd",
			wantErr:  "/001/path/outside/cwd: no such file or directory",
		},
		{
			name: "parent with children",
			before: func(dir string) billy.Filesystem {
				os.MkdirAll(filepath.Join(dir, "parent/children"), 0o600)
				return New(dir)
			},
			filename: "parent",
		},
		{
			name: "inexistent dir",
			before: func(dir string) billy.Filesystem {
				return New(dir)
			},
			filename: "inexistent",
			wantErr:  "inexistent: no such file or directory",
		},
		{
			name: "same dir file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "test-file",
		},
		{
			name: "symlink: same dir",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				os.WriteFile(target, []byte("anything"), 0o600)
				os.Symlink(target, filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
		},
		{
			name: "rel path to file above cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "abs symlink: pointing outside is forced to descend",
			before: func(dir string) billy.Filesystem {
				cwd := filepath.Join(dir, "current-dir")
				outsideFile := filepath.Join(cwd, dir, "outside-cwd/file")

				os.Mkdir(cwd, 0o700)
				os.MkdirAll(filepath.Dir(outsideFile), 0o700)
				os.WriteFile(outsideFile, []byte("anything"), 0o600)
				os.Symlink(outsideFile, filepath.Join(cwd, "remove-abs-symlink"))
				return New(cwd)
			},
			filename: "current-dir/remove-abs-symlink",
			wantErr:  "/001/current-dir/current-dir/remove-abs-symlink: no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()
			fs := New(dir)

			if tt.before != nil {
				fs = tt.before(dir)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			err := fs.Remove(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		makeAbs  bool
		before   func(dir string) billy.Filesystem
		wantErr  string
	}{
		{
			name: "parent with children",
			before: func(dir string) billy.Filesystem {
				os.MkdirAll(filepath.Join(dir, "parent/children"), 0o600)
				return New(dir)
			},
			filename: "parent",
		},
		{
			name:     "inexistent dir",
			filename: "inexistent",
		},
		{
			name: "same dir file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "test-file",
		},
		{
			name: "same dir symlink",
			before: func(dir string) billy.Filesystem {
				target := filepath.Join(dir, "target-file")
				os.WriteFile(target, []byte("anything"), 0o600)
				os.Symlink(target, filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
		},
		{
			name: "rel path to file above cwd",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "rel-above-cwd"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "../../rel-above-cwd",
		},
		{
			name: "abs file",
			before: func(dir string) billy.Filesystem {
				os.WriteFile(filepath.Join(dir, "abs-test-file"), []byte("anything"), 0o600)
				return New(dir)
			},
			filename: "abs-test-file",
			makeAbs:  true,
		},
		{
			name: "abs symlink",
			before: func(dir string) billy.Filesystem {
				os.Symlink("/etc/passwd", filepath.Join(dir, "symlink"))
				return New(dir)
			},
			filename: "symlink",
			makeAbs:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()
			fs := New(dir).(*OS)

			if tt.before != nil {
				fs = tt.before(dir).(*OS)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(dir, filename)
			}

			err := fs.RemoveAll(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		elems  []string
		wanted string
	}{
		{
			elems:  []string{},
			wanted: "",
		},
		{
			elems:  []string{"/a", "b", "c"},
			wanted: "/a/b/c",
		},
		{
			elems:  []string{"/a", "b/c"},
			wanted: "/a/b/c",
		},
		{
			elems:  []string{"/a", ""},
			wanted: "/a",
		},
		{
			elems:  []string{"/a", "/", "b"},
			wanted: "/a/b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.wanted, func(t *testing.T) {
			g := NewWithT(t)
			fs := New(t.TempDir())

			got := fs.Join(tt.elems...)
			g.Expect(got).To(Equal(tt.wanted))
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		name            string
		cwd             string
		filename        string
		makeAbs         bool
		expected        string
		makeExpectedAbs bool
		wantErr         string
		before          func(dir string)
	}{
		{
			name:     "path: same dir rel file",
			cwd:      "/working/dir",
			filename: "./file",
			expected: "/working/dir/file",
		},
		{
			name:     "path: descending rel file",
			cwd:      "/working/dir",
			filename: "file",
			expected: "/working/dir/file",
		},
		{
			name:     "path: ascending rel file 1",
			cwd:      "/working/dir",
			filename: "../file",
			expected: "/working/dir/file",
		},
		{
			name:     "path: ascending rel file 2",
			cwd:      "/working/dir",
			filename: "../../file",
			expected: "/working/dir/file",
		},
		{
			name:     "path: ascending rel file 3",
			cwd:      "/working/dir",
			filename: "/../../file",
			expected: "/working/dir/file",
		},
		{
			name:     "path: abs file within cwd",
			cwd:      "/working/dir",
			filename: "/working/dir/abs-file",
			expected: "/working/dir/abs-file",
		},
		{
			name:     "path: abs file within cwd",
			cwd:      "/working/dir",
			filename: "/outside/dir/abs-file",
			expected: "/working/dir/outside/dir/abs-file",
		},
		{
			name:            "abs symlink: within cwd w abs descending target",
			filename:        "ln-cwd-cwd",
			makeAbs:         true,
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink(filepath.Join(dir, "within-cwd"), filepath.Join(dir, "ln-cwd-cwd"))
			},
		},
		{
			name:            "abs symlink: within cwd w rel descending target",
			filename:        "ln-rel-cwd-cwd",
			makeAbs:         true,
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("within-cwd", filepath.Join(dir, "ln-rel-cwd-cwd"))
			},
		},
		{
			name:            "abs symlink: within cwd w abs ascending target",
			filename:        "ln-cwd-up",
			makeAbs:         true,
			expected:        "/some/outside/dir",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("/some/outside/dir", filepath.Join(dir, "ln-cwd-up"))
			},
		},
		{
			name:            "abs symlink within cwd w rel ascending target",
			filename:        "ln-rel-cwd-up",
			makeAbs:         true,
			expected:        "outside-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("../../outside-cwd", filepath.Join(dir, "ln-rel-cwd-up"))
			},
		},
		{
			name:            "rel symlink: within cwd w abs descending target",
			filename:        "ln-cwd-cwd",
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink(filepath.Join(dir, "within-cwd"), filepath.Join(dir, "ln-cwd-cwd"))
			},
		},
		{
			name:            "rel symlink: within cwd w rel descending target",
			filename:        "ln-rel-cwd-cwd2",
			expected:        "within-cwd",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("within-cwd", filepath.Join(dir, "ln-rel-cwd-cwd2"))
			},
		},
		{
			name:            "rel symlink: within cwd w abs ascending target",
			filename:        "ln-cwd-up2",
			expected:        "/outside/path/up",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("/outside/path/up", filepath.Join(dir, "ln-cwd-up2"))
			},
		},
		{
			name:            "rel symlink: within cwd w rel ascending target",
			filename:        "ln-rel-cwd-up2",
			expected:        "outside",
			makeExpectedAbs: true,
			before: func(dir string) {
				os.Symlink("../../../../outside", filepath.Join(dir, "ln-rel-cwd-up2"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cwd := tt.cwd
			if cwd == "" {
				cwd = t.TempDir()
			}

			fs := New(cwd).(*OS)
			if tt.before != nil {
				tt.before(cwd)
			}

			filename := tt.filename
			if tt.makeAbs {
				filename = filepath.Join(cwd, filename)
			}

			expected := tt.expected
			if tt.makeExpectedAbs {
				expected = filepath.Join(cwd, expected)
			}

			got, err := fs.abs(filename)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			g.Expect(got).To(Equal(expected))
		})
	}
}

func TestReadDir(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	fs := New(dir)

	f, err := os.Create(filepath.Join(dir, "file1"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(f).ToNot(BeNil())

	f, err = os.Create(filepath.Join(dir, "file2"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(f).ToNot(BeNil())

	dirs, err := fs.ReadDir(dir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(dirs).ToNot(BeNil())
	g.Expect(dirs).To(HaveLen(2))

	dirs, err = fs.ReadDir(".")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(dirs).ToNot(BeNil())
	g.Expect(dirs).To(HaveLen(2))

	os.Symlink("/some/path/outside/cwd", filepath.Join(dir, "symlink"))
	dirs, err = fs.ReadDir("symlink")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(dir + "/some/path/outside/cwd: no such file or directory"))
	g.Expect(dirs).To(BeNil())
}

func TestMkdirAll(t *testing.T) {
	g := NewWithT(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	target := "abc"
	targetAbs := filepath.Join(cwd, target)
	fs := New(cwd)

	// Even if CWD is changed outside of fs the instance,
	// the current working dir must still be observed.
	err := os.Chdir(os.TempDir())
	g.Expect(err).ToNot(HaveOccurred())

	err = fs.MkdirAll(target, 0o700)
	g.Expect(err).ToNot(HaveOccurred())

	fi, err := os.Stat(targetAbs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fi).ToNot(BeNil())

	os.Mkdir(filepath.Join(root, "outside"), 0o700)
	os.Symlink(filepath.Join(root, "outside"), filepath.Join(cwd, "symlink"))
	err = fs.MkdirAll(filepath.Join(cwd, "symlink/new-dir"), 0o700)
	g.Expect(err).ToNot(HaveOccurred())

	mustExist(filepath.Join(cwd, filepath.Join(cwd, "../outside/new-dir")))
}

func TestRename(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	fs := New(dir)

	oldFile := "old-file"
	newFile := "newdir/newfile"

	// Even if CWD is changed outside of fs the instance,
	// the current working dir must still be observed.
	err := os.Chdir(os.TempDir())
	g.Expect(err).ToNot(HaveOccurred())

	_, err = fs.Create(oldFile)
	g.Expect(err).ToNot(HaveOccurred())

	err = fs.Rename(oldFile, newFile)
	g.Expect(err).ToNot(HaveOccurred())

	fi, err := os.Stat(filepath.Join(dir, newFile))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fi).ToNot(BeNil())

	err = fs.Rename("/tmp/outside/cwd/file1", newFile)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("newdir/newfile: no such file or directory"))

	err = fs.Rename(oldFile, "/tmp/outside/cwd/file2")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("outside/cwd/file2: no such file or directory"))
}

func mustExist(filename string) {
	fi, err := os.Stat(filename)
	if err != nil || fi == nil {
		panic(fmt.Sprintf("file %s should exist", filename))
	}
}
