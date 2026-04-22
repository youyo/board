package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindInvoice performs a cross-resource search for invoices, returning
// invoices with their associated client and project.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
func (s *Service) FindInvoice(ctx context.Context, q FindInvoiceQuery) ([]InvoiceResult, error) {
	if q.ID == 0 && q.ClientName == "" && q.ProjectName == "" && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, ClientName, ProjectName, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var invoices []boardapi.InvoiceEntity

	switch {
	case q.ID != 0:
		inv, err := s.invoices.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		invoices = []boardapi.InvoiceEntity{*inv}

	case q.ClientName != "":
		clients, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range clients {
			is, err := s.invoices.Search(ctx, boardapi.InvoiceSearchParams{ClientID: c.ID}, opts)
			if err != nil {
				return nil, err
			}
			invoices = append(invoices, is...)
		}

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectSearchParams{Name: q.ProjectName}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			is, err := s.invoices.Search(ctx, boardapi.InvoiceSearchParams{ProjectID: p.ID}, opts)
			if err != nil {
				return nil, err
			}
			invoices = append(invoices, is...)
		}

	case q.Text != "":
		all, err := s.invoices.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, inv := range all {
			if containsText(q.Text, inv.Title, inv.Memo) {
				invoices = append(invoices, inv)
			}
		}

	case q.Status != "":
		all, err := s.invoices.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, inv := range all {
			if inv.Status == q.Status {
				invoices = append(invoices, inv)
			}
		}
	}

	// Apply status post-filter for non-status-only search modes
	if q.Status != "" && q.ID == 0 && (q.ClientName != "" || q.ProjectName != "" || q.Text != "") {
		invoices = filterInvoicesByStatus(invoices, q.Status)
	}

	results := make([]InvoiceResult, 0, len(invoices))
	for _, inv := range invoices {
		client, project := s.resolveClientAndProject(ctx, inv.ClientID, inv.ProjectID, opts)
		results = append(results, InvoiceResult{
			Invoice: inv,
			Client:  client,
			Project: project,
		})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// filterInvoicesByStatus filters invoices by status.
func filterInvoicesByStatus(invoices []boardapi.InvoiceEntity, status string) []boardapi.InvoiceEntity {
	filtered := make([]boardapi.InvoiceEntity, 0, len(invoices))
	for _, inv := range invoices {
		if inv.Status == status {
			filtered = append(filtered, inv)
		}
	}
	return filtered
}
