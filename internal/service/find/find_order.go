package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindOrder performs a cross-resource search for orders, returning
// orders with their associated client and project.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
func (s *Service) FindOrder(ctx context.Context, q FindOrderQuery) ([]OrderResult, error) {
	if q.ID == 0 && q.ClientName == "" && q.ProjectName == "" && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, ClientName, ProjectName, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var orders []boardapi.OrderEntity

	switch {
	case q.ID != 0:
		o, err := s.orders.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		orders = []boardapi.OrderEntity{*o}

	case q.ClientName != "":
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			os, err := s.orders.Search(ctx, boardapi.OrderSearchParams{ClientID: c.ID}, opts)
			if err != nil {
				return nil, err
			}
			orders = append(orders, os...)
		}

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			os, err := s.orders.Search(ctx, boardapi.OrderSearchParams{ProjectID: p.ID}, opts)
			if err != nil {
				return nil, err
			}
			orders = append(orders, os...)
		}

	case q.Text != "":
		all, err := s.orders.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, o := range all {
			if containsText(q.Text, o.Title, o.Memo) {
				orders = append(orders, o)
			}
		}

	case q.Status != "":
		all, err := s.orders.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, o := range all {
			if o.Status == q.Status {
				orders = append(orders, o)
			}
		}
	}

	// Apply status post-filter for non-status-only search modes
	if q.Status != "" && q.ID == 0 && !(q.ClientName == "" && q.ProjectName == "" && q.Text == "") {
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
