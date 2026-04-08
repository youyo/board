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

// VendorRepository manages cache -> refresh -> API fallback for the vendors resource.
type VendorRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewVendorRepository creates a new VendorRepository.
func NewVendorRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *VendorRepository {
	return &VendorRepository{
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

const vendorsResource = "vendors"

// List returns all vendors from the cache.
func (r *VendorRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.VendorEntity, error) {
	fetcher := &vendorsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, vendorsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, vendorsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, vendorsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, vendorsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.VendorEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the vendor with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *VendorRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.VendorEntity, error) {
	fetcher := &vendorsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, vendorsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: vendorsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.VendorEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetVendor(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, vendorsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns vendors filtered by the given parameters from the cache.
func (r *VendorRepository) Search(ctx context.Context, params boardapi.VendorSearchParams, opts ReadOptions) ([]boardapi.VendorEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterVendors(all, params), nil
}

// filterVendors performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
func filterVendors(entities []boardapi.VendorEntity, params boardapi.VendorSearchParams) []boardapi.VendorEntity {
	var result []boardapi.VendorEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}
