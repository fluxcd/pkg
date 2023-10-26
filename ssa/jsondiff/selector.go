/*
Copyright 2019 The Kubernetes Authors
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

Much of the code in this file is derived from Kustomize:

https://github.com/kubernetes-sigs/kustomize/blob/4b34ff3075c79b0d52493cdc60cf45e075f77372/api/types/selector.go
https://github.com/kubernetes-sigs/kustomize/blob/fb7ee2f4871d4ef054ecd9d2e1bc9b10cbfde4a9/kyaml/yaml/rnode.go#L1154-L1170
*/

package jsondiff

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// Selector is a struct that contains the information needed to select a
// Kubernetes resource. All fields are optional.
type Selector struct {
	// Group defines a regular expression to filter resources by their
	// API group.
	Group string

	// Version defines a regular expression to filter resources by their
	// API version.
	Version string

	// Kind defines a regular expression to filter resources by their
	// API kind.
	Kind string

	// Name defines a regular expression to filter resources by their
	// name.
	Name string

	// Namespace defines a regular expression to filter resources by their
	// namespace.
	Namespace string

	// AnnotationSelector defines a selector to filter resources by their
	// annotations in the format of a label selection expression.
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	AnnotationSelector string

	// LabelSelector defines a selector to filter resources by their labels
	// in the format of a label selection expression.
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	LabelSelector string
}

// SelectorRegex is a struct that contains the regular expressions needed to
// select a Kubernetes resource.
type SelectorRegex struct {
	selector           *Selector
	groupRegex         *regexp.Regexp
	versionRegex       *regexp.Regexp
	kindRegex          *regexp.Regexp
	nameRegex          *regexp.Regexp
	namespaceRegex     *regexp.Regexp
	labelSelector      labels.Selector
	annotationSelector labels.Selector
}

// NewSelectorRegex returns a pointer to a new SelectorRegex
// which uses the same condition as s.
func NewSelectorRegex(s *Selector) (sr *SelectorRegex, err error) {
	if s == nil {
		return nil, nil
	}

	sr = &SelectorRegex{
		selector: s,
	}

	sr.groupRegex, err = regexp.Compile(anchorRegex(s.Group))
	if err != nil {
		return nil, fmt.Errorf("invalid group regex: %w", err)
	}
	sr.versionRegex, err = regexp.Compile(anchorRegex(s.Version))
	if err != nil {
		return nil, fmt.Errorf("invalid version regex: %w", err)
	}
	sr.kindRegex, err = regexp.Compile(anchorRegex(s.Kind))
	if err != nil {
		return nil, fmt.Errorf("invalid kind regex: %w", err)
	}
	sr.nameRegex, err = regexp.Compile(anchorRegex(s.Name))
	if err != nil {
		return nil, fmt.Errorf("invalid name regex: %w", err)
	}
	sr.namespaceRegex, err = regexp.Compile(anchorRegex(s.Namespace))
	if err != nil {
		return nil, fmt.Errorf("invalid namespace regex: %w", err)
	}

	if s.LabelSelector != "" {
		sr.labelSelector, err = labels.Parse(s.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}
	}
	if s.AnnotationSelector != "" {
		sr.annotationSelector, err = labels.Parse(s.AnnotationSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid annotation selector: %w", err)
		}
	}

	return sr, nil
}

// MatchUnstructured returns true if the unstructured object matches all the
// conditions in the selector. If the selector is nil, it returns true.
func (s *SelectorRegex) MatchUnstructured(obj *unstructured.Unstructured) bool {
	if s == nil {
		return true
	}

	if !s.MatchNamespace(obj.GetNamespace()) {
		return false
	}

	if !s.MatchName(obj.GetName()) {
		return false
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if !s.MatchGVK(gvk.Group, gvk.Version, gvk.Kind) {
		return false
	}

	if !s.MatchLabelSelector(obj.GetLabels()) {
		return false
	}

	if !s.MatchAnnotationSelector(obj.GetAnnotations()) {
		return false
	}

	return true
}

// MatchGVK returns true if the group, version and kind in selector are empty
// or the group, version and kind match the group, version and kind in selector.
// If the selector is nil, it returns true.
func (s *SelectorRegex) MatchGVK(group, version, kind string) bool {
	if s == nil {
		return true
	}

	if len(s.selector.Group) > 0 {
		if !s.groupRegex.MatchString(group) {
			return false
		}
	}
	if len(s.selector.Version) > 0 {
		if !s.versionRegex.MatchString(version) {
			return false
		}
	}
	if len(s.selector.Kind) > 0 {
		if !s.kindRegex.MatchString(kind) {
			return false
		}
	}
	return true
}

// MatchName returns true if the name in selector is empty or the name matches
// the name in selector. If the selector is nil, it returns true.
func (s *SelectorRegex) MatchName(n string) bool {
	if s == nil || s.selector.Name == "" {
		return true
	}
	return s.nameRegex.MatchString(n)
}

// MatchNamespace returns true if the namespace in selector is empty or the
// namespace matches the namespace in selector. If the selector is nil, it
// returns true.
func (s *SelectorRegex) MatchNamespace(ns string) bool {
	if s == nil || s.selector.Namespace == "" {
		return true
	}
	return s.namespaceRegex.MatchString(ns)
}

// MatchAnnotationSelector returns true if the annotation selector in selector
// is empty or the annotation selector matches the annotations in selector.
// If the selector is nil, it returns true.
func (s *SelectorRegex) MatchAnnotationSelector(a map[string]string) bool {
	if s == nil || s.selector.AnnotationSelector == "" {
		return true
	}
	return s.annotationSelector.Matches(labels.Set(a))
}

// MatchLabelSelector returns true if the label selector in selector is empty
// or the label selector matches the labels in selector. If the selector is
// nil, it returns true.
func (s *SelectorRegex) MatchLabelSelector(l map[string]string) bool {
	if s == nil || s.selector.LabelSelector == "" {
		return true
	}
	return s.labelSelector.Matches(labels.Set(l))
}

func anchorRegex(pattern string) string {
	if pattern == "" {
		return pattern
	}
	return "^(?:" + pattern + ")$"
}
