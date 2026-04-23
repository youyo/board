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
//
// 注意: BOARD API に領収書の List エンドポイントは存在しない（OpenAPI 実測）。
// 領収書は projects の response_group=receipt 経由の埋め込み取得が正道。
// 個別取得は GET /v1/documents/receipts/{id} のみ。
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
// Returns an *ItemResult carrying the entity together with response metadata
// (ETag, rate limits, Last-Modified). Introduced in M53 as part of Phase L
// Get* → *ItemResult[XEntity] unification.
func (c *Client) GetReceipt(ctx context.Context, documentID int) (*ItemResult[ReceiptEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/receipts/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ReceiptEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetReceipt: unmarshal: " + err.Error()}
	}
	return &ItemResult[ReceiptEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// GetReceiptRaw retrieves a single receipt and returns the raw HTTP
// response body byte-for-byte along with response headers.
//
// Intended for E2E strict field diff; regular callers should use GetReceipt.
// The returned http.Header enables parseItemMeta in E2E tests (M53).
func (c *Client) GetReceiptRaw(ctx context.Context, documentID int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/receipts/%d", documentID), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
