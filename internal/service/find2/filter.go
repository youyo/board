package find2

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

// filterByStatus は単一のステータス文字列で items を絞り込む。
// status が空の場合は items をそのまま返す（no-op）。
func filterByStatus[T any](items []T, getStatus func(T) string, status string) []T {
	if status == "" {
		return items
	}
	return filterByStatuses(items, getStatus, []string{status})
}
