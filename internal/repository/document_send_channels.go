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

// DocumentSendChannelRepository manages cache -> refresh -> API fallback for the document_send_channels resource.
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

// NewDocumentSendChannelRepository creates a new DocumentSendChannelRepository.
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

// List returns all document send channels from the cache.
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

	return applyLimit(entities, opts.Limit), nil
}

// GetByID returns the document send channel with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
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

	// Cache miss → fetch single entity from API
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

// Search returns document send channels filtered by the given parameters from the cache.
func (r *DocumentSendChannelRepository) Search(ctx context.Context, params boardapi.DocumentSendChannelSearchParams, opts ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	listOpts := opts
	listOpts.Limit = 0
	all, err := r.List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	return applyLimit(filterDocumentSendChannels(all, params), opts.Limit), nil
}

// filterDocumentSendChannels performs in-memory filtering.
// UpdatedAtFrom is used as a delta fetch cursor and is not included in the filter.
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

// ListPage retrieves a single page of DocumentSendChannelEntity directly from the API (cache bypass).
// TODO(M57): PageResult は M57 で ListResult[T] に移行予定。
//
//nolint:staticcheck
func (r *DocumentSendChannelRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.DocumentSendChannelEntity], error) {
	return r.api.ListDocumentSendChannelsPage(ctx, page, perPage)
}
