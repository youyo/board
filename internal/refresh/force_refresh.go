package refresh

import (
	"context"
	"time"
)

// ForceRefreshResult is a summary of the full fetch result.
type ForceRefreshResult struct {
	Profile      string
	Resource     string
	FetchedCount int
}

// ForceRefresh fetches all items and calls DeleteAll on existing cache then UpsertMany.
//
// Algorithm:
//  1. Fetch all items via fetcher.ListAll(ctx)
//  2. Convert to Entry slice via rawToEntries
//  3. Clear existing cache via ResourceCache.DeleteAll
//  4. Insert all items via ResourceCache.UpsertMany
//  5. Update sync_state via Updater.MarkForceSuccess
func (r *Refresher) ForceRefresh(
	ctx context.Context,
	profile string,
	fetcher Fetcher,
	now time.Time,
	tz *time.Location,
) (*ForceRefreshResult, error) {
	resource := fetcher.ResourceName()

	// 1. fetch all items
	raws, err := fetcher.ListAll(ctx)
	if err != nil {
		_ = r.updater.MarkError(ctx, profile, resource, "", err.Error(), now)
		return nil, err
	}

	// 2. convert to Entry
	entries, err := rawToEntries(profile, resource, raws)
	if err != nil {
		return nil, err
	}

	// 3. clear existing cache
	if err := r.resourceCache.DeleteAll(ctx, profile, resource); err != nil {
		return nil, err
	}

	// 4. insert all items
	if err := r.resourceCache.UpsertMany(ctx, entries); err != nil {
		return nil, err
	}

	// 5. update sync_state
	if err := r.updater.MarkForceSuccess(ctx, profile, resource, now, tz); err != nil {
		return nil, err
	}

	return &ForceRefreshResult{
		Profile:      profile,
		Resource:     resource,
		FetchedCount: len(entries),
	}, nil
}
