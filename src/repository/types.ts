import type { ItemMeta, ListMeta } from "../board/meta";
import type { CacheInfo } from "../cache/info";

/** MCP の `refresh` 引数(enum)。none = 更新しない(既定)/ delta = 差分を取る / full = 全件を取り直す(再開あり)。 */
export type RefreshChoice = "none" | "delta" | "full";

export interface ReadOptions {
  refresh?: RefreshChoice;
  /** 返す件数の上限(フィルタのあとに適用)。0・未指定は全件。 */
  limit?: number;
}

export interface ListReadResult<E> {
  items: E[];
  /** cache = キャッシュから返した。api = BOARD から直接取った(このときは cache は空)。 */
  source: "cache" | "api";
  /** 読んだリソースの鮮度情報。M05 の find 層がまとめて応答の `cache` 配列にする。 */
  cache: CacheInfo[];
  /** api のときだけ: ページ数の上限で止めた。 */
  truncated?: boolean;
  /** api のときだけ: 最終ページの応答ヘッダーから作った `_meta`。 */
  _meta?: ListMeta;
}

export interface GetReadResult<E> {
  item: E;
  source: "cache" | "api";
  cache: CacheInfo[];
  _meta?: ItemMeta;
}
