package cache

import "context"

// Info は単一 resource のキャッシュ鮮度情報。
//
// LLM が `cached_at` / `full_refreshed_at` を見て `--refresh` / `--refresh-full`
// を判断するための材料として、find のレスポンスに同梱する。
type Info struct {
	Resource        string `json:"resource"`
	CachedAt        string `json:"cached_at,omitempty"`         // RFC3339, 未取得なら ""
	FullRefreshedAt string `json:"full_refreshed_at,omitempty"` // RFC3339, 未取得なら ""
}

// LoadInfos は指定された resource list に対する Info を sync_state から読み出す。
// resource の sync_state が存在しない場合は CachedAt/FullRefreshedAt を "" のまま返す
// （LLM 側で「未取得」を判別可能）。
//
// 戻り値は必ず空でない場合のみ要素を持つ slice（nil ではなく `[]Info{}` 初期化）。
// JSON 出力で常に配列として表現するためで、これは仕様で空配列で返す方針。
func LoadInfos(ctx context.Context, ss *SyncStateStore, profile string, resources []string) []Info {
	infos := make([]Info, 0, len(resources))
	for _, r := range resources {
		info := Info{Resource: r}
		if ss != nil {
			if st, err := ss.Get(ctx, profile, r); err == nil && st != nil {
				if st.LastSyncedAt.Valid {
					info.CachedAt = st.LastSyncedAt.String
				}
				if st.LastFullSyncedAt.Valid {
					info.FullRefreshedAt = st.LastFullSyncedAt.String
				}
			}
		}
		infos = append(infos, info)
	}
	return infos
}
