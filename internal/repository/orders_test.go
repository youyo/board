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

func makeOrderRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.OrderRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewOrderRepository("default", apiClient, rc, ss, refresher, lm, time.UTC, autoRefresh)
}

func seedOrderCache(t *testing.T, db *cache.DB, entities []boardapi.OrderEntity) {
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

func newOrderAPIServer(t *testing.T, entities []boardapi.OrderEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleOrders = []boardapi.OrderEntity{
	{ID: 1, ClientID: 10, ProjectID: 100, Title: "OrderA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, ClientID: 10, ProjectID: 101, Title: "OrderB", Status: "confirmed", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, ClientID: 20, ProjectID: 102, Title: "OrderC", Status: "completed", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_ORD01: List - cache hit -> returns cached data
func TestOrderRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedOrderCache(t, db, sampleOrders)
	markSynced(t, db, "orders")

	srv := newOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleOrders) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleOrders))
	}
}

// T_ORD02: List - no cache (initial load) -> returns data after ForceRefresh
func TestOrderRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newOrderAPIServer(t, sampleOrders)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleOrders) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleOrders))
	}
}

// T_ORD03: GetByID - cache hit -> returns from cache
func TestOrderRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedOrderCache(t, db, sampleOrders)
	markSynced(t, db, "orders")

	srv := newOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_ORD04: GetByID - cache miss, API success -> fetches from API and returns
func TestOrderRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "orders")

	target := boardapi.OrderEntity{ID: 99, Title: "Test Order", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_ORD05: GetByID - cache miss, API error -> returns error
func TestOrderRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "orders")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_ORD06: Search - ClientID filter -> returns matching items
func TestOrderRepository_Search_ClientIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedOrderCache(t, db, sampleOrders)
	markSynced(t, db, "orders")

	srv := newOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.OrderSearchParams{ClientID: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_ORD07: Search - Status filter -> returns matching items
func TestOrderRepository_Search_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	seedOrderCache(t, db, sampleOrders)
	markSynced(t, db, "orders")

	srv := newOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.OrderSearchParams{Status: "draft"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Status != "draft" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_ORD08: Search - no filter -> returns all items
func TestOrderRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedOrderCache(t, db, sampleOrders)
	markSynced(t, db, "orders")

	srv := newOrderAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeOrderRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.OrderSearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleOrders) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleOrders))
	}
}
