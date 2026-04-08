package refresh

import (
	"context"
	"time"
)

// ForceRefreshResult は全件取得の結果サマリ。
type ForceRefreshResult struct {
	Profile      string
	Resource     string
	FetchedCount int
}

// ForceRefresh は全件を取得し、既存キャッシュを DeleteAll 後に UpsertMany する。
//
// アルゴリズム:
//  1. fetcher.ListAll(ctx) で全件取得
//  2. rawToEntries で Entry スライスに変換
//  3. ResourceCache.DeleteAll で既存キャッシュを全消去
//  4. ResourceCache.UpsertMany で全件挿入
//  5. Updater.MarkForceSuccess で sync_state 更新
func (r *Refresher) ForceRefresh(
	ctx context.Context,
	profile string,
	fetcher Fetcher,
	now time.Time,
	tz *time.Location,
) (*ForceRefreshResult, error) {
	resource := fetcher.ResourceName()

	// 1. 全件取得
	raws, err := fetcher.ListAll(ctx)
	if err != nil {
		_ = r.updater.MarkError(ctx, profile, resource, "", err.Error(), now)
		return nil, err
	}

	// 2. Entry に変換
	entries, err := rawToEntries(profile, resource, raws)
	if err != nil {
		return nil, err
	}

	// 3. 既存キャッシュを全消去
	if err := r.resourceCache.DeleteAll(ctx, profile, resource); err != nil {
		return nil, err
	}

	// 4. 全件挿入
	if err := r.resourceCache.UpsertMany(ctx, entries); err != nil {
		return nil, err
	}

	// 5. sync_state 更新
	if err := r.updater.MarkForceSuccess(ctx, profile, resource, now, tz); err != nil {
		return nil, err
	}

	return &ForceRefreshResult{
		Profile:      profile,
		Resource:     resource,
		FetchedCount: len(entries),
	}, nil
}
