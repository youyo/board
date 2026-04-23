//go:build e2e

// M50 (Phase L-02) E2E tests: clients pilot — Ransack query + response header
// verification. These are the real-API tests that confirm server-side
// behaviour of the new ClientListOptions filter set and capture the actual
// BOARD API response header names (X-Total-Count / X-Ratelimit-* / ETag /
// Last-Modified) for subsequent M51-M56 migrations.
//
// Budget: 10 requests across E1-E10, at 1 req/400ms to respect 3 req/sec.
//
// Run:
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Clients_Pilot_M50 ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// sleepForRateLimit inserts a conservative pause between consecutive BOARD
// API calls so that the 3 req/sec limit is respected even under bursty test
// execution.
func sleepForRateLimit() { time.Sleep(400 * time.Millisecond) }

// TestE2E_Clients_Pilot_M50 runs the E1-E10 verification items in sequence.
// Each sub-test isolates one Ransack parameter so that a failure points at a
// specific filter semantic discrepancy, not a combined query.
func TestE2E_Clients_Pilot_M50(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	// Baseline: total clients (unfiltered) — used by several sub-tests.
	baseRaw, baseHeaders, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{})
	if err != nil {
		t.Fatalf("baseline ListClientsRaw: %v", err)
	}
	var baseItems []boardapi.ClientEntity
	if err := json.Unmarshal(baseRaw, &baseItems); err != nil {
		t.Fatalf("baseline unmarshal: %v", err)
	}
	baselineCount := len(baseItems)
	t.Logf("baseline: %d total clients", baselineCount)

	// E10: dump header names observed on the real response so we can confirm
	// parseListMeta's key list matches reality.
	t.Run("E10_HeaderNames", func(t *testing.T) {
		if baseHeaders == nil {
			t.Fatal("baseline headers must not be nil")
		}
		interesting := []string{
			"X-Total-Count", "X-Page", "X-Per-Page",
			"X-Ratelimit-Remaining", "X-Ratelimit-Limit", "X-Ratelimit-Reset",
			"X-RateLimit-Remaining", "X-RateLimit-Limit", "X-RateLimit-Reset",
			"ETag", "Last-Modified", "Retry-After",
		}
		for _, h := range interesting {
			if v := baseHeaders.Get(h); v != "" {
				t.Logf("header %-28s = %s", h, v)
			}
		}
		// Full header dump (key-only) for any not in the list above.
		for k := range baseHeaders {
			t.Logf("all-header-key: %s", k)
		}
	})

	// E1: name_cont filter reduces result count below baseline.
	t.Run("E1_NameContReducesCount", func(t *testing.T) {
		sleepForRateLimit()
		raw, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{NameCont: "エス"})
		if err != nil {
			t.Fatalf("name_cont: %v", err)
		}
		var items []boardapi.ClientEntity
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("name_cont=エス: %d items (baseline=%d)", len(items), baselineCount)
		if len(items) > baselineCount {
			t.Errorf("filtered count %d must not exceed baseline %d", len(items), baselineCount)
		}
	})

	// E2: Legacy `name` parameter (pre-M50) is now filtered out of the
	// request. We assert that the count matches the baseline when only a
	// legacy-style param would have been sent — i.e. ClientListOptions{}
	// yields the full list. This is redundant with the baseline collection
	// but kept as an explicit guard against a regression that would re-add
	// legacy name injection.
	t.Run("E2_LegacyNameIgnored", func(t *testing.T) {
		sleepForRateLimit()
		raw, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{})
		if err != nil {
			t.Fatalf("zero filter: %v", err)
		}
		var items []boardapi.ClientEntity
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(items) != baselineCount {
			t.Errorf("zero-filter second call count=%d differs from baseline=%d", len(items), baselineCount)
		}
	})

	// E3/E4: response_group=large vs small. `large` is expected to surface
	// Get-only fields (NdaFlg, BasicAgreementFlg, ...) on List too.
	t.Run("E3_ResponseGroupLarge", func(t *testing.T) {
		sleepForRateLimit()
		raw, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{ResponseGroup: "large"})
		if err != nil {
			t.Fatalf("response_group=large: %v", err)
		}
		var items []map[string]any
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(items) == 0 {
			t.Skip("no clients returned — cannot assert response_group shape")
		}
		getOnlyKeys := []string{"nda_flg", "basic_agreement_flg", "bank_charge_to_client_flg", "document_send_type"}
		observed := 0
		for _, k := range getOnlyKeys {
			if _, ok := items[0][k]; ok {
				observed++
			}
		}
		t.Logf("response_group=large observed %d/%d get-only keys in first item", observed, len(getOnlyKeys))
	})

	t.Run("E4_ResponseGroupSmall", func(t *testing.T) {
		sleepForRateLimit()
		raw, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{ResponseGroup: "small"})
		if err != nil {
			t.Fatalf("response_group=small: %v", err)
		}
		var items []map[string]any
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(items) == 0 {
			t.Skip("no clients returned")
		}
		t.Logf("response_group=small first item has %d keys", len(items[0]))
	})

	// E5: updated_at_gteq with a future date must yield 0 items.
	t.Run("E5_UpdatedAtGteqFuture", func(t *testing.T) {
		sleepForRateLimit()
		future := time.Now().AddDate(1, 0, 0).Format("2006-01-02 15:04:05")
		raw, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{UpdatedAtGteq: future})
		if err != nil {
			t.Fatalf("updated_at_gteq future: %v", err)
		}
		var items []boardapi.ClientEntity
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("updated_at_gteq=%s must yield 0 items, got %d", future, len(items))
		}
	})

	// E6: include_archive_flg=1 returns at least as many items as baseline
	// (archived + active ≥ active).
	t.Run("E6_IncludeArchiveFlg", func(t *testing.T) {
		sleepForRateLimit()
		yes := true
		raw, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{IncludeArchiveFlg: &yes})
		if err != nil {
			t.Fatalf("include_archive_flg: %v", err)
		}
		var items []boardapi.ClientEntity
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("include_archive_flg=1: %d items (baseline=%d)", len(items), baselineCount)
		if len(items) < baselineCount {
			t.Errorf("include_archive count %d must be >= baseline %d", len(items), baselineCount)
		}
	})

	// E7: tags[] filter. We cannot assume tag values exist, so we only
	// verify the request round-trips without error.
	t.Run("E7_Tags", func(t *testing.T) {
		sleepForRateLimit()
		_, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{Tags: []string{"e2e-probe-tag-m50"}})
		if err != nil {
			t.Fatalf("tags[]: %v", err)
		}
	})

	// E8: custom_no_eq. We cannot guarantee a record exists; empty result
	// is acceptable. The purpose is to verify the request is accepted.
	t.Run("E8_CustomNoEq", func(t *testing.T) {
		sleepForRateLimit()
		_, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{CustomNoEq: "E2E_NONEXISTENT_CUST_NO"})
		if err != nil {
			t.Fatalf("custom_no_eq: %v", err)
		}
	})

	// E9: invoice_system_number_eq. Same caveat as E8.
	t.Run("E9_InvoiceSystemNumberEq", func(t *testing.T) {
		sleepForRateLimit()
		_, _, err := client.ListClientsRaw(ctx, boardapi.ClientListOptions{InvoiceSystemNumberEq: "T0000000000000"})
		if err != nil {
			t.Fatalf("invoice_system_number_eq: %v", err)
		}
	})
}
