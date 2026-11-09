# issue #28: OpenAPI 仕様と食い違う残りのリソースを直す

## 背景

- #27(PR #29)で users / invoices / purchase_orders / payments を BOARD 公式の OpenAPI 仕様 v1.9.0(`https://developers.the-board.jp/doc/board_openapi.json`)に合わせた。その調査で、残りのリソースにも仕様と食い違う箇所が見つかった(issue #28 の本文)。
- #27 と同じ「エラーにならず無視される・0 件になる」種類の不具合を、仕様からの機械的な検査で捕まえられるようにするのが主目的。

## 決定事項

### 検査(先に作る。Red はここから)

- **生成スクリプト**を `scripts/openapi-fields/` に置く(入口は `mise run` のタスクにする。例: `mise run openapi:fields -- <board_openapi.json のパス>`)。OpenAPI の JSON はコミットしない。生成物(`test/helpers/openapi-fields.ts`。#27 の手書きの定数を置き換える)の先頭に、仕様のバージョン(1.9.0)と取得元の URL を書く。
  - 22 リソース全部の**応答キー**(一覧と単体取得。response_group で増えるキーを含む和集合。#27 の expenditures と同じ扱い)
  - 一覧の**クエリ名**(`x[]` の形も含めて仕様の parameters の名前どおり)
- 全リソースの型に `keyof Entity ⊆ 仕様の応答キー` の型レベルの検査を掛ける(#27 の `test/unit/openapi-entity-shape.test.ts` を広げる)。index signature が残っていれば落ちる。
- 全リソースの `build*Query` が出すキーが仕様のクエリ名の部分集合であることを unit で検査する(キャッシュ専用のキーは API に送らないので対象外。明示の許可リストは作らない)。

### 一覧のクエリ

- client_branches / contacts / vendor_branches / vendor_contacts の `name_cont` / `email_cont` は**引数ごと削除**する(API に無い。これらは親の ID〔`client_id_eq` / `payee_id_eq`〕で取るのが主経路で、find 層も親の ID で取る)。
- 共通フィルタ(`updated_at_gteq` / `updated_at_lteq` / `include_archive_flg`)は、仕様のクエリ名にあるリソースにだけ付ける。どのリソースに付けるかは生成した定数から導く(手で表を書かない)。
- 仕様にあって未対応だったものを足す(サーバー専用。キャッシュでは絞らない):
  - project_types / purchase_types / accounting_types の `project_type_kbn_eq` / `expenditure_type_kbn_eq` / `accounting_type_kbn_eq`(1〜3。未指定は 1 と同じ)
  - projects の一覧の `response_group` に `medium` と `project_cost`
- vendors の `custom_no_eq` などの追加はしない(範囲外)。

### キャッシュ

- clients の `custom_no_eq` は**サーバー専用に降格**する(cacheable から外す)。`custom_no` は small に含まれず、#27 の「キャッシュで絞るのは実データに出る項目だけ」に合わせる。
- delta refresh: 仕様の一覧クエリに `updated_at_gteq` が無いリソースは、delta を **full として実行**し、`mode: "full"` を返す(カーソルのない delta と同じ分岐)。対象は生成した定数から導く(users / groups / payment_terms / 3 つの区分 / document_send_channels / project_costs の想定)。既存の下流では、これらの次の delta が全件の取得になる。

### 型

- 全リソースの型を仕様の応答キーに合わせる。仕様に無い項目(vendors の `code` / `memo`、区分などの `memo`、projects に埋め込む `project_costs` の `id` / `project_id` / `created_at` / `updated_at` など)を消す。
- vendor_branches / vendor_contacts の入れ子 `vendor` は `payee` に改名する。読んでいる箇所(find 層など)があれば合わせる。payee_branches の `payee` は仕様の schema が `{}` で形が未確認なので、その旨のコメントを残す。

### 単体取得のパス

- 仕様に無い `GET /v1/{invoices,expenditure_payments,users,groups,payment_terms,project_types,expenditure_types,accounting_types,document_send_channels}/{id}` は、**コードを変えない**。一次証拠が無く、存在しなければ `not_found` という見えるエラーになるため(無言で 0 件になる #27 とは重さが違う)。`docs/manual-verification.md` に V14 として 9 本を列挙し、下流で確かめてもらう(`cache_clear` のあとに ID で取得する。キャッシュに当たると確認にならない)。

### バージョン

- **1.5.0**(ツールの引数がまた変わる。1.4.0 はタグ未打ちだが、下流は main を取り込んでいるので、版番号で切り分けられるようにする)。

## 完了条件

- `mise run test` と `mise run lint` が通る。`test/snapshots/tools_list.json` を意図して更新する。
- 生成スクリプトを再実行しても `test/helpers/openapi-fields.ts` に差分が出ない。
- roadmap の Changelog、m03 / m04 の該当行に「仕様に照らした訂正(issue #28)」の注記、manual-verification の V14。
