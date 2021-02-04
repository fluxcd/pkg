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
	"strings"

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

	var (
		gid     *int
		project *gitlab.Project
	)
	if p.IsPersonal {
		project, err = p.getPersonalProject(ctx, gl, r)
	} else {
		gid, project, err = p.getGroupProject(ctx, gl, r)
	}
	if err != nil {
		return false, fmt.Errorf("failed to get project, error: %w", err)
	}
	if project != nil {
		return false, nil
	}

	visibility := gitlab.PublicVisibility
	if p.IsPrivate {
		visibility = gitlab.PrivateVisibility
	}

	cpo := &gitlab.CreateProjectOptions{
		Path:                 gitlab.String(r.Name),
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
	var project *gitlab.Project
	if p.IsPersonal {
		project, err = p.getPersonalProject(ctx, gl, r)
	} else {
		_, project, err = p.getGroupProject(ctx, gl, r)
	}
	if err != nil {
		return false, fmt.Errorf("failed to get project, error: %w", err)
	}
	if project == nil {
		return false, fmt.Errorf("no project found")
	}

	// check if the key exists
	keys, _, err := gl.DeployKeys.ListProjectDeployKeys(project.ID, &gitlab.ListProjectDeployKeysOptions{})
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
		_, err := gl.DeployKeys.DeleteDeployKey(project.ID, existingKey.ID, gitlab.WithContext(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to delete deploy key '%s', error: %w", keyName, err)
		}
	}

	// create key
	if shouldCreateKey {
		_, _, err := gl.DeployKeys.AddDeployKey(project.ID, &gitlab.AddDeployKeyOptions{
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

func (p *GitLabProvider) getPersonalProject(ctx context.Context, gl *gitlab.Client, r *Repository) (*gitlab.Project, error) {
	luo := &gitlab.ListUsersOptions{
		Search: gitlab.String(r.Owner),
	}
	users, _, err := gl.Users.ListUsers(luo, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list users, error: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("failed to find user '%s'", r.Owner)
	}

	lpo := &gitlab.ListProjectsOptions{
		Search: gitlab.String(r.Name),
		Owned:  gitlab.Bool(true),
	}
	projects, _, err := gl.Projects.ListUserProjects(users[0].ID, lpo, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list projects, error: %w", err)
	}
	if len(projects) == 0 {
		return nil, nil
	}
	return projects[0], nil
}

func (p *GitLabProvider) getGroupProject(ctx context.Context, gl *gitlab.Client, r *Repository) (*int, *gitlab.Project, error) {
	var (
		gid      *int
		projects []*gitlab.Project
		err      error
	)

	groupAndSubGroupPaths := strings.Split(r.Owner, "/")

	lgo := &gitlab.ListGroupsOptions{
		Search: gitlab.String(groupAndSubGroupPaths[0]),
	}
	groups, _, err := gl.Groups.ListGroups(lgo, gitlab.WithContext(ctx))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list groups, error: %w", err)
	}

	group := findGroupByPath(groups, groupAndSubGroupPaths[0])
	if len(groups) == 0 || group == nil {
		return nil, nil, fmt.Errorf("failed to find group named '%s'", groupAndSubGroupPaths[0])
	}
	gid = &group.ID

	if len(groupAndSubGroupPaths) > 1 {
		lastSubGroup := groupAndSubGroupPaths[len(groupAndSubGroupPaths)-1]
		ldgo := &gitlab.ListDescendantGroupsOptions{
			Search: gitlab.String(lastSubGroup),
		}
		subGroups, _, err := gl.Groups.ListDescendantGroups(*gid, ldgo, gitlab.WithContext(ctx))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list subgroups, error: %w", err)
		}
		subGroup := findGroupByPath(subGroups, lastSubGroup)
		if len(subGroups) == 0 || subGroup == nil {
			return nil, nil, fmt.Errorf("failed to list subgroups named '%s'", lastSubGroup)
		}
		gid = &subGroup.ID
	}

	lpo := &gitlab.ListGroupProjectsOptions{
		Search: gitlab.String(r.Name),
	}
	projects, _, err = gl.Groups.ListGroupProjects(*gid, lpo, gitlab.WithContext(ctx))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list projects, error: %w", err)
	}

	var project *gitlab.Project
	if len(projects) > 0 {
		project = projects[0]
	}
	return gid, project, nil
}

func findGroupByPath(groups []*gitlab.Group, path string) *gitlab.Group {
	for _, group := range groups {
		if group.Path == path {
			return group
		}
	}

	return nil
}
