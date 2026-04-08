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

// PaymentTermRepository は payment_terms リソースのキャッシュ → リフレッシュ → API フォールバックを管理する。
type PaymentTermRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewPaymentTermRepository は PaymentTermRepository を生成する。
func NewPaymentTermRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *PaymentTermRepository {
	return &PaymentTermRepository{
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

const paymentTermsResource = "payment_terms"

// List は全支払条件をキャッシュから返す。
func (r *PaymentTermRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	fetcher := &paymentTermsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentTermsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, paymentTermsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, paymentTermsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, paymentTermsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, paymentTermsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.PaymentTermEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID は指定 ID の支払条件をキャッシュから返す。
// キャッシュミス時は API から単体取得して upsert する。
func (r *PaymentTermRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.PaymentTermEntity, error) {
	fetcher := &paymentTermsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, paymentTermsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, paymentTermsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: paymentTermsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.PaymentTermEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// キャッシュミス → API 単体取得
	entity, err := r.api.GetPaymentTerm(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, paymentTermsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search はパラメータでフィルタした支払条件をキャッシュから返す。
func (r *PaymentTermRepository) Search(ctx context.Context, params boardapi.PaymentTermSearchParams, opts ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterPaymentTerms(all, params), nil
}

// filterPaymentTerms はインメモリフィルタリングを行う。
// UpdatedAtFrom は差分取得カーソルとして使用するためフィルタには含めない。
func filterPaymentTerms(entities []boardapi.PaymentTermEntity, params boardapi.PaymentTermSearchParams) []boardapi.PaymentTermEntity {
	var result []boardapi.PaymentTermEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}
