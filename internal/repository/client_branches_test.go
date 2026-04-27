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

// makeClientBranchRepo constructs a ClientBranchRepository for testing.
func makeClientBranchRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.ClientBranchRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewClientBranchRepository("default", apiClient, rc, ss, refresher, lm, tz)
}

// seedClientBranchCache writes ClientBranchEntity records directly into the cache.
func seedClientBranchCache(t *testing.T, db *cache.DB, entities []boardapi.ClientBranchEntity) {
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
				Resource: "client_branches",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedClientBranchCache: %v", err)
		}
	}
}

// newClientBranchAPIServer returns an httptest.Server that serves entities for /v1/client_branches.
// If entities is nil the handler returns an empty array.
// The handler also wraps the response in the ListResult JSON format: {"items":[...]}.
func newClientBranchAPIServer(t *testing.T, entities []boardapi.ClientBranchEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newClientBranchAPIServerWithFilter returns an httptest.Server that filters branches by
// the name_cont query parameter, simulating server-side Ransack filtering.
func newClientBranchAPIServerWithFilter(t *testing.T, entities []boardapi.ClientBranchEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		nameCont := r.URL.Query().Get("name_cont")
		if nameCont == "" {
			w.Write(jsonArrayOf(entities))
			return
		}
		var filtered []boardapi.ClientBranchEntity
		for _, e := range entities {
			if containsStr(e.Name, nameCont) {
				filtered = append(filtered, e)
			}
		}
		w.Write(jsonArrayOf(filtered))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// containsStr reports whether s contains substr (case-sensitive).
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// M39: sampleClientBranches を実 API 準拠の新スキーマに更新。
// ClientID は廃止（フィールドから accessor へ）、Client ネスト構造を使用。
var sampleClientBranches = []boardapi.ClientBranchEntity{
	{ID: 1, Client: &boardapi.ClientRef{ID: 10, Name: "株式会社テスト", NameDisp: "テスト", CustomNo: ""}, Name: "Tokyo Branch", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Client: &boardapi.ClientRef{ID: 10, Name: "株式会社テスト", NameDisp: "テスト", CustomNo: ""}, Name: "Osaka Branch", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Client: &boardapi.ClientRef{ID: 20, Name: "株式会社サンプル", NameDisp: "サンプル", CustomNo: ""}, Name: "Nagoya Branch", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_R16: List - cache hit, autoRefresh=false -> returns cached data
func TestClientBranchRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedClientBranchCache(t, db, sampleClientBranches)
	markSynced(t, db, "client_branches")

	srv := newClientBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleClientBranches) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleClientBranches))
	}
}

// T_R19: List - opts.ForceRefresh=true -> returns data after ForceRefresh
func TestClientBranchRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newClientBranchAPIServer(t, sampleClientBranches)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true}, boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleClientBranches) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleClientBranches))
	}
}

// T_R20: List - opts.Refresh=true -> returns data after DeltaRefresh
func TestClientBranchRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedClientBranchCache(t, db, sampleClientBranches[:1])
	markSynced(t, db, "client_branches")

	srv := newClientBranchAPIServer(t, sampleClientBranches)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R21: List - opts.Limit=2, 3 items in cache -> returns only 2
func TestClientBranchRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedClientBranchCache(t, db, sampleClientBranches)
	markSynced(t, db, "client_branches")

	srv := newClientBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2}, boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != 2 {
		t.Errorf("len(got.Items) = %d, want 2", len(got.Items))
	}
}

// T_R22: List - opts.Refresh=true, API error -> returns stale cache
func TestClientBranchRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedClientBranchCache(t, db, sampleClientBranches)
	markSynced(t, db, "client_branches")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("expected no error on delta refresh failure, got: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R23: Search - ClientID filter -> calls API directly (not cache) and returns API results.
// The BOARD API nests the parent client as {"client": {"id": N, ...}} rather than
// providing a flat client_id field, so in-memory ClientID filtering is unreliable.
// Search with ClientID now delegates to api.SearchClientBranches which sends client_id
// as a query parameter to the API, bypassing the cache entirely for this code path.
func TestClientBranchRepository_Search_ClientIDFilter(t *testing.T) {
	db := newTestDB(t)
	// Cache contains all three branches but Search(ClientID:10) should bypass cache
	// and call the API directly. The mock API returns only the ClientID=10 branches.
	seedClientBranchCache(t, db, sampleClientBranches)
	markSynced(t, db, "client_branches")

	clientID10Branches := sampleClientBranches[:2] // ID=1 and ID=2, both ClientID=10
	srv := newClientBranchAPIServer(t, clientID10Branches)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.ClientBranchListOptions{ClientIDEq: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R24: Search - NameCont filter -> calls API with name_cont query param, returns matching items
func TestClientBranchRepository_Search_NameContFilter(t *testing.T) {
	db := newTestDB(t)
	// NameCont is a non-zero filter: bypasses cache and calls API directly.
	// Use a server that simulates Ransack name_cont filtering.
	srv := newClientBranchAPIServerWithFilter(t, sampleClientBranches)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.ClientBranchListOptions{NameCont: "Tokyo"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Tokyo Branch" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_R25: GetByID - cache hit -> returns from cache
func TestClientBranchRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedClientBranchCache(t, db, sampleClientBranches)
	markSynced(t, db, "client_branches")

	srv := newClientBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_R26: GetByID - cache miss, API success -> fetches from API, upserts, and returns
func TestClientBranchRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "client_branches")

	target := boardapi.ClientBranchEntity{ID: 99, Client: &boardapi.ClientRef{ID: 10, Name: "株式会社テスト", NameDisp: "テスト", CustomNo: ""}, Name: "Test Branch"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_R27: GetByID - cache miss, API error -> returns error
func TestClientBranchRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "client_branches")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_R28: List - Limit=0 (unlimited) -> returns all items
func TestClientBranchRepository_List_NoLimit(t *testing.T) {
	db := newTestDB(t)
	seedClientBranchCache(t, db, sampleClientBranches)
	markSynced(t, db, "client_branches")

	srv := newClientBranchAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeClientBranchRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 0}, boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleClientBranches) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleClientBranches))
	}
}
