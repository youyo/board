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

func makeUserRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.UserRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	return repository.NewUserRepository("default", apiClient, rc, ss, refresher, lm, time.UTC)
}

func seedUserCache(t *testing.T, db *cache.DB, entities []boardapi.UserEntity) {
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
				Resource: "users",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedUserCache: %v", err)
		}
	}
}

func newUserAPIServer(t *testing.T, entities []boardapi.UserEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

var sampleUsers = []boardapi.UserEntity{
	{ID: 1, Name: "Taro Tanaka", Email: "tanaka@example.com", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Name: "Hanako Suzuki", Email: "suzuki@example.com", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Name: "Jiro Sato", Email: "sato@example.com", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_USR01: List - cache hit -> returns cached data
func TestUserRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedUserCache(t, db, sampleUsers)
	markSynced(t, db, "users")

	srv := newUserAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeUserRepo(t, db, apiClient)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.UserListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleUsers) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleUsers))
	}
}

// T_USR03: GetByID - cache hit -> returns from cache
func TestUserRepository_GetByID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedUserCache(t, db, sampleUsers)
	markSynced(t, db, "users")

	srv := newUserAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeUserRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_USR04: GetByID - cache miss, API success -> fetches from API and returns
func TestUserRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "users")

	target := boardapi.UserEntity{ID: 99, Name: "Test User", Email: "test@example.com", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeUserRepo(t, db, apiClient)
	got, err := repo.GetByID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_USR05: GetByID - cache miss, API error -> returns error
func TestUserRepository_GetByID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "users")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeUserRepo(t, db, apiClient)
	_, err := repo.GetByID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_USR06: Search - Name filter -> bypasses cache, returns data from API
func TestUserRepository_Search_NameFilter(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "users")

	// API サーバーは name_cont=Taro Tanaka に合致する 1 件を返すと想定
	filtered := []boardapi.UserEntity{
		{ID: 1, Name: "Taro Tanaka", Email: "tanaka@example.com", UpdatedAt: "2026-01-01T00:00:00Z"},
	}
	srv := newUserAPIServer(t, filtered)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeUserRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.UserListOptions{NameCont: "Taro Tanaka"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Taro Tanaka" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_USR07: Search - Email filter -> bypasses cache, returns data from API
func TestUserRepository_Search_EmailFilter(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "users")

	// API サーバーは email_cont=suzuki@example.com に合致する 1 件を返すと想定
	filtered := []boardapi.UserEntity{
		{ID: 2, Name: "Hanako Suzuki", Email: "suzuki@example.com", UpdatedAt: "2026-01-02T00:00:00Z"},
	}
	srv := newUserAPIServer(t, filtered)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeUserRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.UserListOptions{EmailCont: "suzuki@example.com"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Email != "suzuki@example.com" {
		t.Errorf("unexpected result: %+v", got)
	}
}

// T_USR08: Search - no filter -> returns all items
func TestUserRepository_Search_NoFilter(t *testing.T) {
	db := newTestDB(t)
	seedUserCache(t, db, sampleUsers)
	markSynced(t, db, "users")

	srv := newUserAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeUserRepo(t, db, apiClient)
	got, err := repo.Search(context.Background(), boardapi.UserListOptions{}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != len(sampleUsers) {
		t.Errorf("len(got) = %d, want %d", len(got), len(sampleUsers))
	}
}
