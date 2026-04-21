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

// VendorContactRepository manages cache -> refresh -> API fallback for the vendor_contacts resource.
type VendorContactRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewVendorContactRepository creates a new VendorContactRepository.
func NewVendorContactRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *VendorContactRepository {
	return &VendorContactRepository{
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

const vendorContactsResource = "vendor_contacts"

// List returns all vendor contacts from the cache.
func (r *VendorContactRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.VendorContactEntity, error) {
	fetcher := &vendorContactsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorContactsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, vendorContactsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, vendorContactsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, vendorContactsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, vendorContactsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.VendorContactEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the vendor contact with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *VendorContactRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.VendorContactEntity, error) {
	fetcher := &vendorContactsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorContactsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, vendorContactsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: vendorContactsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.VendorContactEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetVendorContact(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, vendorContactsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns vendor contacts filtered by the given parameters from the cache.
func (r *VendorContactRepository) Search(ctx context.Context, params boardapi.VendorContactSearchParams, opts ReadOptions) ([]boardapi.VendorContactEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterVendorContacts(all, params), nil
}

// filterVendorContacts は in-memory フィルタリングを行う。
// VendorID は accessor（VendorID()）経由で参照する（M42 再設計: nested Vendor 構造）。
// Name は DisplayName()（LastName+FirstName）で部分一致検索する。
// Email は *string 型のため nil ガード付きで参照する。
func filterVendorContacts(entities []boardapi.VendorContactEntity, params boardapi.VendorContactSearchParams) []boardapi.VendorContactEntity {
	var result []boardapi.VendorContactEntity
	for _, e := range entities {
		if params.VendorID != 0 && e.VendorID() != params.VendorID {
			continue
		}
		if params.Name != "" && !strings.Contains(e.DisplayName(), params.Name) {
			continue
		}
		if params.Email != "" {
			if e.Email == nil || !strings.Contains(*e.Email, params.Email) {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of VendorContactEntity directly from the API (cache bypass).
func (r *VendorContactRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.VendorContactEntity], error) {
	return r.api.ListVendorContactsPage(ctx, page, perPage)
}
