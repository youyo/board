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
) *PaymentRepository {
	return &PaymentRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const paymentsResource = "payments"

// paymentFilterIsZero reports whether the given filter is empty.
func paymentFilterIsZero(f boardapi.PaymentListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.VendorIDEq == 0 &&
		f.PurchaseOrderIDEq == 0 &&
		f.StatusEq == "" &&
		f.ResponseGroup == ""
}

// cacheablePaymentFilter は cache + Go-side filter で代替できるフィルタかを判定する。
func cacheablePaymentFilter(f boardapi.PaymentListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.ResponseGroup == ""
}

// matchPaymentFilter は PaymentEntity が filter に合致するかを Go-side で判定する。
func matchPaymentFilter(p boardapi.PaymentEntity, f boardapi.PaymentListOptions) bool {
	if f.VendorIDEq != 0 && p.VendorID != f.VendorIDEq {
		return false
	}
	if f.PurchaseOrderIDEq != 0 && p.PurchaseOrderID != f.PurchaseOrderIDEq {
		return false
	}
	if f.StatusEq != "" && p.Status != f.StatusEq {
		return false
	}
	return true
}

// List returns payments.
//
// Behavior:
//   - Zero filter (boardapi.PaymentListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListPayments directly
//     so that server-side filter semantics (Ransack _eq / _gteq) take effect.
//     Returns *ListResult with Meta populated from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *PaymentRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.PaymentListOptions) (*boardapi.ListResult[boardapi.PaymentEntity], error) {
	fetcher := &paymentsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, paymentsResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	if !paymentFilterIsZero(filter) {
		if cacheablePaymentFilter(filter) {
			if entities, ok := tryCacheFilter[boardapi.PaymentEntity](
				ctx, r.cache, r.syncStore, r.profile, paymentsResource,
				func(p boardapi.PaymentEntity) bool { return matchPaymentFilter(p, filter) },
			); ok {
				return &boardapi.ListResult[boardapi.PaymentEntity]{Items: applyLimit(entities, readOpts.Limit)}, nil
			}
		}
		result, err := r.api.ListPayments(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	entries, err := r.cache.List(ctx, r.profile, paymentsResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.PaymentEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.PaymentEntity]{Items: entities}, nil
}

// ListEntities は List のラッパで []PaymentEntity を返す。
// find 層など *ListResult を必要としない呼び出し元が使用する（Phase L の方針）。
func (r *PaymentRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.PaymentListOptions) ([]boardapi.PaymentEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
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

	if err := maybeRefresh(ctx, r.profile, paymentsResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetPayment(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, paymentsResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities に委譲する。
//
// find 層は *ListResult を扱わず []PaymentEntity を維持する（Phase L の方針）。
// Phase M で MCP / find の仕上げを行う際にインターフェースを再検討する。
func (r *PaymentRepository) Search(ctx context.Context, filter boardapi.PaymentListOptions, opts ReadOptions) ([]boardapi.PaymentEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
