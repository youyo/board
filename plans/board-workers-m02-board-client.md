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
- 再試行(Go の `retry.go`): 429・5xx・通信失敗(タイムアウト含む)を再試行する(追補で最大 5 回から 3 回に変更。下の「追補」を参照)。
  待機は `Retry-After`(整数秒。1 以上)を優先し 60 秒で頭打ち。なければ 500ms × 2^試行番号(上限 30 秒)の full jitter。
  `sleep` と `random` は注入できる。4xx(429 以外)は再試行しない
- タイムアウト: 1 試行ごとに 30 秒。応答ヘッダーだけでなく本文の読み取りまで同じ期限で守る
- 応答本文の上限: 32 MiB(Go の `LimitReader` 相当。超えたら `api_error`)
- エラー(`src/board/errors.ts`): `BoardApiError`(`BoardError` の子)。401 → `authentication_error` / 403 → `permission_error` /
  404 → `not_found` / 429 → `rate_limited` / 400・422 → `argument_error` / それ以外 → `api_error`。
  `retryable` は 429・5xx・通信失敗。メッセージは BOARD の `message`(なければ `error`)を載せるが、キー・トークンは伏せる
- 秘密情報: キー・トークンはエラーメッセージ・ログ・例外の原因に出さない。通信の例外は原因を捨てて固定文にする。クライアントはログを出さない

### `_meta`(`src/board/meta.ts`、Go の `result.go`)
- 一覧: `total_count` / `page` / `per_page` /
  `retry_after` / `etag` / `last_modified`。0・空は出さない。一覧は最終ページの応答ヘッダーから作る(`rate_limit_*` は v1.7.0 で削除: BOARD の OpenAPI に `X-Ratelimit-*` の記載が無く、実 API も返さない。issue #33)
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

## 追補: 締め切りとキャンセル

M02 の verifier が次を指摘した: 1 回の `client.get` が最悪 300〜480 秒待ちうる(6 試行 x 30 秒 + 待機最大 60 秒)。一覧は最大 10 ページなのでその 10 倍。
呼び出し元の AbortSignal を受け取らないので、MCP クライアントが切れても Worker だけが待ち続ける。直列化がクライアントのインスタンス内に閉じているため、リクエストごとにクライアントを作ると効かない。

### 仕様
- **呼び出しコンテキスト**(`src/board/context.ts`): `CallContext = { signal?: AbortSignal; deadline?: number(epoch ms) }`。
  `client.get(path, query?, ctx?)`、`fetchAllPages(fetchPage, { ctx })`、`listResource`、各リソースの list / get 関数の最後の任意引数として渡す。
  一覧のページ巡回は、開始時に締め切りを 1 つに決めて全ページで共有する。
- **既定の締め切り**: `deadline` がなければ呼び出し開始から `DEFAULT_CALL_BUDGET_MS = 30_000`。
  ツール層(M04)はツール呼び出しごとに 1 つのコンテキストを作って渡す(同じ呼び出しの中の複数回の API 呼び出しも同じ締め切りを共有する)。
- **待機の規則**: 次の試行までの待機(`Retry-After` またはバックオフ)が残り時間以上なら、待たずに即座に失敗させる(切り詰めて待たない)。
  code は元が 429 なら `rate_limited`、5xx・通信失敗なら `api_error`、`retryable: true`。メッセージは "BOARD API did not respond before the deadline."(秘密情報なし)。
- **1 試行の期限**: `min(timeoutMs(30 秒), 残り時間)`。
- **再試行の上限**: `DEFAULT_MAX_RETRIES` を 5 から **3** に下げた(最大 4 試行)。理由: Go の 5 は常駐しない CLI が長く待てる前提の値。
  Worker は MCP クライアントのタイムアウトより短い持ち時間で返す必要があり、呼び出しあたり 30 秒の締め切りの中では 4 試行目以降に届きにくい。
- **キャンセル**: `signal` が abort されたら、fetch・待機(sleep)・直列化キューの待ちのすべてを即座に中断する。
  エラーは code `api_error`・`retryable: true`・"BOARD API call was aborted."。注入する `sleep` は `(ms, signal?)` を受け取る
  (signal を無視する sleep でもクライアント側で abort と競わせて中断する)。
- **キュー待ちも締め切りに含める**: 順番待ちの間に締め切りを過ぎたら BOARD に送らずに失敗させる(code `api_error`・`retryable: true`)。
  待ちをやめた呼び出しは自分の枠をすぐ手放すので、後続は塞がれない。
- **クライアントの共有**: `createBoardClientFromEnv` は同じ設定(base URL・キー・トークン)なら isolate 内で同じインスタンスを返す(モジュールスコープ)。
  キーやトークンは Map のキーにせず、設定の値をそのまま保持して `===` で比較する(ハッシュ化も文字列化もしないので、ログに出る経路がない)。
  保持数は 8 件まで(超えたら古い順に捨てる)。`overrides` を渡したときは共有しない。テスト用に `clearBoardClientCache()` を export する。

### 1 回のツール呼び出しで待ちうる最悪の時間
- 変更前: 1 回の `get` で、6 試行 x 30 秒 + 待機(Retry-After 60 秒 x 5 回)= 最大 480 秒。一覧 10 ページで最大 4,800 秒。
- 変更後: ツール呼び出しごとのコンテキスト 1 つにつき **30 秒**(キュー待ち・fetch・待機・全ページの合計)。
  コンテキストを渡さなければ、`get` 1 回あたり 30 秒、一覧は 1 回の呼び出し全体で 30 秒。
  締め切りの判定は fetch・本文の読み取り・待機の境界で行うため、実際の超過は数十 ms 程度に収まる。

### 追加・変更したテスト(`test/unit/board-client.test.ts`)
追加: Retry-After / バックオフが残り時間を超えると待たずに失敗(429 → `rate_limited`、5xx → `api_error`)、収まる Retry-After は待って成功、
既定の締め切り 30 秒、1 試行の期限、一覧の 2 ページ目での期限切れ、ctx を省いた一覧が全ページで 1 つの締め切りを共有、get 系の ctx 伝播、
abort(fetch 中・abort 済み・sleep 中・キュー待ち中)、キュー待ち中の期限切れ、`createBoardClientFromEnv` の共有(同一・別・clear・overrides・直列化)、
締め切り・中断のエラーに秘密情報が出ないこと。
仕様変更として意図的に変えた既存テスト: 既定値(再試行 5 → 3)、上限まで失敗したときの試行数(6 → 4)と待機数(5 → 3)、
バックオフの待機列(5 個 → 3 個)、「Retry-After は 60 秒で頭打ち」(既定の締め切り 30 秒では 60 秒待てないので、締め切りを 120 秒に広げて渡す)。
