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

// Search returns contacts filtered by the given parameters.
// When ClientID is set, the API-side client_id filter is used directly because
// the BOARD API response does not include a flat client_id field; it nests the
// parent client as {"client": {"id": N, ...}}. In-memory filtering on
// ContactEntity.ClientID (which is always 0 after unmarshal) would silently
// return zero results. Name/Email-only searches fall back to a full list with
// in-memory filtering (BOARD API ignores name/email parameters).
func (r *ContactRepository) Search(ctx context.Context, params boardapi.ContactSearchParams, opts ReadOptions) ([]boardapi.ContactEntity, error) {
	if params.ClientID != 0 {
		// Use API-side filter; apply Name/Email in-memory afterward if needed.
		entities, err := r.api.SearchContacts(ctx, params)
		if err != nil {
			return nil, err
		}
		return filterContactsByNameEmail(entities, params.Name, params.Email), nil
	}
	// Name/Email-only (or empty) filter: fall back to full list + in-memory.
	all, err := r.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return filterContactsByNameEmail(all, params.Name, params.Email), nil
}

// filterContactsByNameEmail performs in-memory name and email filtering.
// Name matching uses DisplayName() (LastName + FirstName).
// Email matching dereferences the *string pointer (nil email never matches).
func filterContactsByNameEmail(entities []boardapi.ContactEntity, name, email string) []boardapi.ContactEntity {
	if name == "" && email == "" {
		return entities
	}
	var result []boardapi.ContactEntity
	for _, e := range entities {
		if name != "" && !strings.Contains(e.DisplayName(), name) {
			continue
		}
		if email != "" {
			if e.Email == nil || !strings.Contains(*e.Email, email) {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

// ListPage retrieves a single page of ContactEntity directly from the API (cache bypass).
func (r *ContactRepository) ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ContactEntity], error) {
	return r.api.ListContactsPage(ctx, page, perPage)
}
