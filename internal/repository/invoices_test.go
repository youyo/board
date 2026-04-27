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

func makeInvoiceRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.InvoiceRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewInvoiceRepository("default", apiClient, rc, ss, refresher, lm, time.UTC)
}

func seedInvoiceCache(t *testing.T, db *cache.DB, entities []boardapi.InvoiceEntity) {
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
				Resource: "invoices",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedInvoiceCache: %v", err)
		}
	}
}

func newInvoiceAPIServer(t *testing.T, entities []boardapi.InvoiceEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleInvoices = []boardapi.InvoiceEntity{
	{ID: 1, ClientID: 10, ProjectID: 100, Title: "InvoiceA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, ClientID: 10, ProjectID: 101, Title: "InvoiceB", Status: "sent", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, ClientID: 20, ProjectID: 102, Title: "InvoiceC", Status: "paid", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_INV01: List - cache hit -> returns cached data
func TestInvoiceRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedInvoiceCache(t, db, sampleInvoices)
	markSynced(t, db, "invoices")

	srv := newInvoiceAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeInvoiceRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.InvoiceListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleInvoices) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleInvoices))
	}
}

// T_INV03: GetByID - cache hit -> returns from cache
func TestInvoiceRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedInvoiceCache(t, db, sampleInvoices)
	markSynced(t, db, "invoices")

	srv := newInvoiceAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeInvoiceRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_INV04: GetByID - cache miss, API success -> fetches from API and returns
func TestInvoiceRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "invoices")

	target := boardapi.InvoiceEntity{ID: 99, Title: "Test Invoice", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeInvoiceRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_INV05: GetByID - cache miss, API error -> returns error
func TestInvoiceRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "invoices")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeInvoiceRepo(t, db, apiClient)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Search ClientIDEq -> cache-first Go-side filter.
func TestInvoiceRepository_Search_ClientIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedInvoiceCache(t, db, []boardapi.InvoiceEntity{
		{ID: 1, ClientID: 10, ProjectID: 100, Title: "InvoiceA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, ClientID: 10, ProjectID: 101, Title: "InvoiceB", Status: "sent", UpdatedAt: "2026-01-02T00:00:00Z"},
		{ID: 3, ClientID: 99, ProjectID: 102, Title: "Other", Status: "draft", UpdatedAt: "2026-01-03T00:00:00Z"},
	})
	markSynced(t, db, "invoices")

	apiClient := boardapi.New("", "key", "token", 5*time.Second, boardapi.WithRetryMax(0))
	repo := makeInvoiceRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.InvoiceListOptions{ClientIDEq: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// Search StatusEq -> cache-first Go-side filter.
func TestInvoiceRepository_Search_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	seedInvoiceCache(t, db, []boardapi.InvoiceEntity{
		{ID: 1, ClientID: 10, ProjectID: 100, Title: "InvoiceA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: 3, ClientID: 20, ProjectID: 102, Title: "InvoiceC", Status: "paid", UpdatedAt: "2026-01-03T00:00:00Z"},
	})
	markSynced(t, db, "invoices")

	apiClient := boardapi.New("", "key", "token", 5*time.Second, boardapi.WithRetryMax(0))
	repo := makeInvoiceRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.InvoiceListOptions{StatusEq: "paid"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Status != "paid" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_INV08: Search - no filter -> zero filter routes through cache
func TestInvoiceRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedInvoiceCache(t, db, sampleInvoices)
	markSynced(t, db, "invoices")

	srv := newInvoiceAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeInvoiceRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.InvoiceListOptions{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleInvoices) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleInvoices))
	}
}
