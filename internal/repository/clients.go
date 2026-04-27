package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/cache/fold"
	"github.com/youyo/board/internal/refresh"
)

// ClientRepository manages cache -> refresh -> API fallback for the clients resource.
type ClientRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
}

// NewClientRepository creates a new ClientRepository.
func NewClientRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
) *ClientRepository {
	return &ClientRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const clientsResource = "clients"

// clientFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter is dispatched to either cache-first filter or API direct
// depending on whether the filter is server-only (cacheableClientFilter).
func clientFilterIsZero(f boardapi.ClientListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.NameCont == "" &&
		f.NameDispCont == "" &&
		f.InvoiceSystemNumberEq == "" &&
		f.CustomNoEq == "" &&
		len(f.Tags) == 0 &&
		f.ResponseGroup == ""
}

// cacheableClientFilter は cache + Go-side filter で代替できるフィルタかを判定する。
// Page/PerPage/UpdatedAt*/Tags/ResponseGroup/IncludeArchiveFlg はサーバー側専用フィールド
// のため含まれていれば API 直叩きにフォールバックする。
func cacheableClientFilter(f boardapi.ClientListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		len(f.Tags) == 0 &&
		f.ResponseGroup == ""
}

// matchClientFilter は ClientEntity が filter に合致するかを Go-side で判定する。
// BOARD API の Ransack 挙動 (NFKC + ToLower + TrimSpace、name_cont は name のみ対象) を近似する。
func matchClientFilter(c boardapi.ClientEntity, f boardapi.ClientListOptions) bool {
	if f.NameCont != "" && !fold.Contains(c.Name, f.NameCont) {
		return false
	}
	if f.NameDispCont != "" && !fold.Contains(c.NameDisp, f.NameDispCont) {
		return false
	}
	if f.InvoiceSystemNumberEq != "" {
		num := ""
		if c.InvoiceSystemNumber != nil {
			num = *c.InvoiceSystemNumber
		}
		if num != f.InvoiceSystemNumberEq {
			return false
		}
	}
	if f.CustomNoEq != "" {
		num := ""
		if c.CustomNo != nil {
			num = *c.CustomNo
		}
		if num != f.CustomNoEq {
			return false
		}
	}
	return true
}

// List returns clients.
//
// Behavior:
//   - Zero filter (boardapi.ClientListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListClients directly
//     so that server-side filter semantics (Ransack _cont / _eq / _gteq / tags[]
//     / response_group) take effect. Returns *ListResult with Meta populated
//     from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *ClientRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.ClientListOptions) (*boardapi.ListResult[boardapi.ClientEntity], error) {
	fetcher := &clientsFetcher{api: r.api}
	now := time.Now()

	// 共通の refresh フェーズ（明示 refresh 指示がある場合のみ動く）
	state, err := r.syncStore.Get(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, clientsResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	// 非ゼロフィルタ: cache-first → API fallback。
	// cache 可能な filter は Go-side で適用、サーバー専用フィールド (Page/Tags/ResponseGroup 等)
	// が含まれていれば API 直叩きする。
	if !clientFilterIsZero(filter) {
		if cacheableClientFilter(filter) {
			if entities, ok := tryCacheFilter[boardapi.ClientEntity](
				ctx, r.cache, r.syncStore, r.profile, clientsResource,
				func(c boardapi.ClientEntity) bool { return matchClientFilter(c, filter) },
			); ok {
				return &boardapi.ListResult[boardapi.ClientEntity]{Items: applyLimit(entities, readOpts.Limit)}, nil
			}
		}
		result, err := r.api.ListClients(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: cache 全件返却（state があれば）。state が無ければ空。
	entries, err := r.cache.List(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.ClientEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.ClientEntity]{Items: entities}, nil
}

// ListEntities is a convenience wrapper around List that returns only the
// items slice. Used by service/find (Phase L: find 層は *ListResult を受け取らず
// []ClientEntity を維持、Phase M で再検討）and other callers that do not need
// response metadata.
func (r *ClientRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.ClientListOptions) ([]boardapi.ClientEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the client with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ClientRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ClientEntity, error) {
	fetcher := &clientsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, clientsResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: clientsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ClientEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetClient(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, clientsResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search is a thin alias for ListEntities kept for the find layer's
// per-resource Search idiom. Behaviorally identical to ListEntities.
//
// find 層は *ListResult を扱わず []ClientEntity を維持する（Phase L の方針）。
// Phase M で MCP / find の仕上げを行う際にインターフェースを再検討する。
func (r *ClientRepository) Search(ctx context.Context, filter boardapi.ClientListOptions, opts ReadOptions) ([]boardapi.ClientEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
