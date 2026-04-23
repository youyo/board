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

// ContactRepository manages cache -> refresh -> API fallback for the contacts resource.
type ContactRepository struct {
	profile     string
	api         *boardapi.Client
	cache       *cache.ResourceCache
	syncStore   *cache.SyncStateStore
	refresher   *refresh.Refresher
	lockManager *refresh.LockManager
	tz          *time.Location
	autoRefresh bool
}

// NewContactRepository creates a new ContactRepository.
func NewContactRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *ContactRepository {
	return &ContactRepository{
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

const contactsResource = "contacts"

// contactFilterIsZero reports whether the given filter is empty (all fields
// are zero values / nil). A zero filter routes through the local cache; a
// non-zero filter bypasses the cache and calls the API directly because
// filtered results must not poison the full-entity cache.
func contactFilterIsZero(f boardapi.ContactListOptions) bool {
	return f.Page == 0 &&
		f.PerPage == 0 &&
		f.UpdatedAtGteq == "" &&
		f.UpdatedAtLteq == "" &&
		f.IncludeArchiveFlg == nil &&
		f.ClientIDEq == 0 &&
		f.NameCont == "" &&
		f.EmailCont == ""
}

// List returns contacts.
//
// Behavior:
//   - Zero filter (boardapi.ContactListOptions{}): uses the local cache with
//     refresh-on-demand (daily auto refresh, explicit Refresh / ForceRefresh).
//     Returns *ListResult with Meta zero-valued (cache is source of truth).
//   - Non-zero filter: bypasses the cache and calls api.ListContacts directly
//     so that server-side filter semantics (Ransack) take effect.
//     Returns *ListResult with Meta populated from the final page's response headers.
//
// Limit from readOpts is applied to the final result in either path.
func (r *ContactRepository) List(ctx context.Context, readOpts ReadOptions, filter boardapi.ContactListOptions) (*boardapi.ListResult[boardapi.ContactEntity], error) {
	if !contactFilterIsZero(filter) {
		// API 直接呼び出し（cache bypass）: フィルタ結果でキャッシュを汚染しない。
		result, err := r.api.ListContacts(ctx, filter)
		if err != nil {
			return nil, err
		}
		result.Items = applyLimit(result.Items, readOpts.Limit)
		return result, nil
	}

	// ゼロフィルタ: 既存の cache → refresh → API fallback 経路
	fetcher := &contactsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, contactsResource)
	if err != nil {
		return nil, err
	}
	if err := maybeRefresh(ctx, r.profile, contactsResource, readOpts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}
	entries, err := r.cache.List(ctx, r.profile, contactsResource)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 && state == nil {
		if err := r.lockManager.WithLock(ctx, r.profile, contactsResource, func() error {
			_, err := r.refresher.ForceRefresh(ctx, r.profile, fetcher, now, r.tz)
			return err
		}); err != nil {
			return nil, err
		}
		entries, err = r.cache.List(ctx, r.profile, contactsResource)
		if err != nil {
			return nil, err
		}
	}
	entities, err := decodeEntries[boardapi.ContactEntity](entries)
	if err != nil {
		return nil, err
	}
	entities = applyLimit(entities, readOpts.Limit)
	return &boardapi.ListResult[boardapi.ContactEntity]{Items: entities}, nil
}

// ListEntities は List の items のみを返す薄いラッパ。
// find 層（Phase L では *ListResult を扱わない）向け。
func (r *ContactRepository) ListEntities(ctx context.Context, readOpts ReadOptions, filter boardapi.ContactListOptions) ([]boardapi.ContactEntity, error) {
	result, err := r.List(ctx, readOpts, filter)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetByID returns the contact with the given ID from the cache.
// On cache miss, it fetches from the API and upserts the result.
func (r *ContactRepository) GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ContactEntity, error) {
	fetcher := &contactsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, contactsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, contactsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
		return nil, err
	}

	key := cache.EntityKey{Profile: r.profile, Resource: contactsResource, EntityID: strconv.Itoa(id)}
	entry, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if entry != nil {
		var entity boardapi.ContactEntity
		if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		return &entity, nil
	}

	// Cache miss -> fetch single entry from API
	result, err := r.api.GetContact(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(result.Item)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, contactsResource, raw); err != nil {
		return nil, err
	}

	return result.Item, nil
}

// Search は find 層向けの薄いラッパ。ListEntities と機能的に同等。
//
// find 層は *ListResult を扱わず []ContactEntity を維持する（Phase L の方針）。
func (r *ContactRepository) Search(ctx context.Context, filter boardapi.ContactListOptions, opts ReadOptions) ([]boardapi.ContactEntity, error) {
	return r.ListEntities(ctx, opts, filter)
}
