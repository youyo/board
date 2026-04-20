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
