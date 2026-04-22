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

// VendorBranchRepository manages cache -> refresh -> API fallback for the vendor_branches resource.
type VendorBranchRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewVendorBranchRepository creates a new VendorBranchRepository.
func NewVendorBranchRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *VendorBranchRepository {
	return &VendorBranchRepository{
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

const vendorBranchesResource = "vendor_branches"

// List returns all vendor branches from the cache.
func (r *VendorBranchRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	fetcher := &vendorBranchesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorBranchesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, vendorBranchesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, vendorBranchesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, vendorBranchesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, vendorBranchesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.VendorBranchEntity](entries)
	if err != nil {
		return nil, err
	}

	return applyLimit(entities, opts.Limit), nil
}

// GetByID returns the vendor branch with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *VendorBranchRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.VendorBranchEntity, error) {
	fetcher := &vendorBranchesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorBranchesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, vendorBranchesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: vendorBranchesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.VendorBranchEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss → fetch single entity from API
	entity, err := r.api.GetVendorBranch(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, vendorBranchesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns vendor branches filtered by the given parameters from the cache.
func (r *VendorBranchRepository) Search(ctx context.Context, params boardapi.VendorBranchSearchParams, opts ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	listOpts := opts
	listOpts.Limit = 0
	all, err := r.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return applyLimit(filterVendorBranches(all, params), opts.Limit), nil
}

// filterVendorBranches performs in-memory filtering.
func filterVendorBranches(entities []boardapi.VendorBranchEntity, params boardapi.VendorBranchSearchParams) []boardapi.VendorBranchEntity {
	var result []boardapi.VendorBranchEntity
	for _, e := range entities {
		if params.VendorID != 0 && e.VendorID() != params.VendorID {
			continue
		}
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of VendorBranchEntity directly from the API (cache bypass).
func (r *VendorBranchRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.VendorBranchEntity], error) {
	return r.api.ListVendorBranchesPage(ctx, page, perPage)
}
