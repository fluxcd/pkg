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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fluxcd/pkg/apis/kustomize"
)

// PollerWithCustomHealthChecks creates a list of constructors for
// custom status readers from a list of custom health checks. If
// there are multiple healthchecks defined for the same GroupKind,
// only the first one is used. The context is used to control the
// execution of the underlying status readers.
func PollerWithCustomHealthChecks(ctx context.Context,
	healthchecks []kustomize.CustomHealthCheck) ([]func(meta.RESTMapper) engine.StatusReader, error) {

	if len(healthchecks) == 0 {
		return nil, nil
	}

	ctors := make([]func(meta.RESTMapper) engine.StatusReader, 0, len(healthchecks))
	types := make(map[schema.GroupKind]struct{}, len(healthchecks))
	for i, hc := range healthchecks {
		gk := schema.FromAPIVersionAndKind(hc.APIVersion, hc.Kind).GroupKind()
		if _, ok := types[gk]; !ok {
			se, err := NewStatusEvaluator(&hc.HealthCheckExpressions)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to create custom status evaluator for healthchecks[%d]: %w", i, err)
			}

			ctors = append(ctors, func(mapper meta.RESTMapper) engine.StatusReader {
				return NewStatusReader(ctx, mapper, gk, se)
			})
			types[gk] = struct{}{}
		}
	}

	return ctors, nil
}
