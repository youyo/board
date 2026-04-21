package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ReceiptEntity is a BOARD API receipt entity.
// Retrieved via GET /v1/documents/receipts/{documentID}.
// フィールドは実 API レスポンス（tmp/e2e-artifacts/receipts_*.json）に準拠。
type ReceiptEntity struct {
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
	ReceiptDate           string                 `json:"receipt_date"`
	DispReceiptDate       *string                `json:"disp_receipt_date"`
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

// GetReceiptRaw retrieves a single receipt and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetReceipt.
func (c *Client) GetReceiptRaw(ctx context.Context, documentID int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/receipts/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}
