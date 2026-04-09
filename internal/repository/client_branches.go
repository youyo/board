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

// ClientBranchRepository manages cache -> refresh -> API fallback for the client_branches resource.
type ClientBranchRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
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
	autoRefresh bool,
) *ClientBranchRepository {
	return &ClientBranchRepository{
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

const clientBranchesResource = "client_branches"

// List returns all client branches from the cache.
func (r *ClientBranchRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	fetcher := &clientBranchesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, clientBranchesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, clientBranchesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, clientBranchesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, clientBranchesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, clientBranchesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.ClientBranchEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the client branch with the given ID from the cache.
func (r *ClientBranchRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ClientBranchEntity, error) {
	fetcher := &clientBranchesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, clientBranchesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, clientBranchesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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
	entity, err := r.api.GetClientBranch(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, clientBranchesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns client branches filtered by the given parameters from the cache.
func (r *ClientBranchRepository) Search(ctx context.Context, params boardapi.ClientBranchSearchParams, opts ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterClientBranches(all, params), nil
}

// filterClientBranches performs in-memory filtering.
func filterClientBranches(entities []boardapi.ClientBranchEntity, params boardapi.ClientBranchSearchParams) []boardapi.ClientBranchEntity {
	var result []boardapi.ClientBranchEntity
	for _, e := range entities {
		if params.ClientID != 0 && e.ClientID != params.ClientID {
			continue
		}
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of ClientBranchEntity directly from the API (cache bypass).
func (r *ClientBranchRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ClientBranchEntity], error) {
	return r.api.ListClientBranchesPage(ctx, page, perPage)
}
