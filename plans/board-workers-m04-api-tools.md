# M04: api 層の MCP ツール(22 リソースの list / get)

ロードマップ: `plans/board-workers-roadmap.md`(決定済み事項はそちらが正。ここでは覆さない)
前提: `plans/board-workers-m03-d1-cache.md`(repository 層の振る舞い。ここでは再掲しない)

## 目的

Go 版 `board api <resource> list|get` を、MCP ツール 40 個(list 18 + get 22)として移す。
キャッシュ・refresh・API フォールバックの判断は M03 の repository が持つ。M04 のツールは「引数の検証 → repository の呼び出し → 出力の整形」だけで、業務ロジックを書かない。

## 完了条件

- `mise run lint` と `mise run test`(unit + e2e)が通る
- `tools/list` が 41 個(`server_info` + 40)。tools/list のスナップショットを `mise run test:update-snapshots` で意図して更新した
- 実 BOARD API には通信しない(`mockOutboundFetch` は未登録の宛先を例外にする)。`cloudflare:test` は import しない

## 構成

| パス | 役割 |
|---|---|
| `src/mcp/args.ts` | 入力スキーマのビルダー(`optStr` `optInt` `optEnum` `idArg` ほか。logbutler の `args.ts` に揃えた。整数は `refine` で検証し、JSON Schema 上は `number`) |
| `src/mcp/tools/api-resources.ts` | 22 リソースの表(名前・単数形・モード・日本語名・説明・list のフィルタ引数と repository のオプションの対応) |
| `src/mcp/tools/api.ts` | 表から 40 ツールを作る(`API_TOOLS`)。引数 → フィルタの変換、`CallContext` の作成、出力の整形 |
| `src/mcp/tools/index.ts`(変更) | `ALL_TOOLS = [serverInfo, ...API_TOOLS]` |
| `src/mcp/registry.ts`(変更) | `ToolContext` に `signal`(MCP のリクエストの abort signal)と `repositories()` を追加。`RegisterDeps.repositories` |
| `src/mcp/server.ts`(変更) | `createMcpServer({ env })` が D1 + BOARD クライアントから repository を最初の利用時に 1 度だけ作る(`repositories` で差し替え可) |
| `src/app.ts`(変更) | `createMcpServer({ env: c.env })` |

## 設計

### 名前
- list は `list_<リソース名>`(複数形 snake_case)、get は `get_<単数形>`。接頭辞は付けない(Portal が前置する)。
- 単数形は表(`ResourceDef.singular`)で明示する。自動の単数化はしない。
- 一覧 API のない書類 4 種(estimates / orders / deliveries / receipts)は get のみ。

### list の引数
- Go 版 CLI の list のフィルタフラグを、ハイフンを `_` にして持つ(値の種類と `response_group` の列挙値も Go に合わせる)。**フラグ名が揺れているのも Go のまま**
  (`client_branches` / `contacts` / `projects` の `client_id`、`project_costs` の `project_id`、`invoices` の `client_id_eq`、`vendor_branches` / `vendor_contacts` の `payee_id_eq` など)。
- 全 list 共通のフィルタ: `updated_at_gteq` `updated_at_lteq` `include_archive_flg`。
- 共通引数: `refresh`(`none|delta|full`、既定 `none`)・`limit`(0 以上の整数、0 は無制限)・`show_meta`(boolean)・`max_response_tokens`(`defineTool` が付ける)。
- `tags` は文字列の配列、`*_in` は整数の配列(Go の `StringSlice` / `IntSlice`。カンマ区切りの文字列ではなく配列で受ける)。
- `include_archive_flg` / `include_lost_flg` は boolean。`false` を明示すると `=0` を送り、repository のフィルタ判定では『指定あり』になる(Go のポインタ `*bool` と同じ)。
- 入力は `z.strictObject`。未知の引数は `argument_error`(JSON Schema は `additionalProperties: false`)。

### get の引数
- `id`(1 以上の整数)・`refresh`・`show_meta`。projects だけ `response_group`(`estimate|order|delivery|invoice|receipt|all`)を持つ。
- **Go との相違**: 書類 4 種の Go フラグは `--document-id`。ツールは決定どおり `id` に揃えた。
- `get_project` に `response_group` を渡すと `getWithGroup` で常に API を直接呼び(キャッシュは読み書きしない)、`refresh` は無視する(Go の `GetProjectWithGroup` と同じ)。
- 書類 4 種の `refresh`: `full` はキャッシュを使わず取り直し、`delta` は何もしない(M03)。description に書いた。

### refresh の扱い(description に明記)
- cache-first の 7 リソース(clients / projects / invoices / purchase_orders / payments / users / vendors): 絞り込みなし → キャッシュ全件(空なら空。API は呼ばない)。キャッシュ側で絞れる項目だけなら、同期済みでキャッシュを絞る。
  それ以外のサーバー側の項目(`updated_at_*` / `include_archive_flg` / `tags` / `response_group` など)や未同期は **API を直接呼ぶ**(`cache` は空)。
- nonzero-api の 11 リソース: 非ゼロのフィルタを 1 つでも指定すると、キャッシュも refresh も経由せず API を直接呼ぶ。**このとき `refresh` は無視される**(M03 の仕様)。
- 表の `cacheable` が M03 の `cacheFirst.keys` と一致することをテストで固定した。

### 出力
- list: `{ "items": [...], "cache": [...], "truncated"?: true, "_meta"?: {...} }`
- get: `{ "item": {...}, "cache": [...], "_meta"?: {...} }`
- `_meta` は `show_meta=true` かつ BOARD から直接取ったとき(応答ヘッダーがあるとき)だけ付く。キャッシュから返したときは、`show_meta=true` でも付かない。
- `cache` は M03 の `CacheInfo[]`(`cached_at` / `full_refreshed_at` / `refresh_state` / `refresh_error_code`)。API から直接取ったときは空。
- BOARD の生データは BOARD のキーのまま。JSON を text content で返す(structuredContent は使わない)。
- エラーは既存のエンベロープ。`refresh_in_progress` は `retry_after_seconds: 30` 付き、`retryable: true`。

### 呼び出しコンテキスト
- ツール呼び出しごとに 1 つの `CallContext` を作る: `signal` = MCP のリクエストの abort signal、`deadline` = `ctx.now() + DEFAULT_CALL_BUDGET_MS`(30 秒)。refresh・API 呼び出し・ページ巡回で共有する。
- **abort signal は取れる**: `@modelcontextprotocol/server` 2.3.1 のツールのコールバックの第 2 引数(`ServerContext`)の `mcpReq.signal`(`createMcpHandler-*.d.mts` の `BaseContext.mcpReq.signal: AbortSignal`)。
  `registerTools` がそれを `ToolContext.signal` に渡し、api ツールが `CallContext.signal` にする。
- **`maxApiCalls` 相当は `defineTool` にない**(logbutler にだけある概念)ので入れない。1 回の呼び出しの BOARD への通信は、30 秒の締め切りと『1 クライアントで直列に実行』が上限になる。
  list の API 直接呼び出しはページ数の上限(`fetchAllPages` の `maxPages` 既定 10)でも止まる(`truncated: true`)。

### Go との相違点

| 項目 | Go | Workers 版 | 理由 |
|---|---|---|---|
| 書類の get の ID | `--document-id` | `id` | 決定済み(get の引数は `id`) |
| `--show-meta` の既定 | true(CLI は `_meta` 付きが既定) | false | 決定済み(LLM のトークンを節約。必要なら `show_meta=true`) |
| `--refresh` / `--refresh-full` | 2 つの bool | `refresh` の enum | M03 |
| `tags` / `*_in` | カンマ区切り | 配列 | JSON のほうが自然 |
| `--pretty` | あり | なし | MCP では不要 |

## ツール一覧(40)

共通: list は `refresh` `limit` `show_meta` `max_response_tokens` と `updated_at_gteq` `updated_at_lteq` `include_archive_flg` を持つ(下の表のフィルタ引数に含めない)。
get は `id` `refresh` `show_meta` `max_response_tokens`(projects は `response_group` も)。Go の根拠ファイルは `internal/cli/` 以下。

### list(18)

| ツール | リソース | モード | フィルタ引数(共通を除く) | Go の根拠 |
|---|---|---|---|---|
| `list_clients` | clients | cache-first | `name_cont` `name_disp_cont` `invoice_system_number_eq` `custom_no_eq` `tags` `response_group`(small/large) | `api_clients.go` |
| `list_client_branches` | client_branches | nonzero-api | `client_id` `name_cont` | `api_client_branches.go` |
| `list_contacts` | contacts | nonzero-api | `client_id` `name_cont` `email_cont` | `api_contacts.go` |
| `list_projects` | projects | cache-first | `name_cont` `client_id` `client_name_cont` `order_status_in` `delivery_status_in` `project_no_eq` `management_no_eq` `delivery_date_gteq` `delivery_date_lteq` `invoice_date_gteq` `invoice_date_lteq` `invoice_timing_kbn_in` `tags` `response_group`(8 種) `created_at_gteq` `created_at_lteq` `include_lost_flg` | `api_projects.go` |
| `list_project_costs` | project_costs | nonzero-api | `project_id` | `api_project_costs.go` |
| `list_invoices` | invoices | cache-first | `client_id_eq` `project_id_eq` `status_eq` `response_group`(small/large) | `api_invoices.go` |
| `list_vendors` | vendors | cache-first | `name_cont` | `api_vendors.go` |
| `list_vendor_branches` | vendor_branches | nonzero-api | `payee_id_eq` `name_cont` | `api_vendor_branches.go` |
| `list_vendor_contacts` | vendor_contacts | nonzero-api | `payee_id_eq` `name_cont` `email_cont` | `api_vendor_contacts.go` |
| `list_purchase_orders` | purchase_orders | cache-first | `vendor_id_eq` `project_id_eq` `status_eq` `response_group`(small/large) | `api_purchase_orders.go` |
| `list_payments` | payments | cache-first | `vendor_id_eq` `purchase_order_id_eq` `status_eq` `response_group`(small/large) | `api_payments.go` |
| `list_users` | users | cache-first | `name_cont` `email_cont` | `api_users.go` |
| `list_groups` | groups | nonzero-api | `name_cont` | `api_groups.go` |
| `list_payment_terms` | payment_terms | nonzero-api | `name_cont` | `api_payment_terms.go` |
| `list_project_types` | project_types | nonzero-api | `name_cont` | `api_project_types.go` |
| `list_purchase_types` | purchase_types | nonzero-api | `name_cont` | `api_purchase_types.go` |
| `list_accounting_types` | accounting_types | nonzero-api | `name_cont` | `api_accounting_types.go` |
| `list_document_send_channels` | document_send_channels | nonzero-api | `name_cont` | `api_document_send_channels.go` |

### get(22)

| ツール | リソース | モード | 追加の引数 | Go の根拠 |
|---|---|---|---|---|
| `get_client` | clients | cache-first | | `api_clients.go` |
| `get_client_branch` | client_branches | nonzero-api | | `api_client_branches.go` |
| `get_contact` | contacts | nonzero-api | | `api_contacts.go` |
| `get_project` | projects | cache-first | `response_group`(estimate/order/delivery/invoice/receipt/all。指定すると API 直接・refresh 無視) | `api_projects.go` |
| `get_project_cost` | project_costs | nonzero-api | | `api_project_costs.go` |
| `get_estimate` | estimates | document | | `api_estimates.go`(`--document-id`) |
| `get_invoice` | invoices | cache-first | | `api_invoices.go` |
| `get_order` | orders | document | | `api_orders.go`(`--document-id`) |
| `get_delivery` | deliveries | document | | `api_deliveries.go`(`--document-id`) |
| `get_receipt` | receipts | document | | `api_receipts.go`(`--document-id`) |
| `get_vendor` | vendors | cache-first | | `api_vendors.go` |
| `get_vendor_branch` | vendor_branches | nonzero-api | | `api_vendor_branches.go` |
| `get_vendor_contact` | vendor_contacts | nonzero-api | | `api_vendor_contacts.go` |
| `get_purchase_order` | purchase_orders | cache-first | | `api_purchase_orders.go` |
| `get_payment` | payments | cache-first | | `api_payments.go` |
| `get_user` | users | cache-first | | `api_users.go` |
| `get_group` | groups | nonzero-api | | `api_groups.go` |
| `get_payment_term` | payment_terms | nonzero-api | | `api_payment_terms.go` |
| `get_project_type` | project_types | nonzero-api | | `api_project_types.go` |
| `get_purchase_type` | purchase_types | nonzero-api | | `api_purchase_types.go` |
| `get_accounting_type` | accounting_types | nonzero-api | | `api_accounting_types.go` |
| `get_document_send_channel` | document_send_channels | nonzero-api | | `api_document_send_channels.go` |

(get の refresh は nonzero-api / cache-first では『先に一覧キャッシュを refresh してから読む』、document では full=取り直し・delta=何もしない。)

## やらないこと

find 層(M05)、cache 系ツール・README・ツール一覧と README の突き合わせのテスト(M06)、依存の追加、コミット・push。

## TDD の進め方

テストを先に書き、`mise run test:unit` が『ツールが存在しない / `ctx.signal` が undefined』の理由で 97 件失敗するのを確認してから実装した。

## テスト一覧

`test/unit/`:
- `api-tools-definitions.test.ts`: 名前 40 個(単数形の対応表を明示)・ReadOnly・list の引数が Go のフラグ表と一致(18 リソース)・get の引数(22)・`response_group` の列挙値・description の明記事項
  (nonzero-api の『refresh は無視』、cache-first の『API を直接』、書類の『refresh=full / delta は何もしない』、`get_project`)・引数の検証 19 ケース(不正な id / 列挙値 / 未知の引数 / limit / 型 → `argument_error` で BOARD に通信しない)・
  `additionalProperties: true` が tools/list にない・表と M03 の `cacheFirst.keys` / モード分類の整合
- `api-tools-call.test.ts`: Go のフラグ → クエリの突き合わせ 15 リソース(clients / projects / invoices / purchase_orders / payments / vendors / users / client_branches / contacts / project_costs / vendor_branches / vendor_contacts / groups / purchase_types / document_send_channels)・
  空配列はゼロフィルタ・list の出力(未取得 / `show_meta` の有無 / `limit` / `refresh=full` → `none` / nonzero-api で refresh 無視・D1 に触れない / 生データのキー)・
  get の出力(キャッシュ / `_meta` / 書類 / `get_project` の `response_group` / 404)・`refresh_in_progress` のエンベロープ・abort signal が BOARD への fetch に届き中断される・締め切りが既定 30 秒
- `tool-registry.test.ts`(変更): ハンドラの ctx に abort signal が渡る / repositories 未設定は generic_error
- `server-info.test.ts`(変更): 先頭が `server_info`、全体 41 個
- `tools-list-snapshot.test.ts`: スナップショットを更新(`test/snapshots/tools_list.json`)

`test/e2e/api-tools-flow.test.ts`(Access JWT つきの Worker 全体への HTTP。BOARD はモック fetch、D1 は実物):
`tools/list` 41 個 / `list_clients`(空 → `refresh=full` → `refresh=none` はキャッシュ・`limit`)/ `get_delivery`(API → キャッシュ)/ `list_contacts`(非ゼロのフィルタは API 直接・D1 に残らない)/
締め切り超過で `retryable`(Date だけを偽装して 60 秒進め、再試行せずに失敗することを確かめる)/ `refresh_in_progress` / 秘密情報(正常・BOARD のエラー本文にキーが混ざる場合・引数エラー・D1 の全テーブル)。
`test/e2e/mcp-flow.test.ts`(変更): `tools/list` が 41 個。

## 変更した既存ファイル

`src/app.ts` `src/mcp/registry.ts` `src/mcp/server.ts` `src/mcp/tools/index.ts` `test/unit/server-info.test.ts` `test/unit/tool-registry.test.ts` `test/e2e/mcp-flow.test.ts` `test/snapshots/tools_list.json`

## 既知のリスク・未解決

- **応答サイズ**: cache-first の `refresh=full` 後のゼロフィルタ list は全件を返す(clients / projects は数千件)。`max_response_tokens`(既定 50,000)を超えると `response_too_large` になるので、LLM は `limit` や絞り込みを使う。description に書いてはいるが、実運用で既定の上限に当たりやすいかは M06 の `test:live` で確かめる。
- **`include_archive_flg: false` の明示**: cache-first / nonzero-api のどちらでも『指定あり』になり API を直接呼ぶ(Go の `*bool` と同じ)。LLM が不必要に `false` を付けるとキャッシュを使わなくなる。description に『false を明示すると 0 を送る』と書いた。

## M05 での追補(M04 の検証の指摘 4 点への対応)

- **cache-first の list × サーバー専用の項目 × `refresh=delta/full`**: refresh を走らせず BOARD を直接呼ぶ(nonzero-api の『非ゼロのフィルタなら refresh を無視』と同じ扱い)。以前は refresh のあとに API を呼んでいて、通信が無駄になり `refresh_in_progress` にもなりえた。
  repository の `list` を変更(`src/repository/repository.ts`)。description に『このとき refresh は無視される』を追記。キャッシュで絞れる項目だけの指定、および未同期のフォールバックは従来どおり(refresh は効く)。
- **`limit` で切ったことの通知**: list の出力に、切ったときだけ `limited: true` と `total_before_limit`(切る前の件数)。`truncated`(API のページ打ち切り)とは別の意味。`limit` の description に明記。repository の結果に `totalBeforeLimit`。
- **締め切りの時計**: `callContextOf` は `Date.now() + 30 秒`(BOARD クライアントの判定と同じ時計)。`ctx.now()`(テストで差し替える時計)は使わない。`src/mcp/call-context.ts` に移した(find 層と共有)。
- **`response_too_large`**: 『絞り込む』に加えて『`limit` を指定する』を案内する。cache-first の list の description に『まず limit か絞り込みを指定する』。
- 変えた既存テストの期待値: `test/snapshots/tools_list.json`(description の変更のみ)。それ以外の既存テストの期待値は変えていない(新しいテストを足した)。
