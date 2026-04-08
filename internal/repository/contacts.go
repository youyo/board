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

// List returns all contacts from the cache.
func (r *ContactRepository) List(ctx context.Context, opts ReadOptions) ([]boardapi.ContactEntity, error) {
	fetcher := &contactsFetcher{api: r.api}
	now := time.Now()

	state, err := r.syncStore.Get(ctx, r.profile, contactsResource)
	if err != nil {
		return nil, err
	}

	if err := maybeRefresh(ctx, r.profile, contactsResource, opts, state, r.autoRefresh, r.tz, r.lockManager, r.refresher, fetcher, now); err != nil {
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

	if opts.Limit > 0 && len(entities) > opts.Limit {
		entities = entities[:opts.Limit]
	}
	return entities, nil
}

// GetByID returns the contact with the given ID from the cache.
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
	entity, err := r.api.GetContact(ctx, id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, contactsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}

// Search returns contacts filtered by the given parameters from the cache.
func (r *ContactRepository) Search(ctx context.Context, params boardapi.ContactSearchParams, opts ReadOptions) ([]boardapi.ContactEntity, error) {
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterContacts(all, params), nil
}

// filterContacts performs in-memory filtering.
func filterContacts(entities []boardapi.ContactEntity, params boardapi.ContactSearchParams) []boardapi.ContactEntity {
	var result []boardapi.ContactEntity
	for _, e := range entities {
		if params.ClientID != 0 && e.ClientID != params.ClientID {
			continue
		}
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
