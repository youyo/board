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

func makeAccountingTypeRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.AccountingTypeRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewAccountingTypeRepository("default", apiClient, rc, ss, refresher, lm, time.UTC, autoRefresh)
}

func seedAccountingTypeCache(t *testing.T, db *cache.DB, entities []boardapi.AccountingTypeEntity) {
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
				Resource: "accounting_types",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedAccountingTypeCache: %v", err)
		}
	}
}

func newAccountingTypeAPIServer(t *testing.T, entities []boardapi.AccountingTypeEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleAccountingTypes = []boardapi.AccountingTypeEntity{
	{ID: 1, Name: "Revenue", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Name: "Outsourcing Fee", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Name: "Consumable Supplies", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_ACT01: List - cache hit -> returns cached data
func TestAccountingTypeRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedAccountingTypeCache(t, db, sampleAccountingTypes)
	markSynced(t, db, "accounting_types")

	srv := newAccountingTypeAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeAccountingTypeRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleAccountingTypes) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleAccountingTypes))
	}
}

// T_ACT02: List - no cache (initial load) -> returns data after ForceRefresh
func TestAccountingTypeRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newAccountingTypeAPIServer(t, sampleAccountingTypes)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeAccountingTypeRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleAccountingTypes) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleAccountingTypes))
	}
}

// T_ACT03: GetByID - cache hit -> returns from cache
func TestAccountingTypeRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedAccountingTypeCache(t, db, sampleAccountingTypes)
	markSynced(t, db, "accounting_types")

	srv := newAccountingTypeAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeAccountingTypeRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_ACT04: GetByID - cache miss, API success -> fetches from API and returns
func TestAccountingTypeRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "accounting_types")

	target := boardapi.AccountingTypeEntity{ID: 99, Name: "Test Account Type", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeAccountingTypeRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_ACT05: GetByID - cache miss, API error -> returns error
func TestAccountingTypeRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "accounting_types")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeAccountingTypeRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_ACT06: Search - Name filter -> returns matching items
func TestAccountingTypeRepository_Search_NameFilter(t *testing.T) {
	db := newTestDB(t)
	seedAccountingTypeCache(t, db, sampleAccountingTypes)
	markSynced(t, db, "accounting_types")

	srv := newAccountingTypeAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeAccountingTypeRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.AccountingTypeSearchParams{Name: "Revenue"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Revenue" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_ACT07: Search - no filter -> returns all items
func TestAccountingTypeRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedAccountingTypeCache(t, db, sampleAccountingTypes)
	markSynced(t, db, "accounting_types")

	srv := newAccountingTypeAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeAccountingTypeRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.AccountingTypeSearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleAccountingTypes) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleAccountingTypes))
	}
}
