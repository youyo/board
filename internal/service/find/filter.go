package find

import "github.com/youyo/board/internal/boardapi"

// filterProjectsByStatuses は OrderStatusName または DeliveryStatusName が
// statuses 集合に含まれる ProjectEntity を返す。
// statuses が空の場合は projects をそのまま返す（no-op）。
//
// 注意: ProjectEntity は M44 で Status フィールドが廃止され、
// OrderStatusName / DeliveryStatusName の 2 フィールドに分離された。
// 単一フィールド比較の filterByStatuses[T] では表現できないため、
// projects 専用ヘルパーとして実装する。
func filterProjectsByStatuses(projects []boardapi.ProjectEntity, statuses []string) []boardapi.ProjectEntity {
	if len(statuses) == 0 {
		return projects
	}
	set := make(map[string]struct{}, len(statuses))
	for _, s := range statuses {
		set[s] = struct{}{}
	}
	out := make([]boardapi.ProjectEntity, 0, len(projects))
	for _, p := range projects {
		if _, ok := set[p.OrderStatusName]; ok {
			out = append(out, p)
			continue
		}
		if _, ok := set[p.DeliveryStatusName]; ok {
			out = append(out, p)
		}
	}
	return out
}

// filterProjectsByStatus は単一の status 文字列で OR 絞り込み。
func filterProjectsByStatus(projects []boardapi.ProjectEntity, status string) []boardapi.ProjectEntity {
	if status == "" {
		return projects
	}
	return filterProjectsByStatuses(projects, []string{status})
}

// filterByStatuses はジェネリクスを用いて items をステータス集合で絞り込む。
// statuses が空の場合は items をそのまま返す（no-op）。
func filterByStatuses[T any](items []T, getStatus func(T) string, statuses []string) []T {
	if len(statuses) == 0 {
		return items
	}
	set := make(map[string]struct{}, len(statuses))
	for _, s := range statuses {
		set[s] = struct{}{}
	}
	out := make([]T, 0, len(items))
	for _, it := range items {
		if _, ok := set[getStatus(it)]; ok {
			out = append(out, it)
		}
	}
	return out
}
