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

// vendorContactFilterIsZero は filter が空（全フィールドがゼロ値 / nil）かどうかを返す。
// ゼロフィルタはローカルキャッシュ経路を使い、非ゼロフィルタは cache bypass で API を直呼びする。
func vendorContactFilterIsZero(f boardapi.VendorContactListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.PayeeIDEq == 0 &&
		f.NameCont == "" &&
		f.EmailCont == ""
}

// List は仕入先担当者を返す。
//
// 動作:
//   - ゼロフィルタ（boardapi.VendorContactListOptions{}）: ローカルキャッシュを使い
//     refresh-on-demand（daily auto refresh, explicit Refresh / ForceRefresh）を行う。
//     返り値の *ListResult.Meta はゼロ値（キャッシュが正）。
//   - 非ゼロフィルタ: cache bypass で api.ListVendorContacts を直接呼び、サーバーサイドの
//     Ransack フィルタ意味論を活かす。返り値の *ListResult.Meta はヘッダーから埋まる。
//
// readOpts.Limit は両経路の最終結果に適用される。
func (r *VendorContactRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.VendorContactListOptions) (*boardapi.ListResult[boardapi.VendorContactEntity], error) {
	if !vendorContactFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListVendorContacts(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &vendorContactsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorContactsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, vendorContactsResource, readOpts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.VendorContactEntity]{Items: entities}, nil
}

// ListEntities は List の items のみを返す薄いラッパ。
// find 層（Phase L では *ListResult を扱わない）向け。
func (r *VendorContactRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.VendorContactListOptions) ([]boardapi.VendorContactEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
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

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetVendorContact(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, vendorContactsResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities と機能的に同等。
//
// find 層は *ListResult を扱わず []VendorContactEntity を維持する（Phase L の方針）。
func (r *VendorContactRepository) Search(ctx context.Context, filter boardapi.VendorContactListOptions, opts ReadOptions) ([]boardapi.VendorContactEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
