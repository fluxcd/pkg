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

package storage_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/artifact/config"
	. "github.com/fluxcd/pkg/artifact/storage"
)

func TestStorage_getGarbageFiles(t *testing.T) {
	artifactFolder := filepath.Join("foo", "bar")
	tests := []struct {
		name                 string
		artifactPaths        []string
		createPause          time.Duration
		ttl                  time.Duration
		maxItemsToBeRetained int
		totalCountLimit      int
		wantDeleted          []string
	}{
		{
			name: "delete files based on maxItemsToBeRetained",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Millisecond * 10,
			ttl:                  time.Minute * 2,
			totalCountLimit:      10,
			maxItemsToBeRetained: 2,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
			},
		},
		{
			name: "delete files based on maxItemsToBeRetained, ignore lock files",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Millisecond * 10,
			ttl:                  time.Minute * 2,
			totalCountLimit:      10,
			maxItemsToBeRetained: 2,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Second * 1,
			ttl:                  time.Second*3 + time.Millisecond*500,
			totalCountLimit:      10,
			maxItemsToBeRetained: 4,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl, ignore lock files",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Second * 1,
			ttl:                  time.Second*3 + time.Millisecond*500,
			totalCountLimit:      10,
			maxItemsToBeRetained: 4,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl and maxItemsToBeRetained",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
				filepath.Join(artifactFolder, "artifact6.tar.gz"),
			},
			createPause:          time.Second * 1,
			ttl:                  time.Second*5 + time.Millisecond*500,
			totalCountLimit:      10,
			maxItemsToBeRetained: 4,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl and maxItemsToBeRetained and totalCountLimit",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
				filepath.Join(artifactFolder, "artifact6.tar.gz"),
			},
			createPause:          time.Millisecond * 500,
			ttl:                  time.Millisecond * 500,
			totalCountLimit:      3,
			maxItemsToBeRetained: 2,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()

			opts := &config.Options{
				StoragePath:              dir,
				StorageAddress:           ":9090",
				ArtifactRetentionTTL:     tt.ttl,
				ArtifactRetentionRecords: tt.maxItemsToBeRetained,
			}
			s, err := New(opts)
			g.Expect(err).ToNot(HaveOccurred(), "failed to create new storage")

			artifact := meta.Artifact{
				Path: tt.artifactPaths[len(tt.artifactPaths)-1],
			}
			g.Expect(os.MkdirAll(filepath.Join(dir, artifactFolder), 0o750)).ToNot(HaveOccurred())
			for _, artifactPath := range tt.artifactPaths {
				f, err := os.Create(filepath.Join(dir, artifactPath))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(f.Close()).ToNot(HaveOccurred())
				time.Sleep(tt.createPause)
			}

			deletedPaths, err := s.GetGarbageFiles(artifact, tt.totalCountLimit, tt.maxItemsToBeRetained, tt.ttl)
			g.Expect(err).ToNot(HaveOccurred(), "failed to collect garbage files")
			g.Expect(len(tt.wantDeleted)).To(Equal(len(deletedPaths)))
			for _, wantDeletedPath := range tt.wantDeleted {
				present := false
				for _, deletedPath := range deletedPaths {
					if strings.Contains(deletedPath, wantDeletedPath) {
						present = true
						break
					}
				}
				if !present {
					g.Fail(fmt.Sprintf("expected file to be deleted, still exists: %s", wantDeletedPath))
				}
			}
		})
	}
}

func TestStorage_GarbageCollect(t *testing.T) {
	artifactFolder := filepath.Join("foo", "bar")
	tests := []struct {
		name          string
		artifactPaths []string
		wantCollected []string
		wantDeleted   []string
		wantErr       string
		ctxTimeout    time.Duration
	}{
		{
			name: "garbage collects",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
			},
			wantCollected: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
			},
			ctxTimeout: time.Second * 1,
		},
		{
			name: "garbage collection fails with context timeout",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
			},
			wantErr:    "context deadline exceeded",
			ctxTimeout: time.Nanosecond * 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()

			opts := &config.Options{
				StoragePath:              dir,
				StorageAddress:           ":9090",
				ArtifactRetentionTTL:     time.Second * 2,
				ArtifactRetentionRecords: 2,
			}
			s, err := New(opts)
			g.Expect(err).ToNot(HaveOccurred(), "failed to create new storage")

			artifact := meta.Artifact{
				Path: tt.artifactPaths[len(tt.artifactPaths)-1],
			}
			g.Expect(os.MkdirAll(filepath.Join(dir, artifactFolder), 0o750)).ToNot(HaveOccurred())
			for i, artifactPath := range tt.artifactPaths {
				f, err := os.Create(filepath.Join(dir, artifactPath))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(f.Close()).ToNot(HaveOccurred())
				if i != len(tt.artifactPaths)-1 {
					time.Sleep(time.Second * 1)
				}
			}

			collectedPaths, err := s.GarbageCollect(context.TODO(), artifact, tt.ctxTimeout)
			if tt.wantErr == "" {
				g.Expect(err).ToNot(HaveOccurred(), "failed to collect garbage files")
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
			if len(tt.wantCollected) > 0 {
				g.Expect(len(tt.wantCollected)).To(Equal(len(collectedPaths)))
				for _, wantCollectedPath := range tt.wantCollected {
					present := false
					for _, collectedPath := range collectedPaths {
						if strings.Contains(collectedPath, wantCollectedPath) {
							g.Expect(collectedPath).ToNot(BeAnExistingFile())
							present = true
							break
						}
					}
					if present == false {
						g.Fail(fmt.Sprintf("expected file to be garbage collected, still exists: %s", wantCollectedPath))
					}
				}
			}
			for _, delFile := range tt.wantDeleted {
				g.Expect(filepath.Join(dir, delFile)).ToNot(BeAnExistingFile())
			}
		})
	}
}
