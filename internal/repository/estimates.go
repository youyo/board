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

// EstimateRepository manages cache -> refresh -> API fallback for the estimates resource.
type EstimateRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewEstimateRepository creates a new EstimateRepository.
func NewEstimateRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *EstimateRepository {
	return &EstimateRepository{
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

const estimatesResource = "estimates"

// List returns all estimates from the cache.
func (r *EstimateRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.EstimateEntity, error) {
	fetcher := &estimatesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, estimatesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, estimatesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, estimatesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, estimatesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, estimatesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.EstimateEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the estimate with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *EstimateRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.EstimateEntity, error) {
	fetcher := &estimatesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, estimatesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, estimatesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: estimatesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.EstimateEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetEstimate(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, estimatesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns estimates filtered by the given parameters from the cache.
func (r *EstimateRepository) Search(ctx context.Context, params boardapi.EstimateSearchParams, opts ReadOptions) ([]boardapi.EstimateEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterEstimates(all, params), nil
}

// filterEstimates performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterEstimates(entities []boardapi.EstimateEntity, params boardapi.EstimateSearchParams) []boardapi.EstimateEntity {
	var result []boardapi.EstimateEntity
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
