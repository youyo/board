package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindProject は ID / ClientID / Name / Status[es] によるプロジェクト横断検索を行う。
// ID 指定時は他の絞込フィールドを無視し直接 lookup。それ以外は ClientID / Name を
// API 側 (ClientIDEq + NameCont) で AND 評価する。
// Status / Statuses / ContractStatus は post-filter（ID 指定時はスキップ、UX 配慮）。
func (s *Service) FindProject(ctx context.Context, q FindProjectQuery) ([]ProjectResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var projects []boardapi.ProjectEntity
	if q.ID != 0 {
		p, err := s.projects.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		projects = []boardapi.ProjectEntity{*p}
	} else {
		list, err := s.projects.Search(ctx, boardapi.ProjectListOptions{
			ClientIDEq: q.ClientID,
			NameCont:   q.Name,
		}, opts)
		if err != nil {
			return nil, err
		}
		projects = list
	}
	// Status/Statuses-only ケースは types.go の validate() で reject 済（advisor R3）。
	// ここに到達した時点で必ず ID/ClientID/Name/Text のいずれかが処理されている。

	// Status/ContractStatus post-filter: ID 検索時はスキップ（旧 find_project.go 踏襲、UX 配慮）。
	// ContractStatus → Statuses → Status の優先順で評価（validateStatusGroup で排他保証済）。
	if q.ID == 0 {
		switch {
		case q.ContractStatus != "":
			filtered, err := filterProjectsByContractStatus(projects, q.ContractStatus)
			if err != nil {
				return nil, err
			}
			projects = filtered
		case len(q.Statuses) > 0:
			projects = filterProjectsByStatuses(projects, q.Statuses)
		case q.Status != "":
			projects = filterProjectsByStatus(projects, q.Status)
		}
	}

	results := make([]ProjectResult, 0, len(projects))
	for _, p := range projects {
		results = append(results, s.resolveProjectClient(ctx, p, opts))
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	for i := range results {
		results[i].URL = projectURL(s.uiBaseURL, results[i].Project.ID)
	}
	return results, nil
}
