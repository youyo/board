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
	autoRefresh bool
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
	autoRefresh bool,
) *ProjectCostRepository {
	return &ProjectCostRepository{
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

const projectCostsResource = "project_costs"

// List returns all project costs from the cache.
func (r *ProjectCostRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	fetcher := &projectCostsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectCostsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, projectCostsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, projectCostsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, projectCostsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, projectCostsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.ProjectCostEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the project cost with the given ID from the cache.
func (r *ProjectCostRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ProjectCostEntity, error) {
	fetcher := &projectCostsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectCostsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, projectCostsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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
	entity, err := r.api.GetProjectCost(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, projectCostsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns project costs filtered by the given parameters from the cache.
func (r *ProjectCostRepository) Search(ctx context.Context, params boardapi.ProjectCostSearchParams, opts ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterProjectCosts(all, params), nil
}

// filterProjectCosts performs in-memory filtering.
func filterProjectCosts(entities []boardapi.ProjectCostEntity, params boardapi.ProjectCostSearchParams) []boardapi.ProjectCostEntity {
	var result []boardapi.ProjectCostEntity
	for _, e := range entities {
		if params.ProjectID != 0 && e.ProjectID != params.ProjectID {
			continue
		}
		result = append(result, e)
	}
	return result
}
