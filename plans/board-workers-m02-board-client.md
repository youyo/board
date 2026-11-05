# M02: BOARD API クライアントと 22 リソースの型

ロードマップ: `plans/board-workers-roadmap.md`(決定済み事項はそちらが正。ここでは覆さない)

## 目的

BOARD API(`BOARD_BASE_URL` + `/v1/...`)を呼ぶクライアントと、22 リソースの型・list / get 関数を作る。
MCP ツールにはまだつなげない(M04)。D1 キャッシュもまだ持たない(M03)。移植元は Go 版の
`internal/boardapi/`(`go-cli-final` タグ)。型・クエリ・パスの正は Go の構造体と `build*Query`。

## 完了条件

- `mise run lint` と `mise run test`(unit + e2e)が通る
- 22 リソースすべての型と、Go 版 CLI に存在する list / get 関数がそろっている(下の表)
- 実 API には通信しない(`mockOutboundFetch` は未登録の宛先を例外にする)

## 設計

### クライアント(`src/board/client.ts`)
- 認証: `x-api-key: <BOARD_API_KEY>` と `Authorization: Bearer <BOARD_API_TOKEN>`(Go の `auth.go`)。GET のみ
- URL: `BOARD_BASE_URL`(末尾のスラッシュは除く)+ パス + `?` + クエリ
- 直列化: 1 クライアントの中では同時に 1 リクエスト(再試行の待機を含む)。前のリクエストが失敗しても後続は流す
- 再試行(Go の `retry.go`): 429・5xx・通信失敗(タイムアウト含む)を最大 5 回再試行(最大 6 試行)。
  待機は `Retry-After`(整数秒。1 以上)を優先し 60 秒で頭打ち。なければ 500ms × 2^試行番号(上限 30 秒)の full jitter。
  `sleep` と `random` は注入できる。4xx(429 以外)は再試行しない
- タイムアウト: 1 試行ごとに 30 秒。応答ヘッダーだけでなく本文の読み取りまで同じ期限で守る
- 応答本文の上限: 32 MiB(Go の `LimitReader` 相当。超えたら `api_error`)
- エラー(`src/board/errors.ts`): `BoardApiError`(`BoardError` の子)。401 → `authentication_error` / 403 → `permission_error` /
  404 → `not_found` / 429 → `rate_limited` / 400・422 → `argument_error` / それ以外 → `api_error`。
  `retryable` は 429・5xx・通信失敗。メッセージは BOARD の `message`(なければ `error`)を載せるが、キー・トークンは伏せる
- 秘密情報: キー・トークンはエラーメッセージ・ログ・例外の原因に出さない。通信の例外は原因を捨てて固定文にする。クライアントはログを出さない

### `_meta`(`src/board/meta.ts`、Go の `result.go`)
- 一覧: `total_count` / `page` / `per_page` / `rate_limit_remaining` / `rate_limit_limit` / `rate_limit_reset`(ISO 8601 UTC)/
  `retry_after` / `etag` / `last_modified`。0・空は出さない。一覧は最終ページの応答ヘッダーから作る
- 単体: 上のうちページング関係と `retry_after` を除いたもの

### ページング(`src/board/paging.ts`、Go の `pagination.go`)
- `page` / `per_page`(既定 100)で順番に取る。件数が `per_page` 未満のページで終わり。
- Go との違い: ページ数の上限 `maxPages`(既定 10 = 1,000 件)を付け、上限で止めたときは `truncated: true`

### クエリ(`src/board/query.ts`、Go の `query.go`)
- Go の `url.Values.Encode()` と同じ形式にする(キーを辞書順、`QueryEscape` 相当、空白は `+`、`[]` は `%5B%5D`)
- ゼロ値・空文字は送らない。真偽値は `0` / `1`。`undefined` は送らない

### リソース(`src/board/resources/<resource>.ts`)
- 各ファイルに `XxxEntity`(Go の構造体に対応する型。未知のフィールドも素通しできるよう index signature を持つ)、
  `XxxListOptions`(Go の `XxxListOptions` を camelCase にしたもの)、`buildXxxQuery`、`listXxx` / `getXxx`
- 応答は検証せずそのまま返す(BOARD の生データ、snake_case のまま)。`id` は 1 以上の安全な整数に限る(パスの組み立てのため)

| resource | パス | list | get | 根拠 |
|---|---|---|---|---|
| clients | /v1/clients | ○ | ○ | clients.go / api_clients.go |
| client_branches | /v1/client_branches | ○ | ○ | client_branches.go |
| contacts | /v1/contacts | ○ | ○ | contacts.go |
| projects | /v1/projects | ○ | ○(`response_group` 付き可) | projects.go(`GetProjectWithGroup`) |
| project_costs | /v1/project_costs | ○ | ○ | project_costs.go |
| estimates | /v1/documents/estimates/{id} | - | ○ | estimates.go(`GetEstimate` のみ) |
| invoices | /v1/invoices | ○ | ○ | invoices.go |
| orders | /v1/documents/orders/{id} | - | ○ | orders.go |
| deliveries | /v1/documents/deliveries/{id} | - | ○ | deliveries.go |
| receipts | /v1/documents/receipts/{id} | - | ○ | receipts.go |
| vendors | /v1/payees | ○ | ○ | vendors.go |
| vendor_branches | /v1/payee_branches | ○ | ○ | vendor_branches.go |
| vendor_contacts | /v1/payee_contacts | ○ | ○ | vendor_contacts.go |
| purchase_orders | /v1/expenditures | ○ | ○ | purchase_orders.go |
| payments | /v1/expenditure_payments | ○ | ○ | payments.go |
| users | /v1/users | ○ | ○ | users.go |
| groups | /v1/groups | ○ | ○ | groups.go |
| payment_terms | /v1/payment_terms | ○ | ○ | payment_terms.go |
| project_types | /v1/project_types | ○ | ○ | project_types.go |
| purchase_types | /v1/expenditure_types | ○ | ○ | purchase_types.go |
| accounting_types | /v1/accounting_types | ○ | ○ | accounting_types.go |
| document_send_channels | /v1/document_send_channels | ○ | ○ | document_send_channels.go |

## やらないこと

MCP ツール(M04)、D1 キャッシュ・refresh(M03)、find 層(M05)、1 回のツール呼び出しあたりの API 呼び出し回数の上限(ツール層で扱う)、
書き込み系の API(Go 版にもない)、依存の追加、コミット・push・デプロイ。

## TDD の進め方

テストを先に書き、`mise run test:unit` が「実装がない」理由で失敗するのを確認してから実装する。

## テスト一覧(`test/unit/`)

- `board-query.test.ts`: QueryBuilder(Go の `query_test.go` を転記。ゼロ値スキップ、`_in[]`、Flg01、日本語、キー順、Go と同じエスケープ)
- `board-meta.test.ts`: ヘッダー → `_meta`(Go の `result_test.go`)。名前の揺れ、reset の ISO 化、空ヘッダーは `{}`
- `board-errors.test.ts`: ステータス → code と retryable、メッセージの秘密情報の除去
- `board-client.test.ts`: 認証ヘッダー、URL 組み立て、再試行(429 / 5xx / 通信失敗 / 4xx は再試行しない / 上限)、Retry-After(優先・0・小数・上限)、
  バックオフ(指数・jitter・上限)、直列化、タイムアウト(ヘッダー待ち・本文待ち)、本文上限、非 JSON、秘密情報が出ないこと
- `board-paging.test.ts`: 短いページで終了、ちょうど割り切れる場合、空、maxPages と truncated、順番に取る、エラーの伝播、引数の検証
- `board-resources.test.ts`: 22 リソースの一覧(list / get の有無・パス)、各 list のクエリ文字列(Go のテストの期待値)、get のパスと `response_group`、id の検証
- `board-list-flow.test.ts`: モック fetch に対する「list → ページング → 全件」の通し(認証ヘッダー・クエリ・`_meta`・truncated)
