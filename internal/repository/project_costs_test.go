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

// makeProjectCostRepo constructs a ProjectCostRepository for testing.
func makeProjectCostRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.ProjectCostRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewProjectCostRepository("default", apiClient, rc, ss, refresher, lm, tz)
}

// seedProjectCostCache writes ProjectCostEntity records directly into the cache.
func seedProjectCostCache(t *testing.T, db *cache.DB, entities []boardapi.ProjectCostEntity) {
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
				Resource: "project_costs",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedProjectCostCache: %v", err)
		}
	}
}

// newProjectCostAPIServer returns an httptest.Server that serves entities for /v1/project_costs.
func newProjectCostAPIServer(t *testing.T, entities []boardapi.ProjectCostEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newProjectCostAPIServerWithFilter returns an httptest.Server that simulates Ransack filtering
// for the project_id_eq query parameter.
func newProjectCostAPIServerWithFilter(t *testing.T, entities []boardapi.ProjectCostEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		projectIDEqStr := r.URL.Query().Get("project_id_eq")
		if projectIDEqStr == "" {
			w.Write(jsonArrayOf(entities))
			return
		}
		projectIDEq := 0
		for _, c := range projectIDEqStr {
			if c >= '0' && c <= '9' {
				projectIDEq = projectIDEq*10 + int(c-'0')
			}
		}
		var filtered []boardapi.ProjectCostEntity
		for _, e := range entities {
			if e.ProjectID == projectIDEq {
				filtered = append(filtered, e)
			}
		}
		w.Write(jsonArrayOf(filtered))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleProjectCosts = []boardapi.ProjectCostEntity{
	{ID: 1, ProjectID: 10, Description: "労務費", Cost: 100000},
	{ID: 2, ProjectID: 10, Description: "外注費", Cost: 50000},
	{ID: 3, ProjectID: 20, Description: "旅費交通費", Cost: 5000},
}

// T_R55: List - cache hit -> returns cached data
func TestProjectCostRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.ProjectCostListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleProjectCosts) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleProjectCosts))
	}
}

// T_R58: List - opts.ForceRefresh=true -> returns data after ForceRefresh
func TestProjectCostRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newProjectCostAPIServer(t, sampleProjectCosts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true}, boardapi.ProjectCostListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleProjectCosts) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleProjectCosts))
	}
}

// T_R59: List - opts.Refresh=true -> returns data after DeltaRefresh
func TestProjectCostRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts[:1])
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, sampleProjectCosts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ProjectCostListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R60: List - opts.Limit=2 -> returns only 2 items
func TestProjectCostRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2}, boardapi.ProjectCostListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != 2 {
		t.Errorf("len(got.Items) = %d, want 2", len(got.Items))
	}
}

// T_R61: List - opts.Refresh=true, API error -> returns stale cache
func TestProjectCostRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ProjectCostListOptions{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R62: Search - ProjectIDEq filter -> calls API with project_id_eq query param, returns matching items
func TestProjectCostRepository_Search_ProjectIDEqFilter(t *testing.T) {
	db := newTestDB(t)
	// ProjectIDEq is a non-zero filter: bypasses cache and calls API directly.
	// Use a server that simulates Ransack project_id_eq filtering.
	srv := newProjectCostAPIServerWithFilter(t, sampleProjectCosts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.ProjectCostListOptions{ProjectIDEq: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R63: GetByID - cache hit
func TestProjectCostRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_R64: GetByID - cache miss, API success
func TestProjectCostRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "project_costs")

	target := boardapi.ProjectCostEntity{ID: 99, ProjectID: 10, Description: "テストコスト", Cost: 9999}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_R65: GetByID - cache miss, API error -> returns error
func TestProjectCostRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "project_costs")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_R66: Search - no filter -> returns all items
func TestProjectCostRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.ProjectCostListOptions{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleProjectCosts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjectCosts))
	}
}

// T_R67: List - Limit=0 (unlimited) -> returns all items
func TestProjectCostRepository_List_NoLimit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 0}, boardapi.ProjectCostListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleProjectCosts) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleProjectCosts))
	}
}
