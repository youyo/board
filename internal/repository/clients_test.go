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

// --- テスト共通ヘルパー ---

// newTestDB はテスト用 SQLite DB（一時ファイル）を作成する。
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

// jsonArrayOf は entities を JSON 配列形式にマーシャルする。
func jsonArrayOf(entities interface{}) []byte {
	b, _ := json.Marshal(entities)
	return b
}

// newClientAPIServer は /v1/clients に対して entities を返す httptest.Server を返す。
func newClientAPIServer(t *testing.T, entities []boardapi.ClientEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newErrorAPIServer は 500 エラーを返す httptest.Server を返す。
func newErrorAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// makeClientRepo はテスト用 ClientRepository を構築する。
func makeClientRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.ClientRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewClientRepository("default", apiClient, rc, ss, refresher, lm, tz, autoRefresh)
}

// seedClientCache はキャッシュに ClientEntity を直接書き込む。
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

// markSynced は SyncState を「本日同期済み」に設定する。
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
	{ID: 1, Name: "顧客A", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Name: "顧客B", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Name: "別顧客C", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_R01: List - キャッシュあり、autoRefresh=false → キャッシュのデータを返す
func TestClientRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil) // API は呼ばれないはず
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

// T_R02: List - キャッシュなし（初回）、autoRefresh=false → ForceRefresh 後データを返す
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

// T_R03: List - autoRefresh=true、NeedsDailyRefresh=true → DeltaRefresh 後データを返す
func TestClientRepository_List_AutoRefresh(t *testing.T) {
	db := newTestDB(t)
	// キャッシュにデータを入れるが SyncState は「昨日」にする
	seedClientCache(t, db, sampleClients[:1])
	ss := cache.NewSyncStateStore(db)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_ = ss.Upsert(context.Background(), cache.SyncState{
		ProfileName:          "default",
		ResourceName:         "clients",
		LastDailyRefreshDate: sql.NullString{Valid: true, String: yesterday},
	})

	// API は全件返す
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

// T_R04: List - opts.ForceRefresh=true → ForceRefresh 後データを返す
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

// T_R05: List - opts.Refresh=true → DeltaRefresh 後データを返す
func TestClientRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients[:1])
	markSynced(t, db, "clients")

	// DeltaRefresh は差分（全件）を返す
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

// T_R06: List - opts.Limit=2、キャッシュに3件 → 2件のみ返す
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

// T_R07: List - opts.Refresh=true、API エラー → stale キャッシュを返す（エラーなし）
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

// T_R08: GetByID - キャッシュヒット → キャッシュから返す
func TestClientRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil) // API 呼ばれないはず
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

// T_R09: GetByID - キャッシュミス、API 成功 → API 取得後 upsert して返す
func TestClientRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "clients")

	target := boardapi.ClientEntity{ID: 42, Name: "テスト顧客", UpdatedAt: "2026-01-01T00:00:00Z"}
	// /v1/clients/42 に対応するレスポンス
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

// T_R10: GetByID - キャッシュミス、API エラー → error を返す
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

// T_R11: Search - キャッシュあり、パラメータなし → 全件返す
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

// T_R12: Search - Name フィルタ → 一致するものを返す
func TestClientRepository_Search_NameFilter(t *testing.T) {
	db := newTestDB(t)
	seedClientCache(t, db, sampleClients)
	markSynced(t, db, "clients")

	srv := newClientAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ClientSearchParams{Name: "顧客A"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len(got) = %d, want 1", len(got))
	}
	if got[0].Name != "顧客A" {
		t.Errorf("got[0].Name = %q, want 顧客A", got[0].Name)
	}
}

// T_R13: Search - opts.ForceRefresh=true → ForceRefresh 後にフィルタ
func TestClientRepository_Search_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newClientAPIServer(t, sampleClients)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ClientSearchParams{Name: "顧客"}, repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// 「顧客A」「顧客B」がマッチ（「別顧客C」も「顧客」を含む）
	if len(got) == 0 {
		t.Error("expected at least one result")
	}
}

// T_R14: List - Limit=0（無制限） → 全件返す
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

// T_R15: List - コンテキストキャンセル → context.Canceled を返す
func TestClientRepository_List_ContextCanceled(t *testing.T) {
	db := newTestDB(t)

	// API サーバーは遅延させる
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// コンテキストがキャンセルされるまで待機
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientRepo(t, db, apiClient, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即座にキャンセル

	_, err := repo.List(ctx, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}
