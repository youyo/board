package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
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

// List returns all projects from the cache.
func (r *ProjectRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ProjectEntity, error) {
	fetcher := &projectsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, projectsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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

	return applyLimit(entities, opts.Limit), nil
}

// GetByID returns the project with the given ID from the cache.
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
	entity, err := r.api.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, projectsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// GetByIDWithGroup fetches a project directly from the API with a response_group parameter.
// Cache is bypassed because response_group data should not be cached.
func (r *ProjectRepository) GetByIDWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error) {
	return r.api.GetProjectWithGroup(ctx, id, responseGroup)
}

// Search returns projects filtered by the given parameters from the cache.
// If params.ResponseGroup is set, the cache is bypassed and the API is called directly.
func (r *ProjectRepository) Search(ctx context.Context, params boardapi.ProjectSearchParams, opts ReadOptions) ([]boardapi.ProjectEntity, error) {
	if params.ResponseGroup != "" {
		result, err := r.api.SearchProjects(ctx, params)
		if err != nil {
			return nil, err
		}
		return applyLimit(result, opts.Limit), nil
	}
	listOpts := opts
	listOpts.Limit = 0
	all, err := r.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return applyLimit(filterProjects(all, params), opts.Limit), nil
}

// filterProjects performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
// NOTE: Status フィルタは BOARD API の order_status_name / delivery_status_name
// の完全一致でフィルタリングする。実 API でも status パラメータは無視されるため
// in-memory での name ベースフィルタで代替する。
func filterProjects(entities []boardapi.ProjectEntity, params boardapi.ProjectSearchParams) []boardapi.ProjectEntity {
	var result []boardapi.ProjectEntity
	for _, e := range entities {
		// M44: ClientID は nested Client.ID に統合
		if params.ClientID != 0 {
			if e.Client == nil || e.Client.ID != params.ClientID {
				continue
			}
		}
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		// M44: Status は order_status_name / delivery_status_name で代替
		// BOARD API 実測で status パラメータは無視されるため name ベースで in-memory フィルタ
		if params.Status != "" && e.OrderStatusName != params.Status && e.DeliveryStatusName != params.Status {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of ProjectEntity directly from the API (cache bypass).
func (r *ProjectRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ProjectEntity], error) {
	return r.api.ListProjectsPage(ctx, page, perPage)
}
