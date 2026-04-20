package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/repository"
)

func makeOrderRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.OrderRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	return repository.NewOrderRepository("default", apiClient, rc)
}

func seedOrderCache(t *testing.T, db *cache.DB, entities []boardapi.OrderEntity) {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ctx := context.Background()
	for _, e := range entities {
		raw, _ := json.Marshal(e)
		// M36: OrderEntity は updated_at を持たないため固定値を使用する。
		updatedAt := sql.NullString{Valid: true, String: "2026-01-01T00:00:00Z"}
		err := rc.Upsert(ctx, cache.Entry{
			Key: cache.EntityKey{
				Profile:  "default",
				Resource: "orders",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedOrderCache: %v", err)
		}
	}
}

var sampleOrders = []boardapi.OrderEntity{
	{ID: 1, Total: "50000.0", Tax: "5000.0", TaxWithholding: "0.0"},
	{ID: 2, Total: "80000.0", Tax: "8000.0", TaxWithholding: "0.0"},
	{ID: 3, Total: "120000.0", Tax: "12000.0", TaxWithholding: "0.0"},
}

// T_ORD03: GetByDocumentID - cache hit -> returns from cache
func TestOrderRepository_GetByDocumentID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedOrderCache(t, db, sampleOrders)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByDocumentID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_ORD04: GetByDocumentID - cache miss, API success -> fetches from API and returns
func TestOrderRepository_GetByDocumentID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)

	target := boardapi.OrderEntity{ID: 99, Total: "50000.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByDocumentID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_ORD05: GetByDocumentID - cache miss, API error -> returns error
func TestOrderRepository_GetByDocumentID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient)
	_, err := repo.GetByDocumentID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_ORD06: GetByDocumentID - ForceRefresh bypasses cache
func TestOrderRepository_GetByDocumentID_ForceRefresh(t *testing.T) {
	db := newTestDB(t)
	seedOrderCache(t, db, sampleOrders)

	updated := boardapi.OrderEntity{ID: 1, Total: "999999.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(updated)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 1, repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("GetByDocumentID ForceRefresh: %v", err)
	}
	if got == nil || got.Total != "999999.0" {
		t.Errorf("expected updated total, got %+v", got)
	}
}
