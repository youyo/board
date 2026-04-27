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

// PurchaseOrderRepository manages cache -> refresh -> API fallback for the purchase_orders resource.
type PurchaseOrderRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
}

// NewPurchaseOrderRepository creates a new PurchaseOrderRepository.
func NewPurchaseOrderRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
) *PurchaseOrderRepository {
	return &PurchaseOrderRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const purchaseOrdersResource = "purchase_orders"

// purchaseOrderFilterIsZero reports whether the given filter is empty.
func purchaseOrderFilterIsZero(f boardapi.PurchaseOrderListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.VendorIDEq == 0 &&
		f.ProjectIDEq == 0 &&
		f.StatusEq == "" &&
		f.ResponseGroup == ""
}

// cacheablePurchaseOrderFilter は cache + Go-side filter で代替できるフィルタかを判定する。
func cacheablePurchaseOrderFilter(f boardapi.PurchaseOrderListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.ResponseGroup == ""
}

// matchPurchaseOrderFilter は PurchaseOrderEntity が filter に合致するかを Go-side で判定する。
func matchPurchaseOrderFilter(p boardapi.PurchaseOrderEntity, f boardapi.PurchaseOrderListOptions) bool {
	if f.VendorIDEq != 0 && p.VendorID != f.VendorIDEq {
		return false
	}
	if f.ProjectIDEq != 0 && p.ProjectID != f.ProjectIDEq {
		return false
	}
	if f.StatusEq != "" && p.Status != f.StatusEq {
		return false
	}
	return true
}

// List returns purchase orders.
//
// Behavior:
//   - Zero filter (boardapi.PurchaseOrderListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListPurchaseOrders directly
//     so that server-side filter semantics (Ransack _eq / _gteq) take effect.
//     Returns *ListResult with Meta populated from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *PurchaseOrderRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.PurchaseOrderListOptions) (*boardapi.ListResult[boardapi.PurchaseOrderEntity], error) {
	fetcher := &purchaseOrdersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, purchaseOrdersResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, purchaseOrdersResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	if !purchaseOrderFilterIsZero(filter) {
		if cacheablePurchaseOrderFilter(filter) {
			if entities, ok := tryCacheFilter[boardapi.PurchaseOrderEntity](
				ctx, r.cache, r.syncStore, r.profile, purchaseOrdersResource,
				func(p boardapi.PurchaseOrderEntity) bool { return matchPurchaseOrderFilter(p, filter) },
			); ok {
				return &boardapi.ListResult[boardapi.PurchaseOrderEntity]{Items: applyLimit(entities, readOpts.Limit)}, nil
			}
		}
		result, err := r.api.ListPurchaseOrders(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	entries, err := r.cache.List(ctx, r.profile, purchaseOrdersResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.PurchaseOrderEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.PurchaseOrderEntity]{Items: entities}, nil
}

// ListEntities は List のラッパで []PurchaseOrderEntity を返す。
// find 層など *ListResult を必要としない呼び出し元が使用する（Phase L の方針）。
func (r *PurchaseOrderRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.PurchaseOrderListOptions) ([]boardapi.PurchaseOrderEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the purchase order with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *PurchaseOrderRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	fetcher := &purchaseOrdersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, purchaseOrdersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, purchaseOrdersResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: purchaseOrdersResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.PurchaseOrderEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetPurchaseOrder(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, purchaseOrdersResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities に委譲する。
//
// find 層は *ListResult を扱わず []PurchaseOrderEntity を維持する（Phase L の方針）。
// Phase M で MCP / find の仕上げを行う際にインターフェースを再検討する。
func (r *PurchaseOrderRepository) Search(ctx context.Context, filter boardapi.PurchaseOrderListOptions, opts ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
