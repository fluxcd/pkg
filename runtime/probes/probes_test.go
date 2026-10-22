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

package probes

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestLeaderElectedReadyz(t *testing.T) {
	g := NewWithT(t)

	check := LeaderElectedReadyz(make(chan struct{}))
	g.Expect(check(nil)).To(MatchError(ContainSubstring("not the leader")))

	elected := make(chan struct{})
	close(elected)
	check = LeaderElectedReadyz(elected)
	g.Expect(check(nil)).To(Succeed())
}
