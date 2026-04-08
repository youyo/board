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

func makeReceiptRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.ReceiptRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewReceiptRepository("default", apiClient, rc, ss, refresher, lm, time.UTC, autoRefresh)
}

func seedReceiptCache(t *testing.T, db *cache.DB, entities []boardapi.ReceiptEntity) {
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
				Resource: "receipts",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedReceiptCache: %v", err)
		}
	}
}

func newReceiptAPIServer(t *testing.T, entities []boardapi.ReceiptEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleReceipts = []boardapi.ReceiptEntity{
	{ID: 1, ClientID: 10, ProjectID: 100, Title: "ReceiptA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, ClientID: 10, ProjectID: 101, Title: "ReceiptB", Status: "issued", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, ClientID: 20, ProjectID: 102, Title: "ReceiptC", Status: "issued", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_REC01: List - cache hit -> returns cached data
func TestReceiptRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedReceiptCache(t, db, sampleReceipts)
	markSynced(t, db, "receipts")

	srv := newReceiptAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleReceipts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleReceipts))
	}
}

// T_REC02: List - no cache (initial load) -> returns data after ForceRefresh
func TestReceiptRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newReceiptAPIServer(t, sampleReceipts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleReceipts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleReceipts))
	}
}

// T_REC03: GetByID - cache hit -> returns from cache
func TestReceiptRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedReceiptCache(t, db, sampleReceipts)
	markSynced(t, db, "receipts")

	srv := newReceiptAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_REC04: GetByID - cache miss, API success -> fetches from API and returns
func TestReceiptRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "receipts")

	target := boardapi.ReceiptEntity{ID: 99, Title: "Test Receipt", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_REC05: GetByID - cache miss, API error -> returns error
func TestReceiptRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "receipts")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_REC06: Search - ClientID filter -> returns matching items
func TestReceiptRepository_Search_ClientIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedReceiptCache(t, db, sampleReceipts)
	markSynced(t, db, "receipts")

	srv := newReceiptAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ReceiptSearchParams{ClientID: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_REC07: Search - Status filter -> returns matching items
func TestReceiptRepository_Search_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	seedReceiptCache(t, db, sampleReceipts)
	markSynced(t, db, "receipts")

	srv := newReceiptAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ReceiptSearchParams{Status: "draft"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Status != "draft" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_REC08: Search - no filter -> returns all items
func TestReceiptRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedReceiptCache(t, db, sampleReceipts)
	markSynced(t, db, "receipts")

	srv := newReceiptAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ReceiptSearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleReceipts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleReceipts))
	}
}
