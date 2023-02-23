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

package git

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSecurePath(t *testing.T) {
	g := NewWithT(t)

	tmp := t.TempDir()
	securePath, err := SecurePath(tmp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(securePath).To(Equal(tmp))

	wd, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())

	rel := "./relative"
	securePath, err = SecurePath(rel)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(securePath).To(Equal(filepath.Join(wd, "relative")))

	base := "../../outside"
	securePath, err = SecurePath(base)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(securePath).To(Equal(filepath.Join(wd, "outside")))
}

func TestTransformRevision(t *testing.T) {
	tests := []struct {
		name string
		rev  string
		want string
	}{
		{
			name: "revision with branch and digest",
			rev:  "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "revision with digest",
			rev:  "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "revision with slash branch and digest",
			rev:  "feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "legacy revision with branch and hash",
			rev:  "main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "legacy revision with slash branch and hash",
			rev:  "feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "legacy revision with hash",
			rev:  "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "legacy revision with HEAD named pointer and hash",
			rev:  "HEAD/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "empty revision",
			rev:  "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := TransformRevision(tt.rev)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestSplitRevision(t *testing.T) {
	tests := []struct {
		name        string
		rev         string
		wantPointer string
		wantHash    Hash
	}{
		{
			name:        "revision with branch and digest",
			rev:         "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			wantPointer: "main",
			wantHash:    Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name:        "revision with reference name and digest",
			rev:         "refs/pull/420/head@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			wantPointer: "refs/pull/420/head",
			wantHash:    Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name:     "revision with digest",
			rev:      "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			wantHash: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name:        "revision with slash branch and digest",
			rev:         "feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			wantPointer: "feature/branch",
			wantHash:    Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name:        "legacy revision with branch and hash",
			rev:         "main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			wantPointer: "main",
			wantHash:    Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name:     "legacy revision with hash",
			rev:      "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			wantHash: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name:        "empty revision",
			rev:         "",
			wantPointer: "",
			wantHash:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			p, h := SplitRevision(tt.rev)
			g.Expect(p).To(Equal(tt.wantPointer))
			g.Expect(h).To(Equal(tt.wantHash))
		})
	}
}

func TestExtractNamedPointerFromRevision(t *testing.T) {
	tests := []struct {
		name string
		rev  string
		want string
	}{
		{
			name: "revision with branch and digest",
			rev:  "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "main",
		},
		{
			name: "revision with ref name and digest",
			rev:  "refs/merge-request/1/head@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "refs/merge-request/1/head",
		},
		{
			name: "revision with digest",
			rev:  "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "",
		},
		{
			name: "revision with slash branch and digest",
			rev:  "feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "feature/branch",
		},
		{
			name: "legacy revision with branch and hash",
			rev:  "main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "main",
		},
		{
			name: "legacy revision with slash branch and hash",
			rev:  "feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "feature/branch",
		},
		{
			name: "legacy revision with hash",
			rev:  "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "",
		},
		{
			name: "legacy revision with HEAD named pointer and hash",
			rev:  "HEAD/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(ExtractNamedPointerFromRevision(tt.rev)).To(Equal(tt.want))
		})
	}
}

func TestExtractHashFromRevision(t *testing.T) {
	tests := []struct {
		name string
		rev  string
		want Hash
	}{
		{
			name: "revision with branch and digest",
			rev:  "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name: "revision with ref name and digest",
			rev:  "refs/pull/1/head@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name: "revision with digest",
			rev:  "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name: "revision with slash branch and digest",
			rev:  "feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name: "legacy revision with branch and hash",
			rev:  "main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name: "legacy revision with slash branch and hash",
			rev:  "feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
		{
			name: "legacy revision with hash",
			rev:  "5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
			want: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(ExtractHashFromRevision(tt.rev)).To(Equal(tt.want))
		})
	}
}
