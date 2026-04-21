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
//
// M38 NOTE: ReceiptEntity は実 API 準拠に再設計されたため、Status/ClientID/ProjectID
// フィールドは存在しない。ReceiptDate は実在するため引き続き利用可能。
// Status post-filter および client/project enrichment は各ブランチのコンテキスト情報から復元する。
// ID lookup では client/project を特定できないため nil を返す。
// TODO(M25-M32): find 層の全体再設計で enrichment を復元する。
func (s *Service) FindReceipt(ctx context.Context, q FindReceiptQuery) ([]ReceiptResult, error) {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" && q.ProjectName == "" {
		return nil, errors.New("at least one of ID, ProjectID, ClientName, or ProjectName must be set")
	}

	opts := repoOpts(q.Opts)

	// results を直接ブランチ内で構築する。
	// Status post-filter は ReceiptEntity に Status フィールドが無いため無効化。
	// TODO(M25-M32): Status post-filter を再設計で復元する。
	results := make([]ReceiptResult, 0)

	switch {
	case q.ID != 0:
		// Direct lookup by document ID.
		// client/project は特定できないため nil。
		r, err := s.receipts.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, ReceiptResult{
			Receipt: *r,
			Client:  nil,
			Project: nil,
		})

	case q.ProjectID != 0:
		// Lookup project with receipt group, then fetch document.
		// project コンテキストから client/project を解決。
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
				client, project := s.resolveClientAndProject(ctx, p.ClientID, p.ID, opts)
				results = append(results, ReceiptResult{
					Receipt: *r,
					Client:  client,
					Project: project,
				})
			}
		}

	case q.ClientName != "":
		// Resolve client name → search projects with receipt group → hydrate.
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
				client, project := s.resolveClientAndProject(ctx, p.ClientID, p.ID, opts)
				results = append(results, ReceiptResult{
					Receipt: *r,
					Client:  client,
					Project: project,
				})
			}
		}

	case q.ProjectName != "":
		// Search projects by name with receipt group → hydrate.
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
			client, project := s.resolveClientAndProject(ctx, p.ClientID, p.ID, opts)
			results = append(results, ReceiptResult{
				Receipt: *r,
				Client:  client,
				Project: project,
			})
		}
	}

	// Limit 適用
	if q.Limit > 0 && len(results) > q.Limit {
		results = results[:q.Limit]
	}

	return results, nil
}
