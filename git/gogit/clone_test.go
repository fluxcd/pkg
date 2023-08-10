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

package gogit

import (
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/go-git/go-billy/v5/memfs"
	extgogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
	. "github.com/onsi/gomega"
	cryptossh "golang.org/x/crypto/ssh"

	"github.com/fluxcd/gitkit"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit/fs"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/fluxcd/pkg/gittestserver"
	"github.com/fluxcd/pkg/ssh"
)

const testRepositoryPath = "../testdata/git/repo"

func TestClone_cloneBranch(t *testing.T) {
	repo, repoPath, err := initRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	firstCommit, err := commitFile(repo, "branch", "init", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if err = createBranch(repo, "test"); err != nil {
		t.Fatal(err)
	}

	secondCommit, err := commitFile(repo, "branch", "second", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	_, emptyRepoPath, err := initRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name                   string
		branch                 string
		filesCreated           map[string]string
		lastRevision           string
		expectedCommit         string
		expectedConcreteCommit bool
		expectedErr            string
		expectedEmpty          bool
	}{
		{
			name:                   "Default branch",
			branch:                 "master",
			filesCreated:           map[string]string{"branch": "init"},
			expectedCommit:         firstCommit.String(),
			expectedConcreteCommit: true,
		},
		{
			name:                   "skip clone if LastRevision hasn't changed",
			branch:                 "master",
			filesCreated:           map[string]string{"branch": "init"},
			lastRevision:           fmt.Sprintf("master@%s", git.Hash(firstCommit.String()).Digest()),
			expectedCommit:         firstCommit.String(),
			expectedConcreteCommit: false,
		},
		{
			name:                   "skip clone if LastRevision hasn't changed (legacy)",
			branch:                 "master",
			filesCreated:           map[string]string{"branch": "init"},
			lastRevision:           fmt.Sprintf("master/%s", firstCommit.String()),
			expectedCommit:         firstCommit.String(),
			expectedConcreteCommit: false,
		},
		{
			name:                   "Other branch - revision has changed",
			branch:                 "test",
			filesCreated:           map[string]string{"branch": "second"},
			lastRevision:           fmt.Sprintf("master@%s", git.Hash(firstCommit.String()).Digest()),
			expectedCommit:         secondCommit.String(),
			expectedConcreteCommit: true,
		},
		{
			name:                   "Other branch - revision has changed (legacy)",
			branch:                 "test",
			filesCreated:           map[string]string{"branch": "second"},
			lastRevision:           fmt.Sprintf("master/%s", firstCommit.String()),
			expectedCommit:         secondCommit.String(),
			expectedConcreteCommit: true,
		},
		{
			name:        "Non existing branch",
			branch:      "invalid",
			expectedErr: "couldn't find remote ref \"refs/heads/invalid\"",
		},
		{
			name:          "empty repo",
			branch:        "master",
			expectedEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tmpDir := t.TempDir()
			ggc, err := NewClient(tmpDir, &git.AuthOptions{Transport: git.HTTP})
			g.Expect(err).ToNot(HaveOccurred())

			var upstreamPath string
			if tt.expectedEmpty {
				upstreamPath = emptyRepoPath
			} else {
				upstreamPath = repoPath
			}

			cc, err := ggc.Clone(context.TODO(), upstreamPath, repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					Branch: tt.branch,
				},
				ShallowClone:       true,
				LastObservedCommit: tt.lastRevision,
			})
			if tt.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
				g.Expect(cc).To(BeNil())
				return
			}

			if tt.expectedEmpty {
				g.Expect(cc).To(BeNil())
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(filepath.Join(ggc.path, ".git")).To(BeADirectory())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cc.String()).To(Equal(tt.branch + "@" + git.HashTypeSHA1 + ":" + tt.expectedCommit))
			g.Expect(git.IsConcreteCommit(*cc)).To(Equal(tt.expectedConcreteCommit))

			if tt.expectedConcreteCommit {
				for k, v := range tt.filesCreated {
					g.Expect(filepath.Join(tmpDir, k)).To(BeARegularFile())
					g.Expect(os.ReadFile(filepath.Join(tmpDir, k))).To(BeEquivalentTo(v))
				}
			}
		})
	}
}

func TestClone_cloneTag(t *testing.T) {
	type testTag struct {
		name      string
		annotated bool
	}

	tests := []struct {
		name                 string
		tagsInRepo           []testTag
		checkoutTag          string
		lastRevTag           string
		expectConcreteCommit bool
		expectErr            string
	}{
		{
			name:                 "Tag",
			tagsInRepo:           []testTag{{"tag-1", false}},
			checkoutTag:          "tag-1",
			expectConcreteCommit: true,
		},
		{
			name:                 "Annotated",
			tagsInRepo:           []testTag{{"annotated", true}},
			checkoutTag:          "annotated",
			expectConcreteCommit: true,
		},
		{
			name: "Non existing tag",
			// Without this go-git returns error "remote repository is empty".
			tagsInRepo:  []testTag{{"tag-1", false}},
			checkoutTag: "invalid",
			expectErr:   "couldn't find remote ref \"refs/tags/invalid\"",
		},
		{
			name:                 "Skip clone - last revision unchanged",
			tagsInRepo:           []testTag{{"tag-1", false}},
			checkoutTag:          "tag-1",
			lastRevTag:           "tag-1",
			expectConcreteCommit: false,
		},
		{
			name:                 "Last revision changed",
			tagsInRepo:           []testTag{{"tag-1", false}, {"tag-2", false}},
			checkoutTag:          "tag-2",
			lastRevTag:           "tag-1",
			expectConcreteCommit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			repo, path, err := initRepo(t.TempDir())
			g.Expect(err).ToNot(HaveOccurred())

			// Collect tags and their associated commit hash for later
			// reference.
			tagCommits := map[string]string{}

			// Populate the repo with commits and tags.
			if tt.tagsInRepo != nil {
				for _, tr := range tt.tagsInRepo {
					h, err := commitFile(repo, "tag", tr.name, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					_, err = tag(repo, h, tr.annotated, tr.name, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					tagCommits[tr.name] = h.String()
				}
			}

			tmpDir := t.TempDir()
			ggc, err := NewClient(tmpDir, &git.AuthOptions{Transport: git.HTTP})
			g.Expect(err).ToNot(HaveOccurred())

			opts := repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					Tag: tt.checkoutTag,
				},
				ShallowClone: true,
			}

			// If last revision is provided, configure it.
			if tt.lastRevTag != "" {
				lc := tagCommits[tt.lastRevTag]
				opts.LastObservedCommit = fmt.Sprintf("%s@%s", tt.lastRevTag, git.Hash(lc).Digest())
			}

			cc, err := ggc.Clone(context.TODO(), path, opts)

			if tt.expectErr != "" {
				g.Expect(err).ToNot(BeNil())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectErr))
				g.Expect(cc).To(BeNil())
				return
			}

			// Check if commit has a parent if the tag was annotated.
			for _, tagInRepo := range tt.tagsInRepo {
				if tagInRepo.annotated {
					g.Expect(cc.ReferencingTag).ToNot(BeNil())
					g.Expect(cc.ReferencingTag.Message).To(Equal(fmt.Sprintf("Annotated tag for: %s\n", tagInRepo.name)))
				} else {
					g.Expect(cc.ReferencingTag).To(BeNil())
				}
			}

			// Check successful checkout results.
			g.Expect(git.IsConcreteCommit(*cc)).To(Equal(tt.expectConcreteCommit))
			targetTagHash := tagCommits[tt.checkoutTag]
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cc.String()).To(Equal(tt.checkoutTag + "@" + git.HashTypeSHA1 + ":" + targetTagHash))

			// Check file content only when there's an actual checkout.
			if tt.lastRevTag != tt.checkoutTag {
				g.Expect(filepath.Join(tmpDir, "tag")).To(BeARegularFile())
				g.Expect(os.ReadFile(filepath.Join(tmpDir, "tag"))).To(BeEquivalentTo(tt.checkoutTag))
			}
		})
	}
}

func TestClone_cloneCommit(t *testing.T) {
	repo, path, err := initRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	firstCommit, err := commitFile(repo, "commit", "init", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = createBranch(repo, "other-branch"); err != nil {
		t.Fatal(err)
	}
	secondCommit, err := commitFile(repo, "commit", "second", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		commit       string
		branch       string
		expectCommit string
		expectFile   string
		expectError  string
	}{
		{
			name:         "Commit",
			commit:       firstCommit.String(),
			expectCommit: git.HashTypeSHA1 + ":" + firstCommit.String(),
			expectFile:   "init",
		},
		{
			name:         "Commit in specific branch",
			commit:       secondCommit.String(),
			branch:       "other-branch",
			expectCommit: "other-branch@" + git.HashTypeSHA1 + ":" + secondCommit.String(),
			expectFile:   "second",
		},
		{
			name:        "Non existing commit",
			commit:      "a-random-invalid-commit",
			expectError: "unable to resolve commit object for 'a-random-invalid-commit': object not found",
		},
		{
			name:        "Non existing commit in specific branch",
			commit:      secondCommit.String(),
			branch:      "master",
			expectError: "object not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tmpDir := t.TempDir()
			opts := repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					Branch: tt.branch,
					Commit: tt.commit,
				},
				ShallowClone: true,
			}
			ggc, err := NewClient(tmpDir, nil)
			g.Expect(err).ToNot(HaveOccurred())

			cc, err := ggc.Clone(context.TODO(), path, opts)
			if tt.expectError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
				g.Expect(cc).To(BeNil())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cc).ToNot(BeNil())
			g.Expect(cc.String()).To(Equal(tt.expectCommit))
			g.Expect(filepath.Join(tmpDir, "commit")).To(BeARegularFile())
			g.Expect(os.ReadFile(filepath.Join(tmpDir, "commit"))).To(BeEquivalentTo(tt.expectFile))
		})
	}
}

func TestClone_cloneSemVer(t *testing.T) {
	now := time.Now()

	tags := []struct {
		tag        string
		annotated  bool
		commitTime time.Time
		tagTime    time.Time
	}{
		{
			tag:        "v0.0.1",
			annotated:  false,
			commitTime: now,
		},
		{
			tag:        "v0.1.0+build-1",
			annotated:  true,
			commitTime: now.Add(10 * time.Minute),
			tagTime:    now.Add(2 * time.Hour), // This should be ignored during TS comparisons
		},
		{
			tag:        "v0.1.0+build-2",
			annotated:  false,
			commitTime: now.Add(30 * time.Minute),
		},
		{
			tag:        "v0.1.0+build-3",
			annotated:  false,
			commitTime: now.Add(1 * time.Hour),
			tagTime:    now.Add(1 * time.Hour), // This should be ignored during TS comparisons
		},
		{
			tag:        "0.2.0",
			annotated:  true,
			commitTime: now,
			tagTime:    now,
		},
	}
	tests := []struct {
		name       string
		constraint string
		annotated  bool
		expectErr  error
		expectTag  string
	}{
		{
			name:       "Orders by SemVer",
			constraint: ">0.1.0",
			expectTag:  "0.2.0",
			annotated:  true,
		},
		{
			name:       "Orders by SemVer and timestamp",
			constraint: "<0.2.0",
			expectTag:  "v0.1.0+build-3",
		},
		{
			name:       "Errors without match",
			constraint: ">=1.0.0",
			expectErr:  errors.New("no match found for semver: >=1.0.0"),
		},
	}

	repo, path, err := initRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	refs := make(map[string]string, len(tags))
	for _, tt := range tags {
		ref, err := commitFile(repo, "tag", tt.tag, tt.commitTime)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tag(repo, ref, tt.annotated, tt.tag, tt.tagTime)
		if err != nil {
			t.Fatal(err)
		}
		refs[tt.tag] = ref.String()
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tmpDir := t.TempDir()
			ggc, err := NewClient(tmpDir, nil)
			g.Expect(err).ToNot(HaveOccurred())

			opts := repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					SemVer: tt.constraint,
				},
				ShallowClone: true,
			}

			cc, err := ggc.Clone(context.TODO(), path, opts)
			if tt.expectErr != nil {
				g.Expect(err).To(Equal(tt.expectErr))
				g.Expect(cc).To(BeNil())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cc.String()).To(Equal(tt.expectTag + "@" + git.HashTypeSHA1 + ":" + refs[tt.expectTag]))
			g.Expect(filepath.Join(tmpDir, "tag")).To(BeARegularFile())
			g.Expect(os.ReadFile(filepath.Join(tmpDir, "tag"))).To(BeEquivalentTo(tt.expectTag))
			if tt.annotated {
				g.Expect(cc.ReferencingTag).ToNot(BeNil())
				g.Expect(cc.ReferencingTag.Message).To(Equal(fmt.Sprintf("Annotated tag for: %s\n", tt.expectTag)))
			} else {
				g.Expect(cc.ReferencingTag).To(BeNil())
			}
		})
	}
}

func TestClone_cloneRefName(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())
	err = server.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())
	defer server.StopHTTP()

	repoPath := "test.git"
	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, repoPath)
	g.Expect(err).ToNot(HaveOccurred())
	repoURL := server.HTTPAddress() + "/" + repoPath
	repo, err := extgogit.PlainClone(t.TempDir(), false, &extgogit.CloneOptions{
		URL: repoURL,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// head is the current HEAD on master
	head, err := repo.Head()
	g.Expect(err).ToNot(HaveOccurred())
	err = createBranch(repo, "test")
	g.Expect(err).ToNot(HaveOccurred())
	err = repo.Push(&extgogit.PushOptions{})
	g.Expect(err).ToNot(HaveOccurred())

	// create a new branch for testing tags in order to avoid disturbing the state
	// of the current branch that's used for testing branches later.
	err = createBranch(repo, "tag-testing")
	g.Expect(err).ToNot(HaveOccurred())
	hash, err := commitFile(repo, "bar.txt", "this is the way", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	err = repo.Push(&extgogit.PushOptions{})
	g.Expect(err).ToNot(HaveOccurred())
	_, err = tag(repo, hash, true, "v0.1.0", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	err = repo.Push(&extgogit.PushOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/tags/v0.1.0" + ":refs/tags/v0.1.0"),
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	// set a custom reference, in the format of GitHub PRs.
	err = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("/refs/pull/1/head"), hash))
	g.Expect(err).ToNot(HaveOccurred())
	err = repo.Push(&extgogit.PushOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/pull/1/head" + ":refs/pull/1/head"),
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name                   string
		refName                string
		filesCreated           map[string]string
		lastRevision           string
		expectedCommit         string
		expectedConcreteCommit bool
		expectedErr            string
	}{
		{
			name:                   "ref name pointing to a branch",
			refName:                "refs/heads/master",
			filesCreated:           map[string]string{"foo.txt": "test file\n"},
			expectedCommit:         head.Hash().String(),
			expectedConcreteCommit: true,
		},
		{
			name:                   "skip clone if LastRevision is unchanged",
			refName:                "refs/heads/master",
			lastRevision:           "refs/heads/master" + "@" + git.HashTypeSHA1 + ":" + head.Hash().String(),
			expectedCommit:         head.Hash().String(),
			expectedConcreteCommit: false,
		},
		{
			name:                   "skip clone if LastRevision is unchanged even if the reference changes",
			refName:                "refs/heads/test",
			lastRevision:           "refs/heads/test" + "@" + git.HashTypeSHA1 + ":" + head.Hash().String(),
			expectedCommit:         head.Hash().String(),
			expectedConcreteCommit: false,
		},
		{
			name:                   "ref name pointing to a tag",
			refName:                "refs/tags/v0.1.0",
			filesCreated:           map[string]string{"bar.txt": "this is the way"},
			lastRevision:           "refs/heads/test" + "@" + git.HashTypeSHA1 + ":" + head.Hash().String(),
			expectedCommit:         hash.String(),
			expectedConcreteCommit: true,
		},
		{
			name:                   "ref name with dereference suffix pointing to a tag",
			refName:                "refs/tags/v0.1.0" + tagDereferenceSuffix,
			filesCreated:           map[string]string{"bar.txt": "this is the way"},
			lastRevision:           "refs/heads/test" + "@" + git.HashTypeSHA1 + ":" + head.Hash().String(),
			expectedCommit:         hash.String(),
			expectedConcreteCommit: true,
		},
		{
			name:                   "ref name pointing to a pull request",
			refName:                "refs/pull/1/head",
			filesCreated:           map[string]string{"bar.txt": "this is the way"},
			expectedCommit:         hash.String(),
			expectedConcreteCommit: true,
		},
		{
			name:        "non existing ref",
			refName:     "refs/tags/v0.2.0",
			expectedErr: "unable to resolve ref 'refs/tags/v0.2.0' to a specific commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tmpDir := t.TempDir()
			ggc, err := NewClient(tmpDir, &git.AuthOptions{Transport: git.HTTP})
			g.Expect(err).ToNot(HaveOccurred())

			cc, err := ggc.Clone(context.TODO(), repoURL, repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					RefName: tt.refName,
				},
				LastObservedCommit: tt.lastRevision,
			})

			if tt.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
				g.Expect(cc).To(BeNil())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cc.AbsoluteReference()).To(Equal(tt.refName + "@" + git.HashTypeSHA1 + ":" + tt.expectedCommit))
			g.Expect(git.IsConcreteCommit(*cc)).To(Equal(tt.expectedConcreteCommit))
			if strings.Contains(tt.refName, "tags") && !strings.HasSuffix(tt.refName, tagDereferenceSuffix) {
				g.Expect(cc.ReferencingTag).ToNot(BeNil())
				g.Expect(cc.ReferencingTag.Message).To(ContainSubstring("Annotated tag for"))
			}

			for k, v := range tt.filesCreated {
				g.Expect(filepath.Join(tmpDir, k)).To(BeARegularFile())
				content, err := os.ReadFile(filepath.Join(tmpDir, k))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(content)).To(Equal(v))
			}
		})
	}
}

func Test_cloneSubmodule(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())

	err = server.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())
	defer server.StopHTTP()

	baseRepoPath := "base.git"
	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, baseRepoPath)
	g.Expect(err).ToNot(HaveOccurred())

	icingRepoPath := "icing.git"
	err = server.InitRepo("../testdata/git/repo2", git.DefaultBranch, icingRepoPath)
	g.Expect(err).ToNot(HaveOccurred())

	tmp := t.TempDir()
	icingRepo, err := extgogit.PlainClone(tmp, false, &extgogit.CloneOptions{
		URL:           server.HTTPAddress() + "/" + icingRepoPath,
		ReferenceName: plumbing.NewBranchReferenceName(git.DefaultBranch),
		Tags:          extgogit.NoTags,
	})
	g.Expect(err).ToNot(HaveOccurred())

	cmd := exec.Command("git", "submodule", "add", fmt.Sprintf("%s/%s", server.HTTPAddress(), baseRepoPath))
	cmd.Dir = tmp
	_, err = cmd.Output()
	g.Expect(err).ToNot(HaveOccurred())

	wt, err := icingRepo.Worktree()
	g.Expect(err).ToNot(HaveOccurred())
	_, err = wt.Add(".gitmodules")
	g.Expect(err).ToNot(HaveOccurred())
	_, err = wt.Commit("submod", &extgogit.CommitOptions{
		Author: &object.Signature{
			Name: "test user",
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	err = icingRepo.Push(&extgogit.PushOptions{})
	g.Expect(err).ToNot(HaveOccurred())

	tmpDir := t.TempDir()
	ggc, err := NewClient(tmpDir, &git.AuthOptions{
		Transport: git.HTTP,
	})
	g.Expect(err).ToNot(HaveOccurred())

	_, err = ggc.Clone(context.TODO(), server.HTTPAddress()+"/"+icingRepoPath, repository.CloneConfig{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: "master",
		},
		ShallowClone:      true,
		RecurseSubmodules: true,
	})

	expectedPaths := []string{"base", "base/foo.txt", "bar.txt", "."}
	var c int
	filepath.Walk(tmpDir, func(path string, d iofs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(path, ".git") {
			return nil
		}
		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}
		for _, expectedPath := range expectedPaths {
			if rel == expectedPath {
				c += 1
			}
		}
		return nil
	})
	g.Expect(c).To(Equal(len(expectedPaths)))
}

// Test_ssh_KeyTypes assures support for the different types of keys
// for SSH Authentication supported by Flux.
func Test_ssh_KeyTypes(t *testing.T) {
	tests := []struct {
		name       string
		keyType    ssh.KeyPairType
		authorized bool
		wantErr    string
	}{
		{name: "RSA 4096", keyType: ssh.RSA_4096, authorized: true},
		{name: "ECDSA P256", keyType: ssh.ECDSA_P256, authorized: true},
		{name: "ECDSA P384", keyType: ssh.ECDSA_P384, authorized: true},
		{name: "ECDSA P521", keyType: ssh.ECDSA_P521, authorized: true},
		{name: "ED25519", keyType: ssh.ED25519, authorized: true},
		{name: "unauthorized key", keyType: ssh.RSA_4096, wantErr: "unable to authenticate, attempted methods [none publickey], no supported methods remain"},
	}

	serverRootDir := t.TempDir()
	server := gittestserver.NewGitServer(serverRootDir)

	// Auth needs to be called, for authentication to be enabled.
	server.Auth("", "")

	var authorizedPublicKey string
	server.PublicKeyLookupFunc(func(content string) (*gitkit.PublicKey, error) {
		authedKey := strings.TrimSuffix(string(authorizedPublicKey), "\n")
		if authedKey == content {
			return &gitkit.PublicKey{Content: content}, nil
		}
		return nil, fmt.Errorf("pubkey provided '%s' does not match %s", content, authedKey)
	})

	g := NewWithT(t)
	timeout := 5 * time.Second

	server.KeyDir(filepath.Join(server.Root(), "keys"))
	g.Expect(server.ListenSSH()).To(Succeed())

	go func() {
		server.StartSSH()
	}()
	defer server.StopSSH()

	repoPath := "test.git"
	err := server.InitRepo(testRepositoryPath, git.DefaultBranch, repoPath)
	g.Expect(err).NotTo(HaveOccurred())

	sshURL := server.SSHAddress()
	repoURL := sshURL + "/" + repoPath

	// Fetch host key.
	u, err := url.Parse(sshURL)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(u.Host).ToNot(BeEmpty())

	knownHosts, err := ssh.ScanHostKey(u.Host, timeout, git.HostKeyAlgos, false)
	g.Expect(err).ToNot(HaveOccurred())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Generate ssh keys based on key type.
			kp, err := ssh.GenerateKeyPair(tt.keyType)
			g.Expect(err).ToNot(HaveOccurred())

			// Update authorized key to ensure only the new key is valid on the server.
			if tt.authorized {
				authorizedPublicKey = string(kp.PublicKey)
			}

			authOpts := git.AuthOptions{
				Transport:  git.SSH,
				Identity:   kp.PrivateKey,
				KnownHosts: knownHosts,
			}
			tmpDir := t.TempDir()

			ctx, cancel := context.WithTimeout(context.TODO(), timeout)
			defer cancel()

			// Checkout the repo.
			ggc, err := NewClient(tmpDir, &authOpts)
			g.Expect(err).ToNot(HaveOccurred())

			cc, err := ggc.Clone(ctx, repoURL, repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					Branch: git.DefaultBranch,
				},
				ShallowClone: true,
			})

			if tt.wantErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(cc).ToNot(BeNil())

				// Confirm checkout actually happened.
				d, err := os.ReadDir(tmpDir)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(d).To(HaveLen(2)) // .git and foo.txt
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring(tt.wantErr))
			}
		})
	}
}

// Test_ssh_KeyExchangeAlgos assures support for the different
// types of SSH key exchange algorithms supported by Flux.
func Test_ssh_KeyExchangeAlgos(t *testing.T) {
	tests := []struct {
		name      string
		ClientKex []string
		ServerKex []string
		wantErr   string
	}{
		{
			name:      "support for kex: diffie-hellman-group14-sha1",
			ClientKex: []string{"diffie-hellman-group14-sha1"},
			ServerKex: []string{"diffie-hellman-group14-sha1"},
		},
		{
			name:      "support for kex: diffie-hellman-group14-sha256",
			ClientKex: []string{"diffie-hellman-group14-sha256"},
			ServerKex: []string{"diffie-hellman-group14-sha256"},
		},
		{
			name:      "support for kex: curve25519-sha256",
			ClientKex: []string{"curve25519-sha256"},
			ServerKex: []string{"curve25519-sha256"},
		},
		{
			name:      "support for kex: ecdh-sha2-nistp256",
			ClientKex: []string{"ecdh-sha2-nistp256"},
			ServerKex: []string{"ecdh-sha2-nistp256"},
		},
		{
			name:      "support for kex: ecdh-sha2-nistp384",
			ClientKex: []string{"ecdh-sha2-nistp384"},
			ServerKex: []string{"ecdh-sha2-nistp384"},
		},
		{
			name:      "support for kex: ecdh-sha2-nistp521",
			ClientKex: []string{"ecdh-sha2-nistp521"},
			ServerKex: []string{"ecdh-sha2-nistp521"},
		},
		{
			name:      "support for kex: curve25519-sha256@libssh.org",
			ClientKex: []string{"curve25519-sha256@libssh.org"},
			ServerKex: []string{"curve25519-sha256@libssh.org"},
		},
		{
			name:      "non-matching kex",
			ClientKex: []string{"ecdh-sha2-nistp521"},
			ServerKex: []string{"curve25519-sha256@libssh.org"},
			wantErr:   "ssh: no common algorithm for key exchange; client offered: [ecdh-sha2-nistp521 ext-info-c], server offered: [curve25519-sha256@libssh.org]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			timeout := 5 * time.Second

			serverRootDir := t.TempDir()
			server := gittestserver.NewGitServer(serverRootDir).WithSSHConfig(&cryptossh.ServerConfig{
				Config: cryptossh.Config{
					KeyExchanges: tt.ServerKex,
				},
			})

			// Set what Client Key Exchange Algos to send
			git.KexAlgos = tt.ClientKex

			server.KeyDir(filepath.Join(server.Root(), "keys"))
			g.Expect(server.ListenSSH()).To(Succeed())

			go func() {
				server.StartSSH()
			}()
			defer server.StopSSH()

			repoPath := "test.git"
			err := server.InitRepo(testRepositoryPath, git.DefaultBranch, repoPath)
			g.Expect(err).NotTo(HaveOccurred())

			sshURL := server.SSHAddress()
			repoURL := sshURL + "/" + repoPath

			// Fetch host key.
			u, err := url.Parse(sshURL)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(u.Host).ToNot(BeEmpty())

			knownHosts, err := ssh.ScanHostKey(u.Host, timeout, git.HostKeyAlgos, false)
			g.Expect(err).ToNot(HaveOccurred())

			// No authentication is required for this test, but it is
			// used here to make the Checkout logic happy.
			kp, err := ssh.GenerateKeyPair(ssh.ED25519)
			g.Expect(err).ToNot(HaveOccurred())

			authOpts := git.AuthOptions{
				Transport:  git.SSH,
				Identity:   kp.PrivateKey,
				KnownHosts: knownHosts,
			}
			tmpDir := t.TempDir()

			ctx, cancel := context.WithTimeout(context.TODO(), timeout)
			defer cancel()

			ggc, err := NewClient(tmpDir, &authOpts)
			g.Expect(err).ToNot(HaveOccurred())

			_, err = ggc.Clone(ctx, repoURL, repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					Branch: git.DefaultBranch,
				},
				ShallowClone: true,
			})

			if tt.wantErr != "" {
				g.Expect(err).Error().Should(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring(tt.wantErr))
			} else {
				g.Expect(err).Error().ShouldNot(HaveOccurred())
			}
		})
	}
}

// Test_ssh_HostKeyAlgos assures support for the different
// types of SSH Host Key algorithms supported by Flux.
func Test_ssh_HostKeyAlgos(t *testing.T) {
	tests := []struct {
		name               string
		keyType            ssh.KeyPairType
		ClientHostKeyAlgos []string
		hashHostNames      bool
	}{
		{
			name:               "support for hostkey: ssh-rsa",
			keyType:            ssh.RSA_4096,
			ClientHostKeyAlgos: []string{"ssh-rsa"},
		},
		{
			name:               "support for hostkey: rsa-sha2-256",
			keyType:            ssh.RSA_4096,
			ClientHostKeyAlgos: []string{"rsa-sha2-256"},
		},
		{
			name:               "support for hostkey: rsa-sha2-512",
			keyType:            ssh.RSA_4096,
			ClientHostKeyAlgos: []string{"rsa-sha2-512"},
		},
		{
			name:               "support for hostkey: ecdsa-sha2-nistp256",
			keyType:            ssh.ECDSA_P256,
			ClientHostKeyAlgos: []string{"ecdsa-sha2-nistp256"},
		},
		{
			name:               "support for hostkey: ecdsa-sha2-nistp384",
			keyType:            ssh.ECDSA_P384,
			ClientHostKeyAlgos: []string{"ecdsa-sha2-nistp384"},
		},
		{
			name:               "support for hostkey: ecdsa-sha2-nistp521",
			keyType:            ssh.ECDSA_P521,
			ClientHostKeyAlgos: []string{"ecdsa-sha2-nistp521"},
		},
		{
			name:               "support for hostkey: ssh-ed25519",
			keyType:            ssh.ED25519,
			ClientHostKeyAlgos: []string{"ssh-ed25519"},
		},
		{
			name:               "support for hostkey: ssh-rsa with hashed host names",
			keyType:            ssh.RSA_4096,
			ClientHostKeyAlgos: []string{"ssh-rsa"},
			hashHostNames:      true,
		},
		{
			name:               "support for hostkey: rsa-sha2-256 with hashed host names",
			keyType:            ssh.RSA_4096,
			ClientHostKeyAlgos: []string{"rsa-sha2-256"},
			hashHostNames:      true,
		},
		{
			name:               "support for hostkey: rsa-sha2-512 with hashed host names",
			keyType:            ssh.RSA_4096,
			ClientHostKeyAlgos: []string{"rsa-sha2-512"},
			hashHostNames:      true,
		},
		{
			name:               "support for hostkey: ecdsa-sha2-nistp256 with hashed host names",
			keyType:            ssh.ECDSA_P256,
			ClientHostKeyAlgos: []string{"ecdsa-sha2-nistp256"},
			hashHostNames:      true,
		},
		{
			name:               "support for hostkey: ecdsa-sha2-nistp384 with hashed host names",
			keyType:            ssh.ECDSA_P384,
			ClientHostKeyAlgos: []string{"ecdsa-sha2-nistp384"},
			hashHostNames:      true,
		},
		{
			name:               "support for hostkey: ecdsa-sha2-nistp521 with hashed host names",
			keyType:            ssh.ECDSA_P521,
			ClientHostKeyAlgos: []string{"ecdsa-sha2-nistp521"},
			hashHostNames:      true,
		},
		{
			name:               "support for hostkey: ssh-ed25519 with hashed host names",
			keyType:            ssh.ED25519,
			ClientHostKeyAlgos: []string{"ssh-ed25519"},
			hashHostNames:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			timeout := 5 * time.Second

			sshConfig := &cryptossh.ServerConfig{}

			// Generate new keypair for the server to use for HostKeys.
			hkp, err := ssh.GenerateKeyPair(tt.keyType)
			g.Expect(err).NotTo(HaveOccurred())
			p, err := cryptossh.ParseRawPrivateKey(hkp.PrivateKey)
			g.Expect(err).NotTo(HaveOccurred())

			// Add key to server.
			signer, err := cryptossh.NewSignerFromKey(p)
			g.Expect(err).NotTo(HaveOccurred())
			sshConfig.AddHostKey(signer)

			serverRootDir := t.TempDir()
			server := gittestserver.NewGitServer(serverRootDir).WithSSHConfig(sshConfig)

			// Set what HostKey Algos will be accepted from a client perspective.
			git.HostKeyAlgos = tt.ClientHostKeyAlgos

			keyDir := filepath.Join(server.Root(), "keys")
			server.KeyDir(keyDir)
			g.Expect(server.ListenSSH()).To(Succeed())

			go func() {
				server.StartSSH()
			}()
			defer server.StopSSH()

			repoPath := "test.git"
			err = server.InitRepo(testRepositoryPath, git.DefaultBranch, repoPath)
			g.Expect(err).NotTo(HaveOccurred())

			sshURL := server.SSHAddress()
			repoURL := sshURL + "/" + repoPath

			// Fetch host key.
			u, err := url.Parse(sshURL)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(u.Host).ToNot(BeEmpty())

			knownHosts, err := ssh.ScanHostKey(u.Host, timeout, git.HostKeyAlgos, tt.hashHostNames)
			g.Expect(err).ToNot(HaveOccurred())

			// No authentication is required for this test, but it is
			// used here to make the Checkout logic happy.
			kp, err := ssh.GenerateKeyPair(ssh.ED25519)
			g.Expect(err).ToNot(HaveOccurred())

			authOpts := git.AuthOptions{
				Transport:  git.SSH,
				Identity:   kp.PrivateKey,
				KnownHosts: knownHosts,
			}
			tmpDir := t.TempDir()

			ctx, cancel := context.WithTimeout(context.TODO(), timeout)
			defer cancel()

			// Checkout the repo.
			ggc, err := NewClient(tmpDir, &authOpts)
			g.Expect(err).ToNot(HaveOccurred())

			_, err = ggc.Clone(ctx, repoURL, repository.CloneConfig{
				CheckoutStrategy: repository.CheckoutStrategy{
					Branch: git.DefaultBranch,
				},
				ShallowClone: true,
			})

			g.Expect(err).Error().ShouldNot(HaveOccurred())
		})
	}
}

func TestCloneAndPush_WithProxy(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(server.Root())

	err = server.StartHTTP()
	g.Expect(err).ToNot(HaveOccurred())
	defer server.StopHTTP()

	repoPath := "proxy.git"
	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, repoPath)
	g.Expect(err).ToNot(HaveOccurred())
	repoURL := server.HTTPAddress() + "/" + repoPath

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	var proxiedRequests int32
	setupHTTPProxy(proxy, &proxiedRequests)

	httpListener, err := net.Listen("tcp", ":0")
	g.Expect(err).ToNot(HaveOccurred())
	defer httpListener.Close()

	httpProxyAddr := fmt.Sprintf("http://localhost:%d", httpListener.Addr().(*net.TCPAddr).Port)
	proxyServer := http.Server{
		Addr:    httpProxyAddr,
		Handler: proxy,
	}
	go proxyServer.Serve(httpListener)
	defer proxyServer.Close()

	proxyOpts := transport.ProxyOptions{
		URL: httpProxyAddr,
	}
	authOpts := &git.AuthOptions{
		Transport: git.HTTP,
	}
	ggc, err := NewClient(t.TempDir(), authOpts, WithDiskStorage(), WithProxy(proxyOpts))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ggc.proxy.URL).ToNot(BeEmpty())

	_, err = ggc.Clone(context.TODO(), repoURL, repository.CloneConfig{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: git.DefaultBranch,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(proxiedRequests).To(BeNumerically(">", 0))

	// reset proxy requests counter
	proxiedRequests = 0
	// make a commit on master and push it.
	cc1, err := commitFile(ggc.repository, "test", "testing gogit push", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	err = ggc.Push(context.TODO(), repository.PushConfig{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(proxiedRequests).To(BeNumerically(">", 0))

	// check if we indeed pushed the commit to remote.
	ggc, err = NewClient(t.TempDir(), authOpts, WithDiskStorage(), WithProxy(proxyOpts))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ggc.proxy.URL).ToNot(BeEmpty())
	cc2, err := ggc.Clone(context.TODO(), repoURL, repository.CloneConfig{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: git.DefaultBranch,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cc1.String()).To(Equal(cc2.Hash.String()))
}

func Test_getRemoteHEAD(t *testing.T) {
	g := NewWithT(t)
	repo, path, err := initRepo(t.TempDir())
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(path)

	cc, err := commitFile(repo, "test", "testing current head branch", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	ref := plumbing.NewBranchReferenceName(git.DefaultBranch)
	ggc, err := NewClient("", nil)
	g.Expect(err).ToNot(HaveOccurred())

	head, err := ggc.getRemoteHEAD(context.TODO(), path, ref, nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(head).To(Equal(fmt.Sprintf("refs/heads/%s@%s", git.DefaultBranch, git.Hash(cc.String()).Digest())))

	cc, err = commitFile(repo, "test", "testing current head tag", time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	_, err = tag(repo, cc, true, "v0.1.0", time.Now())
	g.Expect(err).ToNot(HaveOccurred())

	ref = plumbing.NewTagReferenceName("v0.1.0")
	head, err = ggc.getRemoteHEAD(context.TODO(), path, ref, nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(head).To(Equal(fmt.Sprintf("refs/tags/%s@%s", "v0.1.0", git.Hash(cc.String()).Digest())))

	ref = plumbing.NewTagReferenceName("v0.1.0" + tagDereferenceSuffix)
	head, err = ggc.getRemoteHEAD(context.TODO(), path, ref, nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(head).To(Equal(fmt.Sprintf("refs/tags/%s@%s", "v0.1.0"+tagDereferenceSuffix, git.Hash(cc.String()).Digest())))

	ref = plumbing.ReferenceName("/refs/heads/main")
	head, err = ggc.getRemoteHEAD(context.TODO(), path, ref, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(fmt.Sprintf("ref %s is invalid; Git refs cannot begin or end with a slash '/'", ref.String())))

	ref = plumbing.ReferenceName("refs/heads/main/")
	head, err = ggc.getRemoteHEAD(context.TODO(), path, ref, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(fmt.Sprintf("ref %s is invalid; Git refs cannot begin or end with a slash '/'", ref.String())))
}

func Test_filterRefs(t *testing.T) {

	refStrings := []string{"refs/heads/main", "refs/tags/v1.0.0", "refs/pull/1/head", "refs/tags/v1.1.0", "refs/tags/v1.0.0" + tagDereferenceSuffix}
	dummyHash := "84d9be20ca15d29bebc629e5b6f29dab78cc69ba"
	annotatedTagHash := "9000be6daa3323cb7009075259bb7bd62498d32f"
	var refs []*plumbing.Reference
	for _, refString := range refStrings {
		if strings.HasSuffix(refString, tagDereferenceSuffix) {
			refs = append(refs, plumbing.NewReferenceFromStrings(refString, annotatedTagHash))
		} else {
			refs = append(refs, plumbing.NewReferenceFromStrings(refString, dummyHash))
		}
	}
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "branch ref",
			ref:      "refs/heads/main",
			expected: "refs/heads/main@sha1:" + dummyHash,
		},
		{
			name:     "pull request ref",
			ref:      "refs/pull/1/head",
			expected: "refs/pull/1/head@sha1:" + dummyHash,
		},
		{
			name:     "tag ref",
			ref:      "refs/tags/v1.1.0",
			expected: "refs/tags/v1.1.0@sha1:" + dummyHash,
		},
		{
			name:     "annotated tag ref",
			ref:      "refs/tags/v1.0.0" + tagDereferenceSuffix,
			expected: fmt.Sprintf("refs/tags/v1.0.0%s@sha1:%s", tagDereferenceSuffix, annotatedTagHash),
		},
		{
			name:     "tag ref but is resolved to its commit",
			ref:      "refs/tags/v1.0.0",
			expected: "refs/tags/v1.0.0@sha1:" + annotatedTagHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := filterRefs(refs, plumbing.ReferenceName(tt.ref))
			g.Expect(got).To(Equal(tt.expected))
		})
	}
}

func TestClone_CredentialsOverHttp(t *testing.T) {
	tests := []struct {
		name                     string
		username                 string
		password                 string
		bearerToken              string
		allowCredentialsOverHttp bool
		transformURL             func(string) string
		expectCloneErr           string
		expectRequest            bool
	}{
		{
			name:           "blocked: basic auth over HTTP (name)",
			username:       "just-name",
			expectCloneErr: "basic auth cannot be sent over HTTP",
		},
		{
			name:           "blocked: basic auth over HTTP (password)",
			password:       "just-pass",
			expectCloneErr: "basic auth cannot be sent over HTTP",
		},
		{
			name:           "blocked: basic auth over HTTP (name and password)",
			username:       "name",
			password:       "pass",
			expectCloneErr: "basic auth cannot be sent over HTTP",
		},
		{
			name:           "blocked: bearer token over HTTP",
			bearerToken:    "token",
			expectCloneErr: "bearer token cannot be sent over HTTP",
		},
		{
			name: "blocked: URL based credential over HTTP (name)",
			transformURL: func(s string) string {
				u, _ := url.Parse(s)
				u.User = url.User("some-joe")
				return u.String()
			},
			expectCloneErr: "URL cannot contain credentials when using HTTP",
		},
		{
			name: "blocked: URL based credential over HTTP (name and password)",
			transformURL: func(s string) string {
				u, _ := url.Parse(s)
				u.User = url.UserPassword("joe", "doe")
				return u.String()
			},
			expectCloneErr: "URL cannot contain credentials when using HTTP",
		},
		{
			name: "blocked: URL based credential over HTTP (without scheme)",
			transformURL: func(s string) string {
				u, _ := url.Parse(s)
				u.User = url.UserPassword("joe", "doe")
				u.Scheme = ""
				return u.String()
			},
			expectCloneErr: "URL cannot contain credentials when using HTTP",
		},
		{
			name: "blocked: URL based credential over HTTP (scheme with mixed casing)",
			transformURL: func(s string) string {
				u, _ := url.Parse(s)
				u.User = url.UserPassword("joe", "doe")
				u.Scheme = "HTtp"
				return u.String()
			},
			expectCloneErr: "URL cannot contain credentials when using HTTP",
		},
		{
			name:                     "allowed: basic auth over HTTP (name)",
			username:                 "just-name",
			expectCloneErr:           "unable to clone",
			allowCredentialsOverHttp: true,
			expectRequest:            true,
		},
		{
			name:                     "allowed: basic auth over HTTP (password)",
			password:                 "just-pass",
			expectCloneErr:           "unable to clone",
			allowCredentialsOverHttp: true,
			expectRequest:            true,
		},
		{
			name:                     "allowed: basic auth over HTTP (name and password)",
			username:                 "name",
			password:                 "pass",
			expectCloneErr:           "unable to clone",
			allowCredentialsOverHttp: true,
			expectRequest:            true,
		},
		{
			name:                     "allowed: bearer token over HTTP",
			bearerToken:              "token",
			expectCloneErr:           "unable to clone",
			allowCredentialsOverHttp: true,
			expectRequest:            true,
		},
		{
			name: "allowed: URL based credential over HTTP (name)",
			transformURL: func(s string) string {
				u, _ := url.Parse(s)
				u.User = url.User("some-joe")
				return u.String()
			},
			expectCloneErr:           "unable to clone",
			allowCredentialsOverHttp: true,
			expectRequest:            true,
		},
		{
			name: "allowed: URL based credential over HTTP (name and password)",
			transformURL: func(s string) string {
				u, _ := url.Parse(s)
				u.User = url.UserPassword("joe", "doe")
				return u.String()
			},
			expectCloneErr:           "unable to clone",
			allowCredentialsOverHttp: true,
			expectRequest:            true,
		},
		{
			name: "allowed: URL based credential over HTTP (without scheme)",
			transformURL: func(s string) string {
				u, _ := url.Parse(s)
				u.User = url.UserPassword("joe", "doe")
				u.Scheme = ""
				return u.String()
			},
			expectCloneErr:           "unable to clone",
			allowCredentialsOverHttp: true,
		},
		{
			name:           "execute request without creds",
			expectCloneErr: "unable to clone",
			expectRequest:  true,
		},
	}

	totalRequests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		totalRequests++
	}))
	defer ts.Close()

	previousRequestCount := 0
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			previousRequestCount = totalRequests

			tmpDir := t.TempDir()
			opts := []ClientOption{WithDiskStorage()}
			if tt.allowCredentialsOverHttp {
				opts = append(opts, WithInsecureCredentialsOverHTTP())
			}

			ggc, err := NewClient(tmpDir, &git.AuthOptions{
				Transport:   git.HTTP,
				Username:    tt.username,
				Password:    tt.password,
				BearerToken: tt.bearerToken,
			}, opts...)

			g.Expect(err).ToNot(HaveOccurred())

			repoURL := ts.URL
			if tt.transformURL != nil {
				repoURL = tt.transformURL(ts.URL)
			}

			_, err = ggc.Clone(context.TODO(), repoURL, repository.CloneConfig{})

			if tt.expectCloneErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectCloneErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.expectRequest {
				g.Expect(totalRequests).To(BeNumerically(">", previousRequestCount))
			} else {
				g.Expect(totalRequests).To(Equal(previousRequestCount))
			}
		})
	}
}

func TestGoGitErrorReplace(t *testing.T) {
	// this is what go-git uses as the error message is if the remote
	// sends a blank first line
	unknownMessage := `unknown error: remote: `
	err := errors.New(unknownMessage)
	err = goGitError(err)
	reformattedMessage := err.Error()
	if reformattedMessage == unknownMessage {
		t.Errorf("expected rewritten error, got %q", reformattedMessage)
	}
}

func TestGoGitErrorUnchanged(t *testing.T) {
	// this is (roughly) what GitHub sends if the deploy key doesn't
	// have write access; go-git passes this on verbatim
	regularMessage := `remote: ERROR: deploy key does not have write access`
	expectedReformat := regularMessage
	err := errors.New(regularMessage)
	err = goGitError(err)
	reformattedMessage := err.Error()
	// test that it's been rewritten, without checking the exact content
	if len(reformattedMessage) > len(expectedReformat) {
		t.Errorf("expected %q, got %q", expectedReformat, reformattedMessage)
	}
}

func Fuzz_GoGitError(f *testing.F) {
	f.Add("")
	f.Add("unknown error: remote: ")
	f.Add("some other error")

	f.Fuzz(func(t *testing.T, msg string) {
		var err error
		if msg != "" {
			err = errors.New(msg)
		}

		_ = goGitError(err)
	})
}

func initRepo(tmpDir string) (*extgogit.Repository, string, error) {
	sto := filesystem.NewStorage(fs.New(tmpDir), cache.NewObjectLRUDefault())
	repo, err := extgogit.Init(sto, memfs.New())
	if err != nil {
		return nil, "", err
	}
	return repo, tmpDir, err
}

func createBranch(repo *extgogit.Repository, branch string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	h, err := repo.Head()
	if err != nil {
		return err
	}
	return wt.Checkout(&extgogit.CheckoutOptions{
		Hash:   h.Hash(),
		Branch: plumbing.ReferenceName("refs/heads/" + branch),
		Create: true,
	})
}

func commitFile(repo *extgogit.Repository, path, content string, time time.Time) (plumbing.Hash, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return plumbing.Hash{}, err
	}
	f, err := wt.Filesystem.Create(path)
	if err != nil {
		return plumbing.Hash{}, err
	}
	if _, err = f.Write([]byte(content)); err != nil {
		f.Close()
		return plumbing.Hash{}, err
	}
	if err = f.Close(); err != nil {
		return plumbing.Hash{}, err
	}
	if _, err = wt.Add(path); err != nil {
		return plumbing.Hash{}, err
	}
	return wt.Commit("Adding: "+path, &extgogit.CommitOptions{
		Author:    mockSignature(time),
		Committer: mockSignature(time),
	})
}

func tag(repo *extgogit.Repository, commit plumbing.Hash, annotated bool, tag string, time time.Time) (*plumbing.Reference, error) {
	var opts *extgogit.CreateTagOptions
	if annotated {
		opts = &extgogit.CreateTagOptions{
			Tagger:  mockSignature(time),
			Message: "Annotated tag for: " + tag,
		}
	}
	return repo.CreateTag(tag, commit, opts)
}

func mockSignature(time time.Time) *object.Signature {
	return &object.Signature{
		Name:  "Jane Doe",
		Email: "jane@example.com",
		When:  time,
	}
}

func setupHTTPProxy(proxy *goproxy.ProxyHttpServer, proxiedRequests *int32) {
	var proxyHandler goproxy.FuncReqHandler = func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if strings.Contains(req.Host, "127.0.0.1") {
			*proxiedRequests++
			return req, nil
		}
		// Reject if it isn't our request.
		return req, goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusForbidden, "")
	}
	proxy.OnRequest().Do(proxyHandler)
}
