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

// InvoiceRepository manages cache -> refresh -> API fallback for the invoices resource.
type InvoiceRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
}

// NewInvoiceRepository creates a new InvoiceRepository.
func NewInvoiceRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
) *InvoiceRepository {
	return &InvoiceRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const invoicesResource = "invoices"

// invoiceFilterIsZero reports whether the given filter is empty.
func invoiceFilterIsZero(f boardapi.InvoiceListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.ClientIDEq == 0 &&
		f.ProjectIDEq == 0 &&
		f.StatusEq == "" &&
		f.ResponseGroup == ""
}

// cacheableInvoiceFilter は cache + Go-side filter で代替できるフィルタかを判定する。
func cacheableInvoiceFilter(f boardapi.InvoiceListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.ResponseGroup == ""
}

// matchInvoiceFilter は InvoiceEntity が filter に合致するかを Go-side で判定する。
func matchInvoiceFilter(i boardapi.InvoiceEntity, f boardapi.InvoiceListOptions) bool {
	if f.ClientIDEq != 0 && i.ClientID != f.ClientIDEq {
		return false
	}
	if f.ProjectIDEq != 0 && i.ProjectID != f.ProjectIDEq {
		return false
	}
	if f.StatusEq != "" && i.Status != f.StatusEq {
		return false
	}
	return true
}

// List returns invoices.
//
// Behavior:
//   - Zero filter (boardapi.InvoiceListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListInvoices directly
//     so that server-side filter semantics (Ransack _eq / _gteq) take effect.
//     Returns *ListResult with Meta populated from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *InvoiceRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.InvoiceListOptions) (*boardapi.ListResult[boardapi.InvoiceEntity], error) {
	fetcher := &invoicesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, invoicesResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, invoicesResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	if !invoiceFilterIsZero(filter) {
		if cacheableInvoiceFilter(filter) {
			if entities, ok := tryCacheFilter[boardapi.InvoiceEntity](
				ctx, r.cache, r.syncStore, r.profile, invoicesResource,
				func(i boardapi.InvoiceEntity) bool { return matchInvoiceFilter(i, filter) },
			); ok {
				return &boardapi.ListResult[boardapi.InvoiceEntity]{Items: applyLimit(entities, readOpts.Limit)}, nil
			}
		}
		result, err := r.api.ListInvoices(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	entries, err := r.cache.List(ctx, r.profile, invoicesResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.InvoiceEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.InvoiceEntity]{Items: entities}, nil
}

// ListEntities は List のラッパで []InvoiceEntity を返す。
// find 層など *ListResult を必要としない呼び出し元が使用する（Phase L の方針）。
func (r *InvoiceRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.InvoiceListOptions) ([]boardapi.InvoiceEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the invoice with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *InvoiceRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.InvoiceEntity, error) {
	fetcher := &invoicesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, invoicesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, invoicesResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: invoicesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.InvoiceEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetInvoice(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, invoicesResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities に委譲する。
//
// find 層は *ListResult を扱わず []InvoiceEntity を維持する（Phase L の方針）。
// Phase M で MCP / find の仕上げを行う際にインターフェースを再検討する。
func (r *InvoiceRepository) Search(ctx context.Context, filter boardapi.InvoiceListOptions, opts ReadOptions) ([]boardapi.InvoiceEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
