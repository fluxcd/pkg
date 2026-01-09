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
)

// StatusReader implements the engine.StatusReader interface for a specific GroupKind and
// set of healthcheck expressions.
type StatusReader struct {
	gk     schema.GroupKind
	se     *StatusEvaluator
	mapper meta.RESTMapper
}

// NewStatusReader returns a new StatusReader for the given GroupKind and healthcheck expressions.
func NewStatusReader(mapper meta.RESTMapper, gk schema.GroupKind, se *StatusEvaluator) engine.StatusReader {
	return &StatusReader{
		gk:     gk,
		se:     se,
		mapper: mapper,
	}
}

// Supports returns true if the StatusReader supports the given GroupKind.
func (g *StatusReader) Supports(gk schema.GroupKind) bool {
	return gk == g.gk
}

// ReadStatus reads the status of the resource with the given metadata.
func (g *StatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader,
	resource object.ObjMetadata) (*event.ResourceStatus, error) {
	return g.genericStatusReader(ctx).ReadStatus(ctx, reader, resource)
}

// ReadStatusForObject reads the status of the given resource.
func (g *StatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader,
	resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return g.genericStatusReader(ctx).ReadStatusForObject(ctx, reader, resource)
}

// genericStatusReader returns the underlying generic status reader.
func (g *StatusReader) genericStatusReader(ctx context.Context) engine.StatusReader {
	statusFunc := func(u *unstructured.Unstructured) (*status.Result, error) {
		return g.se.Evaluate(ctx, u)
	}
	return kstatusreaders.NewGenericStatusReader(g.mapper, statusFunc)
}
