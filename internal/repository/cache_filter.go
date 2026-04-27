package repository

import (
	"context"
	"encoding/json"

	"github.com/youyo/board/internal/cache"
)

// tryCacheFilter は cache state があれば cache から list を読み込み、Go-side filter
// を適用した entities を返す。state==nil / cache 読み込み失敗時は (nil, false) を返し、
// 呼び出し元は API fallback で対応する。
//
// filterFn は entity ごとに maintain/skip を返す。空フィルタでも全件 true を返す関数を
// 渡せばそのまま全件取得できる。
//
// 各 Repository は cache-first filter を有効にする場合、まず filter が cache 可能な範囲
// (Page/PerPage/UpdatedAt*/Tags/ResponseGroup などサーバー側専用フィールドを含まない) か
// を判定してから本関数を呼ぶ。
func tryCacheFilter[T any](
	ctx context.Context,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	profile, resource string,
	filterFn func(T) bool,
) ([]T, bool) {
	state, err := ss.Get(ctx, profile, resource)
	if err != nil || state == nil {
		return nil, false
	}
	entries, err := rc.List(ctx, profile, resource)
	if err != nil {
		return nil, false
	}
	out := make([]T, 0, len(entries))
	for _, e := range entries {
		var entity T
		if err := json.Unmarshal(e.PayloadJSON, &entity); err != nil {
			// payload corruption: cache fallback unsafe → bail out
			return nil, false
		}
		if filterFn(entity) {
			out = append(out, entity)
		}
	}
	return out, true
}
