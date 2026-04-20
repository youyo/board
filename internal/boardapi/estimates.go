package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// EstimateEntity is a BOARD API estimate entity.
// Retrieved via GET /v1/documents/estimates/{documentID}.
type EstimateEntity struct {
	ID             int     `json:"id"`
	ClientID       int     `json:"client_id"`
	ProjectID      int     `json:"project_id"`
	Title          string  `json:"title"`
	TotalAmount    float64 `json:"total_amount"`
	Status         string  `json:"status"`
	EstimateDate   string  `json:"estimate_date"`   // ISO 8601 date
	ExpirationDate string  `json:"expiration_date"` // ISO 8601 date
	Memo           string  `json:"memo"`
	UpdatedAt      string  `json:"updated_at"` // ISO 8601
	CreatedAt      string  `json:"created_at"` // ISO 8601
}

// GetEstimate retrieves the estimate with the specified document ID.
func (c *Client) GetEstimate(ctx context.Context, documentID int) (*EstimateEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/estimates/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x EstimateEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetEstimate: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// GetEstimateRaw retrieves a single estimate and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetEstimate.
func (c *Client) GetEstimateRaw(ctx context.Context, documentID int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/estimates/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}
