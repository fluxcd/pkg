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

package storage

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	kerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/fluxcd/pkg/apis/meta"
)

const GarbageCountLimit = 1000

// GetGarbageFiles returns all files that need to be garbage collected for the given artifact.
// Garbage files are determined based on the below flow:
// 1. collect all artifact files with an expired ttl
// 2. if we satisfy maxItemsToBeRetained, then return
// 3. else, collect all artifact files till the latest n files remain, where n=maxItemsToBeRetained
func (s Storage) GetGarbageFiles(artifact meta.Artifact,
	totalCountLimit, maxItemsToBeRetained int,
	ttl time.Duration) (garbageFiles []string, _ error) {
	localPath := s.LocalPath(artifact)
	dir := filepath.Dir(localPath)
	artifactFilesWithCreatedTs := make(map[time.Time]string)
	// sortedPaths contain all files sorted according to their created ts.
	sortedPaths := []string{}
	now := time.Now().UTC()
	totalArtifactFiles := 0
	var errors []string
	creationTimestamps := []time.Time{}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errors = append(errors, err.Error())
			return nil
		}
		if totalArtifactFiles >= totalCountLimit {
			return fmt.Errorf("reached file walking limit, already walked over: %d", totalArtifactFiles)
		}
		info, err := d.Info()
		if err != nil {
			errors = append(errors, err.Error())
			return nil
		}
		createdAt := info.ModTime().UTC()
		diff := now.Sub(createdAt)
		// Compare the time difference between now and the time at which the file was created
		// with the provided TTL. Delete if the difference is greater than the TTL. Since the
		// below logic just deals with determining if an artifact needs to be garbage collected,
		// we avoid all lock files, adding them at the end to the list of garbage files.
		expired := diff > ttl
		if !info.IsDir() && info.Mode()&os.ModeSymlink != os.ModeSymlink && filepath.Ext(path) != ".lock" {
			if path != localPath && expired {
				garbageFiles = append(garbageFiles, path)
			}
			totalArtifactFiles += 1
			artifactFilesWithCreatedTs[createdAt] = path
			creationTimestamps = append(creationTimestamps, createdAt)
		}
		return nil

	})
	if len(errors) > 0 {
		return nil, fmt.Errorf("can't walk over file: %s", strings.Join(errors, ","))
	}

	// We already collected enough garbage files to satisfy the no. of max
	// items that are supposed to be retained, so exit early.
	if totalArtifactFiles-len(garbageFiles) < maxItemsToBeRetained {
		return garbageFiles, nil
	}

	// sort all timestamps in ascending order.
	sort.Slice(creationTimestamps, func(i, j int) bool { return creationTimestamps[i].Before(creationTimestamps[j]) })
	for _, ts := range creationTimestamps {
		p, ok := artifactFilesWithCreatedTs[ts]
		if !ok {
			return garbageFiles, fmt.Errorf("failed to fetch file for created ts: %v", ts)
		}
		sortedPaths = append(sortedPaths, p)
	}

	var collected int
	noOfGarbageFiles := len(garbageFiles)
	for _, sortedPath := range sortedPaths {
		if sortedPath != localPath && filepath.Ext(sortedPath) != ".lock" && !stringInSlice(sortedPath, garbageFiles) {
			// If we previously collected some garbage files with an expired ttl, then take that into account
			// when checking whether we need to remove more files to satisfy the max no. of items allowed
			// in the filesystem, along with the no. of files already removed in this loop.
			if noOfGarbageFiles > 0 {
				if (len(sortedPaths) - collected - len(garbageFiles)) > maxItemsToBeRetained {
					garbageFiles = append(garbageFiles, sortedPath)
					collected += 1
				}
			} else {
				if len(sortedPaths)-collected > maxItemsToBeRetained {
					garbageFiles = append(garbageFiles, sortedPath)
					collected += 1
				}
			}
		}
	}

	return garbageFiles, nil
}

// GarbageCollect removes all garbage files in the artifact dir according to the provided retention options.
func (s Storage) GarbageCollect(ctx context.Context, artifact meta.Artifact, timeout time.Duration) ([]string, error) {
	delFilesChan := make(chan []string)
	errChan := make(chan error)
	// Abort if it takes more than the provided timeout duration.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go func() {
		garbageFiles, err := s.GetGarbageFiles(artifact, GarbageCountLimit, s.ArtifactRetentionRecords, s.ArtifactRetentionTTL)
		if err != nil {
			errChan <- err
			return
		}
		var errors []error
		var deleted []string
		if len(garbageFiles) > 0 {
			for _, file := range garbageFiles {
				err := os.Remove(file)
				if err != nil {
					errors = append(errors, err)
				} else {
					deleted = append(deleted, file)
				}
				// If a lock file exists for this garbage artifact, remove that too.
				lockFile := file + ".lock"
				if _, err = os.Lstat(lockFile); err == nil {
					err = os.Remove(lockFile)
					if err != nil {
						errors = append(errors, err)
					}
				}
			}
		}
		if len(errors) > 0 {
			errChan <- kerrors.NewAggregate(errors)
			return
		}
		delFilesChan <- deleted
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case delFiles := <-delFilesChan:
			return delFiles, nil
		case err := <-errChan:
			return nil, err
		}
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
