package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindReceipt performs a cross-resource search for receipts, returning
// receipts with their associated client and project.
// Uses project response_group API to discover document IDs, then fetches via GetByDocumentID.
// Field priority: ID > ProjectID > ClientName > ProjectName.
// Status is a post-filter only.
func (s *Service) FindReceipt(ctx context.Context, q FindReceiptQuery) ([]ReceiptResult, error) {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" && q.ProjectName == "" {
		return nil, errors.New("at least one of ID, ProjectID, ClientName, or ProjectName must be set")
	}

	opts := repoOpts(q.Opts)

	var receipts []boardapi.ReceiptEntity

	switch {
	case q.ID != 0:
		// Direct lookup by document ID
		r, err := s.receipts.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		receipts = []boardapi.ReceiptEntity{*r}

	case q.ProjectID != 0:
		// Lookup project with receipt group, then fetch document
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "receipt")
		if err != nil {
			return nil, err
		}
		if p.Receipt != nil {
			r, err := s.receipts.GetByDocumentID(ctx, p.Receipt.ID, opts)
			if err != nil && !boardapi.IsNotFound(err) {
				return nil, err
			}
			if err == nil {
				receipts = []boardapi.ReceiptEntity{*r}
			}
		}

	case q.ClientName != "":
		// Resolve client name → search projects with receipt group → hydrate
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{ClientID: c.ID, ResponseGroup: "receipt"}, opts)
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				if p.Receipt == nil {
					continue
				}
				r, err := s.receipts.GetByDocumentID(ctx, p.Receipt.ID, opts)
				if boardapi.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				receipts = append(receipts, *r)
			}
		}

	case q.ProjectName != "":
		// Search projects by name with receipt group → hydrate
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName, ResponseGroup: "receipt"}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			if p.Receipt == nil {
				continue
			}
			r, err := s.receipts.GetByDocumentID(ctx, p.Receipt.ID, opts)
			if boardapi.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			receipts = append(receipts, *r)
		}
	}

	// Apply status post-filter
	if q.Status != "" {
		receipts = filterReceiptsByStatus(receipts, q.Status)
	}

	results := make([]ReceiptResult, 0, len(receipts))
	for _, r := range receipts {
		client, project := s.resolveClientAndProject(ctx, r.ClientID, r.ProjectID, opts)
		results = append(results, ReceiptResult{
			Receipt: r,
			Client:  client,
			Project: project,
		})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// filterReceiptsByStatus filters receipts by status.
func filterReceiptsByStatus(receipts []boardapi.ReceiptEntity, status string) []boardapi.ReceiptEntity {
	filtered := make([]boardapi.ReceiptEntity, 0, len(receipts))
	for _, r := range receipts {
		if r.Status == status {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
