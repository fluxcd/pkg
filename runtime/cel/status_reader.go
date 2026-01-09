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
	"fmt"

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
	mapper     meta.RESTMapper
	evaluators map[schema.GroupKind]*StatusEvaluator
}

// NewStatusReader returns a new StatusReader for the given GroupKind and healthcheck expressions.
func NewStatusReader(healthchecks []kustomize.CustomHealthCheck) (func(meta.RESTMapper) engine.StatusReader, error) {
	// Build evaluators map.
	evaluators := make(map[schema.GroupKind]*StatusEvaluator, len(healthchecks))
	for i, hc := range healthchecks {
		gk := schema.FromAPIVersionAndKind(hc.APIVersion, hc.Kind).GroupKind()
		if _, ok := evaluators[gk]; ok {
			return nil, fmt.Errorf(
				"duplicate custom health check for GroupKind %s at healthchecks[%d]", gk.String(), i)
		}
		se, err := NewStatusEvaluator(&hc.HealthCheckExpressions)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create custom status evaluator for healthchecks[%d]: %w", i, err)
		}
		evaluators[gk] = se
	}

	return func(mapper meta.RESTMapper) engine.StatusReader {
		return &StatusReader{
			mapper:     mapper,
			evaluators: evaluators,
		}
	}, nil
}

// Supports returns true if the StatusReader supports the given GroupKind.
func (g *StatusReader) Supports(gk schema.GroupKind) bool {
	_, ok := g.evaluators[gk]
	return ok
}

// ReadStatus reads the status of the resource with the given metadata.
func (g *StatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader,
	resource object.ObjMetadata) (*event.ResourceStatus, error) {

	if !g.Supports(resource.GroupKind) {
		return nil, fmt.Errorf("the GroupKind %s is not supported", resource.GroupKind.String())
	}

	return g.genericStatusReader(ctx, resource.GroupKind).ReadStatus(ctx, reader, resource)
}

// ReadStatusForObject reads the status of the given resource.
func (g *StatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader,
	resource *unstructured.Unstructured) (*event.ResourceStatus, error) {

	// Compute GroupKind.
	apiVersion, ok, _ := unstructured.NestedFieldCopy(resource.Object, "apiVersion")
	if !ok {
		return nil, fmt.Errorf("resource is missing apiVersion field")
	}
	kind, ok, _ := unstructured.NestedFieldCopy(resource.Object, "kind")
	if !ok {
		return nil, fmt.Errorf("resource is missing kind field")
	}
	gk := schema.FromAPIVersionAndKind(apiVersion.(string), kind.(string)).GroupKind()
	if !g.Supports(gk) {
		return nil, fmt.Errorf("the GroupKind %s is not supported", gk.String())
	}

	return g.genericStatusReader(ctx, gk).ReadStatusForObject(ctx, reader, resource)
}

// genericStatusReader returns the underlying generic status reader.
func (g *StatusReader) genericStatusReader(ctx context.Context, gk schema.GroupKind) engine.StatusReader {
	statusFunc := func(u *unstructured.Unstructured) (*status.Result, error) {
		return g.evaluators[gk].Evaluate(ctx, u)
	}
	return kstatusreaders.NewGenericStatusReader(g.mapper, statusFunc)
}
