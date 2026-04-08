package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindReceipt performs a cross-resource search for receipts, returning
// receipts with their associated client and project.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
func (s *Service) FindReceipt(ctx context.Context, q FindReceiptQuery) ([]ReceiptResult, error) {
	if q.ID == 0 && q.ClientName == "" && q.ProjectName == "" && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, ClientName, ProjectName, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var receipts []boardapi.ReceiptEntity

	switch {
	case q.ID != 0:
		r, err := s.receipts.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		receipts = []boardapi.ReceiptEntity{*r}

	case q.ClientName != "":
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			rs, err := s.receipts.Search(ctx, boardapi.ReceiptSearchParams{ClientID: c.ID}, opts)
			if err != nil {
				return nil, err
			}
			receipts = append(receipts, rs...)
		}

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			rs, err := s.receipts.Search(ctx, boardapi.ReceiptSearchParams{ProjectID: p.ID}, opts)
			if err != nil {
				return nil, err
			}
			receipts = append(receipts, rs...)
		}

	case q.Text != "":
		all, err := s.receipts.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range all {
			if containsText(q.Text, r.Title, r.Memo) {
				receipts = append(receipts, r)
			}
		}

	case q.Status != "":
		all, err := s.receipts.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range all {
			if r.Status == q.Status {
				receipts = append(receipts, r)
			}
		}
	}

	// Apply status post-filter for non-status-only search modes
	if q.Status != "" && q.ID == 0 && !(q.ClientName == "" && q.ProjectName == "" && q.Text == "") {
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
