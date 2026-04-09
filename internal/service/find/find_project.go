package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// FindProject performs a cross-resource search for projects, optionally
// resolving client names to client IDs first.
// Field priority: ID > ClientName > Name > Text. Status is always applied as post-filter.
func (s *Service) FindProject(ctx context.Context, q FindProjectQuery) ([]ProjectResult, error) {
	if q.ID == 0 && q.ClientName == "" && q.Name == "" && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, ClientName, Name, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var projects []boardapi.ProjectEntity

	switch {
	case q.ID != 0:
		// ID has highest priority: direct lookup
		p, err := s.projects.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		projects = []boardapi.ProjectEntity{*p}

	case q.ClientName != "":
		// Resolve client name -> client IDs -> search projects
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			ps, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{ClientID: c.ID}, opts)
			if err != nil {
				return nil, err
			}
			projects = append(projects, ps...)
		}

	case q.Name != "":
		// Name search: delegate to repo
		result, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		projects = result

	case q.Text != "":
		// Text search: list all, filter by name/code/memo
		all, err := s.projects.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			if containsText(q.Text, p.Name, p.Code, p.Memo) {
				projects = append(projects, p)
			}
		}

	case q.Status != "":
		// Status-only search: list all, filter by status
		all, err := s.projects.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			if p.Status == q.Status {
				projects = append(projects, p)
			}
		}
	}

	// Apply status post-filter (for non-status-only search modes)
	if q.Status != "" && q.ID == 0 && !(q.ClientName == "" && q.Name == "" && q.Text == "") {
		projects = filterByStatus(projects, q.Status)
	}

	// Build results with client resolution
	results := make([]ProjectResult, 0, len(projects))
	for _, p := range projects {
		r, err := s.resolveProjectClient(ctx, p, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, r)

		// Apply limit
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// resolveProjectClient fetches the associated client and estimate for a project.
// Both resolutions are non-fatal: nil is returned on lookup error.
func (s *Service) resolveProjectClient(ctx context.Context, project boardapi.ProjectEntity, opts repository.ReadOptions) (ProjectResult, error) {
	var client *boardapi.ClientEntity
	if project.ClientID != 0 {
		c, err := s.clients.GetByID(ctx, project.ClientID, opts)
		if err != nil {
			// Client resolution failure is non-fatal; project still returned
			client = nil
		} else {
			client = c
		}
	}

	// Enrich with estimate via response_group
	var estimate *boardapi.EstimateEntity
	p, err := s.projects.GetByIDWithGroup(ctx, project.ID, "estimate")
	if err == nil && p.Estimate != nil {
		e, err := s.estimates.GetByDocumentID(ctx, p.Estimate.ID, opts)
		if err == nil {
			estimate = e
		}
	}

	return ProjectResult{
		Project:  project,
		Client:   client,
		Estimate: estimate,
	}, nil
}

// filterByStatus filters projects by status.
func filterByStatus(projects []boardapi.ProjectEntity, status string) []boardapi.ProjectEntity {
	filtered := make([]boardapi.ProjectEntity, 0, len(projects))
	for _, p := range projects {
		if p.Status == status {
			filtered = append(filtered, p)
		}
	}
	return filtered
}
