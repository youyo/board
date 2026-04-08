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

// InvoiceRepository は invoices リソースのキャッシュ → リフレッシュ → API フォールバックを管理する。
type InvoiceRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewInvoiceRepository は InvoiceRepository を生成する。
func NewInvoiceRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *InvoiceRepository {
	return &InvoiceRepository{
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

const invoicesResource = "invoices"

// List は全請求をキャッシュから返す。
func (r *InvoiceRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.InvoiceEntity, error) {
	fetcher := &invoicesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, invoicesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, invoicesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, invoicesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, invoicesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, invoicesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.InvoiceEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID は指定 ID の請求をキャッシュから返す。
// キャッシュミス時は API から単体取得して upsert する。
func (r *InvoiceRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.InvoiceEntity, error) {
	fetcher := &invoicesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, invoicesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, invoicesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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

	// キャッシュミス → API 単体取得
	entity, err := r.api.GetInvoice(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, invoicesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search はパラメータでフィルタした請求をキャッシュから返す。
func (r *InvoiceRepository) Search(ctx context.Context, params boardapi.InvoiceSearchParams, opts ReadOptions) ([]boardapi.InvoiceEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterInvoices(all, params), nil
}

// filterInvoices はインメモリフィルタリングを行う。
// UpdatedAtFrom は差分取得カーソルとして使用するためフィルタには含めない。
func filterInvoices(entities []boardapi.InvoiceEntity, params boardapi.InvoiceSearchParams) []boardapi.InvoiceEntity {
	var result []boardapi.InvoiceEntity
	for _, e := range entities {
		if params.ClientID != 0 && e.ClientID != params.ClientID {
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
