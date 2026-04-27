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

// makeProjectRepo constructs a ProjectRepository for testing.
func makeProjectRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.ProjectRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewProjectRepository("default", apiClient, rc, ss, refresher, lm, tz)
}

// seedProjectCache writes ProjectEntity records directly into the cache.
func seedProjectCache(t *testing.T, db *cache.DB, entities []boardapi.ProjectEntity) {
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
				Resource: "projects",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedProjectCache: %v", err)
		}
	}
}

// newProjectAPIServer returns an httptest.Server that serves entities for /v1/projects.
func newProjectAPIServer(t *testing.T, entities []boardapi.ProjectEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// M44: ClientID/Status フィールド廃止。Client nested / OrderStatusName で代替。
var sampleProjects = []boardapi.ProjectEntity{
	{ID: 1, Client: &boardapi.ClientRef{ID: 10}, Name: "ProjectA", OrderStatusName: "active", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Client: &boardapi.ClientRef{ID: 10}, Name: "ProjectB", OrderStatusName: "closed", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Client: &boardapi.ClientRef{ID: 20}, Name: "ProjectC", OrderStatusName: "active", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_R42: List - cache hit -> returns cached data
func TestProjectRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.ProjectListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleProjects) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleProjects))
	}
}

// T_R45: List - opts.ForceRefresh=true -> returns data after ForceRefresh
func TestProjectRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newProjectAPIServer(t, sampleProjects)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true}, boardapi.ProjectListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleProjects) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleProjects))
	}
}

// T_R46: List - opts.Refresh=true -> returns data after DeltaRefresh
func TestProjectRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects[:1])
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, sampleProjects)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ProjectListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R47: List - opts.Limit=2 -> returns only 2 items
func TestProjectRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2}, boardapi.ProjectListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != 2 {
		t.Errorf("len(got.Items) = %d, want 2", len(got.Items))
	}
}

// T_R48: List - opts.Refresh=true, API error -> returns stale cache
func TestProjectRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ProjectListOptions{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R49: Search with NameCont filter -> bypasses cache and calls API directly
func TestProjectRepository_Search_NameContFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	// API server returns only matching results
	filtered := []boardapi.ProjectEntity{sampleProjects[0]}
	var observedNameCont string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedNameCont = r.URL.Query().Get("name_cont")
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(filtered))
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.ProjectListOptions{NameCont: "ProjectA"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if observedNameCont != "ProjectA" {
		t.Errorf("name_cont sent to API = %q, want ProjectA", observedNameCont)
	}
	if len(got) != 1 {
		t.Errorf("len(got) = %d, want 1", len(got))
	}
}

// T_R50: Search with zero filter -> uses cache (no API call for cache bypass)
func TestProjectRepository_Search_ZeroFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	apiCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(sampleProjects))
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.ProjectListOptions{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// ゼロフィルタはキャッシュを使うため API コールなし
	if apiCallCount != 0 {
		t.Errorf("expected 0 API calls for zero filter, got %d", apiCallCount)
	}
	if len(got) != len(sampleProjects) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjects))
	}
}

// T_R51: GetByID - cache hit
func TestProjectRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_R52: GetByID - cache miss, API success
func TestProjectRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "projects")

	// M44: ClientID/Status フィールド廃止
	target := boardapi.ProjectEntity{ID: 99, Client: &boardapi.ClientRef{ID: 10}, Name: "Test Project", OrderStatusName: "active"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_R53: GetByID - cache miss, API error -> returns error
func TestProjectRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "projects")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_R54: Search - ClientIDEq filter -> bypasses cache and calls API directly
func TestProjectRepository_Search_ClientIDEqFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	// API server returns filtered results
	filtered := []boardapi.ProjectEntity{sampleProjects[0], sampleProjects[1]}
	var observedClientIDEq string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedClientIDEq = r.URL.Query().Get("client_id_eq")
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(filtered))
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.ProjectListOptions{ClientIDEq: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if observedClientIDEq != "10" {
		t.Errorf("client_id_eq sent to API = %q, want 10", observedClientIDEq)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}
