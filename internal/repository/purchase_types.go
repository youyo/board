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

// PurchaseTypeRepository は purchase_types リソースのキャッシュ → リフレッシュ → API フォールバックを管理する。
type PurchaseTypeRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewPurchaseTypeRepository は PurchaseTypeRepository を生成する。
func NewPurchaseTypeRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *PurchaseTypeRepository {
	return &PurchaseTypeRepository{
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

const purchaseTypesResource = "purchase_types"

// List は全発注区分をキャッシュから返す。
func (r *PurchaseTypeRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	fetcher := &purchaseTypesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, purchaseTypesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, purchaseTypesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, purchaseTypesResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, purchaseTypesResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, purchaseTypesResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.PurchaseTypeEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID は指定 ID の発注区分をキャッシュから返す。
// キャッシュミス時は API から単体取得して upsert する。
func (r *PurchaseTypeRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.PurchaseTypeEntity, error) {
	fetcher := &purchaseTypesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, purchaseTypesResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, purchaseTypesResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: purchaseTypesResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.PurchaseTypeEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// キャッシュミス → API 単体取得
	entity, err := r.api.GetPurchaseType(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, purchaseTypesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search はパラメータでフィルタした発注区分をキャッシュから返す。
func (r *PurchaseTypeRepository) Search(ctx context.Context, params boardapi.PurchaseTypeSearchParams, opts ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterPurchaseTypes(all, params), nil
}

// filterPurchaseTypes はインメモリフィルタリングを行う。
// UpdatedAtFrom は差分取得カーソルとして使用するためフィルタには含めない。
func filterPurchaseTypes(entities []boardapi.PurchaseTypeEntity, params boardapi.PurchaseTypeSearchParams) []boardapi.PurchaseTypeEntity {
	var result []boardapi.PurchaseTypeEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}
