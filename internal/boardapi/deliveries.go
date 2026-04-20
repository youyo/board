package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DeliveryEntity is a BOARD API delivery entity.
// Retrieved via GET /v1/documents/deliveries/{documentID}.
// フィールドは実 API レスポンス（tmp/e2e-artifacts/deliveries_*.json）に準拠。
type DeliveryEntity struct {
	ID                      int                    `json:"id"`
	Message                 *string                `json:"message"`
	Total                   string                 `json:"total"`
	Tax                     string                 `json:"tax"`
	TaxWithholding          string                 `json:"tax_withholding"`
	SealApprovalStatus      int                    `json:"seal_approval_status"`
	DocumentAmountDispKbn   int                    `json:"document_amount_disp_kbn"`
	BlankDateFlg            int                    `json:"blank_date_flg"`
	LockFlg                 int                    `json:"lock_flg"`
	DeliveryPlace           *string                `json:"delivery_place"`
	Details                 []DocumentDetailEntity `json:"details"`
	DeliveryDate            string                 `json:"delivery_date"`
	DispDeliveryDate        *string                `json:"disp_delivery_date"`
	DispDeliveryReceiveDate *string                `json:"disp_delivery_receive_date"`
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

// GetDeliveryRaw retrieves a single delivery and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetDelivery.
func (c *Client) GetDeliveryRaw(ctx context.Context, documentID int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/deliveries/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}
