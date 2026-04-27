package find

import (
	"context"
	"log/slog"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// FindEstimate は ID / ProjectID / ClientName / ProjectName による見積書横断検索を行う。
// 検索フィールド優先順位: ID > ProjectID > ClientName > ProjectName。
//
// EstimateEntity は projectID / clientID をトップレベルに持たない（実 API 仕様）。
// このため:
//   - ID branch は reverseMapper で documentID → projectID を逆引きし、その後 projects.GetByID で
//     project を取得する。reverseMapper miss / timeout 時は ProjectID=0 で部分結果を返す（non-fatal）。
//   - ProjectID branch は projects.GetByIDWithGroup(id, "estimate") で project と Estimate.ID を取得。
//   - ClientName/ProjectName branch は projects.Search(..., ResponseGroup:"estimate") で fanout。
//
// Text フィールドは N06 では未対応（API に document 全文検索が無いため）。Text のみ指定時は空 slice 返却。
//
// enrichment ポリシー:
//   - 主検索（estimates.GetByDocumentID）の失敗は fail-fast。
//   - ID branch の projects.GetByID 失敗は non-fatal（slog.Warn + Project=nil）。
//   - client 取得失敗は non-fatal（slog.Warn + Client=nil）。
//   - Project は既に取得済の `p` を再利用（二重 fetch 回避）。
//   - ClientName branch では outer loop の `c` を Result.Client に再利用（lookupClient 不要）。
func (s *Service) FindEstimate(ctx context.Context, q FindEstimateQuery) ([]EstimateResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)
	results := make([]EstimateResult, 0)

	switch {
	case q.ID != 0:
		e, err := s.estimates.GetByDocumentID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		pid, ok, lerr := s.reverseMappers["estimate"].Lookup(ctx, q.ID, opts)
		if lerr != nil {
			slog.Warn("find.FindEstimate: reverseMap build failed",
				"doc_id", q.ID, "error", lerr)
		}
		if !ok || pid == 0 {
			results = append(results, EstimateResult{Estimate: *e})
			return results, nil
		}
		p, perr := s.projects.GetByID(ctx, pid, opts)
		if perr != nil {
			slog.Warn("find.FindEstimate: project enrichment failed",
				"project_id", pid, "error", perr)
			results = append(results, EstimateResult{Estimate: *e, ProjectID: pid})
			return results, nil
		}
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		results = append(results, EstimateResult{
			Estimate:  *e,
			ProjectID: pid,
			ClientID:  cid,
			Project:   p,
			Client:    client,
		})

	case q.ProjectID != 0:
		p, err := s.projects.GetByIDWithGroup(ctx, q.ProjectID, "estimate")
		if err != nil {
			return nil, err
		}
		if p.Estimate == nil {
			return results, nil
		}
		e, err := s.estimates.GetByDocumentID(ctx, p.Estimate.ID, opts)
		if boardapi.IsNotFound(err) {
			return results, nil
		}
		if err != nil {
			return nil, err
		}
		cid := projectClientIDPtr(p)
		client := s.lookupClient(ctx, cid, p.ID, opts)
		results = append(results, EstimateResult{
			Estimate:  *e,
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
			projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{ClientIDEq: c.ID, ResponseGroup: "estimate"}, opts)
			if err != nil {
				return nil, err
			}
			for _, p := range projects {
				if p.Estimate == nil {
					continue
				}
				e, err := s.estimates.GetByDocumentID(ctx, p.Estimate.ID, opts)
				if boardapi.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				p2 := p
				results = append(results, EstimateResult{
					Estimate:  *e,
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
		projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{NameCont: q.ProjectName, ResponseGroup: "estimate"}, opts)
		if err != nil {
			return nil, err
		}
		if len(projects) > fanoutResolveCap {
			return nil, errFanoutTooMany("project_name", q.ProjectName, len(projects))
		}
		for _, p := range projects {
			if p.Estimate == nil {
				continue
			}
			e, err := s.estimates.GetByDocumentID(ctx, p.Estimate.ID, opts)
			if boardapi.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			p2 := p
			cid := projectClientID(p)
			client := s.lookupClient(ctx, cid, p.ID, opts)
			results = append(results, EstimateResult{
				Estimate:  *e,
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

	return results, nil
}

// lookupClient は ClientID が 0 でない場合に Client を 1 回だけ取得する。
// 失敗時は slog.Warn + nil を返す（non-fatal）。Document 4 種で共通利用。
func (s *Service) lookupClient(ctx context.Context, cid int, projectID int, opts repository.ReadOptions) *boardapi.ClientEntity {
	if cid == 0 {
		return nil
	}
	c, err := s.clients.GetByID(ctx, cid, opts)
	if err != nil {
		slog.Warn("find.lookupClient: client enrichment failed",
			"project_id", projectID, "client_id", cid, "error", err)
		return nil
	}
	return c
}
