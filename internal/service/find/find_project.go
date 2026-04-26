package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindProject は ID / ClientID / Name / Text / Status[es] によるプロジェクト横断検索を行う。
// 検索フィールド優先順位: ID > ClientID > Name > Text。
// Status / Statuses は post-filter で OrderStatusName または DeliveryStatusName に OR 評価。
// ID 検索時は Status post-filter をスキップ（旧 find_project.go 踏襲、UX 配慮）。
func (s *Service) FindProject(ctx context.Context, q FindProjectQuery) ([]ProjectResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var projects []boardapi.ProjectEntity
	switch {
	case q.ID != 0:
		p, err := s.projects.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		projects = []boardapi.ProjectEntity{*p}
	case q.ClientID != 0:
		list, err := s.projects.Search(ctx, boardapi.ProjectListOptions{ClientIDEq: q.ClientID}, opts)
		if err != nil {
			return nil, err
		}
		projects = list
	case q.Name != "":
		list, err := s.projects.Search(ctx, boardapi.ProjectListOptions{NameCont: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		projects = list
	case q.Text != "":
		all, err := s.projects.Search(ctx, boardapi.ProjectListOptions{}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			if containsText(q.Text, p.Name, derefString(p.ManagementNo), derefString(p.InHouseMemo)) {
				projects = append(projects, p)
			}
		}
	}
	// Status/Statuses-only ケースは types.go の validate() で reject 済（advisor R3）。
	// ここに到達した時点で必ず ID/ClientID/Name/Text のいずれかが処理されている。

	// Status post-filter: ID 検索時はスキップ（旧 find_project.go 踏襲、UX 配慮）。
	if q.ID == 0 {
		if len(q.Statuses) > 0 {
			projects = filterProjectsByStatuses(projects, q.Statuses)
		} else if q.Status != "" {
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
	return results, nil
}
