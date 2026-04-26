package find

import (
	"context"
	"log/slog"

	"github.com/youyo/board/internal/boardapi"
)

// FindDelivery は ID / ProjectID / ClientName / ProjectName による納品書横断検索を行う。
// 構造は FindEstimate / FindOrder と類似だが、Delivery は複数 Document が project に紐づくため
// p.Deliveries 配列を全要素ループ実行する点が異なる（N02 §4.4）。
//
// 詳細仕様は find_estimate.go の docstring 参照（reverseMapper / 二重 fetch 回避 / non-fatal enrichment）。
func (s *Service) FindDelivery(ctx context.Context, q FindDeliveryQuery) ([]DeliveryResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)
	results := make([]DeliveryResult, 0)

	switch {
	case q.ID != 0:
		d, err := s.deliveries.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		pid, ok, lerr := s.reverseMappers["delivery"].Lookup(ctx, q.ID, opts)
		if lerr != nil {
			slog.Warn("find.FindDelivery: reverseMap build failed",
				"doc_id", q.ID, "error", lerr)
		}
		if !ok || pid == 0 {
			results = append(results, DeliveryResult{Delivery: *d})
			return results, nil
		}
		p, perr := s.projects.GetByID(ctx, pid, opts)
		if perr != nil {
			slog.Warn("find.FindDelivery: project enrichment failed",
				"project_id", pid, "error", perr)
			results = append(results, DeliveryResult{Delivery: *d, ProjectID: pid})
			return results, nil
		}
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		results = append(results, DeliveryResult{
			Delivery:  *d,
			ProjectID: pid,
			ClientID:  cid,
			Project:   p,
			Client:    client,
		})

	case q.ProjectID != 0:
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "delivery")
		if err != nil {
			return nil, err
		}
		// project 内の全 Deliveries をループ。client 取得は project ごとに 1 回のみ集約。
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		for _, ds := range p.Deliveries {
			doc, err := s.deliveries.GetByDocumentID(ctx, ds.ID, opts)
			if boardapi.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			results = append(results, DeliveryResult{
				Delivery:  *doc,
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
		for _, c := range clients {
			c2 := c
			projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{ClientIDEq: c.ID, ResponseGroup: "delivery"}, opts)
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				p2 := p
				for _, ds := range p.Deliveries {
					doc, err := s.deliveries.GetByDocumentID(ctx, ds.ID, opts)
					if boardapi.IsNotFound(err) {
						continue
					}
					if err != nil {
						return nil, err
					}
					results = append(results, DeliveryResult{
						Delivery:  *doc,
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
		projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{NameCont: q.ProjectName, ResponseGroup: "delivery"}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			p2 := p
			cid := projectClientID(p)
			client := s.lookupClient(ctx, cid, p.ID, opts)
			for _, ds := range p.Deliveries {
				doc, err := s.deliveries.GetByDocumentID(ctx, ds.ID, opts)
				if boardapi.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				results = append(results, DeliveryResult{
					Delivery:  *doc,
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

	return results, nil
}
