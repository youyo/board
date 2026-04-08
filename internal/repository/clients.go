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

// ClientRepository は clients リソースのキャッシュ → リフレッシュ → API フォールバックを管理する。
type ClientRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewClientRepository は ClientRepository を生成する。
func NewClientRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *ClientRepository {
	return &ClientRepository{
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

const clientsResource = "clients"

// List は全顧客をキャッシュから返す。
// opts に応じてリフレッシュを実行する。
func (r *ClientRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ClientEntity, error) {
	fetcher := &clientsFetcher{api: r.api}
	now := time.Now()

	// SyncState を取得
	state, err := r.syncStore.Get(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}

	// リフレッシュ判定・実行
	if err := maybeRefresh(ctx, r.profile, clientsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	// キャッシュから取得
	entries, err := r.cache.List(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}

	// キャッシュ空 + 未リフレッシュ → 暗黙 ForceRefresh
	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, clientsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, clientsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.ClientEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID は指定 ID の顧客をキャッシュから返す。
// キャッシュミス時は API から単体取得して upsert する。
func (r *ClientRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ClientEntity, error) {
	fetcher := &clientsFetcher{api: r.api}
	now := time.Now()

	// SyncState を取得
	state, err := r.syncStore.Get(ctx, r.profile, clientsResource)
	if err != nil {
		return nil, err
	}

	// リフレッシュ判定・実行
	if err := maybeRefresh(ctx, r.profile, clientsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: clientsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ClientEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// キャッシュミス → API 単体取得
	entity, err := r.api.GetClient(ctx, id)
	if err != nil {
		return nil, err
	}

	// upsert
	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, clientsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search はパラメータでフィルタした顧客をキャッシュから返す。
// リフレッシュは List と同様のロジックで制御する。
func (r *ClientRepository) Search(ctx context.Context, params boardapi.ClientSearchParams, opts ReadOptions) ([]boardapi.ClientEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterClients(all, params), nil
}

// filterClients はインメモリフィルタリングを行う。
func filterClients(entities []boardapi.ClientEntity, params boardapi.ClientSearchParams) []boardapi.ClientEntity {
	var result []boardapi.ClientEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}
