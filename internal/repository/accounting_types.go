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

// AccountingTypeRepository manages cache -> refresh -> API fallback for the accounting_types resource.
type AccountingTypeRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewAccountingTypeRepository creates a new AccountingTypeRepository.
func NewAccountingTypeRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *AccountingTypeRepository {
	return &AccountingTypeRepository{
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

const accountingTypesResource = "accounting_types"

// List returns all accounting types from the cache.
func (r *AccountingTypeRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	fetcher := &accountingTypesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, accountingTypesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, accountingTypesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, accountingTypesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, accountingTypesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, accountingTypesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.AccountingTypeEntity](entries)
	if err != nil {
		return nil, err
	}

	return applyLimit(entities, opts.Limit), nil
}

// GetByID returns the accounting type with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *AccountingTypeRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.AccountingTypeEntity, error) {
	fetcher := &accountingTypesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, accountingTypesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, accountingTypesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: accountingTypesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.AccountingTypeEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetAccountingType(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, accountingTypesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns accounting types filtered by the given parameters from the cache.
func (r *AccountingTypeRepository) Search(ctx context.Context, params boardapi.AccountingTypeSearchParams, opts ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	listOpts := opts
	listOpts.Limit = 0
	all, err := r.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return applyLimit(filterAccountingTypes(all, params), opts.Limit), nil
}

// filterAccountingTypes performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterAccountingTypes(entities []boardapi.AccountingTypeEntity, params boardapi.AccountingTypeSearchParams) []boardapi.AccountingTypeEntity {
	var result []boardapi.AccountingTypeEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of AccountingTypeEntity directly from the API (cache bypass).
func (r *AccountingTypeRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.AccountingTypeEntity], error) {
	return r.api.ListAccountingTypesPage(ctx, page, perPage)
}
