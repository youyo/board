package repository

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
)

// EstimateRepository manages on-demand cache -> API fallback for the estimates resource.
// Bulk List/Search are not supported; use GetByDocumentID for individual document access.
type EstimateRepository struct {
	profile string
	api     *boardapi.Client
	cache   *cache.ResourceCache
}

// NewEstimateRepository creates a new EstimateRepository.
func NewEstimateRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
) *EstimateRepository {
	return &EstimateRepository{
		profile: profile,
		api:     api,
		cache:   rc,
	}
}

const estimatesResource = "estimates"

// GetByDocumentID returns the estimate with the given document ID.
// On cache miss (or ForceRefresh), it fetches from the API and upserts the result.
func (r *EstimateRepository) GetByDocumentID(ctx context.Context, documentID int, opts ReadOptions) (*boardapi.EstimateEntity, error) {
	if !opts.ForceRefresh {
		key := cache.EntityKey{Profile: r.profile, Resource: estimatesResource, EntityID: strconv.Itoa(documentID)}
		entry, err := r.cache.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			var entity boardapi.EstimateEntity
			if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
				return nil, err
			}
			return &entity, nil
		}
	}

	// Cache miss (or ForceRefresh) → fetch from API
	result, err := r.api.GetEstimate(ctx, documentID)
	if err != nil {
		return nil, err
	}
	entity := result.Item

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, estimatesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}
