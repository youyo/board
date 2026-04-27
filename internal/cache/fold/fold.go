// Package fold provides string normalization that approximates BOARD API's
// `name_cont` (Ransack) match semantics for cache-side filtering.
//
// 実 API の挙動（実測 2026-04）:
//   - ASCII 大文字小文字を区別しない
//   - 半角/全角 ASCII を統一する (NFKC)
//   - 前後空白を除去する
//   - 漢字↔カナ fold は行わない
//   - その他は literal substring match
//
// このパッケージは上記の前 4 項を Go で再現する。
// 実 API との挙動差は drift test (e2e tag) で監視する。
package fold

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// NameFold は BOARD API の name_cont で使われる正規化を返す。
// NFKC normalize + lower-case + 前後 trim。
func NameFold(s string) string {
	return strings.ToLower(norm.NFKC.String(strings.TrimSpace(s)))
}

// Contains は haystack に needle が（fold 後の）部分文字列として含まれるかを返す。
// needle が空文字（または空白のみ）なら true（フィルタ無指定相当）。
func Contains(haystack, needle string) bool {
	n := NameFold(needle)
	if n == "" {
		return true
	}
	return strings.Contains(NameFold(haystack), n)
}
