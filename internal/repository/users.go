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

// UserRepository は users リソースのキャッシュ → リフレッシュ → API フォールバックを管理する。
type UserRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewUserRepository は UserRepository を生成する。
func NewUserRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *UserRepository {
	return &UserRepository{
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

const usersResource = "users"

// List は全ユーザーをキャッシュから返す。
func (r *UserRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.UserEntity, error) {
	fetcher := &usersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, usersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, usersResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, usersResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, usersResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, usersResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.UserEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID は指定 ID のユーザーをキャッシュから返す。
// キャッシュミス時は API から単体取得して upsert する。
func (r *UserRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.UserEntity, error) {
	fetcher := &usersFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, usersResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, usersResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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

	// キャッシュミス → API 単体取得
	entity, err := r.api.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, usersResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search はパラメータでフィルタしたユーザーをキャッシュから返す。
func (r *UserRepository) Search(ctx context.Context, params boardapi.UserSearchParams, opts ReadOptions) ([]boardapi.UserEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterUsers(all, params), nil
}

// filterUsers はインメモリフィルタリングを行う。
// UpdatedAtFrom は差分取得カーソルとして使用するためフィルタには含めない。
func filterUsers(entities []boardapi.UserEntity, params boardapi.UserSearchParams) []boardapi.UserEntity {
	var result []boardapi.UserEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		if params.Email != "" && !strings.Contains(e.Email, params.Email) {
			continue
		}
		result = append(result, e)
	}
	return result
}
