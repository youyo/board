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

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
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

// Search returns projects filtered by the given parameters from the cache.
func (r *ProjectRepository) Search(ctx context.Context, params boardapi.ProjectSearchParams, opts ReadOptions) ([]boardapi.ProjectEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterProjects(all, params), nil
}

// filterProjects performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterProjects(entities []boardapi.ProjectEntity, params boardapi.ProjectSearchParams) []boardapi.ProjectEntity {
	var result []boardapi.ProjectEntity
	for _, e := range entities {
		if params.ClientID != 0 && e.ClientID != params.ClientID {
			continue
		}
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		if params.Status != "" && e.Status != params.Status {
			continue
		}
		result = append(result, e)
	}
	return result
}
