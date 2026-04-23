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

func makePaymentRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.PaymentRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewPaymentRepository("default", apiClient, rc, ss, refresher, lm, time.UTC, autoRefresh)
}

func seedPaymentCache(t *testing.T, db *cache.DB, entities []boardapi.PaymentEntity) {
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
				Resource: "payments",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedPaymentCache: %v", err)
		}
	}
}

func newPaymentAPIServer(t *testing.T, entities []boardapi.PaymentEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var samplePayments = []boardapi.PaymentEntity{
	{ID: 1, VendorID: 10, PurchaseOrderID: 100, Amount: 10000, Status: "pending", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, VendorID: 10, PurchaseOrderID: 101, Amount: 20000, Status: "paid", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, VendorID: 20, PurchaseOrderID: 102, Amount: 30000, Status: "paid", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_PAY01: List - cache hit -> returns cached data
func TestPaymentRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedPaymentCache(t, db, samplePayments)
	markSynced(t, db, "payments")

	srv := newPaymentAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.PaymentListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(samplePayments) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(samplePayments))
	}
}

// T_PAY02: List - no cache (initial load) -> returns data after ForceRefresh
func TestPaymentRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newPaymentAPIServer(t, samplePayments)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.PaymentListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(samplePayments) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(samplePayments))
	}
}

// T_PAY03: GetByID - cache hit -> returns from cache
func TestPaymentRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedPaymentCache(t, db, samplePayments)
	markSynced(t, db, "payments")

	srv := newPaymentAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_PAY04: GetByID - cache miss, API success -> fetches from API and returns
func TestPaymentRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "payments")

	target := boardapi.PaymentEntity{ID: 99, VendorID: 10, Amount: 50000, UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_PAY05: GetByID - cache miss, API error -> returns error
func TestPaymentRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "payments")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_PAY06: Search - VendorIDEq filter -> non-zero filter bypasses cache, calls API directly
func TestPaymentRepository_Search_VendorIDFilter(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "payments")

	// API returns 2 payments with vendor_id=10
	filtered := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, PurchaseOrderID: 100, Amount: 10000, Status: "pending", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, VendorID: 10, PurchaseOrderID: 101, Amount: 20000, Status: "paid", UpdatedAt: "2026-01-02T00:00:00Z"},
	}
	srv := newPaymentAPIServer(t, filtered)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.PaymentListOptions{VendorIDEq: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_PAY07: Search - StatusEq filter -> non-zero filter bypasses cache, calls API directly
func TestPaymentRepository_Search_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "payments")

	// API returns 1 payment with status=pending
	pending := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, PurchaseOrderID: 100, Amount: 10000, Status: "pending", UpdatedAt: "2026-01-01T00:00:00Z"},
	}
	srv := newPaymentAPIServer(t, pending)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.PaymentListOptions{StatusEq: "pending"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Status != "pending" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_PAY08: Search - no filter -> zero filter routes through cache
func TestPaymentRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedPaymentCache(t, db, samplePayments)
	markSynced(t, db, "payments")

	srv := newPaymentAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makePaymentRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.PaymentListOptions{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(samplePayments) {
		t.Errorf("len(got) = %d, want %d", len(got), len(samplePayments))
	}
}
