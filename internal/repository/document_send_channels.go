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

// DocumentSendChannelRepository は document_send_channels リソースのキャッシュ → リフレッシュ → API フォールバックを管理する。
type DocumentSendChannelRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewDocumentSendChannelRepository は DocumentSendChannelRepository を生成する。
func NewDocumentSendChannelRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *DocumentSendChannelRepository {
	return &DocumentSendChannelRepository{
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

const documentSendChannelsResource = "document_send_channels"

// List は全書類送付方法をキャッシュから返す。
func (r *DocumentSendChannelRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	fetcher := &documentSendChannelsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, documentSendChannelsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, documentSendChannelsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	entries, err := r.cache.List(ctx, r.profile, documentSendChannelsResource)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, documentSendChannelsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, documentSendChannelsResource)
		if err != nil {
			return nil, err
		}
	}

	entities, err := decodeEntries[boardapi.DocumentSendChannelEntity](entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID は指定 ID の書類送付方法をキャッシュから返す。
// キャッシュミス時は API から単体取得して upsert する。
func (r *DocumentSendChannelRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.DocumentSendChannelEntity, error) {
	fetcher := &documentSendChannelsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, documentSendChannelsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, documentSendChannelsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: documentSendChannelsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.DocumentSendChannelEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// キャッシュミス → API 単体取得
	entity, err := r.api.GetDocumentSendChannel(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, documentSendChannelsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search はパラメータでフィルタした書類送付方法をキャッシュから返す。
func (r *DocumentSendChannelRepository) Search(ctx context.Context, params boardapi.DocumentSendChannelSearchParams, opts ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterDocumentSendChannels(all, params), nil
}

// filterDocumentSendChannels はインメモリフィルタリングを行う。
// UpdatedAtFrom は差分取得カーソルとして使用するためフィルタには含めない。
func filterDocumentSendChannels(entities []boardapi.DocumentSendChannelEntity, params boardapi.DocumentSendChannelSearchParams) []boardapi.DocumentSendChannelEntity {
	var result []boardapi.DocumentSendChannelEntity
	for _, e := range entities {
		if params.Name != "" && !strings.Contains(e.Name, params.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}
