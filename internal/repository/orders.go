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

// OrderRepository manages cache -> refresh -> API fallback for the orders resource.
type OrderRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewOrderRepository creates a new OrderRepository.
func NewOrderRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *OrderRepository {
	return &OrderRepository{
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

const ordersResource = "orders"

// List returns all orders from the cache.
func (r *OrderRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.OrderEntity, error) {
	fetcher := &ordersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, ordersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, ordersResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, ordersResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, ordersResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, ordersResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.OrderEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the order with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *OrderRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.OrderEntity, error) {
	fetcher := &ordersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, ordersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, ordersResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: ordersResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.OrderEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, ordersResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns orders filtered by the given parameters from the cache.
func (r *OrderRepository) Search(ctx context.Context, params boardapi.OrderSearchParams, opts ReadOptions) ([]boardapi.OrderEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterOrders(all, params), nil
}

// filterOrders performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterOrders(entities []boardapi.OrderEntity, params boardapi.OrderSearchParams) []boardapi.OrderEntity {
	var result []boardapi.OrderEntity
	for _, e := range entities {
		if params.ClientID != 0 && e.ClientID != params.ClientID {
			continue
		}
		if params.ProjectID != 0 && e.ProjectID != params.ProjectID {
			continue
		}
		if params.Status != "" && e.Status != params.Status {
			continue
		}
		result = append(result, e)
	}
	return result
}
