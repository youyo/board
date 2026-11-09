# issue #27: users / invoices / purchase_orders / payments の型・絞り込みを BOARD の実際の仕様に合わせる

## 背景

- 4 リソースのエンティティ型・一覧のクエリ名・キャッシュの `match`・find 層が、BOARD に実在しない項目(`users.name`、`invoices.client_id` / `status`、`purchase_orders.vendor_id` / `status`、`payments.vendor_id` / `purchase_order_id` / `status`)を前提にしていた。絞り込みは**エラーにならず 0 件**になる。
- 根拠: BOARD 公式の OpenAPI 仕様 v1.9.0(`https://developers.the-board.jp/doc/board_openapi.json`)と、下流の D1 キャッシュの実データ(issue #27 の表。キー名と型のみ)。
- Go 版から写した型で、Go 時代の実 API 突合(StrictFieldDiff)は「実応答にあって型に無いキー」しか検出しない一方向の検査だったため見逃した。
- 一覧の既定(`response_group` 省略時)は OpenAPI に記載がない。実データ(issue #27)は small 相当(purchase_orders に案件の項目が無い)。**キャッシュで絞れるキーは実データに出ている項目だけにする**。

## スコープ

この 4 リソースと、その find ツール(`find_users` / `find_invoices` / `find_purchase_orders` / `find_payments`)だけ。残りのリソースの不一致(client_branches / contacts / vendor_branches / vendor_contacts の `name_cont` / `email_cont`、`vendor` の入れ子名、`memo` / `code`、OpenAPI に無い `GET /{id}`、clients の `custom_no` が small に無い、共通フィルタの未文書キー、users の delta の `updated_at_gteq`)は別 issue。

## 決定事項

### 共通

- ツールの引数名は BOARD のクエリ名(snake_case)と同じにする(既存の規約)。旧引数は一度も効いていないので、互換のための別名は残さない。
- 4 リソースのエンティティ型から `[key: string]: unknown` を外し、OpenAPI の一覧応答の項目で書き直す(他のリソースは触らない)。テストのフィクスチャは `satisfies XxxEntity` で、実在しない項目を型検査で止める。
- 型と OpenAPI の二方向の突合: OpenAPI v1.9.0 から抽出した 4 リソースの一覧応答のキーをテスト側の定数に持ち、`keyof XxxEntity` がその部分集合であることを型レベルで検査する(`tsc` が `mise run lint` で走る)。フィクスチャのキーも unit で同じ定数の部分集合か検査する。
- ステータスの `_in[]` の送り方は projects の `order_status_in[]` と同じ実装に揃える。
- ステータスの値は数値コード。ツールの説明文に「コード: 名称」を全部書く。

### invoices(`/v1/invoices`)

| 旧引数 | 新引数 | API に送る名前 | キャッシュの match |
|---|---|---|---|
| `client_id_eq` | `project_client_id_eq` | `project_client_id_eq` | `client?.id` |
| `status_eq` | `invoice_status_in`(数値の配列) | `invoice_status_in[]` | `invoice_status` を含む |
| `project_id_eq` | (削除。API に無い) | — | — |

請求ステータス: 1 未請求 / 4 請求OK / 2 請求済 / 5 一部入金済 / 3 入金済 / 9 回収不能。

### purchase_orders(`/v1/expenditures`)

| 旧引数 | 新引数 | API に送る名前 | キャッシュの match |
|---|---|---|---|
| `vendor_id_eq` | `payee_id_eq` | `payee_id_eq` | `payee?.id` |
| `status_eq` | `expenditure_status_in`(数値の配列) | `expenditure_status_in[]` | `expenditure_status` を含む |
| (新規) | `delivery_status_in`(数値の配列) | `delivery_status_in[]` | `delivery_status` を含む |
| `project_id_eq` | `project_id_eq`(そのまま) | `project_id_eq` | **キャッシュでは絞らない**(サーバー専用。既存のキャッシュの行に案件の項目が無いため) |

- 一覧の `response_group` は、未指定なら `medium` を送る(refresh・API のフォールバック・list ツールの全経路)。medium には `project`(id / name / project_no / management_no)が含まれ、`find_purchase_orders` の `Project` が付けられる(M05 の約束)。D1 の行は少し大きくなる。既存のキャッシュは次の full refresh で medium になる。
- 発注ステータス: 1 見積中(高) / 2 見積中(中) / 3 見積中(低) / 8 見積中(除) / 4 発注確定 / 5 発注済 / 9 見送り。進捗状況: 1 未着手 / 2 着手中 / 3 納品済 / 4 検収済。

### payments(`/v1/expenditure_payments`)

| 旧引数 | 新引数 | API に送る名前 | キャッシュの match |
|---|---|---|---|
| `vendor_id_eq` | `expenditure_payee_id_eq` | `expenditure_payee_id_eq` | `payee?.id` |
| `status_eq` | `payment_status_in`(数値の配列) | `payment_status_in[]` | `payment_status` を含む |
| `purchase_order_id_eq` | `expenditure_expenditure_no_eq`(発注 **No**。ID ではない) | `expenditure_expenditure_no_eq` | `expenditure_no` |

支払ステータス: 1 請求書未受領 / 2 請求書受領済 / 4 振込予約済 / 3 支払済。

### users(`/v1/users`)

- API の絞り込みは `email_eq` だけで、名前の絞り込みは無い。`name_cont` / `email_cont` は**キャッシュ専用のキー**にする: 同期済みならキャッシュで絞る。未同期なら、絞り込みを付けずに API から全ページ取り、同じ `match` を適用して返す(users は件数が少ない)。ListSpec にこの区分(キャッシュ専用のキー)を 1 つ足す。
- 名前の match: `last_name`・`first_name`・`last_name + first_name`・`last_name + " " + first_name` のどれかに fold の部分一致。`email_cont` は `email` に fold の部分一致。

### find 層

- `find_invoices` / `find_purchase_orders` / `find_payments` の `status` / `statuses` は**名称**(例: `請求済`)で受け、上の表でコードに変換して `*_status_in` に渡す(キャッシュでも API でも効く)。知らない名称は `argument_error`(受け付ける名称の一覧を message に入れる)。purchase_orders の名称は発注ステータスと進捗状況の両方を受け、どちらかに一致すれば OR(projects の `filterProjectsByStatuses` と同じ考え方)。
- 付け足し: invoices の `Client` は `client?.id`、purchase_orders / payments の `Vendor` は `payee?.id` で取る(取得の形はこれまでと同じ)。purchase_orders の `Project` と URL は `project?.id`。payments の `Project` は従来どおり null。
- `find_users` の `name` は上の users の match に従う。

## 完了条件

- `mise run test` と `mise run lint` が通る。`test/snapshots/tools_list.json` を意図して更新する。
- 実在しない項目で一致するテストが残っていない(4 リソースのフィクスチャは `satisfies` と OpenAPI のキーの検査を通る)。
- バージョン 1.4.0(`package.json` と `src/info.ts`)、roadmap の Changelog、m04 / m05 の該当行に「仕様に照らした訂正(issue #27)」の注記。
