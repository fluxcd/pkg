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

// This package was originally copied from the `go-git` project:
//
//	https://github.com/go-git/go-git/tree/v5.16.4/plumbing/format/gitignore
package gitignore

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestSimpleMatch_inclusion(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("!vul?ano", nil)
	r := p.Match([]string{"value", "vulkano", "tail"}, false)
	g.Expect(r).To(Equal(Include))
}

func TestMatch_domainLonger_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value", []string{"head", "middle", "tail"})
	r := p.Match([]string{"head", "middle"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestMatch_domainSameLength_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value", []string{"head", "middle", "tail"})
	r := p.Match([]string{"head", "middle", "tail"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestMatch_domainMismatch_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value", []string{"head", "middle", "tail"})
	r := p.Match([]string{"head", "middle", "_tail_", "value"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestSimpleMatch_withDomain(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("middle/", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "middle", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_onlyMatchInDomain_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("volcano/", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "tail"}, true)
	g.Expect(r).To(Equal(NoMatch))
}

func TestSimpleMatch_atStart(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value", nil)
	r := p.Match([]string{"value", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_inTheMiddle(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value", nil)
	r := p.Match([]string{"head", "value", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_atEnd(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value", nil)
	r := p.Match([]string{"head", "value"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_atStart_dirWanted(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value/", nil)
	r := p.Match([]string{"value", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_inTheMiddle_dirWanted(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value/", nil)
	r := p.Match([]string{"head", "value", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_atEnd_dirWanted(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value/", nil)
	r := p.Match([]string{"head", "value"}, true)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_atEnd_dirWanted_notADir_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value/", nil)
	r := p.Match([]string{"head", "value"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestSimpleMatch_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value", nil)
	r := p.Match([]string{"head", "val", "tail"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestSimpleMatch_valueLonger_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("val", nil)
	r := p.Match([]string{"head", "value", "tail"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestSimpleMatch_withAsterisk(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("v*o", nil)
	r := p.Match([]string{"value", "vulkano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_withQuestionMark(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("vul?ano", nil)
	r := p.Match([]string{"value", "vulkano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_magicChars(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("v[ou]l[kc]ano", nil)
	r := p.Match([]string{"value", "volcano"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestSimpleMatch_wrongPattern_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("v[ou]l[", nil)
	r := p.Match([]string{"value", "vol["}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_fromRootWithSlash(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/value/vul?ano", nil)
	r := p.Match([]string{"value", "vulkano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_withDomain(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("middle/tail/", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "middle", "tail"}, true)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_onlyMatchInDomain_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("volcano/tail", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "tail"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_fromRootWithoutSlash(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value/vul?ano", nil)
	r := p.Match([]string{"value", "vulkano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_fromRoot_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value/vulkano", nil)
	r := p.Match([]string{"value", "volcano"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_fromRoot_tooShort_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("value/vul?ano", nil)
	r := p.Match([]string{"value"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_fromRoot_notAtRoot_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/value/volcano", nil)
	r := p.Match([]string{"value", "value", "volcano"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_leadingAsterisks_atStart(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/*lue/vol?ano", nil)
	r := p.Match([]string{"value", "volcano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_leadingAsterisks_notAtStart(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/*lue/vol?ano", nil)
	r := p.Match([]string{"head", "value", "volcano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_leadingAsterisks_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/*lue/vol?ano", nil)
	r := p.Match([]string{"head", "value", "Volcano", "tail"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_leadingAsterisks_isDir(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/*lue/vol?ano/", nil)
	r := p.Match([]string{"head", "value", "volcano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_leadingAsterisks_isDirAtEnd(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/*lue/vol?ano/", nil)
	r := p.Match([]string{"head", "value", "volcano"}, true)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_leadingAsterisks_isDir_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/*lue/vol?ano/", nil)
	r := p.Match([]string{"head", "value", "Colcano"}, true)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_leadingAsterisks_isDirNoDirAtEnd_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/*lue/vol?ano/", nil)
	r := p.Match([]string{"head", "value", "volcano"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_tailingAsterisks(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/vol?ano/**", nil)
	r := p.Match([]string{"value", "volcano", "tail", "moretail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_tailingAsterisks_exactMatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/vol?ano/**", nil)
	r := p.Match([]string{"value", "volcano"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_middleAsterisks_emptyMatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/**/vol?ano", nil)
	r := p.Match([]string{"value", "volcano"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_middleAsterisks_oneMatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/**/vol?ano", nil)
	r := p.Match([]string{"value", "middle", "volcano"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_middleAsterisks_multiMatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/**/vol?ano", nil)
	r := p.Match([]string{"value", "middle1", "middle2", "volcano"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_middleAsterisks_isDir_trailing(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/**/vol?ano/", nil)
	r := p.Match([]string{"value", "middle1", "middle2", "volcano"}, true)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_middleAsterisks_isDir_trailing_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/**/vol?ano/", nil)
	r := p.Match([]string{"value", "middle1", "middle2", "volcano"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_middleAsterisks_isDir(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/**/vol?ano/", nil)
	r := p.Match([]string{"value", "middle1", "middle2", "volcano", "tail"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_wrongDoubleAsterisk_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/*lue/**foo/vol?ano", nil)
	r := p.Match([]string{"value", "foo", "volcano", "tail"}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_magicChars(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/head/v[ou]l[kc]ano", nil)
	r := p.Match([]string{"value", "head", "volcano"}, false)
	g.Expect(r).To(Equal(Exclude))
}

func TestGlobMatch_wrongPattern_noTraversal_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/head/v[ou]l[", nil)
	r := p.Match([]string{"value", "head", "vol["}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_wrongPattern_onTraversal_mismatch(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("/value/**/v[ou]l[", nil)
	r := p.Match([]string{"value", "head", "vol["}, false)
	g.Expect(r).To(Equal(NoMatch))
}

func TestGlobMatch_issue_923(t *testing.T) {
	g := NewWithT(t)
	p := ParsePattern("**/android/**/GeneratedPluginRegistrant.java", nil)
	r := p.Match([]string{"packages", "flutter_tools", "lib", "src", "android", "gradle.dart"}, false)
	g.Expect(r).To(Equal(NoMatch))
}
