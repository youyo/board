package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// EstimateEntity is a BOARD API estimate entity.
// Retrieved via GET /v1/documents/estimates/{documentID}.
// フィールドは実 API レスポンス（tmp/e2e-artifacts/estimates_*.json）に準拠。
//
// 注意: BOARD API に見積書の List エンドポイントは存在しない（OpenAPI 実測）。
// 見積書は projects の response_group=estimate 経由の埋め込み取得が正道。
// 個別取得は GET /v1/documents/estimates/{id} のみ。
type EstimateEntity struct {
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
	ValidPeriod           string                 `json:"valid_period"`
}

// GetEstimate retrieves the estimate with the specified document ID.
// Returns an *ItemResult carrying the entity together with response metadata
// (ETag, rate limits, Last-Modified). Introduced in M53 as part of Phase L
// Get* → *ItemResult[XEntity] unification.
func (c *Client) GetEstimate(ctx context.Context, documentID int) (*ItemResult[EstimateEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/estimates/%d", documentID), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x EstimateEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetEstimate: unmarshal: " + err.Error()}
	}
	return &ItemResult[EstimateEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// GetEstimateRaw retrieves a single estimate and returns the raw HTTP
// response body byte-for-byte along with response headers.
//
// Intended for E2E strict field diff; regular callers should use GetEstimate.
// The returned http.Header enables parseItemMeta in E2E tests (M53).
func (c *Client) GetEstimateRaw(ctx context.Context, documentID int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/documents/estimates/%d", documentID), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
