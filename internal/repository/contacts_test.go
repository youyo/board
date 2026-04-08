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

// makeContactRepo はテスト用 ContactRepository を構築する。
func makeContactRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.ContactRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewContactRepository("default", apiClient, rc, ss, refresher, lm, tz, autoRefresh)
}

// seedContactCache はキャッシュに ContactEntity を直接書き込む。
func seedContactCache(t *testing.T, db *cache.DB, entities []boardapi.ContactEntity) {
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
				Resource: "contacts",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedContactCache: %v", err)
		}
	}
}

// newContactAPIServer は contacts レスポンスを返す httptest.Server を返す。
func newContactAPIServer(t *testing.T, entities []boardapi.ContactEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleContacts = []boardapi.ContactEntity{
	{ID: 1, ClientID: 10, Name: "田中太郎", Email: "tanaka@example.com", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, ClientID: 10, Name: "鈴木花子", Email: "suzuki@example.com", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, ClientID: 20, Name: "佐藤次郎", Email: "sato@other.com", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_R29: List - キャッシュあり → キャッシュのデータを返す
func TestContactRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleContacts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleContacts))
	}
}

// T_R30: List - キャッシュなし（初回） → ForceRefresh 後データを返す
func TestContactRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newContactAPIServer(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleContacts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleContacts))
	}
}

// T_R31: List - autoRefresh=true、NeedsDailyRefresh=true → DeltaRefresh 後データを返す
func TestContactRepository_List_AutoRefresh(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts[:1])
	ss := cache.NewSyncStateStore(db)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_ = ss.Upsert(context.Background(), cache.SyncState{
		ProfileName:          "default",
		ResourceName:         "contacts",
		LastDailyRefreshDate: sql.NullString{Valid: true, String: yesterday},
	})

	srv := newContactAPIServer(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, true)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after auto refresh")
	}
}

// T_R32: List - opts.ForceRefresh=true → ForceRefresh 後データを返す
func TestContactRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newContactAPIServer(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleContacts) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleContacts))
	}
}

// T_R33: List - opts.Refresh=true → DeltaRefresh 後データを返す
func TestContactRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts[:1])
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R34: List - opts.Limit=2 → 2件のみ返す
func TestContactRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R35: List - opts.Refresh=true、API エラー → stale キャッシュを返す
func TestContactRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R36: Search - Email フィルタ → 一致するものを返す
func TestContactRepository_Search_EmailFilter(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ContactSearchParams{Email: "example.com"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R37: Search - ClientID フィルタ
func TestContactRepository_Search_ClientIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ContactSearchParams{ClientID: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R38: GetByID - キャッシュヒット
func TestContactRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_R39: GetByID - キャッシュミス、API 成功
func TestContactRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "contacts")

	target := boardapi.ContactEntity{ID: 99, ClientID: 10, Name: "テスト担当"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_R40: GetByID - キャッシュミス、API エラー → error を返す
func TestContactRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "contacts")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_R41: Search - Name フィルタ
func TestContactRepository_Search_NameFilter(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ContactSearchParams{Name: "田中"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Name != "田中太郎" {
		t.Errorf("unexpected result: %+v", got)
	}
}
