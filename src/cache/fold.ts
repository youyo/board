/**
 * BOARD API の `name_cont`(Ransack)に近づけた名前の正規化(Go 版 `internal/cache/fold` の移植)。
 *
 * 実 API の挙動(Go 版の実測): ASCII の大文字小文字を区別しない / 半角・全角を統一する(NFKC) /
 * 前後の空白を除く / 漢字とカナの fold はしない / それ以外は文字どおりの部分一致。
 */

/** trim → NFKC → 小文字(Go の `strings.ToLower(norm.NFKC.String(strings.TrimSpace(s)))`)。 */
export function nameFold(s: string): string {
  return s.trim().normalize("NFKC").toLowerCase();
}

/** haystack が needle を(fold 後の)部分文字列として含むか。needle が空(空白のみ)なら true。 */
export function foldContains(haystack: string | null | undefined, needle: string): boolean {
  const n = nameFold(needle);
  if (n === "") return true;
  return nameFold(haystack ?? "").includes(n);
}
