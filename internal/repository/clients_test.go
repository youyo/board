package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/refresh"
	"github.com/youyo/board/internal/repository"
)

// --- Common test helpers ---

// newTestDB creates a SQLite DB (temporary file) for testing.
func newTestDB(t *testing.T) *cache.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := cache.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	if err := cache.Migrate(db); err != nil {
		t.Fatalf("cache.Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// jsonArrayOf marshals entities to a JSON array.
func jsonArrayOf(entities interface{}) []byte {
	b, _ := json.Marshal(entities)
	return b
}

// newClientAPIServer returns an httptest.Server that serves entities for /v1/clients.
func newClientAPIServer(t *testing.T, entities []boardapi.ClientEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newErrorAPIServer returns an httptest.Server that responds with 500 errors.
func newErrorAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// makeClientRepo constructs a ClientRepository for testing.
func makeClientRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.ClientRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewClientRepository("default", apiClient, rc, ss, refresher, lm, tz, autoRefresh)
}

// seedClientCache writes ClientEntity records directly into the cache.
func seedClientCache(t *testing.T, db *cache.DB, entities []boardapi.ClientEntity) {
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
				Resource: "clients",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedClientCache: %v", err)
		}
	}
}

// markSynced sets the SyncState to "synced today".
func markSynced(t *testing.T, db *cache.DB, resource string) {
	t.Helper()
	ss := cache.NewSyncStateStore(db)
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")
	err := ss.Upsert(ctx, cache.SyncState{
		ProfileName:          "default",
		ResourceName:         resource,
		LastDailyRefreshDate: sql.NullString{Valid: true, String: today},
		LastSyncStatus:       sql.NullString{Valid: true, String: "success"},
	})
	if err != nil {
		t.Fatalf("markSynced: %v", err)
	}
}

var sampleClients = []boardapi.ClientEntity{
	{ID: 1, Name: "ClientA", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Name: "ClientB", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Name: "OtherClientC", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_R01: List - cache hit, autoRefresh=false -> returns cached data
func TestClientRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil) // API should not be called
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleClients) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleClients))
	}
}

// T_R02: List - no cache (initial load), autoRefresh=false -> returns data after ForceRefresh
func TestClientRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newClientAPIServer(t, sampleClients)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleClients) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleClients))
	}
}

// T_R03: List - autoRefresh=true, NeedsDailyRefresh=true -> returns data after DeltaRefresh
func TestClientRepository_List_AutoRefresh(t *testing.T) {
	db := newTestDB(t)
	// Seed cache with data but set SyncState to "yesterday"
	seedClientCache(t, db, sampleClients[:1])
	ss := cache.NewSyncStateStore(db)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_ = ss.Upsert(context.Background(), cache.SyncState{
		ProfileName:          "default",
		ResourceName:         "clients",
		LastDailyRefreshDate: sql.NullString{Valid: true, String: yesterday},
	})

	// API returns all entries
	srv := newClientAPIServer(t, sampleClients)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, true)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after auto refresh")
	}
}

// T_R04: List - opts.ForceRefresh=true -> returns data after ForceRefresh
func TestClientRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newClientAPIServer(t, sampleClients)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleClients) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleClients))
	}
}

// T_R05: List - opts.Refresh=true -> returns data after DeltaRefresh
func TestClientRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients[:1])
	markSynced(t, db, "clients")

	// DeltaRefresh returns delta (all entries)
	srv := newClientAPIServer(t, sampleClients)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R06: List - opts.Limit=2, 3 entries in cache -> returns only 2
func TestClientRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R07: List - opts.Refresh=true, API error -> returns stale cache (no error)
func TestClientRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("expected no error on delta refresh failure, got: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R08: GetByID - cache hit -> returns from cache
func TestClientRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil) // API should not be called
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_R09: GetByID - cache miss, API success -> fetches from API, upserts, and returns
func TestClientRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "clients")

	target := boardapi.ClientEntity{ID: 42, Name: "TestClient", UpdatedAt: "2026-01-01T00:00:00Z"}
	// response for /v1/clients/42
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 42, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 42 {
		t.Errorf("expected ID=42, got %+v", got)
	}
}

// T_R10: GetByID - cache miss, API error -> returns error
func TestClientRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "clients")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_R11: Search - cache hit, no params -> returns all
func TestClientRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ClientSearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleClients) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleClients))
	}
}

// T_R12: Search - Name filter -> returns matching entries
func TestClientRepository_Search_NameFilter(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ClientSearchParams{Name: "ClientA"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len(got) = %d, want 1", len(got))
	}
	if got[0].Name != "ClientA" {
		t.Errorf("got[0].Name = %q, want ClientA", got[0].Name)
	}
}

// T_R13: Search - opts.ForceRefresh=true -> filters after ForceRefresh
func TestClientRepository_Search_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newClientAPIServer(t, sampleClients)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ClientSearchParams{Name: "Client"}, repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// "ClientA" and "ClientB" match ("OtherClientC" also contains "Client")
	if len(got) == 0 {
		t.Error("expected at least one result")
	}
}

// T_R14: List - Limit=0 (unlimited) -> returns all entries
func TestClientRepository_List_NoLimit(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleClients) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleClients))
	}
}

// T_R_SEARCH_LIMIT: Search - Limit=2, 3 total entries, only 1 matches filter ->
// bug: List truncates to 2 before filter, so match is lost.
// fix: List is called with Limit=0, filter runs on all, then applyLimit caps the result.
func TestClientRepository_Search_LimitAppliedAfterFilter(t *testing.T) {
	db := newTestDB(t)
	// sampleClients = [ClientA, ClientB, OtherClientC]
	// Only "OtherClientC" matches Name="Other", but it is the 3rd entry.
	// Buggy behaviour: List(Limit=2) -> [ClientA, ClientB], filter -> [], result=0
	// Fixed behaviour: List(Limit=0) -> all 3, filter -> [OtherClientC], applyLimit -> [OtherClientC], result=1
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ClientSearchParams{Name: "Other"}, repository.ReadOptions{Limit: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len(got) = %d, want 1 (Search Limit must apply after filter, not before)", len(got))
	}
	if len(got) == 1 && got[0].Name != "OtherClientC" {
		t.Errorf("got[0].Name = %q, want OtherClientC", got[0].Name)
	}
}

// T_R15: List - context canceled -> returns context.Canceled
func TestClientRepository_List_ContextCanceled(t *testing.T) {
	db := newTestDB(t)

	// API server delays responses
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait until the context is canceled
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := repo.List(ctx, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}
