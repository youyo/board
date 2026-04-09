package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindEstimate performs a cross-resource search for estimates, returning
// estimates with their associated client and project.
// Uses project response_group API to discover document IDs, then fetches via GetByDocumentID.
// Field priority: ID > ProjectID > ClientName > ProjectName.
// Status is a post-filter only.
func (s *Service) FindEstimate(ctx context.Context, q FindEstimateQuery) ([]EstimateResult, error) {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" && q.ProjectName == "" {
		return nil, errors.New("at least one of ID, ProjectID, ClientName, or ProjectName must be set")
	}

	opts := repoOpts(q.Opts)

	var estimates []boardapi.EstimateEntity

	switch {
	case q.ID != 0:
		// Direct lookup by document ID
		e, err := s.estimates.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		estimates = []boardapi.EstimateEntity{*e}

	case q.ProjectID != 0:
		// Lookup project with estimate group, then fetch document
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "estimate")
		if err != nil {
			return nil, err
		}
		if p.Estimate != nil {
			e, err := s.estimates.GetByDocumentID(ctx, p.Estimate.ID, opts)
			if err != nil && !boardapi.IsNotFound(err) {
				return nil, err
			}
			if err == nil {
				estimates = []boardapi.EstimateEntity{*e}
			}
		}

	case q.ClientName != "":
		// Resolve client name → search projects with estimate group → hydrate
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{ClientID: c.ID, ResponseGroup: "estimate"}, opts)
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				if p.Estimate == nil {
					continue
				}
				e, err := s.estimates.GetByDocumentID(ctx, p.Estimate.ID, opts)
				if boardapi.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				estimates = append(estimates, *e)
			}
		}

	case q.ProjectName != "":
		// Search projects by name with estimate group → hydrate
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName, ResponseGroup: "estimate"}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			if p.Estimate == nil {
				continue
			}
			e, err := s.estimates.GetByDocumentID(ctx, p.Estimate.ID, opts)
			if boardapi.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			estimates = append(estimates, *e)
		}
	}

	// Apply status post-filter
	if q.Status != "" {
		estimates = filterEstimatesByStatus(estimates, q.Status)
	}

	// Build results with client/project resolution
	results := make([]EstimateResult, 0, len(estimates))
	for _, e := range estimates {
		client, project := s.resolveClientAndProject(ctx, e.ClientID, e.ProjectID, opts)
		results = append(results, EstimateResult{
			Estimate: e,
			Client:   client,
			Project:  project,
		})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// filterEstimatesByStatus filters estimates by status.
func filterEstimatesByStatus(estimates []boardapi.EstimateEntity, status string) []boardapi.EstimateEntity {
	filtered := make([]boardapi.EstimateEntity, 0, len(estimates))
	for _, e := range estimates {
		if e.Status == status {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
