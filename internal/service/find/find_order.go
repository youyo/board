package find

import (
	"context"
	"log/slog"

	"github.com/youyo/board/internal/boardapi"
)

// FindOrder は ID / ProjectID / ClientName / ProjectName による注文書横断検索を行う。
// 構造は FindEstimate と同形（Order は単数 *DocumentSummary）。
//
// 詳細仕様は find_estimate.go の docstring 参照（reverseMapper / 二重 fetch 回避 / non-fatal enrichment）。
func (s *Service) FindOrder(ctx context.Context, q FindOrderQuery) ([]OrderResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)
	results := make([]OrderResult, 0)

	switch {
	case q.ID != 0:
		o, err := s.orders.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		pid, ok, lerr := s.reverseMappers["order"].Lookup(ctx, q.ID, opts)
		if lerr != nil {
			slog.Warn("find.FindOrder: reverseMap build failed",
				"doc_id", q.ID, "error", lerr)
		}
		if !ok || pid == 0 {
			results = append(results, OrderResult{Order: *o})
			return results, nil
		}
		p, perr := s.projects.GetByID(ctx, pid, opts)
		if perr != nil {
			slog.Warn("find.FindOrder: project enrichment failed",
				"project_id", pid, "error", perr)
			results = append(results, OrderResult{Order: *o, ProjectID: pid})
			return results, nil
		}
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		results = append(results, OrderResult{
			Order:     *o,
			ProjectID: pid,
			ClientID:  cid,
			Project:   p,
			Client:    client,
		})

	case q.ProjectID != 0:
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "order")
		if err != nil {
			return nil, err
		}
		if p.Order == nil {
			return results, nil
		}
		o, err := s.orders.GetByDocumentID(ctx, p.Order.ID, opts)
		if boardapi.IsNotFound(err) {
			return results, nil
		}
		if err != nil {
			return nil, err
		}
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		results = append(results, OrderResult{
			Order:     *o,
			ProjectID: p.ID,
			ClientID:  cid,
			Project:   p,
			Client:    client,
		})

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
			projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{ClientIDEq: c.ID, NameCont: q.ProjectName, ResponseGroup: "order"}, opts)
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
				p2 := p
				results = append(results, OrderResult{
					Order:     *o,
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

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{NameCont: q.ProjectName, ResponseGroup: "order"}, opts)
		if err != nil {
			return nil, err
		}
		if len(projects) > fanoutResolveCap {
			return nil, errFanoutTooMany("project_name", q.ProjectName, len(projects))
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
			p2 := p
			cid := projectClientID(p)
			client := s.lookupClient(ctx, cid, p.ID, opts)
			results = append(results, OrderResult{
				Order:     *o,
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

	for i := range results {
		results[i].URL = documentURL(s.uiBaseURL, results[i].Order.ID)
	}
	return results, nil
}
