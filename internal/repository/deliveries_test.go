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

func makeDeliveryRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.DeliveryRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewDeliveryRepository("default", apiClient, rc, ss, refresher, lm, time.UTC, autoRefresh)
}

func seedDeliveryCache(t *testing.T, db *cache.DB, entities []boardapi.DeliveryEntity) {
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
				Resource: "deliveries",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedDeliveryCache: %v", err)
		}
	}
}

func newDeliveryAPIServer(t *testing.T, entities []boardapi.DeliveryEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleDeliveries = []boardapi.DeliveryEntity{
	{ID: 1, ClientID: 10, ProjectID: 100, Title: "DeliveryA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, ClientID: 10, ProjectID: 101, Title: "DeliveryB", Status: "delivered", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, ClientID: 20, ProjectID: 102, Title: "DeliveryC", Status: "accepted", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_DEL01: List - cache hit -> returns cached data
func TestDeliveryRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedDeliveryCache(t, db, sampleDeliveries)
	markSynced(t, db, "deliveries")

	srv := newDeliveryAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleDeliveries) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleDeliveries))
	}
}

// T_DEL02: List - no cache (initial load) -> returns data after ForceRefresh
func TestDeliveryRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newDeliveryAPIServer(t, sampleDeliveries)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleDeliveries) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleDeliveries))
	}
}

// T_DEL03: GetByID - cache hit -> returns from cache
func TestDeliveryRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedDeliveryCache(t, db, sampleDeliveries)
	markSynced(t, db, "deliveries")

	srv := newDeliveryAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_DEL04: GetByID - cache miss, API success -> fetches from API and returns
func TestDeliveryRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "deliveries")

	target := boardapi.DeliveryEntity{ID: 99, Title: "Test Delivery", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_DEL05: GetByID - cache miss, API error -> returns error
func TestDeliveryRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "deliveries")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_DEL06: Search - ClientID filter -> returns matching items
func TestDeliveryRepository_Search_ClientIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedDeliveryCache(t, db, sampleDeliveries)
	markSynced(t, db, "deliveries")

	srv := newDeliveryAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.DeliverySearchParams{ClientID: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_DEL07: Search - Status filter -> returns matching items
func TestDeliveryRepository_Search_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	seedDeliveryCache(t, db, sampleDeliveries)
	markSynced(t, db, "deliveries")

	srv := newDeliveryAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.DeliverySearchParams{Status: "delivered"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Status != "delivered" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_DEL08: Search - no filter -> returns all items
func TestDeliveryRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedDeliveryCache(t, db, sampleDeliveries)
	markSynced(t, db, "deliveries")

	srv := newDeliveryAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeDeliveryRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.DeliverySearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleDeliveries) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleDeliveries))
	}
}
