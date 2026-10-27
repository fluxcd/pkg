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

package gogit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	extgogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/fluxcd/pkg/gittestserver"
)

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestWorktreeFS(t *testing.T, base string) *boundWorktreeFS {
	t.Helper()
	wfs, err := newBoundWorktreeFS(base)
	if err != nil {
		t.Fatal(err)
	}
	return wfs
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func symlinkTest(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks not supported: %v", err)
		}
		t.Fatal(err)
	}
}

func lexists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func TestBoundWorktreeFS_Remove(t *testing.T) {
	removers := map[string]func(billy.Filesystem, string) error{
		"Remove":         func(f billy.Filesystem, p string) error { return f.Remove(p) },
		"RemoveAll":      func(f billy.Filesystem, p string) error { return f.(*boundWorktreeFS).RemoveAll(p) },
		"util.RemoveAll": func(f billy.Filesystem, p string) error { return util.RemoveAll(f, p) },
		// go-git hides RemoveAll, so util.RemoveAll walks the tree with Remove.
		"util.RemoveAll walker": func(f billy.Filesystem, p string) error {
			return util.RemoveAll(struct{ billy.Filesystem }{f}, p)
		},
	}

	tests := []struct {
		name string
		// setup returns the name to remove.
		setup   func(t *testing.T, base, outside string) string
		only    []string
		wantErr func(error) bool
		// gone and kept are relative to base, or absolute.
		gone []string
		kept []string
	}{
		{
			name: "regular file",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "f"), "x")
				return "f"
			},
			gone: []string{"f"},
		},
		{
			name: "symlink to file keeps target",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "t"), "x")
				symlinkTest(t, "t", filepath.Join(base, "l"))
				return "l"
			},
			gone: []string{"l"},
			kept: []string{"t"},
		},
		{
			name: "dangling symlink",
			setup: func(t *testing.T, base, _ string) string {
				symlinkTest(t, "missing", filepath.Join(base, "l"))
				return "l"
			},
			gone: []string{"l"},
		},
		{
			name: "symlink to dir keeps target content",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "d", "f"), "x")
				symlinkTest(t, "d", filepath.Join(base, "l"))
				return "l"
			},
			gone: []string{"l"},
			kept: []string{"d/f"},
		},
		{
			name: "symlink to root",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "f"), "x")
				symlinkTest(t, ".", filepath.Join(base, "l"))
				return "l"
			},
			gone: []string{"l"},
			kept: []string{"f"},
		},
		{
			name: "absolute symlink keeps in-root target",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "t"), "x")
				symlinkTest(t, "/t", filepath.Join(base, "l"))
				return "l"
			},
			gone: []string{"l"},
			kept: []string{"t"},
		},
		{
			name: "symlink to outside file keeps target",
			setup: func(t *testing.T, base, outside string) string {
				writeTestFile(t, filepath.Join(outside, "v"), "x")
				symlinkTest(t, filepath.Join(outside, "v"), filepath.Join(base, "l"))
				return "l"
			},
			gone: []string{"l"},
			kept: []string{"OUTSIDE/v"},
		},
		{
			name: "symlink in nested dir",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "a", "t"), "x")
				symlinkTest(t, "../t", filepath.Join(base, "a", "b", "l"))
				return "a/b/l"
			},
			gone: []string{"a/b/l"},
			kept: []string{"a/t"},
		},
		{
			name: "dot prefix",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "t"), "x")
				symlinkTest(t, "t", filepath.Join(base, "l"))
				return "./l"
			},
			gone: []string{"l"},
			kept: []string{"t"},
		},
		{
			name: "absolute path under root",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "t"), "x")
				symlinkTest(t, "t", filepath.Join(base, "l"))
				return filepath.Join(base, "l")
			},
			gone: []string{"l"},
			kept: []string{"t"},
		},
		{
			name: "duplicated root prefix",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "f"), "x")
				return strings.TrimLeft(filepath.Join(base, "f"), string(filepath.Separator))
			},
			gone: []string{"f"},
		},
		{
			name: "absolute path outside root is placed under root",
			setup: func(t *testing.T, base, outside string) string {
				writeTestFile(t, filepath.Join(outside, "v"), "x")
				writeTestFile(t, filepath.Join(base, outside, "v"), "x")
				return filepath.Join(outside, "v")
			},
			gone: []string{filepath.Join("BASE", "OUTSIDE", "v")},
			kept: []string{"OUTSIDE/v"},
		},
		{
			name: "dot-dot cannot escape root",
			setup: func(t *testing.T, base, outside string) string {
				writeTestFile(t, filepath.Join(outside, "v"), "x")
				writeTestFile(t, filepath.Join(base, "v"), "x")
				rel, err := filepath.Rel(base, filepath.Join(outside, "v"))
				if err != nil {
					t.Fatal(err)
				}
				return rel
			},
			wantErr: func(err error) bool { return err == nil || os.IsNotExist(err) },
			kept:    []string{"OUTSIDE/v"},
		},
		{
			name: "dot-dot is clamped at root",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "f"), "x")
				return "../../f"
			},
			gone: []string{"f"},
		},
		{
			name: "intermediate symlink escaping root is clamped",
			setup: func(t *testing.T, base, outside string) string {
				writeTestFile(t, filepath.Join(outside, "v"), "x")
				symlinkTest(t, outside, filepath.Join(base, "out"))
				return "out/v"
			},
			wantErr: func(err error) bool { return err == nil || os.IsNotExist(err) },
			kept:    []string{"OUTSIDE/v"},
		},
		{
			name: "intermediate symlink within root",
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "d", "f"), "x")
				symlinkTest(t, "d", filepath.Join(base, "l"))
				return "l/f"
			},
			gone: []string{"d/f"},
		},
		{
			name: "missing path with Remove",
			only: []string{"Remove"},
			setup: func(t *testing.T, _, _ string) string {
				return "missing"
			},
			wantErr: os.IsNotExist,
		},
		{
			name: "missing path with RemoveAll",
			only: []string{"RemoveAll", "util.RemoveAll", "util.RemoveAll walker"},
			setup: func(t *testing.T, _, _ string) string {
				return "missing"
			},
		},
		{
			name: "non-empty dir with Remove",
			only: []string{"Remove"},
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "d", "f"), "x")
				return "d"
			},
			wantErr: func(err error) bool { return err != nil },
			kept:    []string{"d/f"},
		},
		{
			name: "non-empty dir with RemoveAll",
			only: []string{"RemoveAll", "util.RemoveAll", "util.RemoveAll walker"},
			setup: func(t *testing.T, base, _ string) string {
				writeTestFile(t, filepath.Join(base, "d", "e", "f"), "x")
				writeTestFile(t, filepath.Join(base, "t"), "x")
				symlinkTest(t, "../../t", filepath.Join(base, "d", "e", "l"))
				return "d"
			},
			gone: []string{"d"},
			kept: []string{"t"},
		},
	}

	for _, tt := range tests {
		for rname, remove := range removers {
			if len(tt.only) > 0 && !contains(tt.only, rname) {
				continue
			}
			t.Run(tt.name+"/"+rname, func(t *testing.T) {
				g := NewWithT(t)
				base := resolvedTempDir(t)
				outside := resolvedTempDir(t)
				name := tt.setup(t, base, outside)

				err := remove(newTestWorktreeFS(t, base), name)
				if tt.wantErr != nil {
					g.Expect(tt.wantErr(err)).To(BeTrue(), "unexpected error: %v", err)
				} else {
					g.Expect(err).ToNot(HaveOccurred())
				}

				resolve := func(p string) string {
					p = strings.Replace(p, "BASE", base, 1)
					p = strings.Replace(p, "OUTSIDE", outside, 1)
					if filepath.IsAbs(p) {
						return p
					}
					return filepath.Join(base, p)
				}
				for _, p := range tt.gone {
					g.Expect(lexists(t, resolve(p))).To(BeFalse(), "%s should be removed", p)
				}
				for _, p := range tt.kept {
					g.Expect(lexists(t, resolve(p))).To(BeTrue(), "%s should be kept", p)
				}
			})
		}
	}
}

func TestBoundWorktreeFS_RemoveRoot(t *testing.T) {
	g := NewWithT(t)
	base := resolvedTempDir(t)
	writeTestFile(t, filepath.Join(base, "f"), "x")
	wfs := newTestWorktreeFS(t, base)

	for _, name := range []string{"", ".", "./", "..", "../", "/", base, base + "/", "a/.."} {
		g.Expect(wfs.Remove(name)).To(MatchError(osfs.ErrBaseDirCannotBeRemoved), "Remove(%q)", name)
		g.Expect(wfs.RemoveAll(name)).To(MatchError(osfs.ErrBaseDirCannotBeRemoved), "RemoveAll(%q)", name)
	}
	g.Expect(lexists(t, filepath.Join(base, "f"))).To(BeTrue())
}

// TestBoundWorktreeFS_parity compares the wrapper with BoundOS on regular
// files, then checks that the same entries are removed when they are
// symlinks, leaving their targets in place.
func TestBoundWorktreeFS_parity(t *testing.T) {
	files := []string{"f", "d/f", "d/e/f"}
	fixture := func(t *testing.T, links bool) string {
		base := resolvedTempDir(t)
		for i, f := range files {
			if links {
				target := filepath.Join(base, "tgt", fmt.Sprint(i))
				writeTestFile(t, target, "x")
				rel, err := filepath.Rel(filepath.Dir(filepath.Join(base, f)), target)
				if err != nil {
					t.Fatal(err)
				}
				symlinkTest(t, rel, filepath.Join(base, f))
			} else {
				writeTestFile(t, filepath.Join(base, f), "x")
			}
		}
		if err := os.MkdirAll(filepath.Join(base, "empty"), 0o755); err != nil {
			t.Fatal(err)
		}
		symlinkTest(t, "d/e", filepath.Join(base, "s"))
		return base
	}
	trim := func(p string) string { return strings.TrimLeft(p, string(filepath.Separator)) }

	names := map[string]func(base string) string{
		"file":                    func(string) string { return "f" },
		"nested file":             func(string) string { return "d/f" },
		"empty dir":               func(string) string { return "empty" },
		"non-empty dir":           func(string) string { return "d" },
		"missing":                 func(string) string { return "missing" },
		"missing parent":          func(string) string { return "missing/f" },
		"dot prefix":              func(string) string { return "./f" },
		"dot suffix":              func(string) string { return "d/f/." },
		"trailing separator":      func(string) string { return "d/f/" },
		"dot-dot":                 func(string) string { return "../f" },
		"dot-dot nested":          func(string) string { return "../../d/f" },
		"dot-dot to root":         func(string) string { return "d/.." },
		"symlink then dot-dot":    func(string) string { return "s/../f" },
		"through symlinked dir":   func(string) string { return "s/f" },
		"absolute under root":     func(b string) string { return filepath.Join(b, "f") },
		"absolute root":           func(b string) string { return b },
		"absolute root separator": func(b string) string { return b + string(filepath.Separator) },
		"absolute root unclean":   func(b string) string { return filepath.Join(b, "x") + "/../../" + filepath.Base(b) },
		"relative root unclean":   func(string) string { return "d/../." },
		"absolute outside root":   func(string) string { return "/f" },
		"duplicated root":         func(b string) string { return filepath.Join(trim(b), "f") },
		"duplicated root twice":   func(b string) string { return filepath.Join(trim(b), trim(b), "f") },
		"duplicated root nested":  func(b string) string { return filepath.Join(trim(b), "d", "f") },
		"empty":                   func(string) string { return "" },
		"dot":                     func(string) string { return "." },
		"dot slash":               func(string) string { return "./" },
		"nul byte":                func(string) string { return "bad\x00/../f" },
	}

	snapshot := func(t *testing.T, base string) map[string]bool {
		got := map[string]bool{}
		err := filepath.WalkDir(base, func(p string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			if rel != "tgt" && !strings.HasPrefix(rel, "tgt"+string(filepath.Separator)) {
				got[rel] = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	for name, nameFn := range names {
		for _, method := range []string{"Remove", "RemoveAll"} {
			t.Run(name+"/"+method, func(t *testing.T) {
				g := NewWithT(t)
				run := func(f billy.Filesystem, base string) (error, map[string]bool) {
					var err error
					if method == "Remove" {
						err = f.Remove(nameFn(base))
					} else {
						err = f.(interface{ RemoveAll(string) error }).RemoveAll(nameFn(base))
					}
					return err, snapshot(t, base)
				}

				wantBase := fixture(t, false)
				wantErr, wantFiles := run(osfs.New(wantBase, osfs.WithBoundOS()), wantBase)

				gotBase := fixture(t, false)
				gotErr, gotFiles := run(newTestWorktreeFS(t, gotBase), gotBase)
				g.Expect(errorKind(gotErr)).To(Equal(errorKind(wantErr)), "got %v, want %v", gotErr, wantErr)
				g.Expect(gotFiles).To(Equal(wantFiles))

				linkBase := fixture(t, true)
				linkErr, linkFiles := run(newTestWorktreeFS(t, linkBase), linkBase)
				g.Expect(errorKind(linkErr)).To(Equal(errorKind(wantErr)), "got %v, want %v", linkErr, wantErr)
				g.Expect(linkFiles).To(Equal(wantFiles))
				for i := range files {
					g.Expect(filepath.Join(linkBase, "tgt", fmt.Sprint(i))).To(BeARegularFile())
				}
			})
		}
	}
}

// errorKind classifies err by sentinel and errno.
func errorKind(err error) string {
	if err == nil {
		return "nil"
	}
	if errors.Is(err, osfs.ErrBaseDirCannotBeRemoved) {
		return "base dir"
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno.Error()
	}
	return "other: " + err.Error()
}

func TestBoundWorktreeFS_symlinkedRoot(t *testing.T) {
	g := NewWithT(t)
	dir := resolvedTempDir(t)
	real := filepath.Join(dir, "real")
	writeTestFile(t, filepath.Join(real, "t"), "x")
	symlinkTest(t, "t", filepath.Join(real, "l"))
	alias := filepath.Join(dir, "alias")
	symlinkTest(t, "real", alias)

	wfs := newTestWorktreeFS(t, alias)
	for _, name := range []string{"", ".", "./", alias, alias + "/"} {
		g.Expect(wfs.Remove(name)).To(MatchError(osfs.ErrBaseDirCannotBeRemoved), "Remove(%q)", name)
		g.Expect(wfs.RemoveAll(name)).To(MatchError(osfs.ErrBaseDirCannotBeRemoved), "RemoveAll(%q)", name)
	}
	g.Expect(lexists(t, alias)).To(BeTrue())

	g.Expect(wfs.Remove("l")).To(Succeed())
	g.Expect(lexists(t, filepath.Join(real, "l"))).To(BeFalse())
	g.Expect(filepath.Join(real, "t")).To(BeARegularFile())
}

func TestBoundWorktreeFS_missingRoot(t *testing.T) {
	g := NewWithT(t)
	wfs := newTestWorktreeFS(t, filepath.Join(resolvedTempDir(t), "missing"))
	g.Expect(os.IsNotExist(wfs.Remove("f"))).To(BeTrue())
	g.Expect(wfs.RemoveAll("f")).To(Succeed())
}

func TestBoundWorktreeFS_Chroot(t *testing.T) {
	g := NewWithT(t)
	base := resolvedTempDir(t)
	writeTestFile(t, filepath.Join(base, "sub", "t"), "x")
	symlinkTest(t, "t", filepath.Join(base, "sub", "l"))

	c, err := newTestWorktreeFS(t, base).Chroot("sub")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(c).To(BeAssignableToTypeOf(&boundWorktreeFS{}))
	g.Expect(c.Root()).To(Equal(filepath.Join(base, "sub")))

	g.Expect(c.Remove("l")).To(Succeed())
	g.Expect(lexists(t, filepath.Join(base, "sub", "l"))).To(BeFalse())
	g.Expect(lexists(t, filepath.Join(base, "sub", "t"))).To(BeTrue())
}

type testEntry struct {
	content string
	link    string
}

type testTree map[string]testEntry

func TestSwitchBranch_symlinks(t *testing.T) {
	tests := []struct {
		name   string
		from   testTree
		to     testTree
		sparse []string
	}{
		{
			// fluxcd/image-automation-controller#1037
			name: "symlink to file removed earlier in the checkout",
			from: testTree{
				"d/_stacks/config.yaml": {content: "new"},
				"d/app/config.yaml":     {link: "../_stacks/config.yaml"},
			},
			to: testTree{"d/app/config.yaml": {content: "orig"}},
		},
		{
			name: "symlink to file removed earlier in the checkout with sparse checkout",
			from: testTree{
				"d/_stacks/config.yaml": {content: "new"},
				"d/app/config.yaml":     {link: "../_stacks/config.yaml"},
				"other/f":               {content: "x"},
			},
			to:     testTree{"d/app/config.yaml": {content: "orig"}, "other/f": {content: "x"}},
			sparse: []string{"d"},
		},
		{
			name: "symlink to tracked file replaced by file",
			from: testTree{"a.txt": {content: "A"}, "link": {link: "a.txt"}},
			to:   testTree{"a.txt": {content: "A"}, "link": {content: "L"}},
		},
		{
			name: "symlink to tracked file deleted",
			from: testTree{"a.txt": {content: "A"}, "link": {link: "a.txt"}},
			to:   testTree{"a.txt": {content: "A"}},
		},
		{
			name: "absolute symlink to tracked file deleted",
			from: testTree{"a.txt": {content: "A"}, "link": {link: "/a.txt"}},
			to:   testTree{"a.txt": {content: "A"}},
		},
		{
			name: "symlink to git config replaced by file",
			from: testTree{"s": {link: ".git/config"}},
			to:   testTree{"s": {content: "S"}},
		},
		{
			name: "symlink to git config deleted",
			from: testTree{"keep": {content: "k"}, "s": {link: ".git/config"}},
			to:   testTree{"keep": {content: "k"}},
		},
		{
			name: "symlink to git dir replaced by dir",
			from: testTree{"s": {link: ".git"}},
			to:   testTree{"s/config": {content: "S"}},
		},
		{
			name: "symlink to root replaced by file",
			from: testTree{"s": {link: "."}},
			to:   testTree{"s": {content: "S"}},
		},
		{
			name: "symlink to tracked dir deleted",
			from: testTree{"d/f": {content: "F"}, "s": {link: "d"}},
			to:   testTree{"d/f": {content: "F"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			server, err := gittestserver.NewTempGitServer()
			g.Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(server.Root())
			g.Expect(server.InitRepo("../testdata/git/repo", git.DefaultBranch, "test.git")).To(Succeed())
			g.Expect(server.StartHTTP()).To(Succeed())
			defer server.StopHTTP()

			remote, err := extgogit.PlainOpen(filepath.Join(server.Root(), "test.git"))
			g.Expect(err).ToNot(HaveOccurred())
			setBranch(t, remote.Storer, "from", tt.from)
			setBranch(t, remote.Storer, "to", tt.to)

			// WithDiskStorage resolves symlinks in the path.
			path := filepath.Join(t.TempDir(), "alias")
			symlinkTest(t, t.TempDir(), path)
			ggc, err := NewClient(path, &git.AuthOptions{Transport: git.HTTP},
				WithDiskStorage(), WithSingleBranch(false))
			g.Expect(err).ToNot(HaveOccurred())

			_, err = ggc.Clone(context.TODO(), server.HTTPAddress()+"/test.git", repository.CloneConfig{
				CheckoutStrategy:          repository.CheckoutStrategy{Branch: "from"},
				SparseCheckoutDirectories: tt.sparse,
			})
			g.Expect(err).ToNot(HaveOccurred())

			resolved, err := filepath.EvalSymlinks(path)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(worktreeContent(t, resolved)).To(Equal(sparseTree(tt.from, tt.sparse)))

			g.Expect(ggc.SwitchBranch(context.TODO(), "to")).To(Succeed())

			g.Expect(filepath.Join(resolved, ".git", "config")).To(BeARegularFile())
			g.Expect(worktreeContent(t, resolved)).To(Equal(sparseTree(tt.to, tt.sparse)))

			if len(tt.sparse) == 0 {
				clean, err := ggc.IsClean()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(clean).To(BeTrue())
			}
		})
	}
}

// TestClone_commitRemovesSymlink covers fluxcd/source-controller#1921.
func TestClone_commitRemovesSymlink(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())
	g.Expect(server.InitRepo("../testdata/git/repo", git.DefaultBranch, "test.git")).To(Succeed())
	g.Expect(server.StartHTTP()).To(Succeed())
	defer server.StopHTTP()

	remote, err := extgogit.PlainOpen(filepath.Join(server.Root(), "test.git"))
	g.Expect(err).ToNot(HaveOccurred())
	setBranch(t, remote.Storer, git.DefaultBranch, testTree{
		"target.txt": {content: "target"},
		"link.txt":   {link: "target.txt"},
	})
	want := testTree{"target.txt": {content: "target"}}
	commit := setBranch(t, remote.Storer, "delete-symlink", want)

	path := t.TempDir()
	ggc, err := NewClient(path, &git.AuthOptions{Transport: git.HTTP}, WithDiskStorage())
	g.Expect(err).ToNot(HaveOccurred())

	cc, err := ggc.Clone(context.TODO(), server.HTTPAddress()+"/test.git", repository.CloneConfig{
		CheckoutStrategy: repository.CheckoutStrategy{Commit: commit.String()},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cc.Hash.String()).To(Equal(commit.String()))

	path, err = filepath.EvalSymlinks(path)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(worktreeContent(t, path)).To(Equal(want))
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sparseTree(tree testTree, dirs []string) testTree {
	if len(dirs) == 0 {
		return tree
	}
	out := testTree{}
	for p, e := range tree {
		if inSparse(p, dirs) {
			out[p] = e
		}
	}
	return out
}

func inSparse(p string, dirs []string) bool {
	for _, d := range dirs {
		if strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	return false
}

func worktreeContent(t *testing.T, path string) testTree {
	t.Helper()
	got := testTree{}
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		if rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			got[rel] = testEntry{link: target}
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		got[rel] = testEntry{content: string(b)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func setBranch(t *testing.T, st storer.Storer, branch string, tree testTree) plumbing.Hash {
	t.Helper()
	sig := object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}
	commit := &object.Commit{
		Author:    sig,
		Committer: sig,
		Message:   branch,
		TreeHash:  writeTestTree(t, st, tree),
	}
	obj := st.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		t.Fatal(err)
	}
	hash, err := st.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), hash)
	if err := st.SetReference(ref); err != nil {
		t.Fatal(err)
	}
	return hash
}

func writeTestTree(t *testing.T, st storer.Storer, tree testTree) plumbing.Hash {
	t.Helper()
	subtrees := map[string]testTree{}
	var entries []object.TreeEntry
	for p, e := range tree {
		if dir, rest, ok := strings.Cut(p, "/"); ok {
			if subtrees[dir] == nil {
				subtrees[dir] = testTree{}
			}
			subtrees[dir][rest] = e
			continue
		}
		mode, data := filemode.Regular, e.content
		if e.link != "" {
			mode, data = filemode.Symlink, e.link
		}
		obj := st.NewEncodedObject()
		obj.SetType(plumbing.BlobObject)
		w, err := obj.Writer()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		hash, err := st.SetEncodedObject(obj)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, object.TreeEntry{Name: p, Mode: mode, Hash: hash})
	}
	for dir, sub := range subtrees {
		entries = append(entries, object.TreeEntry{Name: dir, Mode: filemode.Dir, Hash: writeTestTree(t, st, sub)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	obj := st.NewEncodedObject()
	if err := (&object.Tree{Entries: entries}).Encode(obj); err != nil {
		t.Fatal(err)
	}
	hash, err := st.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}
