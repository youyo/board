package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindEstimate performs a cross-resource search for estimates, returning
// estimates with their associated client and project.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
// Status also acts as a post-filter when combined with other criteria.
func (s *Service) FindEstimate(ctx context.Context, q FindEstimateQuery) ([]EstimateResult, error) {
	if q.ID == 0 && q.ClientName == "" && q.ProjectName == "" && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, ClientName, ProjectName, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var estimates []boardapi.EstimateEntity

	switch {
	case q.ID != 0:
		e, err := s.estimates.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		estimates = []boardapi.EstimateEntity{*e}

	case q.ClientName != "":
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			es, err := s.estimates.Search(ctx, boardapi.EstimateSearchParams{ClientID: c.ID}, opts)
			if err != nil {
				return nil, err
			}
			estimates = append(estimates, es...)
		}

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			es, err := s.estimates.Search(ctx, boardapi.EstimateSearchParams{ProjectID: p.ID}, opts)
			if err != nil {
				return nil, err
			}
			estimates = append(estimates, es...)
		}

	case q.Text != "":
		all, err := s.estimates.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, e := range all {
			if containsText(q.Text, e.Title, e.Memo) {
				estimates = append(estimates, e)
			}
		}

	case q.Status != "":
		all, err := s.estimates.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, e := range all {
			if e.Status == q.Status {
				estimates = append(estimates, e)
			}
		}
	}

	// Apply status post-filter for non-status-only search modes
	if q.Status != "" && q.ID == 0 && !(q.ClientName == "" && q.ProjectName == "" && q.Text == "") {
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
