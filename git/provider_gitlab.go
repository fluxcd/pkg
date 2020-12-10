/*
Copyright 2020 The Flux authors

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

package git

import (
	"context"
	"fmt"

	"github.com/xanzy/go-gitlab"
)

// GitLabProvider represents a GitLab API wrapper
type GitLabProvider struct {
	IsPrivate  bool
	IsPersonal bool
}

const (
	GitLabTokenName       = "GITLAB_TOKEN"
	GitLabDefaultHostname = "gitlab.com"
)

func (p *GitLabProvider) newClient(r *Repository) (*gitlab.Client, error) {
	gl, err := gitlab.NewClient(r.Token)
	if err != nil {
		return nil, err
	}

	if r.Host != GitLabDefaultHostname {
		gl, err = gitlab.NewClient(r.Token, gitlab.WithBaseURL(fmt.Sprintf("https://%s/api/v4", r.Host)))
		if err != nil {
			return nil, err
		}
	}
	return gl, nil
}

// CreateRepository returns false if the repository already exists
func (p *GitLabProvider) CreateRepository(ctx context.Context, r *Repository) (bool, error) {
	gl, err := p.newClient(r)
	if err != nil {
		return false, fmt.Errorf("client error: %w", err)
	}

	gid, projects, err := p.getProjects(ctx, gl, r)
	if err != nil {
		return false, fmt.Errorf("failed to list projects, error: %w", err)
	}

	if len(projects) > 0 {
		return false, nil
	}

	visibility := gitlab.PublicVisibility
	if p.IsPrivate {
		visibility = gitlab.PrivateVisibility
	}

	cpo := &gitlab.CreateProjectOptions{
		Name:                 gitlab.String(r.Name),
		NamespaceID:          gid,
		Visibility:           &visibility,
		InitializeWithReadme: gitlab.Bool(true),
	}
	_, _, err = gl.Projects.CreateProject(cpo)
	if err != nil {
		return false, fmt.Errorf("failed to create project, error: %w", err)
	}
	return true, nil
}

// AddTeam returns false if the team is already assigned to the repository
func (p *GitLabProvider) AddTeam(ctx context.Context, r *Repository, name, permission string) (bool, error) {
	return false, nil
}

// AddDeployKey returns false if the key exists and the content is the same
func (p *GitLabProvider) AddDeployKey(ctx context.Context, r *Repository, key, keyName string) (bool, error) {
	gl, err := p.newClient(r)
	if err != nil {
		return false, fmt.Errorf("client error: %w", err)
	}

	// list deploy keys
	var projID int
	_, projects, err := p.getProjects(ctx, gl, r)
	if err != nil {
		return false, fmt.Errorf("failed to list projects, error: %w", err)
	}
	if len(projects) > 0 {
		projID = projects[0].ID
	} else {
		return false, fmt.Errorf("no project found")
	}

	// check if the key exists
	keys, _, err := gl.DeployKeys.ListProjectDeployKeys(projID, &gitlab.ListProjectDeployKeysOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list deploy keys, error: %w", err)
	}

	shouldCreateKey := true
	var existingKey *gitlab.DeployKey
	for _, k := range keys {
		if k.Title == keyName {
			if k.Key != key {
				existingKey = k
			} else {
				shouldCreateKey = false
			}
			break
		}
	}

	// delete existing key if the value differs
	if existingKey != nil {
		_, err := gl.DeployKeys.DeleteDeployKey(projID, existingKey.ID, gitlab.WithContext(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to delete deploy key '%s', error: %w", keyName, err)
		}
	}

	// create key
	if shouldCreateKey {
		_, _, err := gl.DeployKeys.AddDeployKey(projID, &gitlab.AddDeployKeyOptions{
			Title:   gitlab.String(keyName),
			Key:     gitlab.String(key),
			CanPush: gitlab.Bool(false),
		}, gitlab.WithContext(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to create deploy key '%s', error: %w", keyName, err)
		}
		return true, nil
	}

	return false, nil
}

// DeleteRepository is not supported by GitLab
func (p *GitLabProvider) DeleteRepository(ctx context.Context, r *Repository) error {
	return fmt.Errorf("repository deletion is not supported by the GitLab API")
}

// getProjects retrieves the list of GitLab projects based on the provided owner type (personal or group)
func (p *GitLabProvider) getProjects(ctx context.Context, gl *gitlab.Client, r *Repository) (*int, []*gitlab.Project, error) {
	var (
		gid      *int
		projects []*gitlab.Project
		err      error
	)
	if !p.IsPersonal {
		lgo := &gitlab.ListGroupsOptions{
			Search:         gitlab.String(r.Owner),
			MinAccessLevel: gitlab.AccessLevel(gitlab.GuestPermissions),
		}
		groups, _, err := gl.Groups.ListGroups(lgo, gitlab.WithContext(ctx))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list groups, error: %w", err)
		}

		if len(groups) == 0 {
			return nil, nil, fmt.Errorf("failed to find group named '%s'", r.Owner)
		}
		gid = &groups[0].ID

		lpo := &gitlab.ListGroupProjectsOptions{
			Search: gitlab.String(r.Name),
		}
		projects, _, err = gl.Groups.ListGroupProjects(*gid, lpo, gitlab.WithContext(ctx))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list projects, error: %w", err)
		}
	} else {
		lpo := &gitlab.ListProjectsOptions{
			Search: gitlab.String(r.Name),
			Owned:  gitlab.Bool(true),
		}
		projects, _, err = gl.Projects.ListProjects(lpo, gitlab.WithContext(ctx))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list projects, error: %w", err)
		}
	}

	return gid, projects, nil
}
