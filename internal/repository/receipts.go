package repository

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
)

// ReceiptRepository manages on-demand cache -> API fallback for the receipts resource.
// Bulk List/Search are not supported; use GetByDocumentID for individual document access.
type ReceiptRepository struct {
	profile string
	api     *boardapi.Client
	cache   *cache.ResourceCache
}

// NewReceiptRepository creates a new ReceiptRepository.
func NewReceiptRepository(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
) *ReceiptRepository {
	return &ReceiptRepository{
		profile: profile,
		api:     api,
		cache:   rc,
	}
}

const receiptsResource = "receipts"

// GetByDocumentID returns the receipt with the given document ID.
// On cache miss (or ForceRefresh), it fetches from the API and upserts the result.
func (r *ReceiptRepository) GetByDocumentID(ctx context.Context, documentID int, opts ReadOptions) (*boardapi.ReceiptEntity, error) {
	if !opts.ForceRefresh {
		key := cache.EntityKey{Profile: r.profile, Resource: receiptsResource, EntityID: strconv.Itoa(documentID)}
		entry, err := r.cache.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			var entity boardapi.ReceiptEntity
			if err := json.Unmarshal(entry.PayloadJSON, &entity); err != nil {
				return nil, err
			}
			return &entity, nil
		}
	}

	// Cache miss (or ForceRefresh) → fetch from API
	result, err := r.api.GetReceipt(ctx, documentID)
	if err != nil {
		return nil, err
	}
	entity := result.Item

	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	if err := upsertRaw(ctx, r.cache, r.profile, receiptsResource, raw); err != nil {
		return nil, err
	}

	return entity, nil
}
