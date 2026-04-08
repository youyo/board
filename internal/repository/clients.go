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

// ClientRepository manages cache -> refresh -> API fallback for the clients resource.
type ClientRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewClientRepository creates a new ClientRepository.
func NewClientRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *ClientRepository {
	return &ClientRepository{
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

const clientsResource = "clients"

// List returns all clients from the cache.
// Refresh is performed according to opts.
func (r *ClientRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ClientEntity, error) {
	fetcher := &clientsFetcher{api: r.api}
	now := time.Now()

	// Get SyncState
	state, err := r.syncStore.Get(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}

	// Determine and execute refresh
	if err := maybeRefresh(ctx, r.profile, clientsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	// Fetch from cache
	entries, err := r.cache.List(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}

	// Empty cache + no sync state -> implicit ForceRefresh
	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, clientsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, clientsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.ClientEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the client with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ClientRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ClientEntity, error) {
	fetcher := &clientsFetcher{api: r.api}
	now := time.Now()

	// Get SyncState
	state, err := r.syncStore.Get(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}

	// Determine and execute refresh
	if err := maybeRefresh(ctx, r.profile, clientsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: clientsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ClientEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	entity, err := r.api.GetClient(ctx, id)
	if err != nil {
		return nil, err
	}

	// upsert
	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, clientsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns clients filtered by the given parameters from the cache.
// Refresh is controlled by the same logic as List.
func (r *ClientRepository) Search(ctx context.Context, params boardapi.ClientSearchParams, opts ReadOptions) ([]boardapi.ClientEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterClients(all, params), nil
}

// filterClients performs in-memory filtering.
func filterClients(entities []boardapi.ClientEntity, params boardapi.ClientSearchParams) []boardapi.ClientEntity {
	var result []boardapi.ClientEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}
