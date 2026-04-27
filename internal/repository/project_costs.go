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

// ProjectCostRepository manages cache -> refresh -> API fallback for the project_costs resource.
type ProjectCostRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
}

// NewProjectCostRepository creates a new ProjectCostRepository.
func NewProjectCostRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
) *ProjectCostRepository {
	return &ProjectCostRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const projectCostsResource = "project_costs"

// projectCostFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter bypasses the cache and calls the API directly because
// filtered results must not poison the full-entity cache.
func projectCostFilterIsZero(f boardapi.ProjectCostListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.ProjectIDEq == 0
}

// List returns project costs.
//
// Behavior:
//   - Zero filter (boardapi.ProjectCostListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListProjectCosts directly
//     so that server-side filter semantics (Ransack) take effect.
//     Returns *ListResult with Meta populated from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *ProjectCostRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.ProjectCostListOptions) (*boardapi.ListResult[boardapi.ProjectCostEntity], error) {
	if !projectCostFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListProjectCosts(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &projectCostsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectCostsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, projectCostsResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}
	entries, err := r.cache.List(ctx, r.profile, projectCostsResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.ProjectCostEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.ProjectCostEntity]{Items: entities}, nil
}

// ListEntities は List の items のみを返す薄いラッパ。
// find 層（Phase L では *ListResult を扱わない）向け。
func (r *ProjectCostRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.ProjectCostListOptions) ([]boardapi.ProjectCostEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the project cost with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ProjectCostRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ProjectCostEntity, error) {
	fetcher := &projectCostsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectCostsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, projectCostsResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: projectCostsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ProjectCostEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetProjectCost(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, projectCostsResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities と機能的に同等。
//
// find 層は *ListResult を扱わず []ProjectCostEntity を維持する（Phase L の方針）。
func (r *ProjectCostRepository) Search(ctx context.Context, filter boardapi.ProjectCostListOptions, opts ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
