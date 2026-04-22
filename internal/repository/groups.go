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

// GroupRepository manages cache -> refresh -> API fallback for the groups resource.
type GroupRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewGroupRepository creates a new GroupRepository.
func NewGroupRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *GroupRepository {
	return &GroupRepository{
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

const groupsResource = "groups"

// List returns all groups from the cache.
func (r *GroupRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.GroupEntity, error) {
	fetcher := &groupsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, groupsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, groupsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, groupsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, groupsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, groupsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.GroupEntity](entries)
	if err != nil {
		return nil, err
	}

	return applyLimit(entities, opts.Limit), nil
}

// GetByID returns the group with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *GroupRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.GroupEntity, error) {
	fetcher := &groupsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, groupsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, groupsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: groupsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.GroupEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, groupsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns groups filtered by the given parameters from the cache.
func (r *GroupRepository) Search(ctx context.Context, params boardapi.GroupSearchParams, opts ReadOptions) ([]boardapi.GroupEntity, error) {
	listOpts := opts
	listOpts.Limit = 0
	all, err := r.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return applyLimit(filterGroups(all, params), opts.Limit), nil
}

// filterGroups performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterGroups(entities []boardapi.GroupEntity, params boardapi.GroupSearchParams) []boardapi.GroupEntity {
	var result []boardapi.GroupEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of GroupEntity directly from the API (cache bypass).
func (r *GroupRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.GroupEntity], error) {
	return r.api.ListGroupsPage(ctx, page, perPage)
}
