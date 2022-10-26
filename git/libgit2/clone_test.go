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

package libgit2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	git2go "github.com/libgit2/git2go/v34"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/libgit2/internal/test"
	"github.com/fluxcd/pkg/gittestserver"
)

func TestClone_cloneBranch(t *testing.T) {
	server, err := gittestserver.NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(server.Root())

	err = server.StartHTTP()
	if err != nil {
		t.Fatal(err)
	}
	defer server.StopHTTP()

	repoPath := "test.git"
	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, repoPath)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := git2go.OpenRepository(filepath.Join(server.Root(), repoPath))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Free()

	defaultBranch := "master"

	firstCommit, err := test.CommitFile(repo, "branch", "init", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Branch off on first commit
	if err = test.CreateBranch(repo, "test", nil); err != nil {
		t.Fatal(err)
	}

	// Create second commit on default branch
	secondCommit, err := test.CommitFile(repo, "branch", "second", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	repoURL := server.HTTPAddress() + "/" + repoPath

	tests := []struct {
		name                   string
		branch                 string
		filesCreated           map[string]string
		lastRevision           string
		expectedCommit         string
		expectedConcreteCommit bool
		expectedErr            string
	}{
		{
			name:                   "Default branch",
			branch:                 defaultBranch,
			filesCreated:           map[string]string{"branch": "second"},
			expectedCommit:         secondCommit.String(),
			expectedConcreteCommit: true,
		},
		{
			name:                   "Other branch",
			branch:                 "test",
			filesCreated:           map[string]string{"branch": "init"},
			expectedCommit:         firstCommit.String(),
			expectedConcreteCommit: true,
		},
		{
			name:                   "Non existing branch",
			branch:                 "invalid",
			expectedErr:            "reference 'refs/remotes/origin/invalid' not found",
			expectedConcreteCommit: true,
		},
		{
			name:                   "skip clone - lastRevision hasn't changed",
			branch:                 defaultBranch,
			filesCreated:           map[string]string{"branch": "second"},
			lastRevision:           fmt.Sprintf("%s/%s", defaultBranch, secondCommit.String()),
			expectedCommit:         secondCommit.String(),
			expectedConcreteCommit: false,
		},
		{
			name:                   "lastRevision is different",
			branch:                 defaultBranch,
			filesCreated:           map[string]string{"branch": "second"},
			lastRevision:           fmt.Sprintf("%s/%s", defaultBranch, firstCommit.String()),
			expectedCommit:         secondCommit.String(),
			expectedConcreteCommit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tmpDir := t.TempDir()
			lgc, err := NewClient(tmpDir, &git.AuthOptions{
				Transport: git.HTTP,
			})
			g.Expect(err).ToNot(HaveOccurred())
			defer lgc.Close()

			cc, err := lgc.Clone(context.TODO(), repoURL, git.CloneOptions{
				CheckoutStrategy: git.CheckoutStrategy{
					Branch: tt.branch,
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
			g.Expect(cc.String()).To(Equal(tt.branch + "/" + tt.expectedCommit))
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
		expectErr            string
		expectConcreteCommit bool
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
			name:        "Non existing tag",
			checkoutTag: "invalid",
			expectErr:   "unable to find 'invalid': no reference found for shorthand 'invalid'",
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

			server, err := gittestserver.NewTempGitServer()
			g.Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(server.Root())

			err = server.StartHTTP()
			g.Expect(err).ToNot(HaveOccurred())
			defer server.StopHTTP()

			repoPath := "test.git"
			err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, repoPath)
			g.Expect(err).ToNot(HaveOccurred())

			repo, err := git2go.OpenRepository(filepath.Join(server.Root(), repoPath))
			g.Expect(err).ToNot(HaveOccurred())
			defer repo.Free()

			// Collect tags and their associated commit for later reference.
			tagCommits := map[string]*git2go.Commit{}

			repoURL := server.HTTPAddress() + "/" + repoPath

			// Populate the repo with commits and tags.
			if tt.tagsInRepo != nil {
				for _, tr := range tt.tagsInRepo {
					var commit *git2go.Commit
					c, err := test.CommitFile(repo, "tag", tr.name, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					if commit, err = repo.LookupCommit(c); err != nil {
						t.Fatal(err)
					}
					_, err = tag(repo, commit.Id(), tr.annotated, tr.name, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					tagCommits[tr.name] = commit
				}
			}

			tmpDir := t.TempDir()
			lgc, err := NewClient(tmpDir, &git.AuthOptions{
				Transport: git.HTTP,
			})
			g.Expect(err).ToNot(HaveOccurred())
			defer lgc.Close()

			cloneOpts := git.CloneOptions{
				CheckoutStrategy: git.CheckoutStrategy{
					Tag: tt.checkoutTag,
				},
			}
			// If last revision is provided, configure it.
			if tt.lastRevTag != "" {
				lc := tagCommits[tt.lastRevTag]
				cloneOpts.LastObservedCommit = fmt.Sprintf("%s/%s", tt.lastRevTag, lc.Id().String())
			}

			cc, err := lgc.Clone(context.TODO(), repoURL, cloneOpts)
			if tt.expectErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectErr))
				g.Expect(cc).To(BeNil())
				return
			}

			// Check successful checkout results.
			targetTagCommit := tagCommits[tt.checkoutTag]
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cc.String()).To(Equal(tt.checkoutTag + "/" + targetTagCommit.Id().String()))
			g.Expect(git.IsConcreteCommit(*cc)).To(Equal(tt.expectConcreteCommit))

			// Check file content only when there's an actual checkout.
			if tt.lastRevTag != tt.checkoutTag {
				g.Expect(filepath.Join(tmpDir, "tag")).To(BeARegularFile())
				g.Expect(os.ReadFile(filepath.Join(tmpDir, "tag"))).To(BeEquivalentTo(tt.checkoutTag))
			}
		})
	}
}

func TestClone_cloneCommit(t *testing.T) {
	g := NewWithT(t)

	server, err := gittestserver.NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(server.Root())

	err = server.StartHTTP()
	if err != nil {
		t.Fatal(err)
	}
	defer server.StopHTTP()

	repoPath := "test.git"
	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, repoPath)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := git2go.OpenRepository(filepath.Join(server.Root(), repoPath))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Free()

	c, err := test.CommitFile(repo, "commit", "init", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = test.CommitFile(repo, "commit", "second", time.Now()); err != nil {
		t.Fatal(err)
	}

	repoURL := server.HTTPAddress() + "/" + repoPath
	tmpDir := t.TempDir()
	lgc, err := NewClient(tmpDir, &git.AuthOptions{
		Transport: git.HTTP,
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer lgc.Close()

	cc, err := lgc.Clone(context.TODO(), repoURL, git.CloneOptions{
		CheckoutStrategy: git.CheckoutStrategy{
			Commit: c.String(),
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cc).ToNot(BeNil())
	g.Expect(cc.String()).To(Equal("HEAD/" + c.String()))
	g.Expect(filepath.Join(tmpDir, "commit")).To(BeARegularFile())
	g.Expect(os.ReadFile(filepath.Join(tmpDir, "commit"))).To(BeEquivalentTo("init"))

	tmpDir2 := t.TempDir()
	lgc, err = NewClient(tmpDir2, &git.AuthOptions{
		Transport: git.HTTP,
	})
	g.Expect(err).ToNot(HaveOccurred())

	cc, err = lgc.Clone(context.TODO(), repoURL, git.CloneOptions{
		CheckoutStrategy: git.CheckoutStrategy{
			Commit: "4dc3185c5fc94eb75048376edeb44571cece25f4",
		},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(HavePrefix("git checkout error: git commit '4dc3185c5fc94eb75048376edeb44571cece25f4' not found:"))
	g.Expect(cc).To(BeNil())
}

func TestClone_cloneSemVer(t *testing.T) {
	g := NewWithT(t)
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
			annotated:  true,
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
		expectErr  error
		expectTag  string
	}{
		{
			name:       "Orders by SemVer",
			constraint: ">0.1.0",
			expectTag:  "0.2.0",
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

	server, err := gittestserver.NewTempGitServer()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(server.Root())

	err = server.StartHTTP()
	if err != nil {
		t.Fatal(err)
	}
	defer server.StopHTTP()

	repoPath := "test.git"
	err = server.InitRepo("../testdata/git/repo", git.DefaultBranch, repoPath)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := git2go.OpenRepository(filepath.Join(server.Root(), repoPath))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Free()
	repoURL := server.HTTPAddress() + "/" + repoPath

	refs := make(map[string]string, len(tags))
	for _, tt := range tags {
		ref, err := test.CommitFile(repo, "tag", tt.tag, tt.commitTime)
		if err != nil {
			t.Fatal(err)
		}
		commit, err := repo.LookupCommit(ref)
		if err != nil {
			t.Fatal(err)
		}
		defer commit.Free()
		refs[tt.tag] = commit.Id().String()
		_, err = tag(repo, ref, tt.annotated, tt.tag, tt.tagTime)
		if err != nil {
			t.Fatal(err)
		}
	}

	c, err := repo.Tags.List()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(c).To(HaveLen(len(tags)))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tmpDir := t.TempDir()
			lgc, err := NewClient(tmpDir, &git.AuthOptions{
				Transport: git.HTTP,
			})
			g.Expect(err).ToNot(HaveOccurred())
			defer lgc.Close()

			cc, err := lgc.Clone(context.TODO(), repoURL, git.CloneOptions{
				CheckoutStrategy: git.CheckoutStrategy{
					SemVer: tt.constraint,
				},
			})
			if tt.expectErr != nil {
				g.Expect(err).To(Equal(tt.expectErr))
				g.Expect(cc).To(BeNil())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cc.String()).To(Equal(tt.expectTag + "/" + refs[tt.expectTag]))
			g.Expect(filepath.Join(tmpDir, "tag")).To(BeARegularFile())
			g.Expect(os.ReadFile(filepath.Join(tmpDir, "tag"))).To(BeEquivalentTo(tt.expectTag))
		})
	}

}

func tag(repo *git2go.Repository, cId *git2go.Oid, annotated bool, tag string, time time.Time) (*git2go.Oid, error) {
	commit, err := repo.LookupCommit(cId)
	if err != nil {
		return nil, err
	}
	if annotated {
		return repo.Tags.Create(tag, commit, test.MockSignature(time), fmt.Sprintf("Annotated tag for %s", tag))
	}
	return repo.Tags.CreateLightweight(tag, commit, false)
}
