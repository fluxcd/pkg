//go:build integration
// +build integration

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

package integration

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	oci "github.com/fluxcd/pkg/oci/client"
)

func TestDelete(t *testing.T) {
	// delete doesn't work with AWS
	if *targetProvider == "aws" {
		return
	}
	g := NewWithT(t)
	c := oci.NewLocalClient()

	for _, repo := range testRepos {
		tags, err := c.List(context.Background(), repo, oci.ListOptions{
			RegexFilter: deleteTag,
		})
		g.Expect(err).To(BeNil())
		g.Expect(len(tags)).To(Equal(1))

		err = c.Delete(context.Background(), fmt.Sprintf("%s:%s", repo, "v0.1.0"))
		if err != nil {
			t.Errorf("unexpected err: %v", err)
		}

		tags, err = c.List(context.Background(), repo, oci.ListOptions{
			RegexFilter: "v0.1.0",
		})
		g.Expect(err).To(BeNil())
		g.Expect(len(tags)).To(BeZero())
	}
}
