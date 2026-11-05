/** repository のフィルタ判定(Go 版の `*FilterIsZero` / `cacheable*Filter` / `applyLimit`)。 */

function isZeroValue(v: unknown): boolean {
  if (v === undefined || v === null || v === "" || v === 0) return true;
  if (Array.isArray(v)) return v.length === 0;
  // 真偽値は false でも『指定した』扱い(Go のポインタ: nil でなければ非ゼロ)。
  return false;
}

/** すべての項目が未指定・0・空文字・空配列か。ゼロフィルタはキャッシュ全件の返却に回る。 */
export function isZeroFilter(filter: object): boolean {
  return Object.values(filter).every(isZeroValue);
}

/** 指定された(ゼロでない)項目がすべて `keys`(キャッシュ側で絞れる項目)に含まれるか。 */
export function isCacheableFilter(filter: object, keys: readonly string[]): boolean {
  return Object.entries(filter).every(([k, v]) => isZeroValue(v) || keys.includes(k));
}

/** limit > 0 なら先頭から limit 件に切り詰める。 */
export function applyLimit<T>(items: T[], limit: number | undefined): T[] {
  return limit !== undefined && limit > 0 && items.length > limit ? items.slice(0, limit) : items;
}

/**
 * limit で切った結果。切ったときだけ `totalBeforeLimit`(切る前の件数)を持つ。
 * 呼び出し側(ツール)が `limited: true` と `total_before_limit` を出し、LLM が『全件ではない』と分かるようにする。
 */
export function limitResult<T>(
  items: T[],
  limit: number | undefined,
): { items: T[]; totalBeforeLimit?: number } {
  const cut = applyLimit(items, limit);
  return cut.length < items.length ? { items: cut, totalBeforeLimit: items.length } : { items };
}
