package repository

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
)

// DeliveryRepository manages on-demand cache -> API fallback for the deliveries resource.
// Bulk List/Search are not supported; use GetByDocumentID for individual document access.
type DeliveryRepository struct {
	profile string
	api     *boardapi.Client
	cache   *cache.ResourceCache
}

// NewDeliveryRepository creates a new DeliveryRepository.
func NewDeliveryRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
) *DeliveryRepository {
	return &DeliveryRepository{
		profile: profile,
		api:     api,
		cache:   rc,
	}
}

const deliveriesResource = "deliveries"

// GetByDocumentID returns the delivery with the given document ID.
// On cache miss (or ForceRefresh), it fetches from the API and upserts the result.
func (r *DeliveryRepository) GetByDocumentID(ctx context.Context, documentID int, opts ReadOptions) (*boardapi.DeliveryEntity, error) {
	if !opts.ForceRefresh {
		key := cache.EntityKey{Profile: r.profile, Resource: deliveriesResource, EntityID: strconv.Itoa(documentID)}
		entry, err := r.cache.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			var entity boardapi.DeliveryEntity
			if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
				return nil, err
			}
			return &entity, nil
		}
	}

	// Cache miss (or ForceRefresh) → fetch from API
	result, err := r.api.GetDelivery(ctx, documentID)
	if err != nil {
		return nil, err
	}
	entity := result.Item

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, deliveriesResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}
