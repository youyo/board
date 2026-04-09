package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindOrder performs a cross-resource search for orders, returning
// orders with their associated client and project.
// Uses project response_group API to discover document IDs, then fetches via GetByDocumentID.
// Field priority: ID > ProjectID > ClientName > ProjectName.
// Status is a post-filter only.
func (s *Service) FindOrder(ctx context.Context, q FindOrderQuery) ([]OrderResult, error) {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" && q.ProjectName == "" {
		return nil, errors.New("at least one of ID, ProjectID, ClientName, or ProjectName must be set")
	}

	opts := repoOpts(q.Opts)

	var orders []boardapi.OrderEntity

	switch {
	case q.ID != 0:
		// Direct lookup by document ID
		o, err := s.orders.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		orders = []boardapi.OrderEntity{*o}

	case q.ProjectID != 0:
		// Lookup project with order group, then fetch document
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "order")
		if err != nil {
			return nil, err
		}
		if p.Order != nil {
			o, err := s.orders.GetByDocumentID(ctx, p.Order.ID, opts)
			if err != nil && !boardapi.IsNotFound(err) {
				return nil, err
			}
			if err == nil {
				orders = []boardapi.OrderEntity{*o}
			}
		}

	case q.ClientName != "":
		// Resolve client name → search projects with order group → hydrate
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{ClientID: c.ID, ResponseGroup: "order"}, opts)
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				if p.Order == nil {
					continue
				}
				o, err := s.orders.GetByDocumentID(ctx, p.Order.ID, opts)
				if boardapi.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				orders = append(orders, *o)
			}
		}

	case q.ProjectName != "":
		// Search projects by name with order group → hydrate
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName, ResponseGroup: "order"}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			if p.Order == nil {
				continue
			}
			o, err := s.orders.GetByDocumentID(ctx, p.Order.ID, opts)
			if boardapi.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			orders = append(orders, *o)
		}
	}

	// Apply status post-filter
	if q.Status != "" {
		orders = filterOrdersByStatus(orders, q.Status)
	}

	results := make([]OrderResult, 0, len(orders))
	for _, o := range orders {
		client, project := s.resolveClientAndProject(ctx, o.ClientID, o.ProjectID, opts)
		results = append(results, OrderResult{
			Order:   o,
			Client:  client,
			Project: project,
		})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// filterOrdersByStatus filters orders by status.
func filterOrdersByStatus(orders []boardapi.OrderEntity, status string) []boardapi.OrderEntity {
	filtered := make([]boardapi.OrderEntity, 0, len(orders))
	for _, o := range orders {
		if o.Status == status {
			filtered = append(filtered, o)
		}
	}
	return filtered
}
