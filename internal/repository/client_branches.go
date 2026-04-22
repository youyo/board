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

	return applyLimit(entities, opts.Limit), nil
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

// Search returns client branches filtered by the given parameters.
// When ClientID is set, the API-side client_id filter is used directly because
// the BOARD API response does not include a flat client_id field; it nests the
// parent client as {"client": {"id": N, ...}}. In-memory filtering on
// ClientBranchEntity.ClientID (which is always 0 after unmarshal) would
// silently return zero results. Name-only searches fall back to a full list
// with in-memory filtering (BOARD API ignores the name parameter).
func (r *ClientBranchRepository) Search(ctx context.Context, params boardapi.ClientBranchSearchParams, opts ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	if params.ClientID != 0 {
		// Use API-side filter; apply Name in-memory afterward if needed.
		entities, err := r.api.SearchClientBranches(ctx, params)
		if err != nil {
			return nil, err
		}
		if params.Name == "" {
			return applyLimit(entities, opts.Limit), nil
		}
		return applyLimit(filterClientBranchesByName(entities, params.Name), opts.Limit), nil
	}
	// Name-only (or empty) filter: fall back to full list + in-memory.
	listOpts := opts
	listOpts.Limit = 0
	all, err := r.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return applyLimit(filterClientBranchesByName(all, params.Name), opts.Limit), nil
}

// filterClientBranchesByName performs in-memory name filtering.
func filterClientBranchesByName(entities []boardapi.ClientBranchEntity, name string) []boardapi.ClientBranchEntity {
	if name == "" {
		return entities
	}
	var result []boardapi.ClientBranchEntity
	for _, e := range entities {
		if strings.Contains(e.Name, name) {
			result = append(result, e)
		}
	}
	return result
}

// ListPage retrieves a single page of ClientBranchEntity directly from the API (cache bypass).
func (r *ClientBranchRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ClientBranchEntity], error) {
	return r.api.ListClientBranchesPage(ctx, page, perPage)
}
