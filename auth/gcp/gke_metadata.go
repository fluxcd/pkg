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

package gcp

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/compute/metadata"
)

type gkeMetadataLoader struct {
	projectID string
	location  string
	name      string

	mu     sync.RWMutex
	loaded bool
}

var gkeMetadata gkeMetadataLoader

func (g *gkeMetadataLoader) getAudience(ctx context.Context) (string, error) {
	if err := g.load(ctx); err != nil {
		return "", err
	}
	wiPool, _ := g.workloadIdentityPool(ctx)
	wiProvider, _ := g.workloadIdentityProvider(ctx)
	return fmt.Sprintf("identitynamespace:%s:%s", wiPool, wiProvider), nil
}

func (g *gkeMetadataLoader) workloadIdentityPool(ctx context.Context) (string, error) {
	if err := g.load(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.svc.id.goog", g.projectID), nil
}

func (g *gkeMetadataLoader) workloadIdentityProvider(ctx context.Context) (string, error) {
	if err := g.load(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s",
		g.projectID,
		g.location,
		g.name), nil
}

// load loads the GKE cluster metadata from the metadata service, assuming the
// pod is running on a GKE node/pod. It will fail otherwise, and this
// is the reason why this method should be called lazily. If this code ran on any
// other cluster that is not GKE it would fail consistently and throw the pods
// in crash loop if running on startup. This method is thread-safe and will
// only load the metadata successfully once.
//
// Technically we could receive options here to use a custom HTTP client with
// a proxy, but this proxy is configured at the object level and here we are
// loading cluster-level metadata that doesn't change during the lifetime of
// the pod. So we can't use an object-level proxy here. Furthermore, this
// implementation targets specifically GKE clusters, and in such clusters the
// metadata server is usually a DaemonSet pod that serves only node-local
// traffic, so a proxy doesn't make sense here anyway.
func (g *gkeMetadataLoader) load(ctx context.Context) error {
	// Bail early if the metadata was already loaded.
	g.mu.RLock()
	loaded := g.loaded
	g.mu.RUnlock()
	if loaded {
		return nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Check again if the metadata was loaded while we were waiting for the lock.
	if g.loaded {
		return nil
	}

	client := metadata.NewClient(nil)

	projectID, err := client.GetWithContext(ctx, "project/project-id")
	if err != nil {
		return fmt.Errorf("failed to get GKE cluster project ID from the metadata service: %w", err)
	}
	if projectID == "" {
		return fmt.Errorf("failed to get GKE cluster project ID from the metadata service: empty value")
	}

	location, err := client.GetWithContext(ctx, "instance/attributes/cluster-location")
	if err != nil {
		return fmt.Errorf("failed to get GKE cluster location from the metadata service: %w", err)
	}
	if location == "" {
		return fmt.Errorf("failed to get GKE cluster location from the metadata service: empty value")
	}

	name, err := client.GetWithContext(ctx, "instance/attributes/cluster-name")
	if err != nil {
		return fmt.Errorf("failed to get GKE cluster name from the metadata service: %w", err)
	}
	if name == "" {
		return fmt.Errorf("failed to get GKE cluster name from the metadata service: empty value")
	}

	g.projectID = projectID
	g.location = location
	g.name = name
	g.loaded = true

	return nil
}
