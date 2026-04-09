package repository

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
)

// OrderRepository manages on-demand cache -> API fallback for the orders resource.
// Bulk List/Search are not supported; use GetByDocumentID for individual document access.
type OrderRepository struct {
	profile string
	api     *boardapi.Client
	cache   *cache.ResourceCache
}

// NewOrderRepository creates a new OrderRepository.
func NewOrderRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
) *OrderRepository {
	return &OrderRepository{
		profile: profile,
		api:     api,
		cache:   rc,
	}
}

const ordersResource = "orders"

// GetByDocumentID returns the order with the given document ID.
// On cache miss (or ForceRefresh), it fetches from the API and upserts the result.
func (r *OrderRepository) GetByDocumentID(ctx context.Context, documentID int, opts ReadOptions) (*boardapi.OrderEntity, error) {
	if !opts.ForceRefresh {
		key := cache.EntityKey{Profile: r.profile, Resource: ordersResource, EntityID: strconv.Itoa(documentID)}
		entry, err := r.cache.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			var entity boardapi.OrderEntity
			if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
				return nil, err
			}
			return &entity, nil
		}
	}

	// Cache miss (or ForceRefresh) → fetch from API
	entity, err := r.api.GetOrder(ctx, documentID)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, ordersResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}
