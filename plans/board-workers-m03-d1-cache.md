# M03: D1 キャッシュ・refresh・repository 層

ロードマップ: `plans/board-workers-roadmap.md`(決定済み事項はそちらが正。ここでは覆さない)
設計の正: Go 版 `plans/board-cache-redesign.md`(LLM が鮮度を決めるモデル)

## 目的

BOARD API のレート制限(3 req/秒・3,000/日)を避けるため、エンティティを D1 にキャッシュし、
LLM が `cached_at` を見て `refresh` 引数(`none | delta | full`)で更新を指示するモデルを Workers 版に移す。
MCP ツールにはまだつなげない(M04・M05)。M04・M05 がそのまま使える公開関数(repository の list / get / search、
cache の status / expire / clear)をここで用意する。移植元は Go 版の `internal/cache/` `internal/refresh/` `internal/repository/`(`go-cli-final` タグ)。

## 完了条件

- `mise run lint` と `mise run test`(unit + e2e)が通る
- 22 リソースの振る舞い(下の表)が repository の表(`src/repository/resources.ts`)にそろっている
- 実 BOARD API には通信しない(`mockOutboundFetch` は未登録の宛先を例外にする)。`cloudflare:test` は import しない

## 構成

| パス | 役割 | Go の移植元 |
|---|---|---|
| `src/cache/schema.ts` | 冪等な DDL・版管理・`ensureSchema`(isolate ごとに 1 回) | `schema.go` `migrate.go` |
| `src/cache/resource-cache.ts` | エンティティ単位の JSON の upsert / 取得 / 一覧 / 削除 | `resource_cache.go` |
| `src/cache/sync-state.ts` | 同期状態・ロック・full の進捗 | `sync_state.go` |
| `src/cache/fold.ts` | 名前の正規化(NFKC + 小文字 + trim) | `fold/fold.go` |
| `src/cache/info.ts` | 応答に同梱する鮮度情報(`cache` 配列の要素) | `info.go` |
| `src/cache/admin.ts` | status / expire / clear(M06 のツールの土台) | `cli/cache.go` |
| `src/refresh/lock.ts` | D1 の条件付き UPDATE による TryLock | `lock.go` |
| `src/refresh/refresher.ts` | delta / full(再開あり) | `resource_refresh.go` `force_refresh.go` `updater.go` |
| `src/repository/` | `createRepositories(deps)`。表(`resources.ts`)+ 汎用実装(`repository.ts`) | `repository/*.go` |

## 設計

### ストレージ(D1、バインディング `DB`)
Go の 3 テーブルを移す。profile の列は持たない(1 デプロイ = 1 アカウント)。`cache_meta` は `db_schema_version` を持つ。
- `resource_cache(resource_name, entity_id INTEGER, payload_json, updated_at, fetched_at, generation, PK(resource_name, entity_id))`
  + 索引 `(resource_name, updated_at)` `(resource_name, generation)`。`entity_id` は Go では TEXT だが、id は 1 以上の整数なので INTEGER にした(順序が決定的になる)。
- `sync_state(resource_name PK, last_synced_at, cursor_updated_at, last_full_synced_at, last_sync_mode, last_sync_status, must_full_resync, expired_at, invalidate_reason, last_error_*, consecutive_failures, refresh_started_at, refresh_owner, full_generation, full_next_page, full_started_at, full_max_updated_at)`
- `cache_meta(key PK, value)`
- スキーマの適用: Worker が isolate ごとの初回利用で `ensureSchema(db)`(`WeakMap<D1Database, Promise>` に記憶、失敗は記憶しない)。
  `CREATE ... IF NOT EXISTS` を `db.batch` で流し、版を `cache_meta` に書く。`wrangler d1 migrations` と `applyD1Migrations` は使わない。
  D1 の `exec` は改行で文を分けるので、複数行の DDL は `prepare` + `batch` で流す。
- 1 文あたりのバインドは 100 まで。行ごとの upsert 文を `db.batch`(原子的)で流す。1 ページ分の upsert と同期状態の更新は同じ `batch` に入れる。
- 時刻は ISO 8601 UTC(秒まで)で保存する。文字列の大小比較がそのまま時刻の前後になる(ロックの stale 判定に使う)。

### 名前の正規化(`fold.ts`)
`s.trim().normalize("NFKC").toLowerCase()`(Go と同じ順)。`foldContains(haystack, needle)` は空の needle で true。Go の `fold_test.go` のケースを転記して固定した。

### ロック(`lock.ts`)
- `INSERT OR IGNORE INTO sync_state(resource_name)` のあと
  `UPDATE sync_state SET refresh_started_at=?, refresh_owner=? WHERE resource_name=? AND (refresh_started_at IS NULL OR refresh_started_at < ?)`(しきい値 = now - 10 分)。
  `meta.changes === 1` なら取得。取れなければ待たずに `RefreshInProgressError`(code `refresh_in_progress`・retryable・`retry_after_seconds: 30`)。
- 取得者は呼び出しごとの `crypto.randomUUID()`。解放は `WHERE refresh_owner = ?` で自分のロックだけ(stale を奪われたあとの元の取得者が、奪った側のロックを外さない)。
- `withLock` は成功・失敗・例外のどれでも finally で解放する。
- エンベロープ: `ERROR_CODES` に `refresh_in_progress` を追加し、`toErrorEnvelope` は `RefreshInProgressError` のときだけ `retry_after_seconds` を載せる(仕様の追加。`errors.test.ts` の列挙の期待値を更新した)。

### refresh(`refresher.ts`)
`refreshResource(deps, target, "delta" | "full", ctx?)`。ロック → 実行 → 解放。ページは自前のループで取る(`fetchAllPages` は再開できない)。
`per_page` は 100。ページ数の上限は付けず、締め切りで止まる。

**delta**(Go と同じ):
1. カーソル(`cursor_updated_at`)を `YYYY-MM-DD HH:MM:SS` にして `updated_at_gteq` に渡す(`isoToBoardDateTime`。UTC に直さず、元のオフセットの時刻部分を使う)。
2. 取得分を upsert(世代は `sync_state.full_generation`)。1 ページ取るごとに進捗(`delta_next_page` / `delta_started_at` / `delta_max_updated_at`)を同じ `batch` で保存する。最後のページまで取れたら、カーソルを `min(見た updated_at の最大値, delta_started_at の 5 分前)`(`cursorAfterFull` と同じ)に進め(取得 0 件なら据え置き)、進捗を消し、`last_synced_at` を更新する。
3. カーソルがなければ全件が必要なので、再開できる full として動く(結果の `mode` は `full`)。
4. 締め切りで止まったら `partial`(取れた分は upsert 済み、進捗は保存済みでカーソルは動かさない)。次の delta は `delta_started_at` から 1 時間以内なら、最後に取り終えたページから(1 ページ重ねて)再開する。過ぎていたら最初のページからやり直す。API の失敗は例外にせず `failed`(古いキャッシュを返す。Go と同じ)。失敗は `sync_state` に記録する。

**full**(再開あり。Go との相違点):
1. 開始時に新しい世代 `full_generation + 1` を払い出し、`full_next_page = 1` `full_started_at` を記録する。
2. 1 ページ取るごとに、その世代で upsert し、`full_next_page` と `full_max_updated_at` を更新する(同じ `batch`)。
3. 次のページを取る前に、残り時間が `max(下限 1 秒, 直前のページの所要時間 x 1.5)` を下回るか、クライアントが締め切りのエラーを返した(`DeadlineExceededError`。最後の 1 試行が残り時間でタイムアウトした場合も含む)ら、進捗を保存して `partial` を返す。ロックは解放する。
4. 次の `refresh=full` は `full_next_page` から再開する(`full_started_at` から 1 時間以内のとき。過ぎていたら新しい世代で最初からやり直す)。
5. 件数が `per_page` 未満のページ(最後)を取り終えたら、同じ `batch` で古い世代の行を削除し、`last_full_synced_at` を更新し、`full_next_page` を NULL にして `complete`。
6. full の途中でも参照は既存の行(新旧の世代の和集合)を返す。途中の delta や `get` 失敗時の保存は進行中の世代を付けるので、full の完了時に消されない。
7. 完了後のカーソル: 見た `updated_at` の最大値。ただし full の開始の 5 分前より新しければ、開始の 5 分前(最大値と同じオフセットで表す)。
8. 失敗(5xx など)は進捗を残して例外を伝える(Go の `ForceRefresh` と同じ)。`full_next_page` は失敗したページを指す。呼び出し元の中断(signal)は失敗として数えない。

### repository(`src/repository/`)
- `createRepositories(deps)`(`deps = { db, client, now?, ... }`)が 22 リソースの repository を返す。一覧のあるリソースは `list` / `search` / `get` / `refresh`、document 4 種は `get` のみ。
  projects だけ `getWithGroup(id, responseGroup)`(常に API)を持つ。公開関数の最後の引数は `ctx?: CallContext`(refresh も API 呼び出しも同じ締め切りを共有)。
- `ReadOptions = { refresh?: "none" | "delta" | "full"; limit?: number }`(既定 `none`)。
- 結果: `{ items | item, source: "cache" | "api", cache: CacheInfo[], truncated?, _meta? }`。`cache` は読んだリソースの `{ resource, cached_at?, full_refreshed_at?, refresh_state?, refresh_error_code? }`
  (`source: "api"` のときは空)。M05 の find 層が、読んだリソース分をまとめて応答の `cache` 配列にする。
  - `refresh_state`: その呼び出しで refresh したときの結果(`complete` / `partial` / `failed`)。refresh しなくても、full が途中なら `partial`。
  - 未取得のリソースは `cached_at` / `full_refreshed_at` を出さない(LLM が『未取得』と判別できる)。

### どのリソースをキャッシュするか、キャッシュが空で `refresh=none` のときの挙動(Go の repository 層の実態)

| mode | リソース | ゼロフィルタの `list` | 非ゼロフィルタの `list` | `get(id)` | 根拠(Go) |
|---|---|---|---|---|---|
| cache-first | clients / projects / invoices / purchase_orders / payments / users / vendors | refresh(指示があれば)→ キャッシュ全件。**空なら空(API を呼ばない)** | refresh(指示があれば)→ キャッシュ側で絞れる項目だけで、**同期済みならキャッシュを fold で絞る**。未同期(キャッシュが空)なら **API にフォールバック**。サーバー専用の項目(page / perPage / updatedAt* / includeArchiveFlg / tags / responseGroup など)が 1 つでもあれば API | refresh → キャッシュ → なければ API で 1 件取って保存 | `clients.go` `projects.go` `invoices.go` `vendors.go` `purchase_orders.go` `payments.go` `users.go` の `List`(`tryCacheFilter` `cacheable*Filter` `match*Filter`)、`cache_filter.go` |
| nonzero-api | client_branches / contacts / project_costs / vendor_branches / vendor_contacts / groups / payment_terms / project_types / purchase_types / accounting_types / document_send_channels | refresh → キャッシュ全件。空なら空 | **refresh もキャッシュも経由せず API を直接呼ぶ**(`refresh` 指定は無視) | cache-first と同じ | `client_branches.go` などの `List`(`*FilterIsZero` が偽なら `api.List*`) |
| document | estimates / orders / deliveries / receipts | (一覧なし) | (一覧なし) | キャッシュ → なければ API → 保存。`refresh=full` はキャッシュを使わず取り直す。`delta` は何もしない | `estimates.go` `orders.go` `deliveries.go` `receipts.go`(`GetByDocumentID`、`ForceRefresh` で迂回) |

キャッシュ側で絞れる項目(`keys`)と判定(Go の `match*Filter`):
clients = name / name_disp(fold の部分一致)・invoice_system_number / custom_no(完全一致、null は空文字扱い)/
projects = name(fold)・client_id(`client.id`)・client_name(fold)・project_no(文字列で完全一致)・management_no(完全一致)/
invoices = client_id / project_id / status / purchase_orders = vendor_id / project_id / status / payments = vendor_id / purchase_order_id / status /
vendors = name(fold)/ users = name / email(fold)。複数の条件は AND。`limit` はフィルタのあとに適用する。

### Go との相違点

| 項目 | Go | Workers 版 | 理由 |
|---|---|---|---|
| ストレージ | SQLite(ファイル) | D1 | Workers にファイルシステムがない(決定済み) |
| profile 列 | あり | なし | 1 デプロイ = 1 アカウント |
| スキーマの適用 | 起動時に `Migrate` | isolate ごとの初回利用で `ensureSchema` | D1 の migrations を使わない(決定済み) |
| full refresh | `DeleteAll` + 全件取得を 1 回の呼び出しで | 世代 + ページ単位の再開。途中は新旧の和集合を返す | 30 秒の締め切りの中では件数の多いリソースの全件取得が終わらない(Go の実測で estimates は 25 ページ・約 19 秒)。**決定済み事項の表に追記** |
| full 完了後のカーソル | `NULL` に戻す(次の delta が全件取得になる) | 見た updated_at の最大値(ただし full の開始の 5 分前を上限) | 次の delta が全件取得になると、締め切りの中で終わらず、delta は再開できない。判断として入れた。**覆すなら `cursorAfterFull` を `null` を返すだけにする(1 行)** |
| delta のカーソルなし | 全件取得 | 再開できる full として動く | 同上 |
| delta の再開 | (なし。1 回で取り切る) | full と同じ方式で再開(`delta_next_page` / `delta_started_at` / `delta_max_updated_at`。スキーマ v3)。完了時のカーソルは min(見た最大値, 開始の 5 分前) | 差分が 30 秒で取り切れないと、毎回先頭からやり直して `complete` にならないため |
| partial な full の再開 | (なし) | 1 時間以内なら、最後に取り終えたページから(1 ページ重ねて)再開。過ぎたら最初から。完了時の旧世代の削除は総件数(X-Total-Count)と照合 | ページ番号での再開は、間に削除・挿入があるとページがずれて行を取りこぼす。重ね取りと件数照合で緩和(下の既知のリスク)。長く経った partial は信用しない |
| ロック | プロセス内 mutex + `refresh_started_at` の確認(読んでから書く) | D1 の 1 つの条件付き UPDATE(`meta.changes` で判定)。解放は自分の取得者のときだけ | isolate が複数あり、読んでから書く方式は競合する |
| 『同期済み』の判定 | `sync_state` の行があるか(ロックを取っただけでも作られる) | `last_synced_at` があるか。partial は進めない | 行の有無では、ロックだけ取った・最初の refresh が失敗した状態を『同期済み』と誤認する |
| delta の失敗 | ログに出して古いキャッシュを返す | 同じ。加えて `cache[].refresh_state: "failed"` と `refresh_error_code` で LLM に伝える | Workers のログは LLM に見えない |
| daily auto-refresh | 廃止済み | なし | 設計の正(決定済み) |
| `expire` | `expired_at` を付ける(daily 廃止後は読み取りに影響しない) | 同じ(情報用)。M06 で意味を決める | Go と同じ |
| `clear` | 行と状態を消す | 同じ。ただし先に refresh と同じロックを取り、refresh 中のリソースがあれば何も消さず `refresh_in_progress`。削除は所有者を条件に 1 つの batch で行う | 進行中の refresh と競合させない。確認と削除の間に refresh が始まってもロック行を消さない |
| `entity_id` の型 | TEXT | INTEGER | 順序が決定的になる |
| 列 | `last_daily_refresh_date` `cache_version` `schema_version` | なし | daily の廃止・版は `cache_meta` に一本化 |
| 再試行 | 5 回 | 3 回(M02 追補で決定済み) | 締め切り |

### 秘密情報
キャッシュに API キー・トークンは保存しない(BOARD のエンティティ JSON だけ)。`sync_state` のエラー欄には**エラーコードだけ**を保存し、
メッセージ・例外の文面は保存しない。ログは出さない。E2E で D1 の全テーブルに秘密が入らないことを確かめる。

## やらないこと

MCP ツール(M04・M05)、cache 系ツール(M06)、find 層(逆引きマップ、名前による ID 解決。M05)、Durable Objects、
依存の追加、コミット・push・デプロイ。

## 既知のリスク

- **ページのずれ**: full の再開は『ページ番号 + 世代』で、**BOARD の一覧の既定の並びが呼び出しの間で安定していること**を前提にしている(並びを指定していない)。
  partial の間に前のほうの行が削除・挿入されるとページがずれ、取りこぼした行は完了時に旧世代として消えうる。緩和策は 2 つ:
  1. **重ね取り**: 再開は最後に取り終えたページ(n)から取り直す(1 ページ重ねる。upsert は冪等)。`full_next_page` は『次に取るページ』のまま保存し、再開時に 1 を引く。
     これで『page n までの間で 1 件削除 → page n+1 の先頭が page n に繰り上がる』ずれは拾える。副作用: 1 回の呼び出しで 1 ページしか取れない予算だと進まない
     (30 秒の締め切りで 1 ページ約 1 秒なので通常は問題にならない)。
  2. **件数照合**: 完了時の旧世代の削除は、BOARD の総件数(最終ページの `X-Total-Count`)と、新しい世代以降の行数を比べ、足りなければ削除しない。
     `refresh_state` は `complete` のまま `sync_state.full_prune_skipped = 1`(outcome の `pruneSkipped`)に記録し、次の full で取り直す。
     総件数のヘッダがなければ従来どおり削除する。
  残るリスク: (a) 呼び出しの合間に重ね取りの 1 ページを超える件数(100 件超)が前のページから消えた場合は、繰り上がった行を拾えない。ただし件数照合が働けば削除は止まる
  (新世代の件数が総件数より少ないため)。(b) 取得済みのページにあった行が合間に BOARD で削除された場合、その行は新世代に入ったまま次の full まで残る。
  (c) 件数照合は『新世代以降の行数 >= 総件数』なので、残った (b) の行が不足分を埋めてしまうと検出できない。
- **delta の再開も、ページ番号のずれの影響を受ける**(2026-11-05 に再開できるようにした。以前は partial になるとカーソルが進まず、差分が 30 秒で取り切れないと delta は何度やっても `complete` にならなかった)。
  full と同じく 1 ページ重ねて取り直すが、合間に差分の並びが変わると取りこぼしうる。カーソルは完了まで動かさず、完了時に開始の 5 分前を上限にするので、取りこぼしの多くは次の delta が拾う。
- **空のリソースではカーソルが null のまま**: full を完了しても取得件数が 0 なら `updated_at` の最大値がなくカーソルが作られず、以後の delta は毎回 full として動く(再開できる full なので安全だが、delta の軽さは得られない)。
- **stale ロックの奪取**: 10 分を超えたロックは奪える。元の保持者が遅れて書こうとしても、すべての書き込み(行の upsert・旧世代の削除・sync_state の更新・失敗の記録)は
  同じ D1 batch の中で `refresh_owner` が自分であることを条件にしているため反映されず、`sync_state` の更新の `meta.changes` が 0 になって `refresh_in_progress` で止まる。
- **D1 の書き込み量**: full は 1 行 1 文の upsert。10 万行で 10 万回の行書き込み。BOARD のデータ量(顧客・案件は数千規模)では問題ない想定。
- **refresh の途中で Worker が落ちた場合**: ロックは 10 分で stale になる。その間、同じリソースの refresh は `refresh_in_progress`。
- **delta の取りこぼし**: BOARD の `updated_at` と Worker の時計のずれ。カーソルは取得分の最大値なので、サーバー側の時刻にだけ依存する(full 完了後の初回だけ Worker の時刻を使い、5 分の余裕を持たせた)。

## TDD の進め方

テストを先に書き、`mise run test:unit` が『モジュールがない / 期待した export がない』理由で失敗するのを確認してから実装した。
unit は miniflare の D1(`env.DB`、`cloudflare:workers` 経由)。D1 の状態はテストファイルごとに隔離され、同じファイルの中では共有されるので、
各テストの `beforeEach` で `resetCache()`(スキーマを適用して全テーブルを DELETE。DROP はしない)を呼ぶ。

## テスト一覧

`test/unit/`:
- `cache-fold.test.ts`: Go の `fold_test.go` の転記(NameFold / Contains)+ 全角スペース・null
- `cache-schema.test.ts`: 冪等(版の記録を消して 2 回流しても同じ・データが壊れない)、profile 列がない、full の列がある、`ensureSchema` の記憶と失敗時のやり直し
- `cache-store.test.ts`: JSON の往復、置き換え、リソース間の分離、id なしの拒否(原子的)、250 件、並び順(updated_at 昇順・NULL 先頭)、削除、古い世代の削除
- `cache-sync-lock.test.ts`: 取得、競合(`refresh_in_progress`・holder・elapsed)、リソース間の独立、stale(10 分)の奪取と境界、奪われたロックを元の取得者が解放できない、同時 3 つで 1 つだけ取得、ロックだけでは『同期済み』にならない、`withLock`(成功・例外・実行中・取れないとき)
- `refresh.test.ts`: `isoToBoardDateTime` / `cursorAfterFull`、full(3 ページ・ちょうど割り切れる・2 回目で削除と更新・残り時間の目安で partial・締め切りのエラーで partial・再開して古い世代を削除・1 時間を過ぎた partial は最初から・途中の delta の世代・失敗時の進捗と記録・ロック・id なし・配列でない応答)、delta(カーソル・差分なし・複数ページ・カーソルなしは full・失敗は failed・締め切りで partial・ロック)、中断
- `cache-admin.test.ts`: status(22 リソース・full の進行・ロック・stale)、expire、clear(ロック中は拒否)、未知のリソース名
- `repository-filter.test.ts`: ゼロフィルタ・cacheable 判定、`applyLimit`、22 リソースのモード分類、Go の `match*Filter` / `cacheable*Filter` の転記(clients / projects / invoices / purchase_orders / payments / vendors / users)
- `repository.test.ts`: cache-first(空・API フォールバック・full 後・fold・サーバー専用項目・limit・search・delta 失敗・full 失敗・ロック・partial・get)、nonzero-api、document、`getWithGroup`、CallContext(abort)
- `errors.test.ts`(変更): `refresh_in_progress` の列挙と `retry_after_seconds`
- `board-client.test.ts`(変更): 締め切りのエラーが型(`isDeadlineError`)で見分けられる

`test/e2e/cache-refresh-flow.test.ts`(実物の D1 + 本物のクライアント + モック fetch。MCP ツールがまだないので repository の公開関数を直接呼ぶ。
キャッシュ → refresh → repository → クライアント → HTTP を通しで確かめるテストなので `test/e2e/` に置いた):
full を 3 回の呼び出しに分けて完了 / delta のあとの参照 / 2 つの refresh を同時に走らせて一方が `refresh_in_progress` / 別リソースは邪魔しない /
キャッシュが空のときの挙動 / 秘密情報が D1 に残らない / document の get。

## 変更した既存ファイル

- `src/errors.ts`: `refresh_in_progress`・`RefreshInProgressError`・エンベロープの `retry_after_seconds`
- `src/board/errors.ts` `src/board/client.ts`: `DeadlineExceededError` / `isDeadlineError`(締め切りを文面ではなく型で見分けるため)
- `test/helpers/mock-fetch.ts`: モックの応答待ちが `fetch` の `signal` の abort で中断されるようにした(本物の `fetch` と同じ)
