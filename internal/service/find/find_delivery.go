package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindDelivery performs a cross-resource search for deliveries, returning
// deliveries with their associated client and project.
// Uses project response_group API to discover document IDs, then fetches via GetByDocumentID.
// Field priority: ID > ProjectID > ClientName > ProjectName.
// Status is a post-filter only.
func (s *Service) FindDelivery(ctx context.Context, q FindDeliveryQuery) ([]DeliveryResult, error) {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" && q.ProjectName == "" {
		return nil, errors.New("at least one of ID, ProjectID, ClientName, or ProjectName must be set")
	}

	opts := repoOpts(q.Opts)

	var deliveries []boardapi.DeliveryEntity

	switch {
	case q.ID != 0:
		// Direct lookup by document ID
		d, err := s.deliveries.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		deliveries = []boardapi.DeliveryEntity{*d}

	case q.ProjectID != 0:
		// Lookup project with delivery group, then fetch document
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "delivery")
		if err != nil {
			return nil, err
		}
		if p.Delivery != nil {
			d, err := s.deliveries.GetByDocumentID(ctx, p.Delivery.ID, opts)
			if err != nil && !boardapi.IsNotFound(err) {
				return nil, err
			}
			if err == nil {
				deliveries = []boardapi.DeliveryEntity{*d}
			}
		}

	case q.ClientName != "":
		// Resolve client name → search projects with delivery group → hydrate
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{ClientID: c.ID, ResponseGroup: "delivery"}, opts)
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				if p.Delivery == nil {
					continue
				}
				d, err := s.deliveries.GetByDocumentID(ctx, p.Delivery.ID, opts)
				if boardapi.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				deliveries = append(deliveries, *d)
			}
		}

	case q.ProjectName != "":
		// Search projects by name with delivery group → hydrate
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName, ResponseGroup: "delivery"}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			if p.Delivery == nil {
				continue
			}
			d, err := s.deliveries.GetByDocumentID(ctx, p.Delivery.ID, opts)
			if boardapi.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			deliveries = append(deliveries, *d)
		}
	}

	// Apply status post-filter
	if q.Status != "" {
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
