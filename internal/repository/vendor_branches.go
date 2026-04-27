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

// VendorBranchRepository manages cache -> refresh -> API fallback for the vendor_branches resource.
type VendorBranchRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
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
) *VendorBranchRepository {
	return &VendorBranchRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const vendorBranchesResource = "vendor_branches"

// vendorBranchFilterIsZero は filter が空（全フィールドがゼロ値 / nil）かどうかを返す。
// ゼロフィルタはローカルキャッシュ経路を使い、非ゼロフィルタは cache bypass で API を直呼びする。
func vendorBranchFilterIsZero(f boardapi.VendorBranchListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.PayeeIDEq == 0 &&
		f.NameCont == ""
}

// List は仕入先拠点を返す。
//
// 動作:
//   - ゼロフィルタ（boardapi.VendorBranchListOptions{}）: ローカルキャッシュを使い
//     refresh-on-demand（daily auto refresh, explicit Refresh / ForceRefresh）を行う。
//     返り値の *ListResult.Meta はゼロ値（キャッシュが正）。
//   - 非ゼロフィルタ: cache bypass で api.ListVendorBranches を直接呼び、サーバーサイドの
//     Ransack フィルタ意味論を活かす。返り値の *ListResult.Meta はヘッダーから埋まる。
//
// readOpts.Limit は両経路の最終結果に適用される。
func (r *VendorBranchRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.VendorBranchListOptions) (*boardapi.ListResult[boardapi.VendorBranchEntity], error) {
	if !vendorBranchFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListVendorBranches(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &vendorBranchesFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, vendorBranchesResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, vendorBranchesResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}
	entries, err := r.cache.List(ctx, r.profile, vendorBranchesResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.VendorBranchEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.VendorBranchEntity]{Items: entities}, nil
}

// ListEntities は List の items のみを返す薄いラッパ。
// find 層（Phase L では *ListResult を扱わない）向け。
func (r *VendorBranchRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.VendorBranchListOptions) ([]boardapi.VendorBranchEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
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

	if err := maybeRefresh(ctx, r.profile, vendorBranchesResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetVendorBranch(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, vendorBranchesResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities と機能的に同等。
//
// find 層は *ListResult を扱わず []VendorBranchEntity を維持する（Phase L の方針）。
func (r *VendorBranchRepository) Search(ctx context.Context, filter boardapi.VendorBranchListOptions, opts ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
