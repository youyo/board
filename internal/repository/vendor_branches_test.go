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

func makeVendorBranchRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.VendorBranchRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewVendorBranchRepository("default", apiClient, rc, ss, refresher, lm, time.UTC, autoRefresh)
}

func seedVendorBranchCache(t *testing.T, db *cache.DB, entities []boardapi.VendorBranchEntity) {
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
				Resource: "vendor_branches",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedVendorBranchCache: %v", err)
		}
	}
}

func newVendorBranchAPIServer(t *testing.T, entities []boardapi.VendorBranchEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleVendorBranches = []boardapi.VendorBranchEntity{
	{ID: 1, VendorID: 10, Name: "支社A", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, VendorID: 10, Name: "支社B", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, VendorID: 20, Name: "支社C", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_VBR01: List - キャッシュあり → キャッシュのデータを返す
func TestVendorBranchRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedVendorBranchCache(t, db, sampleVendorBranches)
	markSynced(t, db, "vendor_branches")

	srv := newVendorBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleVendorBranches) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleVendorBranches))
	}
}

// T_VBR02: List - キャッシュなし（初回）→ ForceRefresh 後データを返す
func TestVendorBranchRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newVendorBranchAPIServer(t, sampleVendorBranches)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(sampleVendorBranches) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleVendorBranches))
	}
}

// T_VBR03: GetByID - キャッシュヒット → キャッシュから返す
func TestVendorBranchRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedVendorBranchCache(t, db, sampleVendorBranches)
	markSynced(t, db, "vendor_branches")

	srv := newVendorBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_VBR04: GetByID - キャッシュミス、API 成功 → API 取得して返す
func TestVendorBranchRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "vendor_branches")

	target := boardapi.VendorBranchEntity{ID: 99, VendorID: 10, Name: "テスト支社", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_VBR05: GetByID - キャッシュミス、API エラー → error を返す
func TestVendorBranchRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "vendor_branches")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_VBR06: Search - VendorID フィルタ → 一致するものを返す
func TestVendorBranchRepository_Search_VendorIDFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorBranchCache(t, db, sampleVendorBranches)
	markSynced(t, db, "vendor_branches")

	srv := newVendorBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.VendorBranchSearchParams{VendorID: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_VBR07: Search - Name フィルタ → 一致するものを返す
func TestVendorBranchRepository_Search_NameFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorBranchCache(t, db, sampleVendorBranches)
	markSynced(t, db, "vendor_branches")

	srv := newVendorBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.VendorBranchSearchParams{Name: "支社A"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Name != "支社A" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_VBR08: Search - パラメータなし → 全件返す
func TestVendorBranchRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedVendorBranchCache(t, db, sampleVendorBranches)
	markSynced(t, db, "vendor_branches")

	srv := newVendorBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeVendorBranchRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.VendorBranchSearchParams{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleVendorBranches) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleVendorBranches))
	}
}
