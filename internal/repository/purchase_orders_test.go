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
	"github.com/youyo/board/internal/refresh"
	"github.com/youyo/board/internal/repository"
)

func makePurchaseOrderRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.PurchaseOrderRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewPurchaseOrderRepository("default", apiClient, rc, ss, refresher, lm, time.UTC)
}

func seedPurchaseOrderCache(t *testing.T, db *cache.DB, entities []boardapi.PurchaseOrderEntity) {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ctx := context.Background()
	for _, e := range entities {
		raw, _ := json.Marshal(e)
		var updatedAt sql.NullString
		if e.UpdatedAt != "" {
			updatedAt = sql.NullString{Valid: true, String: e.UpdatedAt}
		}
		err := rc.Upsert(ctx, cache.Entry{
			Key: cache.EntityKey{
				Profile:  "default",
				Resource: "purchase_orders",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedPurchaseOrderCache: %v", err)
		}
	}
}

func newPurchaseOrderAPIServer(t *testing.T, entities []boardapi.PurchaseOrderEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var samplePurchaseOrders = []boardapi.PurchaseOrderEntity{
	{ID: 1, VendorID: 10, ProjectID: 100, Title: "PurchaseOrderA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, VendorID: 10, ProjectID: 101, Title: "PurchaseOrderB", Status: "ordered", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, VendorID: 20, ProjectID: 102, Title: "PurchaseOrderC", Status: "completed", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_PO01: List - cache hit -> returns cached data
func TestPurchaseOrderRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedPurchaseOrderCache(t, db, samplePurchaseOrders)
	markSynced(t, db, "purchase_orders")

	srv := newPurchaseOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePurchaseOrderRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.PurchaseOrderListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(samplePurchaseOrders) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(samplePurchaseOrders))
	}
}

// T_PO03: GetByID - cache hit -> returns from cache
func TestPurchaseOrderRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedPurchaseOrderCache(t, db, samplePurchaseOrders)
	markSynced(t, db, "purchase_orders")

	srv := newPurchaseOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePurchaseOrderRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_PO04: GetByID - cache miss, API success -> fetches from API and returns
func TestPurchaseOrderRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "purchase_orders")

	target := boardapi.PurchaseOrderEntity{ID: 99, Title: "Test Purchase Order", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePurchaseOrderRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_PO05: GetByID - cache miss, API error -> returns error
func TestPurchaseOrderRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "purchase_orders")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePurchaseOrderRepo(t, db, apiClient)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_PO06: Search - VendorIDEq filter -> non-zero filter bypasses cache, calls API directly
func TestPurchaseOrderRepository_Search_VendorIDFilter(t *testing.T) {
	db := newTestDB(t)
	seed := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, ProjectID: 100, Title: "POA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, VendorID: 10, ProjectID: 101, Title: "POB", Status: "sent", UpdatedAt: "2026-01-02T00:00:00Z"},
		{ID: 3, VendorID: 99, ProjectID: 102, Title: "Other", Status: "paid", UpdatedAt: "2026-01-03T00:00:00Z"},
	}
	seedPurchaseOrderCache(t, db, seed)
	markSynced(t, db, "purchase_orders")

	apiClient := boardapi.New("", "key", "token", 5*time.Second, boardapi.WithRetryMax(0))
	repo := makePurchaseOrderRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.PurchaseOrderListOptions{VendorIDEq: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_PO07: Search - StatusEq filter -> non-zero filter bypasses cache, calls API directly
func TestPurchaseOrderRepository_Search_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	seed := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, ProjectID: 100, Title: "POA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, VendorID: 10, ProjectID: 101, Title: "POB", Status: "sent", UpdatedAt: "2026-01-02T00:00:00Z"},
		{ID: 3, VendorID: 99, ProjectID: 102, Title: "Other", Status: "paid", UpdatedAt: "2026-01-03T00:00:00Z"},
	}
	seedPurchaseOrderCache(t, db, seed)
	markSynced(t, db, "purchase_orders")

	apiClient := boardapi.New("", "key", "token", 5*time.Second, boardapi.WithRetryMax(0))
	repo := makePurchaseOrderRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.PurchaseOrderListOptions{StatusEq: "draft"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Status != "draft" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_PO08: Search - no filter -> zero filter routes through cache
func TestPurchaseOrderRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedPurchaseOrderCache(t, db, samplePurchaseOrders)
	markSynced(t, db, "purchase_orders")

	srv := newPurchaseOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePurchaseOrderRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.PurchaseOrderListOptions{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(samplePurchaseOrders) {
		t.Errorf("len(got) = %d, want %d", len(got), len(samplePurchaseOrders))
	}
}
