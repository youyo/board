package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindDelivery performs a cross-resource search for deliveries, returning
// deliveries with their associated client and project.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
func (s *Service) FindDelivery(ctx context.Context, q FindDeliveryQuery) ([]DeliveryResult, error) {
	if q.ID == 0 && q.ClientName == "" && q.ProjectName == "" && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, ClientName, ProjectName, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var deliveries []boardapi.DeliveryEntity

	switch {
	case q.ID != 0:
		d, err := s.deliveries.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		deliveries = []boardapi.DeliveryEntity{*d}

	case q.ClientName != "":
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			ds, err := s.deliveries.Search(ctx, boardapi.DeliverySearchParams{ClientID: c.ID}, opts)
			if err != nil {
				return nil, err
			}
			deliveries = append(deliveries, ds...)
		}

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			ds, err := s.deliveries.Search(ctx, boardapi.DeliverySearchParams{ProjectID: p.ID}, opts)
			if err != nil {
				return nil, err
			}
			deliveries = append(deliveries, ds...)
		}

	case q.Text != "":
		all, err := s.deliveries.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, d := range all {
			if containsText(q.Text, d.Title, d.Memo) {
				deliveries = append(deliveries, d)
			}
		}

	case q.Status != "":
		all, err := s.deliveries.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, d := range all {
			if d.Status == q.Status {
				deliveries = append(deliveries, d)
			}
		}
	}

	// Apply status post-filter for non-status-only search modes
	if q.Status != "" && q.ID == 0 && !(q.ClientName == "" && q.ProjectName == "" && q.Text == "") {
		deliveries = filterDeliveriesByStatus(deliveries, q.Status)
	}

	results := make([]DeliveryResult, 0, len(deliveries))
	for _, d := range deliveries {
		client, project := s.resolveClientAndProject(ctx, d.ClientID, d.ProjectID, opts)
		results = append(results, DeliveryResult{
			Delivery: d,
			Client:   client,
			Project:  project,
		})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// filterDeliveriesByStatus filters deliveries by status.
func filterDeliveriesByStatus(deliveries []boardapi.DeliveryEntity, status string) []boardapi.DeliveryEntity {
	filtered := make([]boardapi.DeliveryEntity, 0, len(deliveries))
	for _, d := range deliveries {
		if d.Status == status {
			filtered = append(filtered, d)
		}
	}
	return filtered
}
