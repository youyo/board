# ADR 0002: D1 キャッシュと再開できる refresh

- 状態: 採用(M03、2026-11-05。M06 で文書化)
- 関連: ADR 0003(締め切り)、ADR 0004(逆引き)、`plans/board-workers-m03-d1-cache.md`

## 背景

BOARD API は 3 回/秒・3,000 回/日の rate limit がある。LLM が一覧や検索を何度も呼ぶと、すぐに上限に近づく。
Go 版は SQLite にエンティティを置き、LLM が `cached_at` を見て `refresh` 引数(`none | delta | full`)で鮮度を決めるモデルだった。
Go の full refresh は 1 回の呼び出しで全件を取っていたが、Workers には 1 回の呼び出しの締め切り(30 秒。ADR 0003)があり、
件数の多いリソースは終わらない(Go の実測で estimates は 25 ページ・約 19 秒)。

## 決定

- **キャッシュは D1**(バインディング `DB`)。Go の `resource_cache`(エンティティ単位の JSON)+ `sync_state` + `cache_meta` と、名前の正規化(NFKC + 小文字 + trim)、LLM が決める鮮度(`refresh` 引数)を引き継ぐ。
- **スキーマは Worker が isolate ごとの初回に冪等に適用する**(`CREATE ... IF NOT EXISTS` を `cache_meta.db_schema_version` で版管理)。`wrangler d1 migrations` は使わない。
  デプロイ手順に migration の適用を入れずに済む。
- **full refresh は再開できる**: 世代番号 + ページ単位で進め、30 秒の締め切りが近づいたら進捗(`full_next_page`)を保存して `refresh_state: "partial"` で返す。
  次の `refresh=full` が最後に取り終えたページから(1 ページ重ねて)再開する。完了で古い世代の行を削除する。
  BOARD の総件数と照合し、足りなければ旧世代を消さず `full_prune_skipped` を立てる(次の full で取り直す)。
- **ロックは D1 の条件付き UPDATE**(TryLock)。取れなければ待たずに `refresh_in_progress`(`retry_after_seconds` 30)。stale は 10 分。
  Durable Objects は使わない。
- **ロックで守る書き込みは、必ず所有者条件つきの 1 つの batch で行う**: 確認から書き込みまでの間に stale なロックを別の呼び出しに奪われても、
  元の保持者の書き込みが効かないようにする(M03 の full 再開と M05 の逆引き走査で同じ不具合が 2 回出たため、規約にした。CLAUDE.md)。
- 参照はキャッシュを優先する(clients / projects / invoices / purchase_orders / payments / users / vendors の 7 リソース)。親 ID などで絞るリソース(nonzero-api の 11)は、非ゼロのフィルタがあるとキャッシュも refresh も経由せず API を直接呼び、`cache` の鮮度情報には載せない(`refresh` は無視される)。
- 運用ツールは `cache_status`(状態の確認)と `cache_clear`(削除)。`cache_expire` は作らない: Go では daily auto-refresh の廃止後、expire の印は読み取りに影響しなくなった。

## 却下した案

- **KV**: エンティティ単位の絞り込み・ソート・件数の集計ができない。却下。
- **キャッシュなし**: rate limit(3 回/秒・3,000 回/日)に対して、LLM の呼び出し頻度が高すぎる。
- **full を 1 回の呼び出しで全件取る(Go と同じ)**: 締め切りに収まらないリソースがある。
- **Durable Objects / Queues で裏の refresh**: 決定済み事項(DO は使わない)に反する。少人数の利用で、再開できる full で足りる。

## 帰結

- 初回は、リソースごとに `refresh=full` を `complete` になるまで繰り返す必要がある(runbook の『キャッシュの初回構築』)。
- full の途中でも参照できる(未取得分は返らない)。`cache.refresh_state` が `partial` なら不完全。
- ページ番号での再開は、合間に前のほうのエンティティが削除・挿入されるとずれて取りこぼしうる。取りこぼしは次の full か delta で拾う。
- D1 の Workers Free プランは 1 回の Worker 実行あたり 50 クエリまで(Paid は 1,000)。full refresh は Paid を前提にする。
