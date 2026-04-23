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

// ProjectRepository manages cache -> refresh -> API fallback for the projects resource.
type ProjectRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewProjectRepository creates a new ProjectRepository.
func NewProjectRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *ProjectRepository {
	return &ProjectRepository{
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

const projectsResource = "projects"

// projectFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter bypasses the cache and calls the API directly because
// filtered results must not poison the full-entity cache.
func projectFilterIsZero(f boardapi.ProjectListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.CreatedAtGteq == "" &&
		f.CreatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.IncludeLostFlg == nil &&
		f.NameCont == "" &&
		f.ClientIDEq == 0 &&
		f.ClientNameCont == "" &&
		len(f.OrderStatusIn) == 0 &&
		len(f.DeliveryStatusIn) == 0 &&
		f.ProjectNoEq == "" &&
		f.ManagementNoEq == "" &&
		f.DeliveryDateGteq == "" &&
		f.DeliveryDateLteq == "" &&
		f.InvoiceDateGteq == "" &&
		f.InvoiceDateLteq == "" &&
		len(f.InvoiceTimingKbnIn) == 0 &&
		len(f.Tags) == 0 &&
		f.ResponseGroup == ""
}

// List returns projects.
//
// Behavior:
//   - Zero filter (boardapi.ProjectListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListProjects directly
//     so that server-side filter semantics (Ransack _cont / _eq / _in[] /
//     response_group) take effect. Returns *ListResult with Meta populated
//     from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *ProjectRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.ProjectListOptions) (*boardapi.ListResult[boardapi.ProjectEntity], error) {
	if !projectFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListProjects(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &projectsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, projectsResource, readOpts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}
	entries, err := r.cache.List(ctx, r.profile, projectsResource)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, projectsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, projectsResource)
		if err != nil {
			return nil, err
		}
	}
	entities, err := decodeEntries[boardapi.ProjectEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.ProjectEntity]{Items: entities}, nil
}

// ListEntities is a convenience wrapper around List that returns only the
// items slice. Used by service/find (Phase L: find 層は *ListResult を受け取らず
// []ProjectEntity を維持、Phase M で再検討）and other callers that do not need
// response metadata.
func (r *ProjectRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.ProjectListOptions) ([]boardapi.ProjectEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the project with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ProjectRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ProjectEntity, error) {
	fetcher := &projectsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, projectsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: projectsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ProjectEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, projectsResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// GetByIDWithGroup fetches a project directly from the API with a response_group parameter.
// Cache is bypassed because response_group data should not be cached.
func (r *ProjectRepository) GetByIDWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error) {
	result, err := r.api.GetProjectWithGroup(ctx, id, responseGroup)
	if err != nil {
		return nil, err
	}
	return result.Item, nil
}

// Search is a thin alias for ListEntities kept for the find layer's
// per-resource Search idiom. The filter is expressed as ProjectListOptions.
//
// find 層は *ListResult を扱わず []ProjectEntity を維持する（Phase L の方針）。
// Phase M で MCP / find の仕上げを行う際にインターフェースを再検討する。
func (r *ProjectRepository) Search(ctx context.Context, filter boardapi.ProjectListOptions, opts ReadOptions) ([]boardapi.ProjectEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
