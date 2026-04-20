package testhelper_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/youyo/board/internal/testhelper"
)

// Test #1: raw JSON のキーが struct の json タグ集合に無ければ列挙される。
func TestStrictFieldDiff_UnmappedTopLevel(t *testing.T) {
	type S struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	raw := []byte(`{"a":1,"b":2,"c":3}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	want := []string{"c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StrictFieldDiff = %v, want %v", got, want)
	}
}

// Test #2: 完全一致は空スライス。
func TestStrictFieldDiff_ExactMatch(t *testing.T) {
	type S struct {
		A int `json:"a"`
	}
	raw := []byte(`{"a":1}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	if len(got) != 0 {
		t.Fatalf("StrictFieldDiff = %v, want empty", got)
	}
}

// Test #3: ネスト struct を再帰する。
func TestStrictFieldDiff_NestedStruct(t *testing.T) {
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		A Inner `json:"a"`
	}
	raw := []byte(`{"a":{"x":1,"y":2}}`)

	got := testhelper.StrictFieldDiff(t, raw, &Outer{})
	want := []string{"a.y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StrictFieldDiff = %v, want %v", got, want)
	}
}

// Test #4: `json:"-"` のフィールドは json キーとしてマッピングされない。
// raw に同名キーが存在しても unmapped 扱いとなる（struct 側で受け取れないため）。
func TestStrictFieldDiff_JSONDashIgnored(t *testing.T) {
	type S struct {
		Ignored string `json:"-"`
		A       int    `json:"a"`
	}
	raw := []byte(`{"a":1,"Ignored":"x"}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	want := []string{"Ignored"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StrictFieldDiff = %v, want %v", got, want)
	}
}

// Test #5: omitempty などのオプションは剥がして名前部分のみで突合する。
func TestStrictFieldDiff_JSONTagOptions(t *testing.T) {
	type S struct {
		Foo int `json:"foo,omitempty"`
	}
	raw := []byte(`{"foo":1}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	if len(got) != 0 {
		t.Fatalf("StrictFieldDiff = %v, want empty (omitempty should be stripped)", got)
	}
}

// Test #6: 配列要素の各要素を再帰、配列要素のパスは `[].foo` 形式で重複排除する。
func TestStrictFieldDiff_ArrayElement(t *testing.T) {
	type Elem struct {
		A int `json:"a"`
	}
	raw := []byte(`[{"a":1,"b":2},{"a":1,"b":3}]`)

	got := testhelper.StrictFieldDiff(t, raw, &[]Elem{})
	want := []string{"[].b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StrictFieldDiff = %v, want %v", got, want)
	}
}

// Test #7: nil ポインタでも panic せず、struct 側が pointer-to-struct で受ける場合は辿れる。
func TestStrictFieldDiff_PointerField(t *testing.T) {
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		A *Inner `json:"a"`
	}
	raw := []byte(`{"a":{"x":1}}`)

	got := testhelper.StrictFieldDiff(t, raw, &Outer{})
	if len(got) != 0 {
		t.Fatalf("StrictFieldDiff = %v, want empty", got)
	}
}

// Test #7b: 生 JSON に key が無くても問題ない（欠けているのは unmapped の逆方向）。
func TestStrictFieldDiff_MissingInRaw(t *testing.T) {
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		A *Inner `json:"a"`
		B *Inner `json:"b"`
	}
	raw := []byte(`{"a":{"x":1}}`)

	got := testhelper.StrictFieldDiff(t, raw, &Outer{})
	if len(got) != 0 {
		t.Fatalf("StrictFieldDiff = %v, want empty (raw に無い struct フィールドは検知しない)", got)
	}
}

// Test #8: map 型フィールドの値は任意キーなので子要素を突合しない。
func TestStrictFieldDiff_MapField(t *testing.T) {
	type S struct {
		M map[string]int `json:"m"`
	}
	raw := []byte(`{"m":{"k1":1,"k2":2,"k3":3}}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	if len(got) != 0 {
		t.Fatalf("StrictFieldDiff = %v, want empty (map values must be skipped)", got)
	}
}

// Test #9: 複数の未マップキーはソート済みで返す。
func TestStrictFieldDiff_MultipleUnmappedSorted(t *testing.T) {
	type S struct {
		A int `json:"a"`
	}
	raw := []byte(`{"a":1,"zz":9,"m":2,"b":3}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	want := []string{"b", "m", "zz"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StrictFieldDiff = %v, want %v", got, want)
	}
	// ソートされていることを明示（実装保証）。
	if !sort.StringsAreSorted(got) {
		t.Fatalf("result not sorted: %v", got)
	}
}

// Test #10: 空オブジェクトは空スライス。
func TestStrictFieldDiff_EmptyRaw(t *testing.T) {
	type S struct {
		A int `json:"a"`
	}
	raw := []byte(`{}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	if len(got) != 0 {
		t.Fatalf("StrictFieldDiff = %v, want empty", got)
	}
}

// Test #11: embedded struct のタグは親の名前空間に merge される（encoding/json と同じ挙動）。
func TestStrictFieldDiff_EmbeddedStruct(t *testing.T) {
	type Base struct {
		ID int `json:"id"`
	}
	type Child struct {
		Base
		Name string `json:"name"`
	}
	raw := []byte(`{"id":1,"name":"foo","extra":"x"}`)

	got := testhelper.StrictFieldDiff(t, raw, &Child{})
	want := []string{"extra"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StrictFieldDiff = %v, want %v", got, want)
	}
}

// Test #12: interface{} 型フィールドの子要素は再帰しない（動的型のため）。
func TestStrictFieldDiff_InterfaceField(t *testing.T) {
	type S struct {
		Any any `json:"any"`
	}
	raw := []byte(`{"any":{"a":1,"b":2}}`)

	got := testhelper.StrictFieldDiff(t, raw, &S{})
	if len(got) != 0 {
		t.Fatalf("StrictFieldDiff = %v, want empty (any/interface should not recurse)", got)
	}
}
