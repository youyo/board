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

// PaymentTermRepository manages cache -> refresh -> API fallback for the payment_terms resource.
type PaymentTermRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewPaymentTermRepository creates a new PaymentTermRepository.
func NewPaymentTermRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *PaymentTermRepository {
	return &PaymentTermRepository{
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

const paymentTermsResource = "payment_terms"

// List returns all payment terms from the cache.
func (r *PaymentTermRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	fetcher := &paymentTermsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentTermsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, paymentTermsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, paymentTermsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, paymentTermsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, paymentTermsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.PaymentTermEntity](entries)
	if err != nil {
		return nil, err
	}

	return applyLimit(entities, opts.Limit), nil
}

// GetByID returns the payment term with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *PaymentTermRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.PaymentTermEntity, error) {
	fetcher := &paymentTermsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentTermsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, paymentTermsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: paymentTermsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.PaymentTermEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetPaymentTerm(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, paymentTermsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns payment terms filtered by the given parameters from the cache.
func (r *PaymentTermRepository) Search(ctx context.Context, params boardapi.PaymentTermSearchParams, opts ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	listOpts := opts
	listOpts.Limit = 0
	all, err := r.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return applyLimit(filterPaymentTerms(all, params), opts.Limit), nil
}

// filterPaymentTerms performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterPaymentTerms(entities []boardapi.PaymentTermEntity, params boardapi.PaymentTermSearchParams) []boardapi.PaymentTermEntity {
	var result []boardapi.PaymentTermEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of PaymentTermEntity directly from the API (cache bypass).
// TODO(M57): PageResult は M57 で ListResult[T] に移行予定。
//
//nolint:staticcheck
func (r *PaymentTermRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.PaymentTermEntity], error) {
	return r.api.ListPaymentTermsPage(ctx, page, perPage)
}
