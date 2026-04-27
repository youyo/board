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

// paymentFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter bypasses the cache and calls the API directly because
// filtered results must not poison the full-entity cache.
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
	if !paymentFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListPayments(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &paymentsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, paymentsResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
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
