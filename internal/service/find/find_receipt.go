package find

import (
	"context"
	"log/slog"

	"github.com/youyo/board/internal/boardapi"
)

// FindReceipt は ID / ProjectID / ClientName / ProjectName による領収書横断検索を行う。
// 構造は FindDelivery と同形（p.Receipts は配列、全要素ループ実行）。
//
// 詳細仕様は find_estimate.go の docstring 参照（reverseMapper / 二重 fetch 回避 / non-fatal enrichment）。
func (s *Service) FindReceipt(ctx context.Context, q FindReceiptQuery) ([]ReceiptResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)
	results := make([]ReceiptResult, 0)

	switch {
	case q.ID != 0:
		r, err := s.receipts.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		pid, ok, lerr := s.reverseMappers["receipt"].Lookup(ctx, q.ID, opts)
		if lerr != nil {
			slog.Warn("find.FindReceipt: reverseMap build failed",
				"doc_id", q.ID, "error", lerr)
		}
		if !ok || pid == 0 {
			results = append(results, ReceiptResult{Receipt: *r})
			return results, nil
		}
		p, perr := s.projects.GetByID(ctx, pid, opts)
		if perr != nil {
			slog.Warn("find.FindReceipt: project enrichment failed",
				"project_id", pid, "error", perr)
			results = append(results, ReceiptResult{Receipt: *r, ProjectID: pid})
			return results, nil
		}
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		results = append(results, ReceiptResult{
			Receipt:   *r,
			ProjectID: pid,
			ClientID:  cid,
			Project:   p,
			Client:    client,
		})

	case q.ProjectID != 0:
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "receipt")
		if err != nil {
			return nil, err
		}
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		for _, rs := range p.Receipts {
			doc, err := s.receipts.GetByDocumentID(ctx, rs.ID, opts)
			if boardapi.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			results = append(results, ReceiptResult{
				Receipt:   *doc,
				ProjectID: p.ID,
				ClientID:  cid,
				Project:   p,
				Client:    client,
			})
			if q.Limit > 0 && len(results) >= q.Limit {
				return results, nil
			}
		}

	case q.ClientName != "":
		clients, err := s.clients.Search(ctx, boardapi.ClientListOptions{NameCont: q.ClientName}, opts)
		if err != nil {
			return nil, err
		}
		if len(clients) > fanoutResolveCap {
			return nil, errFanoutTooMany("client_name", q.ClientName, len(clients))
		}
		for _, c := range clients {
			c2 := c
			projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{ClientIDEq: c.ID, NameCont: q.ProjectName, ResponseGroup: "receipt"}, opts)
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				p2 := p
				for _, rs := range p.Receipts {
					doc, err := s.receipts.GetByDocumentID(ctx, rs.ID, opts)
					if boardapi.IsNotFound(err) {
						continue
					}
					if err != nil {
						return nil, err
					}
					results = append(results, ReceiptResult{
						Receipt:   *doc,
						ProjectID: p.ID,
						ClientID:  c.ID,
						Project:   &p2,
						Client:    &c2,
					})
					if q.Limit > 0 && len(results) >= q.Limit {
						return results, nil
					}
				}
			}
		}

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{NameCont: q.ProjectName, ResponseGroup: "receipt"}, opts)
		if err != nil {
			return nil, err
		}
		if len(projects) > fanoutResolveCap {
			return nil, errFanoutTooMany("project_name", q.ProjectName, len(projects))
		}
		for _, p := range projects {
			p2 := p
			cid := projectClientID(p)
			client := s.lookupClient(ctx, cid, p.ID, opts)
			for _, rs := range p.Receipts {
				doc, err := s.receipts.GetByDocumentID(ctx, rs.ID, opts)
				if boardapi.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				results = append(results, ReceiptResult{
					Receipt:   *doc,
					ProjectID: p.ID,
					ClientID:  cid,
					Project:   &p2,
					Client:    client,
				})
				if q.Limit > 0 && len(results) >= q.Limit {
					return results, nil
				}
			}
		}
	}

	for i := range results {
		results[i].URL = documentURL(s.uiBaseURL, results[i].ProjectID)
	}
	return results, nil
}
