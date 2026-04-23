package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
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
	autoRefresh bool
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
	autoRefresh bool,
) *ClientRepository {
	return &ClientRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
		autoRefresh: autoRefresh,
	}
}

const clientsResource = "clients"

// clientFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter bypasses the cache and calls the API directly because
// filtered results must not poison the full-entity cache.
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
	if !clientFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListClients(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &clientsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, clientsResource, readOpts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}
	entries, err := r.cache.List(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, clientsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, clientsResource)
		if err != nil {
			return nil, err
		}
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

	if err := maybeRefresh(ctx, r.profile, clientsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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
