package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// DeliveryEntity is a BOARD API delivery entity.
// Corresponds to one element in the GET /v1/deliveries response.
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

// DeliverySearchParams is the parameter for SearchDeliveries.
type DeliverySearchParams struct {
	ClientID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// ListDeliveries retrieves all deliveries.
// Pagination is automatically handled by ListAll.
func (c *Client) ListDeliveries(ctx context.Context) ([]DeliveryEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/deliveries", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]DeliveryEntity, 0, len(items))
	for _, raw := range items {
		var x DeliveryEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListDeliveries: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetDelivery retrieves the delivery with the specified ID.
func (c *Client) GetDelivery(ctx context.Context, id int) (*DeliveryEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/deliveries/%d", id), nil)
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

// SearchDeliveries searches deliveries with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchDeliveries(ctx context.Context, params DeliverySearchParams) ([]DeliveryEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/deliveries", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ClientID != 0 {
			q.Set("client_id", strconv.Itoa(params.ClientID))
		}
		if params.ProjectID != 0 {
			q.Set("project_id", strconv.Itoa(params.ProjectID))
		}
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.UpdatedAtFrom != "" {
			q.Set("updated_at_from", params.UpdatedAtFrom)
		}
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]DeliveryEntity, 0, len(items))
	for _, raw := range items {
		var x DeliveryEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchDeliveries: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
