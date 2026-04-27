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

// ProjectRepository manages cache -> refresh -> API fallback for the projects resource.
type ProjectRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
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
) *ProjectRepository {
	return &ProjectRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const projectsResource = "projects"

// cacheableProjectFilter は cache + Go-side filter で代替できるフィルタかを判定する。
// ResponseGroup が含まれる場合は埋め込み entity (Estimate / Order 等) の再現が
// flat な cache では不可能なため API fallback する。
// OrderStatusIn / DeliveryStatusIn は ProjectEntity に整数フィールドが無いため不可。
func cacheableProjectFilter(f boardapi.ProjectListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.CreatedAtGteq == "" &&
		f.CreatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.IncludeLostFlg == nil &&
		len(f.OrderStatusIn) == 0 &&
		len(f.DeliveryStatusIn) == 0 &&
		f.DeliveryDateGteq == "" &&
		f.DeliveryDateLteq == "" &&
		f.InvoiceDateGteq == "" &&
		f.InvoiceDateLteq == "" &&
		len(f.InvoiceTimingKbnIn) == 0 &&
		len(f.Tags) == 0 &&
		f.ResponseGroup == ""
}

// matchProjectFilter は ProjectEntity が filter に合致するかを Go-side で判定する。
func matchProjectFilter(p boardapi.ProjectEntity, f boardapi.ProjectListOptions) bool {
	if f.NameCont != "" && !fold.Contains(p.Name, f.NameCont) {
		return false
	}
	if f.ClientIDEq != 0 {
		if p.Client == nil || p.Client.ID != f.ClientIDEq {
			return false
		}
	}
	if f.ClientNameCont != "" {
		if p.Client == nil || !fold.Contains(p.Client.Name, f.ClientNameCont) {
			return false
		}
	}
	if f.ProjectNoEq != "" {
		if p.ProjectNo == nil {
			return false
		}
		if strconv.Itoa(*p.ProjectNo) != f.ProjectNoEq {
			return false
		}
	}
	if f.ManagementNoEq != "" {
		mn := ""
		if p.ManagementNo != nil {
			mn = *p.ManagementNo
		}
		if mn != f.ManagementNoEq {
			return false
		}
	}
	return true
}

// projectFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter is dispatched to either cache-first filter or API direct
// depending on whether the filter is cacheable.
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
	fetcher := &projectsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, projectsResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	if !projectFilterIsZero(filter) {
		if cacheableProjectFilter(filter) {
			if entities, ok := tryCacheFilter[boardapi.ProjectEntity](
				ctx, r.cache, r.syncStore, r.profile, projectsResource,
				func(p boardapi.ProjectEntity) bool { return matchProjectFilter(p, filter) },
			); ok {
				return &boardapi.ListResult[boardapi.ProjectEntity]{Items: applyLimit(entities, readOpts.Limit)}, nil
			}
		}
		result, err := r.api.ListProjects(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	entries, err := r.cache.List(ctx, r.profile, projectsResource)
	if err != nil {
		return nil, err
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

	if err := maybeRefresh(ctx, r.profile, projectsResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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
