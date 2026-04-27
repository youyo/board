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

func makeVendorRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.VendorRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewVendorRepository("default", apiClient, rc, ss, refresher, lm, time.UTC)
}

func seedVendorCache(t *testing.T, db *cache.DB, entities []boardapi.VendorEntity) {
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
				Resource: "vendors",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedVendorCache: %v", err)
		}
	}
}

func newVendorAPIServer(t *testing.T, entities []boardapi.VendorEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleVendors = []boardapi.VendorEntity{
	{ID: 1, Name: "VendorA", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Name: "VendorB", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Name: "OtherVendorC", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_VEN01: List - cache hit -> returns cached data
func TestVendorRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedVendorCache(t, db, sampleVendors)
	markSynced(t, db, "vendors")

	srv := newVendorAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.VendorListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleVendors) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleVendors))
	}
}

// T_VEN03: GetByID - cache hit -> returns from cache
func TestVendorRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedVendorCache(t, db, sampleVendors)
	markSynced(t, db, "vendors")

	srv := newVendorAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_VEN04: GetByID - cache miss, API success -> fetches from API and returns
func TestVendorRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "vendors")

	target := boardapi.VendorEntity{ID: 99, Name: "Test Vendor", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_VEN05: GetByID - cache miss, API error -> returns error
func TestVendorRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "vendors")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorRepo(t, db, apiClient)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_VEN06: Search - NameCont filter -> uses cache + Go-side filter (cache-first).
func TestVendorRepository_Search_NameContFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorCache(t, db, []boardapi.VendorEntity{
		{ID: 1, Name: "VendorA", UpdatedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, Name: "VendorB", UpdatedAt: "2026-01-01T00:00:00Z"},
	})
	markSynced(t, db, "vendors")

	apiClient := boardapi.New("", "key", "token", 5*time.Second, boardapi.WithRetryMax(0))
	repo := makeVendorRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.VendorListOptions{NameCont: "VendorA"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Name != "VendorA" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_VEN07: Search - no filter -> returns all items from cache
func TestVendorRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorCache(t, db, sampleVendors)
	markSynced(t, db, "vendors")

	srv := newVendorAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.VendorListOptions{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleVendors) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleVendors))
	}
}
