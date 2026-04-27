package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/cache/fold"
	"github.com/youyo/board/internal/refresh"
)

// UserRepository manages cache -> refresh -> API fallback for the users resource.
type UserRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
) *UserRepository {
	return &UserRepository{
		profile:     profile,
		api:         api,
		cache:       rc,
		syncStore:   ss,
		refresher:   refresher,
		lockManager: lm,
		tz:          tz,
	}
}

const usersResource = "users"

// userFilterIsZero は filter が空（全フィールドがゼロ値 / nil）かどうかを返す。
func userFilterIsZero(f boardapi.UserListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.NameCont == "" &&
		f.EmailCont == ""
}

// cacheableUserFilter は cache + Go-side filter で代替できるフィルタかを判定する。
func cacheableUserFilter(f boardapi.UserListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil
}

// matchUserFilter は UserEntity が filter に合致するかを Go-side で判定する。
func matchUserFilter(u boardapi.UserEntity, f boardapi.UserListOptions) bool {
	if f.NameCont != "" && !fold.Contains(u.Name, f.NameCont) {
		return false
	}
	if f.EmailCont != "" && !fold.Contains(u.Email, f.EmailCont) {
		return false
	}
	return true
}

// List はユーザーを返す。
//
// 動作:
//   - ゼロフィルタ（boardapi.UserListOptions{}）: ローカルキャッシュを使い
//     refresh-on-demand（daily auto refresh, explicit Refresh / ForceRefresh）を行う。
//     返り値の *ListResult.Meta はゼロ値（キャッシュが正）。
//   - 非ゼロフィルタ: cache bypass で api.ListUsers を直接呼び、サーバーサイドの
//     Ransack フィルタ意味論を活かす。返り値の *ListResult.Meta はヘッダーから埋まる。
//
// readOpts.Limit は両経路の最終結果に適用される。
func (r *UserRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.UserListOptions) (*boardapi.ListResult[boardapi.UserEntity], error) {
	fetcher := &usersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, usersResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, usersResource, readOpts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	if !userFilterIsZero(filter) {
		if cacheableUserFilter(filter) {
			if entities, ok := tryCacheFilter[boardapi.UserEntity](
				ctx, r.cache, r.syncStore, r.profile, usersResource,
				func(u boardapi.UserEntity) bool { return matchUserFilter(u, filter) },
			); ok {
				return &boardapi.ListResult[boardapi.UserEntity]{Items: applyLimit(entities, readOpts.Limit)}, nil
			}
		}
		result, err := r.api.ListUsers(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	entries, err := r.cache.List(ctx, r.profile, usersResource)
	if err != nil {
		return nil, err
	}
	entities, err := decodeEntries[boardapi.UserEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.UserEntity]{Items: entities}, nil
}

// ListEntities は List の items のみを返す薄いラッパ。
// find 層（Phase L では *ListResult を扱わない）向け。
func (r *UserRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.UserListOptions) ([]boardapi.UserEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the user with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *UserRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.UserEntity, error) {
	fetcher := &usersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, usersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, usersResource, opts, state, false, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: usersResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.UserEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, usersResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities と機能的に同等。
//
// find 層は *ListResult を扱わず []UserEntity を維持する（Phase L の方針）。
func (r *UserRepository) Search(ctx context.Context, filter boardapi.UserListOptions, opts ReadOptions) ([]boardapi.UserEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
