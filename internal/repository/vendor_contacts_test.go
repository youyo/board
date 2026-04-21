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

func makeVendorContactRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.VendorContactRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewVendorContactRepository("default", apiClient, rc, ss, refresher, lm, time.UTC, autoRefresh)
}

func seedVendorContactCache(t *testing.T, db *cache.DB, entities []boardapi.VendorContactEntity) {
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
				Resource: "vendor_contacts",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedVendorContactCache: %v", err)
		}
	}
}

func newVendorContactAPIServer(t *testing.T, entities []boardapi.VendorContactEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleVendorContacts = []boardapi.VendorContactEntity{
	{ID: 1, Vendor: &boardapi.VendorRef{ID: 10}, LastName: "ContactA", Email: strPtr("a@vendor.com"), UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Vendor: &boardapi.VendorRef{ID: 10}, LastName: "ContactB", Email: strPtr("b@vendor.com"), UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Vendor: &boardapi.VendorRef{ID: 20}, LastName: "ContactC", Email: strPtr("c@vendor.com"), UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_VCO01: List - cache hit -> returns cached data
func TestVendorContactRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedVendorContactCache(t, db, sampleVendorContacts)
	markSynced(t, db, "vendor_contacts")

	srv := newVendorContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleVendorContacts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleVendorContacts))
	}
}

// T_VCO02: List - no cache (initial load) -> returns data after ForceRefresh
func TestVendorContactRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newVendorContactAPIServer(t, sampleVendorContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleVendorContacts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleVendorContacts))
	}
}

// T_VCO03: GetByID - cache hit -> returns from cache
func TestVendorContactRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedVendorContactCache(t, db, sampleVendorContacts)
	markSynced(t, db, "vendor_contacts")

	srv := newVendorContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_VCO04: GetByID - cache miss, API success -> fetches from API and returns
func TestVendorContactRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "vendor_contacts")

	target := boardapi.VendorContactEntity{ID: 99, Vendor: &boardapi.VendorRef{ID: 10}, LastName: "Test Contact", Email: strPtr("test@vendor.com"), UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_VCO05: GetByID - cache miss, API error -> returns error
func TestVendorContactRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "vendor_contacts")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_VCO06: Search - VendorID filter -> returns matching items
func TestVendorContactRepository_Search_VendorIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorContactCache(t, db, sampleVendorContacts)
	markSynced(t, db, "vendor_contacts")

	srv := newVendorContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.VendorContactSearchParams{VendorID: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_VCO07: Search - Email filter -> returns matching items
func TestVendorContactRepository_Search_EmailFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorContactCache(t, db, sampleVendorContacts)
	markSynced(t, db, "vendor_contacts")

	srv := newVendorContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.VendorContactSearchParams{Email: "a@vendor.com"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Email == nil || *got[0].Email != "a@vendor.com" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_VCO08: Search - no filter -> returns all items
func TestVendorContactRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorContactCache(t, db, sampleVendorContacts)
	markSynced(t, db, "vendor_contacts")

	srv := newVendorContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.VendorContactSearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleVendorContacts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleVendorContacts))
	}
}
