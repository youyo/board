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

// DeliveryRepository manages cache -> refresh -> API fallback for the deliveries resource.
type DeliveryRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewDeliveryRepository creates a new DeliveryRepository.
func NewDeliveryRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *DeliveryRepository {
	return &DeliveryRepository{
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

const deliveriesResource = "deliveries"

// List returns all deliveries from the cache.
func (r *DeliveryRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.DeliveryEntity, error) {
	fetcher := &deliveriesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, deliveriesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, deliveriesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, deliveriesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, deliveriesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, deliveriesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.DeliveryEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the delivery with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *DeliveryRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.DeliveryEntity, error) {
	fetcher := &deliveriesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, deliveriesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, deliveriesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: deliveriesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.DeliveryEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetDelivery(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, deliveriesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns deliveries filtered by the given parameters from the cache.
func (r *DeliveryRepository) Search(ctx context.Context, params boardapi.DeliverySearchParams, opts ReadOptions) ([]boardapi.DeliveryEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterDeliveries(all, params), nil
}

// filterDeliveries performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterDeliveries(entities []boardapi.DeliveryEntity, params boardapi.DeliverySearchParams) []boardapi.DeliveryEntity {
	var result []boardapi.DeliveryEntity
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
