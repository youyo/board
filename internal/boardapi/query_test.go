package boardapi_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/youyo/board/internal/boardapi"
)

// U1: Page(1, 100) → page=1&per_page=100
func TestQueryBuilder_Page(t *testing.T) {
	q := boardapi.NewQueryBuilder().Page(1, 100).Encode()
	if !strings.Contains(q, "page=1") {
		t.Errorf("want page=1, got %q", q)
	}
	if !strings.Contains(q, "per_page=100") {
		t.Errorf("want per_page=100, got %q", q)
	}
}

func TestQueryBuilder_PageZerosSkipped(t *testing.T) {
	q := boardapi.NewQueryBuilder().Page(0, 0).Encode()
	if q != "" {
		t.Errorf("zero page and per_page should be skipped, got %q", q)
	}
}

// U2: StrCont("name", "エス") → name_cont=エス (URL エスケープ込み)
func TestQueryBuilder_StrCont_JapaneseURLEncoded(t *testing.T) {
	q := boardapi.NewQueryBuilder().StrCont("name", "エス").Encode()
	// url.Values.Encode はキーと値を個別に QueryEscape する。
	// 「エス」は "%E3%82%A8%E3%82%B9" に UTF-8 で展開される。
	want := "name_cont=" + url.QueryEscape("エス")
	if q != want {
		t.Errorf("want %q, got %q", want, q)
	}
}

func TestQueryBuilder_StrContEmptyValueSkipped(t *testing.T) {
	q := boardapi.NewQueryBuilder().StrCont("name", "").Encode()
	if q != "" {
		t.Errorf("empty value should be skipped, got %q", q)
	}
}

// U3: IntIn("order_status", []int{1,2,3}) → order_status_in[]=1&order_status_in[]=2&order_status_in[]=3
func TestQueryBuilder_IntIn(t *testing.T) {
	q := boardapi.NewQueryBuilder().IntIn("order_status", []int{1, 2, 3}).Encode()
	// url.Values は key 順にソートし、同一キー内は追加順を保つ。
	// encoded key は "order_status_in%5B%5D"
	if !strings.Contains(q, "order_status_in%5B%5D=1") {
		t.Errorf("want order_status_in[]=1 (encoded), got %q", q)
	}
	if !strings.Contains(q, "order_status_in%5B%5D=2") {
		t.Errorf("want order_status_in[]=2 (encoded), got %q", q)
	}
	if !strings.Contains(q, "order_status_in%5B%5D=3") {
		t.Errorf("want order_status_in[]=3 (encoded), got %q", q)
	}
}

func TestQueryBuilder_IntInEmpty(t *testing.T) {
	q := boardapi.NewQueryBuilder().IntIn("order_status", nil).Encode()
	if q != "" {
		t.Errorf("nil slice should produce empty query, got %q", q)
	}
}

// U4: Flg01 with nil pointer → not sent
func TestQueryBuilder_Flg01_NilSkipped(t *testing.T) {
	q := boardapi.NewQueryBuilder().Flg01("include_archive_flg", nil).Encode()
	if q != "" {
		t.Errorf("nil pointer should be skipped, got %q", q)
	}
}

// U5: Flg01 with *true → 1, *false → 0
func TestQueryBuilder_Flg01_TrueFalse(t *testing.T) {
	yes := true
	qYes := boardapi.NewQueryBuilder().Flg01("include_archive_flg", &yes).Encode()
	if qYes != "include_archive_flg=1" {
		t.Errorf("true case: want include_archive_flg=1, got %q", qYes)
	}

	no := false
	qNo := boardapi.NewQueryBuilder().Flg01("include_archive_flg", &no).Encode()
	if qNo != "include_archive_flg=0" {
		t.Errorf("false case: want include_archive_flg=0, got %q", qNo)
	}
}

// U6: Tags([]string{"A","","B"}) → tags[]=A&tags[]=B (空文字は除外)
func TestQueryBuilder_Tags_EmptyValuesSkipped(t *testing.T) {
	q := boardapi.NewQueryBuilder().Tags([]string{"A", "", "B"}).Encode()
	if !strings.Contains(q, "tags%5B%5D=A") {
		t.Errorf("want tags[]=A, got %q", q)
	}
	if !strings.Contains(q, "tags%5B%5D=B") {
		t.Errorf("want tags[]=B, got %q", q)
	}
	// 空文字は含まれないこと (連続する = やセパレータ)
	if strings.Count(q, "tags%5B%5D=") != 2 {
		t.Errorf("want exactly 2 tags params, got %q", q)
	}
}

// IntEq, StrEq, DateGteq/Lteq, ResponseGroup, Set 等の追加カバレッジ
func TestQueryBuilder_IntEq(t *testing.T) {
	q := boardapi.NewQueryBuilder().IntEq("client_id", 42).Encode()
	if q != "client_id_eq=42" {
		t.Errorf("want client_id_eq=42, got %q", q)
	}
	// ゼロ値はスキップ
	q2 := boardapi.NewQueryBuilder().IntEq("client_id", 0).Encode()
	if q2 != "" {
		t.Errorf("zero int should be skipped, got %q", q2)
	}
}

func TestQueryBuilder_StrEq(t *testing.T) {
	q := boardapi.NewQueryBuilder().StrEq("custom_no", "AB-100").Encode()
	if q != "custom_no_eq=AB-100" {
		t.Errorf("want custom_no_eq=AB-100, got %q", q)
	}
}

func TestQueryBuilder_DateGteqLteq(t *testing.T) {
	q := boardapi.NewQueryBuilder().
		DateGteq("updated_at", "2026-04-01").
		DateLteq("updated_at", "2026-04-23").
		Encode()
	if !strings.Contains(q, "updated_at_gteq=2026-04-01") {
		t.Errorf("want updated_at_gteq=2026-04-01, got %q", q)
	}
	if !strings.Contains(q, "updated_at_lteq=2026-04-23") {
		t.Errorf("want updated_at_lteq=2026-04-23, got %q", q)
	}
}

func TestQueryBuilder_ResponseGroup(t *testing.T) {
	q := boardapi.NewQueryBuilder().ResponseGroup("large").Encode()
	if q != "response_group=large" {
		t.Errorf("want response_group=large, got %q", q)
	}
	// 空文字はスキップ
	q2 := boardapi.NewQueryBuilder().ResponseGroup("").Encode()
	if q2 != "" {
		t.Errorf("empty response_group should be skipped, got %q", q2)
	}
}

func TestQueryBuilder_SetEscapeHatch(t *testing.T) {
	q := boardapi.NewQueryBuilder().Set("custom_key", "custom_val").Encode()
	if q != "custom_key=custom_val" {
		t.Errorf("want custom_key=custom_val, got %q", q)
	}
}

func TestQueryBuilder_StrInSkipsEmpty(t *testing.T) {
	q := boardapi.NewQueryBuilder().StrIn("tag", []string{"x", "", "y"}).Encode()
	if !strings.Contains(q, "tag_in%5B%5D=x") {
		t.Errorf("want tag_in[]=x, got %q", q)
	}
	if !strings.Contains(q, "tag_in%5B%5D=y") {
		t.Errorf("want tag_in[]=y, got %q", q)
	}
	if strings.Count(q, "tag_in%5B%5D=") != 2 {
		t.Errorf("want exactly 2 values, got %q", q)
	}
}

func TestQueryBuilder_Chain_Complex(t *testing.T) {
	yes := true
	q := boardapi.NewQueryBuilder().
		Page(2, 50).
		StrCont("name", "foo").
		IntIn("order_status", []int{1, 2}).
		Flg01("include_archive_flg", &yes).
		ResponseGroup("large").
		Encode()
	// 順序は url.Values のキーソート順 (alphabetical)
	// 期待キー: include_archive_flg, name_cont, order_status_in%5B%5D, page, per_page, response_group
	for _, fragment := range []string{
		"include_archive_flg=1",
		"name_cont=foo",
		"order_status_in%5B%5D=1",
		"order_status_in%5B%5D=2",
		"page=2",
		"per_page=50",
		"response_group=large",
	} {
		if !strings.Contains(q, fragment) {
			t.Errorf("want fragment %q in %q", fragment, q)
		}
	}
}
