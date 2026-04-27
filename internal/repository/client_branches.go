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

// ClientBranchRepository manages cache -> refresh -> API fallback for the client_branches resource.
type ClientBranchRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
}

// NewClientBranchRepository creates a new ClientBranchRepository.
func NewClientBranchRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
) *ClientBranchRepository {
	return &ClientBranchRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const clientBranchesResource = "client_branches"

// clientBranchFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter bypasses the cache and calls the API directly because
// filtered results must not poison the full-entity cache.
func clientBranchFilterIsZero(f boardapi.ClientBranchListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.ClientIDEq == 0 &&
		f.NameCont == ""
}

// List returns client branches.
//
// Behavior:
//   - Zero filter (boardapi.ClientBranchListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListClientBranches directly
//     so that server-side filter semantics (Ransack) take effect.
//     Returns *ListResult with Meta populated from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *ClientBranchRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.ClientBranchListOptions) (*boardapi.ListResult[boardapi.ClientBranchEntity], error) {
	if !clientBranchFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListClientBranches(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &clientBranchesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, clientBranchesResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, clientBranchesResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}
	entries, err := r.cache.List(ctx, r.profile, clientBranchesResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.ClientBranchEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.ClientBranchEntity]{Items: entities}, nil
}

// ListEntities は List の items のみを返す薄いラッパ。
// find 層（Phase L では *ListResult を扱わない）向け。
func (r *ClientBranchRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.ClientBranchListOptions) ([]boardapi.ClientBranchEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the client branch with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ClientBranchRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ClientBranchEntity, error) {
	fetcher := &clientBranchesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, clientBranchesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, clientBranchesResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: clientBranchesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ClientBranchEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetClientBranch(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, clientBranchesResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities と機能的に同等。
//
// find 層は *ListResult を扱わず []ClientBranchEntity を維持する（Phase L の方針）。
func (r *ClientBranchRepository) Search(ctx context.Context, filter boardapi.ClientBranchListOptions, opts ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
