package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/refresh"
)

// entitiesToRaw は任意のエンティティスライスを []json.RawMessage に変換する。
func entitiesToRaw[T any](entities []T) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(entities))
	for _, e := range entities {
		raw, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		result = append(result, raw)
	}
	return result, nil
}

// decodeEntries は []cache.Entry を []T に変換する。
func decodeEntries[T any](entries []cache.Entry) ([]T, error) {
	result := make([]T, 0, len(entries))
	for _, e := range entries {
		var entity T
		if err := json.Unmarshal(e.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, nil
}

// upsertRaw は json.RawMessage をキャッシュに upsert する。
// id と updated_at を raw から抽出する。
func upsertRaw(ctx context.Context, rc *cache.ResourceCache, profile, resource string, raw json.RawMessage) error {
	var idHolder struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &idHolder); err != nil {
		return err
	}
	var updatedAtHolder struct {
		UpdatedAt string `json:"updated_at"`
	}
	_ = json.Unmarshal(raw, &updatedAtHolder)

	var updatedAt sql.NullString
	if updatedAtHolder.UpdatedAt != "" {
		updatedAt = sql.NullString{String: updatedAtHolder.UpdatedAt, Valid: true}
	}

	return rc.Upsert(ctx, cache.Entry{
		Key: cache.EntityKey{
			Profile:  profile,
			Resource: resource,
			EntityID: strconv.Itoa(idHolder.ID),
		},
		PayloadJSON: raw,
		UpdatedAt:   updatedAt,
	})
}

// maybeRefresh はリフレッシュ要否を判定し、必要なら実行する。
// ForceRefresh が true なら ForceRefresh を優先。
// Refresh が true または autoRefresh かつ NeedsDailyRefresh なら DeltaRefresh。
// DeltaRefresh エラー時は stale 返却（ログのみ）。
// ForceRefresh エラー時はエラーを伝播。
func maybeRefresh(
	ctx context.Context,
	profile, resource string,
	opts ReadOptions,
	state *cache.SyncState,
	autoRefresh bool,
	tz *time.Location,
	lm *refresh.LockManager,
	refresher *refresh.Refresher,
	fetcher refresh.Fetcher,
	now time.Time,
) error {
	switch {
	case opts.ForceRefresh:
		return lm.WithLock(ctx, profile, resource, func() error {
			_, err := refresher.ForceRefresh(ctx, profile, fetcher, now, tz)
			return err
		})
	case opts.Refresh || (autoRefresh && refresh.NeedsDailyRefresh(state, now, tz)):
		err := lm.WithLock(ctx, profile, resource, func() error {
			_, err := refresher.DeltaRefresh(ctx, profile, fetcher, now, tz)
			return err
		})
		if opts.Refresh && err != nil {
			// DeltaRefresh 失敗は stale 返却（ログのみ）
			slog.Warn("DeltaRefresh failed, returning stale cache",
				"profile", profile,
				"resource", resource,
				"error", err,
			)
			return nil
		}
		return err
	}
	return nil
}

// --- clients Fetcher ---

// clientsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type clientsFetcher struct {
	api *boardapi.Client
}

func (f *clientsFetcher) ResourceName() string { return "clients" }

func (f *clientsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListClients(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *clientsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.ClientSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchClients(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- client_branches Fetcher ---

// clientBranchesFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type clientBranchesFetcher struct {
	api *boardapi.Client
}

func (f *clientBranchesFetcher) ResourceName() string { return "client_branches" }

func (f *clientBranchesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListClientBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: ClientBranchSearchParams に UpdatedAtFrom がないため全件取得。
func (f *clientBranchesFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListClientBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- contacts Fetcher ---

// contactsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type contactsFetcher struct {
	api *boardapi.Client
}

func (f *contactsFetcher) ResourceName() string { return "contacts" }

func (f *contactsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: ContactSearchParams に UpdatedAtFrom がないため全件取得。
func (f *contactsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- projects Fetcher ---

// projectsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type projectsFetcher struct {
	api *boardapi.Client
}

func (f *projectsFetcher) ResourceName() string { return "projects" }

func (f *projectsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *projectsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.ProjectSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchProjects(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- project_costs Fetcher ---

// projectCostsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type projectCostsFetcher struct {
	api *boardapi.Client
}

func (f *projectCostsFetcher) ResourceName() string { return "project_costs" }

func (f *projectCostsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjectCosts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: ProjectCostSearchParams に UpdatedAtFrom がないため全件取得。
func (f *projectCostsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjectCosts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}
