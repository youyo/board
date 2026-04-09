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
	"github.com/youyo/board/internal/repository"
)

func makeReceiptRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.ReceiptRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	return repository.NewReceiptRepository("default", apiClient, rc)
}

func seedReceiptCache(t *testing.T, db *cache.DB, entities []boardapi.ReceiptEntity) {
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
				Resource: "receipts",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedReceiptCache: %v", err)
		}
	}
}

var sampleReceipts = []boardapi.ReceiptEntity{
	{ID: 1, ClientID: 10, ProjectID: 100, Title: "ReceiptA", Status: "draft", UpdatedAt: "2026-01-01T00:00:00Z"},
	{ID: 2, ClientID: 10, ProjectID: 101, Title: "ReceiptB", Status: "issued", UpdatedAt: "2026-01-02T00:00:00Z"},
	{ID: 3, ClientID: 20, ProjectID: 102, Title: "ReceiptC", Status: "issued", UpdatedAt: "2026-01-03T00:00:00Z"},
}

// T_REC03: GetByDocumentID - cache hit -> returns from cache
func TestReceiptRepository_GetByDocumentID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedReceiptCache(t, db, sampleReceipts)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByDocumentID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_REC04: GetByDocumentID - cache miss, API success -> fetches from API and returns
func TestReceiptRepository_GetByDocumentID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)

	target := boardapi.ReceiptEntity{ID: 99, Title: "Test Receipt", UpdatedAt: "2026-01-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByDocumentID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_REC05: GetByDocumentID - cache miss, API error -> returns error
func TestReceiptRepository_GetByDocumentID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient)
	_, err := repo.GetByDocumentID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_REC06: GetByDocumentID - ForceRefresh bypasses cache
func TestReceiptRepository_GetByDocumentID_ForceRefresh(t *testing.T) {
	db := newTestDB(t)
	seedReceiptCache(t, db, sampleReceipts)

	updated := boardapi.ReceiptEntity{ID: 1, Title: "Updated Receipt", UpdatedAt: "2026-06-01T00:00:00Z"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(updated)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeReceiptRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 1, repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("GetByDocumentID ForceRefresh: %v", err)
	}
	if got == nil || got.Title != "Updated Receipt" {
		t.Errorf("expected updated title, got %+v", got)
	}
}
