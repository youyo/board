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

// makeProjectCostRepo はテスト用 ProjectCostRepository を構築する。
func makeProjectCostRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.ProjectCostRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewProjectCostRepository("default", apiClient, rc, ss, refresher, lm, tz, autoRefresh)
}

// seedProjectCostCache はキャッシュに ProjectCostEntity を直接書き込む。
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

// newProjectCostAPIServer は project_costs レスポンスを返す httptest.Server を返す。
func newProjectCostAPIServer(t *testing.T, entities []boardapi.ProjectCostEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleProjectCosts = []boardapi.ProjectCostEntity{
	{ID: 1, ProjectID: 10, Name: "人件費", CostType: "labor", Amount: 100000},
	{ID: 2, ProjectID: 10, Name: "外注費", CostType: "outsource", Amount: 50000},
	{ID: 3, ProjectID: 20, Name: "交通費", CostType: "transport", Amount: 5000},
}

// T_R55: List - キャッシュあり → キャッシュのデータを返す
func TestProjectCostRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleProjectCosts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjectCosts))
	}
}

// T_R56: List - キャッシュなし（初回） → ForceRefresh 後データを返す
func TestProjectCostRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newProjectCostAPIServer(t, sampleProjectCosts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleProjectCosts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjectCosts))
	}
}

// T_R57: List - autoRefresh=true、NeedsDailyRefresh=true → DeltaRefresh 後データを返す
func TestProjectCostRepository_List_AutoRefresh(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts[:1])
	ss := cache.NewSyncStateStore(db)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_ = ss.Upsert(context.Background(), cache.SyncState{
		ProfileName:          "default",
		ResourceName:         "project_costs",
		LastDailyRefreshDate: sql.NullString{Valid: true, String: yesterday},
	})

	srv := newProjectCostAPIServer(t, sampleProjectCosts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, true)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after auto refresh")
	}
}

// T_R58: List - opts.ForceRefresh=true → ForceRefresh 後データを返す
func TestProjectCostRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newProjectCostAPIServer(t, sampleProjectCosts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleProjectCosts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjectCosts))
	}
}

// T_R59: List - opts.Refresh=true → DeltaRefresh 後データを返す
func TestProjectCostRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts[:1])
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, sampleProjectCosts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R60: List - opts.Limit=2 → 2件のみ返す
func TestProjectCostRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R61: List - opts.Refresh=true、API エラー → stale キャッシュを返す
func TestProjectCostRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R62: Search - ProjectID フィルタ → ProjectID が一致するものを返す
func TestProjectCostRepository_Search_ProjectIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ProjectCostSearchParams{ProjectID: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R63: GetByID - キャッシュヒット
func TestProjectCostRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_R64: GetByID - キャッシュミス、API 成功
func TestProjectCostRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "project_costs")

	target := boardapi.ProjectCostEntity{ID: 99, ProjectID: 10, Name: "テスト費用", Amount: 9999}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_R65: GetByID - キャッシュミス、API エラー → error を返す
func TestProjectCostRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "project_costs")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_R66: Search - パラメータなし → 全件返す
func TestProjectCostRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ProjectCostSearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleProjectCosts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjectCosts))
	}
}

// T_R67: List - Limit=0（無制限） → 全件返す
func TestProjectCostRepository_List_NoLimit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCostCache(t, db, sampleProjectCosts)
	markSynced(t, db, "project_costs")

	srv := newProjectCostAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectCostRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleProjectCosts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjectCosts))
	}
}
