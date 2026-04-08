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

// PurchaseOrderRepository は purchase_orders リソースのキャッシュ → リフレッシュ → API フォールバックを管理する。
type PurchaseOrderRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewPurchaseOrderRepository は PurchaseOrderRepository を生成する。
func NewPurchaseOrderRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *PurchaseOrderRepository {
	return &PurchaseOrderRepository{
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

const purchaseOrdersResource = "purchase_orders"

// List は全発注書をキャッシュから返す。
func (r *PurchaseOrderRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	fetcher := &purchaseOrdersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, purchaseOrdersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, purchaseOrdersResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, purchaseOrdersResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, purchaseOrdersResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, purchaseOrdersResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.PurchaseOrderEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID は指定 ID の発注書をキャッシュから返す。
// キャッシュミス時は API から単体取得して upsert する。
func (r *PurchaseOrderRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	fetcher := &purchaseOrdersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, purchaseOrdersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, purchaseOrdersResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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

	// キャッシュミス → API 単体取得
	entity, err := r.api.GetPurchaseOrder(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, purchaseOrdersResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search はパラメータでフィルタした発注書をキャッシュから返す。
func (r *PurchaseOrderRepository) Search(ctx context.Context, params boardapi.PurchaseOrderSearchParams, opts ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterPurchaseOrders(all, params), nil
}

// filterPurchaseOrders はインメモリフィルタリングを行う。
// UpdatedAtFrom は差分取得カーソルとして使用するためフィルタには含めない。
func filterPurchaseOrders(entities []boardapi.PurchaseOrderEntity, params boardapi.PurchaseOrderSearchParams) []boardapi.PurchaseOrderEntity {
	var result []boardapi.PurchaseOrderEntity
	for _, e := range entities {
		if params.VendorID != 0 && e.VendorID != params.VendorID {
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
