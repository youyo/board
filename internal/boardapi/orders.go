package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OrderEntity is a BOARD API order entity.
// Retrieved via GET /v1/documents/orders/{documentID}.
// フィールドは実 API レスポンス（tmp/e2e-artifacts/orders_*.json）に準拠。
type OrderEntity struct {
	ID                    int                    `json:"id"`
	Message               *string                `json:"message"`
	Total                 string                 `json:"total"`
	Tax                   string                 `json:"tax"`
	TaxWithholding        string                 `json:"tax_withholding"`
	SealApprovalStatus    int                    `json:"seal_approval_status"`
	DocumentAmountDispKbn int                    `json:"document_amount_disp_kbn"`
	BlankDateFlg          int                    `json:"blank_date_flg"`
	LockFlg               int                    `json:"lock_flg"`
	DeliveryPlace         *string                `json:"delivery_place"`
	Details               []DocumentDetailEntity `json:"details"`
	DispOrderDate         *string                `json:"disp_order_date"`
	DispOrderReceiveDate  *string                `json:"disp_order_receive_date"`
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

// GetOrderRaw retrieves a single order and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetOrder.
func (c *Client) GetOrderRaw(ctx context.Context, documentID int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/orders/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}
