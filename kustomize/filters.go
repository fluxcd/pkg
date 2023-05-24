/*
Copyright 2023 The Flux authors

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

package kustomize

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/fluxcd/pkg/sourceignore"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	kustypes "sigs.k8s.io/kustomize/api/types"
)

const (
	crds       = "crds"
	resources  = "resources"
	components = "components"
)

// filter must return true if a file should not be included in the archive after inspecting the given path
// and/or os.FileInfo.
type filter func(p string, fi os.FileInfo) bool

func ignoreFileFilter(ps []gitignore.Pattern, domain []string) filter {
	matcher := sourceignore.NewDefaultMatcher(ps, domain)
	return func(p string, fi os.FileInfo) bool {
		return matcher.Match(strings.Split(p, string(filepath.Separator)), fi.IsDir())
	}
}

func filterKsWithIgnoreFiles(ks *kustypes.Kustomization, dirPath string, ignore string) error {
	path, err := filepath.Abs(dirPath)
	if err != nil {
		return err
	}

	ignoreDomain := strings.Split(path, string(filepath.Separator))
	ps, err := sourceignore.LoadIgnorePatterns(path, ignoreDomain)
	if err != nil {
		return err
	}

	if ignore != "" {
		ps = append(ps, sourceignore.ReadPatterns(strings.NewReader(ignore), ignoreDomain)...)
	}

	// filter resources first
	err = filterSlice(ks, path, &ks.Resources, resources, ignoreFileFilter(ps, ignoreDomain))
	if err != nil {
		return err
	}

	// filter components second
	err = filterSlice(ks, path, &ks.Components, components, ignoreFileFilter(ps, ignoreDomain))
	if err != nil {
		return err
	}

	// filter crds third
	err = filterSlice(ks, path, &ks.Crds, crds, ignoreFileFilter(ps, ignoreDomain))
	if err != nil {
		return err
	}

	return nil
}

func filterSlice(ks *kustypes.Kustomization, path string, s *[]string, t string, filter filter) error {
	start := 0
	for _, res := range *s {
		// check if we have a url and skip it
		// this is not needed for crds as they are not allowed to be urls
		if t != crds {
			if u, err := url.ParseRequestURI(res); err == nil && u.Scheme != "" {
				continue
			}
		}
		f := filepath.Join(path, res)
		info, err := os.Lstat(f)
		if err != nil {
			return err
		}
		if filter(f, info) {
			continue
		}
		(*s)[start] = res
		start++
	}
	*s = (*s)[:start]
	return nil
}
