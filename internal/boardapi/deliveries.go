package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DeliveryEntity is a BOARD API delivery entity.
// Retrieved via GET /v1/documents/deliveries/{documentID}.
type DeliveryEntity struct {
	ID           int     `json:"id"`
	ClientID     int     `json:"client_id"`
	ProjectID    int     `json:"project_id"`
	Title        string  `json:"title"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"`
	DeliveryDate string  `json:"delivery_date"` // ISO 8601 date
	Memo         string  `json:"memo"`
	UpdatedAt    string  `json:"updated_at"` // ISO 8601
	CreatedAt    string  `json:"created_at"` // ISO 8601
}

// GetDelivery retrieves the delivery with the specified document ID.
func (c *Client) GetDelivery(ctx context.Context, documentID int) (*DeliveryEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/deliveries/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x DeliveryEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetDelivery: unmarshal: " + err.Error()}
	}
	return &x, nil
}
