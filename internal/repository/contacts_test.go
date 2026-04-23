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

// makeContactRepo constructs a ContactRepository for testing.
func makeContactRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client, autoRefresh bool) *repository.ContactRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "test-owner")
	tz := time.UTC
	return repository.NewContactRepository("default", apiClient, rc, ss, refresher, lm, tz, autoRefresh)
}

// seedContactCache writes ContactEntity records directly into the cache.
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

// newContactAPIServer returns an httptest.Server that serves entities for /v1/contacts.
func newContactAPIServer(t *testing.T, entities []boardapi.ContactEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonArrayOf(entities))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newContactAPIServerWithFilter returns an httptest.Server that simulates Ransack filtering
// for name_cont and email_cont query parameters.
func newContactAPIServerWithFilter(t *testing.T, entities []boardapi.ContactEntity) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		nameCont := r.URL.Query().Get("name_cont")
		emailCont := r.URL.Query().Get("email_cont")
		filtered := entities
		if nameCont != "" {
			var tmp []boardapi.ContactEntity
			for _, e := range filtered {
				if contactContainsName(e, nameCont) {
					tmp = append(tmp, e)
				}
			}
			filtered = tmp
		}
		if emailCont != "" {
			var tmp []boardapi.ContactEntity
			for _, e := range filtered {
				if e.Email != nil && findSubstr(*e.Email, emailCont) {
					tmp = append(tmp, e)
				}
			}
			filtered = tmp
		}
		w.Write(jsonArrayOf(filtered))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// contactContainsName reports whether the contact's full name contains the given substring.
func contactContainsName(e boardapi.ContactEntity, s string) bool {
	return findSubstr(e.LastName+e.FirstName, s) || findSubstr(e.LastName+" "+e.FirstName, s)
}

func strPtr(s string) *string { return &s }

var sampleContacts = []boardapi.ContactEntity{
	{ID: 1, Client: &boardapi.ClientRef{ID: 10}, LastName: "Tanaka", FirstName: "Taro", Email: strPtr("tanaka@example.com"), UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, Client: &boardapi.ClientRef{ID: 10}, LastName: "Suzuki", FirstName: "Hanako", Email: strPtr("suzuki@example.com"), UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, Client: &boardapi.ClientRef{ID: 20}, LastName: "Sato", FirstName: "Jiro", Email: strPtr("sato@other.com"), UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_R29: List - cache hit -> returns cached data
func TestContactRepository_List_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.ContactListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleContacts) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleContacts))
	}
}

// T_R30: List - no cache (initial load) -> returns data after ForceRefresh
func TestContactRepository_List_InitialLoad(t *testing.T) {
	db := newTestDB(t)

	srv := newContactAPIServer(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.ContactListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleContacts) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleContacts))
	}
}

// T_R31: List - autoRefresh=true, NeedsDailyRefresh=true -> returns data after DeltaRefresh
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
	got, err := repo.List(context.Background(), repository.ReadOptions{}, boardapi.ContactListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected non-empty result after auto refresh")
	}
}

// T_R32: List - opts.ForceRefresh=true -> returns data after ForceRefresh
func TestContactRepository_List_ForceRefresh(t *testing.T) {
	db := newTestDB(t)

	srv := newContactAPIServer(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{ForceRefresh: true}, boardapi.ContactListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != len(sampleContacts) {
		t.Errorf("len(got.Items) = %d, want %d", len(got.Items), len(sampleContacts))
	}
}

// T_R33: List - opts.Refresh=true -> returns data after DeltaRefresh
func TestContactRepository_List_DeltaRefresh(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts[:1])
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ContactListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected non-empty result after delta refresh")
	}
}

// T_R34: List - opts.Limit=2 -> returns only 2 items
func TestContactRepository_List_Limit(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newContactAPIServer(t, nil)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Limit: 2}, boardapi.ContactListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got.Items) != 2 {
		t.Errorf("len(got.Items) = %d, want 2", len(got.Items))
	}
}

// T_R35: List - opts.Refresh=true, API error -> returns stale cache
func TestContactRepository_List_DeltaRefreshAPIError_StaleCache(t *testing.T) {
	db := newTestDB(t)
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.List(context.Background(), repository.ReadOptions{Refresh: true}, boardapi.ContactListOptions{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(got.Items) == 0 {
		t.Error("expected stale cache data")
	}
}

// T_R36: Search - EmailCont filter -> calls API with email_cont query param, returns matching items
func TestContactRepository_Search_EmailContFilter(t *testing.T) {
	db := newTestDB(t)
	// EmailCont is a non-zero filter: bypasses cache and calls API directly.
	// Use a server that simulates Ransack email_cont filtering.
	srv := newContactAPIServerWithFilter(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ContactListOptions{EmailCont: "example.com"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R37: Search - ClientID filter -> calls API directly (not cache) and returns API results.
// The BOARD API nests the parent client as {"client": {"id": N, ...}} rather than
// providing a flat client_id field, so in-memory ClientID filtering is unreliable.
// Search with ClientID now delegates to api.SearchContacts which sends client_id
// as a query parameter to the API, bypassing the cache entirely for this code path.
func TestContactRepository_Search_ClientIDFilter(t *testing.T) {
	db := newTestDB(t)
	// Cache contains all three contacts but Search(ClientID:10) should bypass cache
	// and call the API directly. The mock API returns only the ClientID=10 contacts.
	seedContactCache(t, db, sampleContacts)
	markSynced(t, db, "contacts")

	clientID10Contacts := sampleContacts[:2] // ID=1 and ID=2, both ClientID=10
	srv := newContactAPIServer(t, clientID10Contacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ContactListOptions{ClientIDEq: 10}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
}

// T_R38: GetByID - cache hit
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

// T_R39: GetByID - cache miss, API success
func TestContactRepository_GetByID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)
	markSynced(t, db, "contacts")

	target := boardapi.ContactEntity{ID: 99, Client: &boardapi.ClientRef{ID: 10}, LastName: "Test", FirstName: "Contact"}
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

// T_R40: GetByID - cache miss, API error -> returns error
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

// T_R41: Search - NameCont filter -> calls API with name_cont query param, returns matching items
func TestContactRepository_Search_NameContFilter(t *testing.T) {
	db := newTestDB(t)
	// NameCont is a non-zero filter: bypasses cache and calls API directly.
	// Use a server that simulates Ransack name_cont filtering.
	srv := newContactAPIServerWithFilter(t, sampleContacts)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeContactRepo(t, db, apiClient, false)
	got, err := repo.Search(context.Background(), boardapi.ContactListOptions{NameCont: "Tanaka"}, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].DisplayName() != "Tanaka Taro" {
		t.Errorf("unexpected result: %+v", got)
	}
}
