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

// PaymentRepository manages cache -> refresh -> API fallback for the payments resource.
type PaymentRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewPaymentRepository creates a new PaymentRepository.
func NewPaymentRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *PaymentRepository {
	return &PaymentRepository{
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

const paymentsResource = "payments"

// List returns all payments from the cache.
func (r *PaymentRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.PaymentEntity, error) {
	fetcher := &paymentsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, paymentsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, paymentsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, paymentsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, paymentsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.PaymentEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the payment with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *PaymentRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.PaymentEntity, error) {
	fetcher := &paymentsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, paymentsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: paymentsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.PaymentEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetPayment(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, paymentsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns payments filtered by the given parameters from the cache.
func (r *PaymentRepository) Search(ctx context.Context, params boardapi.PaymentSearchParams, opts ReadOptions) ([]boardapi.PaymentEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterPayments(all, params), nil
}

// filterPayments performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterPayments(entities []boardapi.PaymentEntity, params boardapi.PaymentSearchParams) []boardapi.PaymentEntity {
	var result []boardapi.PaymentEntity
	for _, e := range entities {
		if params.VendorID != 0 && e.VendorID != params.VendorID {
			continue
		}
		if params.PurchaseOrderID != 0 && e.PurchaseOrderID != params.PurchaseOrderID {
			continue
		}
		if params.Status != "" && e.Status != params.Status {
			continue
		}
		result = append(result, e)
	}
	return result
}
