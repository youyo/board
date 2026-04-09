package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ReceiptEntity is a BOARD API receipt entity.
// Retrieved via GET /v1/documents/receipts/{documentID}.
type ReceiptEntity struct {
	ID          int     `json:"id"`
	ClientID    int     `json:"client_id"`
	ProjectID   int     `json:"project_id"`
	Title       string  `json:"title"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`
	ReceiptDate string  `json:"receipt_date"` // ISO 8601 date
	Memo        string  `json:"memo"`
	UpdatedAt   string  `json:"updated_at"` // ISO 8601
	CreatedAt   string  `json:"created_at"` // ISO 8601
}

// GetReceipt retrieves the receipt with the specified document ID.
func (c *Client) GetReceipt(ctx context.Context, documentID int) (*ReceiptEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/receipts/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ReceiptEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetReceipt: unmarshal: " + err.Error()}
	}
	return &x, nil
}
