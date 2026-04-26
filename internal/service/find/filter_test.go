package find

import (
	"testing"

	"github.com/youyo/board/internal/boardapi"
)

type statusItem struct {
	S string
}

func getS(it statusItem) string { return it.S }

// T04: 複数ステータスで正しく絞り込める
func TestFilterByStatuses_MatchesMultipleStatuses(t *testing.T) {
	items := []statusItem{{S: "a"}, {S: "b"}, {S: "c"}}
	got := filterByStatuses(items, getS, []string{"a", "c"})
	if len(got) != 2 {
		t.Fatalf("want 2 items, got %d", len(got))
	}
	if got[0].S != "a" || got[1].S != "c" {
		t.Fatalf("unexpected items: %v", got)
	}
}

// T05: statuses が空の場合は入力をそのまま返す（no-op）
func TestFilterByStatuses_EmptyStatuses_ReturnsOriginal(t *testing.T) {
	items := []statusItem{{S: "a"}, {S: "b"}}
	got := filterByStatuses(items, getS, nil)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
}

// T06: 一致なしの場合は空スライスを返す
func TestFilterByStatuses_NoMatch_ReturnsEmpty(t *testing.T) {
	items := []statusItem{{S: "a"}, {S: "b"}}
	got := filterByStatuses(items, getS, []string{"z"})
	if len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}
}

// T07: 実型（EstimateEntity）でジェネリクスが動作する
func TestFilterByStatuses_Generic_WithEstimateEntity(t *testing.T) {
	items := []boardapi.EstimateEntity{
		{SealApprovalStatus: 1},
		{SealApprovalStatus: 2},
		{SealApprovalStatus: 1},
	}
	// SealApprovalStatus は int なので string 変換が必要。
	// ここではラッパー関数でテスト。
	type wrappedEst struct {
		e boardapi.EstimateEntity
	}
	wrapped := []wrappedEst{{items[0]}, {items[1]}, {items[2]}}
	getStatus := func(w wrappedEst) string {
		if w.e.SealApprovalStatus == 1 {
			return "approved"
		}
		return "pending"
	}
	got := filterByStatuses(wrapped, getStatus, []string{"approved"})
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
}

// T13: statuses がちょうど 10 要素の場合は正常動作する（validation は types.go 側）
func TestFilterByStatuses_ExactlyTenStatuses(t *testing.T) {
	statuses := make([]string, 10)
	for i := range statuses {
		statuses[i] = "x"
	}
	items := []statusItem{{S: "x"}}
	got := filterByStatuses(items, getS, statuses)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}

// T14: statuses が 11 要素のケースは validateQuery で弾かれる（filterByStatuses 自体は通す）
// validation は FindXxxQuery.validate() で行い、filter 関数は純粋関数として上限検証しない。
func TestFilterByStatuses_ElevenStatuses_NotRejectedByFilter(t *testing.T) {
	statuses := make([]string, 11)
	for i := range statuses {
		statuses[i] = "x"
	}
	items := []statusItem{{S: "x"}}
	// filter 自体はエラーなしで動作する（validate は Query 側が担う）
	got := filterByStatuses(items, getS, statuses)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}
