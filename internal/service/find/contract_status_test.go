package find

import (
	"strings"
	"testing"

	"github.com/youyo/board/internal/boardapi"
)

// --- C-T01〜C-T08: expandContractStatus ユニットテスト ---

// C-T01: "active" は DeliveryStatusName = {未着手, 着手中, 納品済}
func TestExpandContractStatus_Active(t *testing.T) {
	matchDelivery, matchOrder, err := expandContractStatus("active")
	assertNoError(t, err)
	if len(matchOrder) != 0 {
		t.Errorf("expected matchOrder=nil, got %v", matchOrder)
	}
	want := []string{"未着手", "着手中", "納品済"}
	if len(matchDelivery) != len(want) {
		t.Fatalf("expected matchDelivery=%v, got %v", want, matchDelivery)
	}
	wantSet := setFromSlice(want)
	for _, s := range matchDelivery {
		if _, ok := wantSet[s]; !ok {
			t.Errorf("unexpected matchDelivery item: %q", s)
		}
	}
}

// C-T02: "ended" は DeliveryStatusName = {検収済}
func TestExpandContractStatus_Ended(t *testing.T) {
	matchDelivery, matchOrder, err := expandContractStatus("ended")
	assertNoError(t, err)
	if len(matchOrder) != 0 {
		t.Errorf("expected matchOrder=nil, got %v", matchOrder)
	}
	if len(matchDelivery) != 1 || matchDelivery[0] != "検収済" {
		t.Errorf("expected matchDelivery=[検収済], got %v", matchDelivery)
	}
}

// C-T03: "prospect" は OrderStatusName = {見積中(高), 見積中(中), 見積中(低), 見積中(除)}
func TestExpandContractStatus_Prospect(t *testing.T) {
	matchDelivery, matchOrder, err := expandContractStatus("prospect")
	assertNoError(t, err)
	if len(matchDelivery) != 0 {
		t.Errorf("expected matchDelivery=nil, got %v", matchDelivery)
	}
	want := []string{"見積中(高)", "見積中(中)", "見積中(低)", "見積中(除)"}
	if len(matchOrder) != len(want) {
		t.Fatalf("expected matchOrder=%v, got %v", want, matchOrder)
	}
	wantSet := setFromSlice(want)
	for _, s := range matchOrder {
		if _, ok := wantSet[s]; !ok {
			t.Errorf("unexpected matchOrder item: %q", s)
		}
	}
}

// C-T04: "all" は active∪ended の DeliveryStatusName + prospect の OrderStatusName
func TestExpandContractStatus_All(t *testing.T) {
	matchDelivery, matchOrder, err := expandContractStatus("all")
	assertNoError(t, err)
	// matchDelivery は active + ended = {未着手, 着手中, 納品済, 検収済} の 4 件
	wantDelivery := []string{"未着手", "着手中", "納品済", "検収済"}
	if len(matchDelivery) != len(wantDelivery) {
		t.Fatalf("expected matchDelivery len=%d, got %d: %v", len(wantDelivery), len(matchDelivery), matchDelivery)
	}
	dSet := setFromSlice(matchDelivery)
	for _, s := range wantDelivery {
		if _, ok := dSet[s]; !ok {
			t.Errorf("expected matchDelivery to contain %q", s)
		}
	}
	// matchOrder は prospect = {見積中(高), 見積中(中), 見積中(低), 見積中(除)} の 4 件
	wantOrder := []string{"見積中(高)", "見積中(中)", "見積中(低)", "見積中(除)"}
	if len(matchOrder) != len(wantOrder) {
		t.Fatalf("expected matchOrder len=%d, got %d: %v", len(wantOrder), len(matchOrder), matchOrder)
	}
	oSet := setFromSlice(matchOrder)
	for _, s := range wantOrder {
		if _, ok := oSet[s]; !ok {
			t.Errorf("expected matchOrder to contain %q", s)
		}
	}
}

// C-T05: "ACTIVE" は case-insensitive で active と同等
func TestExpandContractStatus_CaseInsensitive(t *testing.T) {
	matchDelivery, matchOrder, err := expandContractStatus("ACTIVE")
	assertNoError(t, err)
	if len(matchOrder) != 0 {
		t.Errorf("expected matchOrder=nil, got %v", matchOrder)
	}
	if len(matchDelivery) != 3 {
		t.Fatalf("expected 3 matchDelivery items, got %d: %v", len(matchDelivery), matchDelivery)
	}
}

// C-T06: " active " は trim して active と同等
func TestExpandContractStatus_Trim(t *testing.T) {
	matchDelivery, matchOrder, err := expandContractStatus(" active ")
	assertNoError(t, err)
	if len(matchOrder) != 0 {
		t.Errorf("expected matchOrder=nil, got %v", matchOrder)
	}
	if len(matchDelivery) != 3 {
		t.Fatalf("expected 3 matchDelivery items, got %d: %v", len(matchDelivery), matchDelivery)
	}
}

// C-T07: "unknown" は error (valid 値列挙付き)
func TestExpandContractStatus_Unknown_Error(t *testing.T) {
	_, _, err := expandContractStatus("unknown")
	assertError(t, err)
	if !strings.Contains(err.Error(), "unknown contract_status") {
		t.Errorf("expected 'unknown contract_status' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "valid:") {
		t.Errorf("expected 'valid:' in error, got: %v", err)
	}
}

// C-T08: "" は no-op、error なし
func TestExpandContractStatus_Empty_NoError(t *testing.T) {
	matchDelivery, matchOrder, err := expandContractStatus("")
	assertNoError(t, err)
	if len(matchDelivery) != 0 {
		t.Errorf("expected matchDelivery=nil, got %v", matchDelivery)
	}
	if len(matchOrder) != 0 {
		t.Errorf("expected matchOrder=nil, got %v", matchOrder)
	}
}

// --- C-T09〜C-T13: filterProjectsByContractStatus テスト ---

// C-T09: "active" は DeliveryStatusName が active の案件のみ返す
func TestFilterProjectsByContractStatus_Active(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 1, Name: "P1", DeliveryStatusName: "未着手"}, // active
		{ID: 2, Name: "P2", DeliveryStatusName: "検収済"}, // ended
		{ID: 3, Name: "P3", OrderStatusName: "見積中(中)"}, // prospect
	}
	got, err := filterProjectsByContractStatus(projects, "active")
	assertNoError(t, err)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].ID != 1 {
		t.Errorf("expected ID=1, got %d", got[0].ID)
	}
}

// C-T10: "ended" は DeliveryStatusName が検収済の案件のみ返す
func TestFilterProjectsByContractStatus_Ended(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 1, Name: "P1", DeliveryStatusName: "未着手"},
		{ID: 2, Name: "P2", DeliveryStatusName: "検収済"},
	}
	got, err := filterProjectsByContractStatus(projects, "ended")
	assertNoError(t, err)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].ID != 2 {
		t.Errorf("expected ID=2, got %d", got[0].ID)
	}
}

// C-T11: "prospect" は OrderStatusName が見積中(中)の案件のみ返す
func TestFilterProjectsByContractStatus_Prospect(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 1, Name: "P1", OrderStatusName: "見積中(中)"},
		{ID: 2, Name: "P2", OrderStatusName: "受注確定"},
	}
	got, err := filterProjectsByContractStatus(projects, "prospect")
	assertNoError(t, err)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].ID != 1 {
		t.Errorf("expected ID=1, got %d", got[0].ID)
	}
}

// C-T12: "all" は active∪ended∪prospect の全案件を返す
func TestFilterProjectsByContractStatus_All(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 1, Name: "P1", DeliveryStatusName: "未着手"}, // active
		{ID: 2, Name: "P2", DeliveryStatusName: "着手中"}, // active
		{ID: 3, Name: "P3", DeliveryStatusName: "検収済"}, // ended
		{ID: 4, Name: "P4", OrderStatusName: "見積中(高)"}, // prospect
		{ID: 5, Name: "P5", OrderStatusName: "受注済"},    // none of above
	}
	got, err := filterProjectsByContractStatus(projects, "all")
	assertNoError(t, err)
	if len(got) != 4 {
		t.Fatalf("expected 4 results, got %d: %v", len(got), got)
	}
}

// C-T13: "active" で空配列は空配列を返す
func TestFilterProjectsByContractStatus_EmptyInput(t *testing.T) {
	got, err := filterProjectsByContractStatus(nil, "active")
	assertNoError(t, err)
	if len(got) != 0 {
		t.Fatalf("expected 0 results, got %d: %v", len(got), got)
	}
}
