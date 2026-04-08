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

// makeProjectRepo はテスト用 ProjectRepository を構築する。
func makeProjectRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.ProjectRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewProjectRepository("default", apiClient, rc, ss, refresher, lm, tz, autoRefresh)
}

// seedProjectCache はキャッシュに ProjectEntity を直接書き込む。
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

// newProjectAPIServer は projects レスポンスを返す httptest.Server を返す。
func newProjectAPIServer(t *testing.T, entities []boardapi.ProjectEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleProjects = []boardapi.ProjectEntity{
	{ID: 1, ClientID: 10, Name: "プロジェクトA", Status: "active", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, ClientID: 10, Name: "プロジェクトB", Status: "closed", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, ClientID: 20, Name: "プロジェクトC", Status: "active", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_R42: List - キャッシュあり → キャッシュのデータを返す
func TestProjectRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleProjects) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjects))
	}
}

// T_R43: List - キャッシュなし（初回） → ForceRefresh 後データを返す
func TestProjectRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newProjectAPIServer(t, sampleProjects)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleProjects) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjects))
	}
}

// T_R44: List - autoRefresh=true、NeedsDailyRefresh=true → DeltaRefresh 後データを返す
func TestProjectRepository_List_AutoRefresh(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects[:1])
	ss := cache.NewSyncStateStore(db)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_ = ss.Upsert(context.Background(), cache.SyncState{
		ProfileName:          "default",
		ResourceName:         "projects",
		LastDailyRefreshDate: sql.NullString{Valid: true, String: yesterday},
	})

	srv := newProjectAPIServer(t, sampleProjects)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, true)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after auto refresh")
	}
}

// T_R45: List - opts.ForceRefresh=true → ForceRefresh 後データを返す
func TestProjectRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newProjectAPIServer(t, sampleProjects)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleProjects) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleProjects))
	}
}

// T_R46: List - opts.Refresh=true → DeltaRefresh 後データを返す
func TestProjectRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects[:1])
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, sampleProjects)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R47: List - opts.Limit=2 → 2件のみ返す
func TestProjectRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R48: List - opts.Refresh=true、API エラー → stale キャッシュを返す
func TestProjectRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R49: Search - Status フィルタ（"active"）
func TestProjectRepository_Search_StatusFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ProjectSearchParams{Status: "active"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R50: Search - UpdatedAtFrom フィルタ → フィルタに含めない（API 側で絞り込み済み）
func TestProjectRepository_Search_UpdatedAtFrom(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	// UpdatedAtFrom はフィルタに含めないので全件返る
	got, err := repo.Search(context.Background(), boardapi.ProjectSearchParams{UpdatedAtFrom: "2026-01-02T00:00:00Z"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleProjects) {
		t.Errorf("len(got) = %d, want %d (UpdatedAtFrom is not a client-side filter)", len(got), len(sampleProjects))
	}
}

// T_R51: GetByID - キャッシュヒット
func TestProjectRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_R52: GetByID - キャッシュミス、API 成功
func TestProjectRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "projects")

	target := boardapi.ProjectEntity{ID: 99, ClientID: 10, Name: "テスト案件", Status: "active"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_R53: GetByID - キャッシュミス、API エラー → error を返す
func TestProjectRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "projects")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_R54: Search - Name フィルタ + ClientID フィルタ
func TestProjectRepository_Search_MultiFilter(t *testing.T) {
	db := newTestDB(t)
	seedProjectCache(t, db, sampleProjects)
	markSynced(t, db, "projects")

	srv := newProjectAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeProjectRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ProjectSearchParams{ClientID: 10, Name: "プロジェクト"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}
