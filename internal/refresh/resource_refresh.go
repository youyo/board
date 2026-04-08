package refresh

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/youyo/board/internal/cache"
)

// Fetcher is an abstraction for the refresh engine to fetch entities from the API.
// An implementation is provided per resource (clients, projects, ...).
type Fetcher interface {
	// ResourceName returns the resource identifier (e.g., "clients").
	ResourceName() string
	// ListAll fetches all items.
	// Returns a slice of json.RawMessage (each element is one entity).
	ListAll(ctx context.Context) ([]json.RawMessage, error)
	// ListUpdatedSince fetches entities where updated_at >= since.
	// If since is empty, it is equivalent to fetching all items.
	ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error)
}

// DeltaRefreshResult is a summary of the delta fetch result.
type DeltaRefreshResult struct {
	Profile      string
	Resource     string
	FetchedCount int
	NewCursor    string // updated cursor value (empty = no change)
}

// Refresher is the execution engine for delta and full refresh.
type Refresher struct {
	resourceCache *cache.ResourceCache
	syncStore     *cache.SyncStateStore
	updater       *Updater
}

// NewRefresher creates a Refresher.
func NewRefresher(rc *cache.ResourceCache, ss *cache.SyncStateStore) *Refresher {
	return &Refresher{
		resourceCache: rc,
		syncStore:     ss,
		updater:       NewUpdater(ss),
	}
}

// extractID returns the "id" field from a json.RawMessage as a string.
func extractID(raw json.RawMessage) (string, error) {
	var v struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	if v.ID == 0 {
		return "", errors.New("entity has no id or id=0")
	}
	return strconv.Itoa(v.ID), nil
}

// extractUpdatedAt returns the "updated_at" field from a json.RawMessage.
// Returns empty string if not present.
func extractUpdatedAt(raw json.RawMessage) string {
	var v struct {
		UpdatedAt string `json:"updated_at"`
	}
	_ = json.Unmarshal(raw, &v)
	return v.UpdatedAt
}

// rawToEntries converts []json.RawMessage to []cache.Entry.
func rawToEntries(profile, resource string, raws []json.RawMessage) ([]cache.Entry, error) {
	entries := make([]cache.Entry, 0, len(raws))
	for _, raw := range raws {
		id, err := extractID(raw)
		if err != nil {
			return nil, err
		}
		updatedAt := extractUpdatedAt(raw)
		var updatedAtNull sql.NullString
		if updatedAt != "" {
			updatedAtNull = sql.NullString{String: updatedAt, Valid: true}
		}
		entries = append(entries, cache.Entry{
			Key: cache.EntityKey{
				Profile:  profile,
				Resource: resource,
				EntityID: id,
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAtNull,
		})
	}
	return entries, nil
}

// maxUpdatedAt returns the maximum updated_at string among []json.RawMessage.
// Entities without updated_at are skipped.
// Returns empty string if no entity has updated_at.
func maxUpdatedAt(raws []json.RawMessage) string {
	max := ""
	for _, raw := range raws {
		ua := extractUpdatedAt(raw)
		if ua == "" {
			continue
		}
		if ua > max {
			max = ua
		}
	}
	return max
}

// cursorFromState returns the current cursor value from SyncState.
// Returns empty string if state is nil or cursor is invalid.
func cursorFromState(state *cache.SyncState) string {
	if state == nil {
		return ""
	}
	if !state.CursorUpdatedAt.Valid {
		return ""
	}
	return state.CursorUpdatedAt.String
}

// DeltaRefresh fetches incremental changes since cursor_updated_at and upserts them into cache.
//
// Algorithm:
//  1. Read SyncState.cursor_updated_at (nil means "" = fetch all)
//  2. Fetch delta via fetcher.ListUpdatedSince(ctx, cursor)
//  3. Convert to Entry slice via rawToEntries
//  4. Update cache via ResourceCache.UpsertMany
//  5. Calculate new cursor as maximum updated_at from fetched results
//  6. Update sync_state via Updater.MarkDeltaSuccess
func (r *Refresher) DeltaRefresh(
	ctx context.Context,
	profile string,
	fetcher Fetcher,
	now time.Time,
	tz *time.Location,
) (*DeltaRefreshResult, error) {
	resource := fetcher.ResourceName()

	// 1. get current cursor
	state, err := r.syncStore.Get(ctx, profile, resource)
	if err != nil {
		return nil, err
	}
	cursor := cursorFromState(state)

	// 2. fetch delta
	raws, err := fetcher.ListUpdatedSince(ctx, cursor)
	if err != nil {
		_ = r.updater.MarkError(ctx, profile, resource, "", err.Error(), now)
		return nil, err
	}

	// 3. convert to Entry
	entries, err := rawToEntries(profile, resource, raws)
	if err != nil {
		return nil, err
	}

	// 4. update cache
	if err := r.resourceCache.UpsertMany(ctx, entries); err != nil {
		return nil, err
	}

	// 5. calculate new cursor
	newCursor := maxUpdatedAt(raws)

	// 6. update sync_state
	if err := r.updater.MarkDeltaSuccess(ctx, profile, resource, newCursor, now, tz); err != nil {
		return nil, err
	}

	return &DeltaRefreshResult{
		Profile:      profile,
		Resource:     resource,
		FetchedCount: len(entries),
		NewCursor:    newCursor,
	}, nil
}
