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
//
// 注意: BOARD API に納品書の List エンドポイントは存在しない（OpenAPI 実測）。
// 納品書は projects の response_group=delivery 経由の埋め込み取得が正道。
// 個別取得は GET /v1/documents/deliveries/{id} のみ。
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
// Returns an *ItemResult carrying the entity together with response metadata
// (ETag, rate limits, Last-Modified). Introduced in M53 as part of Phase L
// Get* → *ItemResult[XEntity] unification.
func (c *Client) GetDelivery(ctx context.Context, documentID int) (*ItemResult[DeliveryEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/deliveries/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x DeliveryEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetDelivery: unmarshal: " + err.Error()}
	}
	return &ItemResult[DeliveryEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// GetDeliveryRaw retrieves a single delivery and returns the raw HTTP
// response body byte-for-byte along with response headers.
//
// Intended for E2E strict field diff; regular callers should use GetDelivery.
// The returned http.Header enables parseItemMeta in E2E tests (M53).
func (c *Client) GetDeliveryRaw(ctx context.Context, documentID int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/deliveries/%d", documentID), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
