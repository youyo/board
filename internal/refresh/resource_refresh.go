package refresh

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/youyo/board/internal/cache"
)

// Fetcher は refresh エンジンが API から entity を取得するための抽象。
// resource ごとに実装を提供する（clients, projects, ...）。
type Fetcher interface {
	// ResourceName はリソース識別子（例: "clients"）を返す。
	ResourceName() string
	// ListAll は全件取得する。
	// 戻り値は json.RawMessage のスライス（各要素が1 entity）。
	ListAll(ctx context.Context) ([]json.RawMessage, error)
	// ListUpdatedSince は updated_at >= since の entity を取得する。
	// since が空文字の場合は全件取得と同等とする。
	ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error)
}

// DeltaRefreshResult は差分取得の結果サマリ。
type DeltaRefreshResult struct {
	Profile      string
	Resource     string
	FetchedCount int
	NewCursor    string // 更新後のカーソル値（空 = 変化なし）
}

// Refresher は差分・全件リフレッシュの実行エンジン。
type Refresher struct {
	resourceCache *cache.ResourceCache
	syncStore     *cache.SyncStateStore
	updater       *Updater
}

// NewRefresher は Refresher を生成する。
func NewRefresher(rc *cache.ResourceCache, ss *cache.SyncStateStore) *Refresher {
	return &Refresher{
		resourceCache: rc,
		syncStore:     ss,
		updater:       NewUpdater(ss),
	}
}

// extractID は json.RawMessage から "id" フィールドを文字列で返す。
func extractID(raw json.RawMessage) (string, error) {
	var v struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	if v.ID == 0 {
		return "", errors.New("entity has no id or id=0")
	}
	return strconv.Itoa(v.ID), nil
}

// extractUpdatedAt は json.RawMessage から "updated_at" フィールドを返す。
// 存在しない場合は空文字を返す。
func extractUpdatedAt(raw json.RawMessage) string {
	var v struct {
		UpdatedAt string `json:"updated_at"`
	}
	_ = json.Unmarshal(raw, &v)
	return v.UpdatedAt
}

// rawToEntries は []json.RawMessage を []cache.Entry に変換する。
func rawToEntries(profile, resource string, raws []json.RawMessage) ([]cache.Entry, error) {
	entries := make([]cache.Entry, 0, len(raws))
	for _, raw := range raws {
		id, err := extractID(raw)
		if err != nil {
			return nil, err
		}
		updatedAt := extractUpdatedAt(raw)
		var updatedAtNull sql.NullString
		if updatedAt != "" {
			updatedAtNull = sql.NullString{String: updatedAt, Valid: true}
		}
		entries = append(entries, cache.Entry{
			Key: cache.EntityKey{
				Profile:  profile,
				Resource: resource,
				EntityID: id,
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAtNull,
		})
	}
	return entries, nil
}

// maxUpdatedAt は []json.RawMessage の中から最大の updated_at を文字列で返す。
// updated_at が存在しない entity はスキップする。
// 全 entity に updated_at がない場合は空文字を返す。
func maxUpdatedAt(raws []json.RawMessage) string {
	max := ""
	for _, raw := range raws {
		ua := extractUpdatedAt(raw)
		if ua == "" {
			continue
		}
		if ua > max {
			max = ua
		}
	}
	return max
}

// cursorFromState は SyncState から現在のカーソル値を返す。
// state が nil またはカーソルが無効な場合は空文字を返す。
func cursorFromState(state *cache.SyncState) string {
	if state == nil {
		return ""
	}
	if !state.CursorUpdatedAt.Valid {
		return ""
	}
	return state.CursorUpdatedAt.String
}

// DeltaRefresh は cursor_updated_at 以降の差分を取得し、キャッシュへ upsert する。
//
// アルゴリズム:
//  1. SyncState.cursor_updated_at を読む（nil なら "" = 全件）
//  2. fetcher.ListUpdatedSince(ctx, cursor) で差分取得
//  3. rawToEntries で Entry スライスに変換
//  4. ResourceCache.UpsertMany でキャッシュ更新
//  5. 取得結果中の最大 updated_at を新カーソルとして算出
//  6. Updater.MarkDeltaSuccess で sync_state 更新
func (r *Refresher) DeltaRefresh(
	ctx context.Context,
	profile string,
	fetcher Fetcher,
	now time.Time,
	tz *time.Location,
) (*DeltaRefreshResult, error) {
	resource := fetcher.ResourceName()

	// 1. 現在のカーソルを取得
	state, err := r.syncStore.Get(ctx, profile, resource)
	if err != nil {
		return nil, err
	}
	cursor := cursorFromState(state)

	// 2. 差分取得
	raws, err := fetcher.ListUpdatedSince(ctx, cursor)
	if err != nil {
		_ = r.updater.MarkError(ctx, profile, resource, "", err.Error(), now)
		return nil, err
	}

	// 3. Entry に変換
	entries, err := rawToEntries(profile, resource, raws)
	if err != nil {
		return nil, err
	}

	// 4. キャッシュ更新
	if err := r.resourceCache.UpsertMany(ctx, entries); err != nil {
		return nil, err
	}

	// 5. 新カーソル算出
	newCursor := maxUpdatedAt(raws)

	// 6. sync_state 更新
	if err := r.updater.MarkDeltaSuccess(ctx, profile, resource, newCursor, now, tz); err != nil {
		return nil, err
	}

	return &DeltaRefreshResult{
		Profile:      profile,
		Resource:     resource,
		FetchedCount: len(entries),
		NewCursor:    newCursor,
	}, nil
}
