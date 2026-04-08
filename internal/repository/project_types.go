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

// ProjectTypeRepository manages cache -> refresh -> API fallback for the project_types resource.
type ProjectTypeRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewProjectTypeRepository creates a new ProjectTypeRepository.
func NewProjectTypeRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *ProjectTypeRepository {
	return &ProjectTypeRepository{
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

const projectTypesResource = "project_types"

// List returns all project types from the cache.
func (r *ProjectTypeRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	fetcher := &projectTypesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectTypesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, projectTypesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, projectTypesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, projectTypesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, projectTypesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.ProjectTypeEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the project type with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ProjectTypeRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ProjectTypeEntity, error) {
	fetcher := &projectTypesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, projectTypesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, projectTypesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: projectTypesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ProjectTypeEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetProjectType(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, projectTypesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns project types filtered by the given parameters from the cache.
func (r *ProjectTypeRepository) Search(ctx context.Context, params boardapi.ProjectTypeSearchParams, opts ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterProjectTypes(all, params), nil
}

// filterProjectTypes performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterProjectTypes(entities []boardapi.ProjectTypeEntity, params boardapi.ProjectTypeSearchParams) []boardapi.ProjectTypeEntity {
	var result []boardapi.ProjectTypeEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}
