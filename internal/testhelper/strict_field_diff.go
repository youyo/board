// Package testhelper は BOARD 準拠検証用の E2E 共通ヘルパーを提供する。
//
// StrictFieldDiff は生 JSON のキー集合と Go struct の json タグ集合を突合し、
// 未マップキー（API には存在するが Go 側で受け取れないフィールド）を列挙する。
// 直近のリリースで UserEntity / ContactEntity / VendorContactEntity に複数の
// フィールド漏れが発覚したため、後続 E2E での早期検知を目的とする。
package testhelper

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// StrictFieldDiff は raw JSON のキー集合と target の json タグ集合を突合し、
// raw にあって struct にない未マップキーをドット記法で返す。
//
// 挙動:
//   - 返り値は常にソート済み。
//   - ネスト struct / 配列は再帰で辿る。配列要素のパスは `[].field` に集約する。
//   - map 型フィールドおよび interface / any 型フィールドの子要素は
//     動的キーのため再帰せずスキップする。
//   - encoding/json と同じルールで embedded (anonymous) struct を merge する。
//   - `json:"-"` の付いたフィールドは struct 側のキーとして採用しないため、
//     raw 側に同名キーがあれば unmapped として検知される。
//   - struct 側にあるが raw に無いキーは検知しない（API 側が optional のため）。
//
// エラー時（raw が JSON でない等）は t.Fatalf でテストを失敗させる。
//
// 使用例:
//
//	diff := testhelper.StrictFieldDiff(t, rawJSON, &boardapi.ClientEntity{})
//	if len(diff) > 0 {
//	    t.Errorf("unmapped fields: %v", diff)
//	}
func StrictFieldDiff(t *testing.T, raw []byte, target any) []string {
	t.Helper()

	targetType := reflect.TypeOf(target)
	if targetType == nil {
		t.Fatalf("StrictFieldDiff: target is nil")
	}

	var rawValue any
	if err := json.Unmarshal(raw, &rawValue); err != nil {
		t.Fatalf("StrictFieldDiff: raw JSON unmarshal: %v", err)
	}

	set := map[string]struct{}{}
	diffValue(rawValue, targetType, "", set)

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// diffValue は raw 値と対応する Go 型を比較し、未マップキーを out に追記する。
// rawValue は encoding/json が生成する汎用構造（map[string]any / []any / primitive）を想定。
func diffValue(rawValue any, targetType reflect.Type, path string, out map[string]struct{}) {
	// ポインタは辿る。
	for targetType != nil && targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType == nil {
		return
	}

	switch rv := rawValue.(type) {
	case map[string]any:
		diffObject(rv, targetType, path, out)
	case []any:
		diffArray(rv, targetType, path, out)
	default:
		// primitive (string / number / bool / nil) は突合対象のキーを持たないので終了。
		return
	}
}

// diffObject は JSON object を struct フィールド（または map / interface）と突合する。
func diffObject(rawObj map[string]any, targetType reflect.Type, path string, out map[string]struct{}) {
	switch targetType.Kind() {
	case reflect.Struct:
		fieldMap := structJSONFields(targetType)
		for key, value := range rawObj {
			field, ok := fieldMap[key]
			if !ok {
				out[joinPath(path, key)] = struct{}{}
				continue
			}
			// 子要素を再帰的に検査する。
			diffValue(value, field.Type, joinPath(path, key), out)
		}
	case reflect.Map:
		// map 型は任意キーを許容するため、子要素は再帰しない（設計判断）。
		return
	case reflect.Interface:
		// interface{} / any は動的型のため再帰不能。
		return
	default:
		// object を受け取れない型に object が来た場合は型不整合だが、
		// このヘルパーの責務は「未マップフィールド検知」に限るため静かに無視する。
		return
	}
}

// diffArray は JSON 配列の各要素を slice/array の要素型と突合する。
// パスは `[].field` に集約する（同じキーの重複を排除するため）。
func diffArray(rawArr []any, targetType reflect.Type, path string, out map[string]struct{}) {
	// slice / array / pointer-to-slice を剥がす。
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}

	var elemType reflect.Type
	switch targetType.Kind() {
	case reflect.Slice, reflect.Array:
		elemType = targetType.Elem()
	case reflect.Interface:
		return
	default:
		// 配列を受け取れない型には対応しない。
		return
	}

	childPath := joinPath(path, "[]")
	for _, elem := range rawArr {
		diffValue(elem, elemType, childPath, out)
	}
}

// jsonFieldInfo は struct フィールドの json マッピング情報を保持する。
type jsonFieldInfo struct {
	Type reflect.Type
}

// structJSONFields は struct 型の json タグを {jsonKey -> fieldInfo} の map にする。
// encoding/json と同様に embedded (anonymous) struct のフィールドを親の名前空間に merge する。
// `json:"-"` は除外、`json:"name,omitempty"` 等のオプションは剥がす。
func structJSONFields(t reflect.Type) map[string]jsonFieldInfo {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	out := map[string]jsonFieldInfo{}
	collectStructFields(t, out)
	return out
}

// collectStructFields は embedded struct を辿りながらタグ集合を構築する。
// encoding/json の merge 順に合わせ、親側で既に同名キーがある場合は上書きしない
// （encoding/json は depth の浅いフィールドを優先するため、先に入れた方を優先する）。
func collectStructFields(t reflect.Type, out map[string]jsonFieldInfo) {
	// まず自身の non-anonymous フィールドを登録し、その後 embedded を再帰する。
	// これで「親の直接フィールドが embedded を shadow する」という encoding/json の規約を満たす。
	var embedded []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		if f.Anonymous {
			// embedded は後で処理する。
			embedded = append(embedded, f)
			continue
		}
		name, ok := jsonFieldName(f)
		if !ok {
			continue
		}
		if _, exists := out[name]; exists {
			continue
		}
		out[name] = jsonFieldInfo{Type: f.Type}
	}
	for _, f := range embedded {
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			// embedded struct に明示的な json タグが付いている場合は
			// 子の個別フィールドではなく struct 全体が 1 キーに入る。
			if name, ok := jsonFieldName(f); ok && f.Tag.Get("json") != "" {
				if _, exists := out[name]; !exists {
					out[name] = jsonFieldInfo{Type: f.Type}
				}
				continue
			}
			collectStructFields(ft, out)
		}
	}
}

// jsonFieldName はフィールドの json タグから「突合に使う名前」を取り出す。
// 戻り値の第二値が false のときは「このフィールドは json の対象外」を意味する。
func jsonFieldName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if tag == "" {
		// タグ無しは encoding/json と同じくフィールド名をそのまま使う。
		return f.Name, true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		// `json:",omitempty"` のように名前省略された場合はフィールド名を使う。
		return f.Name, true
	}
	return name, true
}

// joinPath はドット区切りのパスを結合する。空要素は除外する。
func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}
