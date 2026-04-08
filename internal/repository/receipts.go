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

// ReceiptRepository manages cache -> refresh -> API fallback for the receipts resource.
type ReceiptRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewReceiptRepository creates a new ReceiptRepository.
func NewReceiptRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *ReceiptRepository {
	return &ReceiptRepository{
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

const receiptsResource = "receipts"

// List returns all receipts from the cache.
func (r *ReceiptRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ReceiptEntity, error) {
	fetcher := &receiptsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, receiptsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, receiptsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, receiptsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, receiptsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, receiptsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.ReceiptEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the receipt with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ReceiptRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ReceiptEntity, error) {
	fetcher := &receiptsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, receiptsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, receiptsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: receiptsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ReceiptEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetReceipt(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, receiptsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns receipts filtered by the given parameters from the cache.
func (r *ReceiptRepository) Search(ctx context.Context, params boardapi.ReceiptSearchParams, opts ReadOptions) ([]boardapi.ReceiptEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterReceipts(all, params), nil
}

// filterReceipts performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterReceipts(entities []boardapi.ReceiptEntity, params boardapi.ReceiptSearchParams) []boardapi.ReceiptEntity {
	var result []boardapi.ReceiptEntity
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
