package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OrderEntity is a BOARD API order entity.
// Retrieved via GET /v1/documents/orders/{documentID}.
type OrderEntity struct {
	ID          int     `json:"id"`
	ClientID    int     `json:"client_id"`
	ProjectID   int     `json:"project_id"`
	Title       string  `json:"title"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`
	OrderDate   string  `json:"order_date"` // ISO 8601 date
	Memo        string  `json:"memo"`
	UpdatedAt   string  `json:"updated_at"` // ISO 8601
	CreatedAt   string  `json:"created_at"` // ISO 8601
}

// GetOrder retrieves the order with the specified document ID.
func (c *Client) GetOrder(ctx context.Context, documentID int) (*OrderEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/orders/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x OrderEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetOrder: unmarshal: " + err.Error()}
	}
	return &x, nil
}
