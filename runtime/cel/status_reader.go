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

package cel

import (
	"context"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	kstatusreaders "github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fluxcd/pkg/apis/kustomize"
)

// StatusReader implements the engine.StatusReader interface for a specific GroupKind and
// set of healthcheck expressions.
type StatusReader struct {
	genericStatusReader engine.StatusReader
	gk                  schema.GroupKind
}

// NewStatusReader returns a new StatusReader for the given GroupKind and healthcheck expressions.
// The context is used to control the execution of the underlying operations performed by the
// the reader.
func NewStatusReader(ctx context.Context, mapper meta.RESTMapper, gk schema.GroupKind,
	exprs *kustomize.HealthCheckExpressions) (engine.StatusReader, error) {

	s, err := NewStatusEvaluator(exprs)
	if err != nil {
		return nil, err
	}

	statusFunc := func(u *unstructured.Unstructured) (*status.Result, error) {
		return s.Evaluate(ctx, u)
	}

	genericStatusReader := kstatusreaders.NewGenericStatusReader(mapper, statusFunc)
	return &StatusReader{
		genericStatusReader: genericStatusReader,
		gk:                  gk,
	}, nil
}

// Supports returns true if the StatusReader supports the given GroupKind.
func (g *StatusReader) Supports(gk schema.GroupKind) bool {
	return gk == g.gk
}

// ReadStatus reads the status of the resource with the given metadata.
func (g *StatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader,
	resource object.ObjMetadata) (*event.ResourceStatus, error) {
	return g.genericStatusReader.ReadStatus(ctx, reader, resource)
}

// ReadStatusForObject reads the status of the given resource.
func (g *StatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader,
	resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return g.genericStatusReader.ReadStatusForObject(ctx, reader, resource)
}
