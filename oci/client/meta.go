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

package client

import (
	"fmt"

	"github.com/fluxcd/pkg/oci"
)

// Metadata holds the upstream information about on artifact's source.
// https://github.com/opencontainers/image-spec/blob/main/annotations.md
type Metadata struct {
	Created     string            `json:"created"`
	Source      string            `json:"source_url"`
	Revision    string            `json:"source_revision"`
	Digest      string            `json:"digest"`
	URL         string            `json:"url"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ToAnnotations returns the OpenContainers annotations map.
func (m *Metadata) ToAnnotations() map[string]string {
	annotations := map[string]string{
		oci.CreatedAnnotation:  m.Created,
		oci.SourceAnnotation:   m.Source,
		oci.RevisionAnnotation: m.Revision,
	}

	for k, v := range m.Annotations {
		annotations[k] = v
	}

	return annotations
}

// MetadataFromAnnotations parses the OpenContainers annotations and returns a Metadata object.
func MetadataFromAnnotations(annotations map[string]string) (*Metadata, error) {
	created, ok := annotations[oci.CreatedAnnotation]
	if !ok {
		return nil, fmt.Errorf("'%s' annotation not found", oci.CreatedAnnotation)
	}

	source, ok := annotations[oci.SourceAnnotation]
	if !ok {
		return nil, fmt.Errorf("'%s' annotation not found", oci.SourceAnnotation)
	}

	revision, ok := annotations[oci.RevisionAnnotation]
	if !ok {
		return nil, fmt.Errorf("'%s' annotation not found", oci.RevisionAnnotation)
	}

	m := Metadata{
		Created:     created,
		Source:      source,
		Revision:    revision,
		Annotations: annotations,
	}

	return &m, nil
}
