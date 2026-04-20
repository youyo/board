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

func makeEstimateRepo(t *testing.T, db *cache.DB, apiClient *boardapi.Client) *repository.EstimateRepository {
	t.Helper()
	rc := cache.NewResourceCache(db)
	return repository.NewEstimateRepository("default", apiClient, rc)
}

func seedEstimateCache(t *testing.T, db *cache.DB, entities []boardapi.EstimateEntity) {
	t.Helper()
	rc := cache.NewResourceCache(db)
	ctx := context.Background()
	for _, e := range entities {
		raw, _ := json.Marshal(e)
		// M35: EstimateEntity は updated_at を持たないため固定値を使用する。
		updatedAt := sql.NullString{Valid: true, String: "2026-01-01T00:00:00Z"}
		err := rc.Upsert(ctx, cache.Entry{
			Key: cache.EntityKey{
				Profile:  "default",
				Resource: "estimates",
				EntityID: fmt.Sprintf("%d", e.ID),
			},
			PayloadJSON: raw,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			t.Fatalf("seedEstimateCache: %v", err)
		}
	}
}

var sampleEstimates = []boardapi.EstimateEntity{
	{ID: 1, Total: "50000.0", Tax: "5000.0", TaxWithholding: "0.0", ValidPeriod: "御見積後１ヶ月"},
	{ID: 2, Total: "80000.0", Tax: "8000.0", TaxWithholding: "0.0", ValidPeriod: "御見積後２ヶ月"},
	{ID: 3, Total: "120000.0", Tax: "12000.0", TaxWithholding: "0.0", ValidPeriod: "御見積後１ヶ月"},
}

// T_EST03: GetByDocumentID - cache hit -> returns from cache
func TestEstimateRepository_GetByDocumentID_CacheHit(t *testing.T) {
	db := newTestDB(t)
	seedEstimateCache(t, db, sampleEstimates)

	// API server that returns error to confirm cache is used
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeEstimateRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 1, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByDocumentID: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("expected ID=1, got %+v", got)
	}
}

// T_EST04: GetByDocumentID - cache miss, API success -> fetches from API and returns
func TestEstimateRepository_GetByDocumentID_CacheMiss_APISuccess(t *testing.T) {
	db := newTestDB(t)

	target := boardapi.EstimateEntity{ID: 99, Total: "50000.0", ValidPeriod: "御見積後１ヶ月"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(target)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)

	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeEstimateRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 99, repository.ReadOptions{})
	if err != nil {
		t.Fatalf("GetByDocumentID: %v", err)
	}
	if got == nil || got.ID != 99 {
		t.Errorf("expected ID=99, got %+v", got)
	}
}

// T_EST05: GetByDocumentID - cache miss, API error -> returns error
func TestEstimateRepository_GetByDocumentID_CacheMiss_APIError(t *testing.T) {
	db := newTestDB(t)

	srv := newErrorAPIServer(t)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeEstimateRepo(t, db, apiClient)
	_, err := repo.GetByDocumentID(context.Background(), 999, repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T_EST06: GetByDocumentID - ForceRefresh bypasses cache
func TestEstimateRepository_GetByDocumentID_ForceRefresh(t *testing.T) {
	db := newTestDB(t)
	// Seed cache with stale data
	seedEstimateCache(t, db, sampleEstimates)

	// API returns updated entity
	updated := boardapi.EstimateEntity{ID: 1, Total: "999999.0", ValidPeriod: "御見積後３ヶ月"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(updated)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	apiClient := boardapi.New(srv.URL, "key", "token", 5*time.Second, boardapi.WithRetryMax(0))

	repo := makeEstimateRepo(t, db, apiClient)
	got, err := repo.GetByDocumentID(context.Background(), 1, repository.ReadOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("GetByDocumentID ForceRefresh: %v", err)
	}
	if got == nil || got.Total != "999999.0" {
		t.Errorf("expected updated total, got %+v", got)
	}
}
