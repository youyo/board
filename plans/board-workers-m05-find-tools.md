# M05: find 層の MCP ツール 11 個

ロードマップ: `plans/board-workers-roadmap.md`(決定済み事項はそちらが正。ここでは覆さない。逆引きの方式だけは今回の決定としてロードマップに 1 行追記した)
前提: `plans/board-workers-m03-d1-cache.md`(repository 層)、`plans/board-workers-m04-api-tools.md`(api 層。末尾に『M05 での追補』)
移植元: Go 版 `internal/service/find/`(`go-cli-final` タグ)と `internal/mcpserver/tools.go`

## 目的

Go 版 MCP の `find_*` 11 個を Workers に移す。名前・引数・description は Go の `tools.go` と同じ。
データは M03 の repository からだけ取る(find 層は独自のキャッシュを持たない)。

## 完了条件

- `mise run lint` と `mise run test`(unit + e2e)が通る
- `tools/list` が 52 個(`server_info` + api 40 + find 11)。スナップショットを `mise run test:update-snapshots` で意図して更新した
- 実 BOARD API には通信しない(`mockOutboundFetch` は未登録の宛先を例外にする)。`cloudflare:test` は import しない

## 構成

| パス | 役割 |
|---|---|
| `src/find/session.ts` | 1 回のツール呼び出しの状態(`CallContext` は 1 つ)。repository の読み取り・refresh を『リソースごとに最初の 1 回だけ』にする・補助情報の取得をメモする・警告と `cache` を集める |
| `src/find/service.ts` | clients / vendors / users / projects / invoices / purchase_orders / payments の検索(Go の `find_*.go`) |
| `src/find/documents.ts` | 書類 4 種(見積・受注・納品・領収)。構造が同じなので表(`DOCUMENT_KIND_INFO`)で 1 つの実装にした |
| `src/find/resolver.ts` | 名前 → ID(0 件は not_found、複数は候補つきの argument_error) |
| `src/find/contract-status.ts` `filter.ts` `url.ts` `fanout.ts` `types.ts` | Go の同名ファイル(`types.ts` は検証と結果の型) |
| `src/repository/document-index.ts` `document-kinds.ts` | 書類 ID → 案件 ID の逆引き(下記) |
| `src/cache/document-map.ts` `schema.ts`(v2) | 逆引きの表と走査の進捗(D1) |
| `src/mcp/tools/find.ts` | 11 ツール。`src/mcp/call-context.ts` は api 層と共有する締め切りの作成 |

## ツール(11)

共通の引数: `refresh`(`none|delta|full`)・`limit`(0 以上の整数。0・未指定は無制限)・`max_response_tokens`。応答は `{ items, cache, warnings?, limited?, total_before_limit? }`。

| ツール | 引数(共通を除く) | 解決・絞り込み | 結果(items の 1 要素のキー) |
|---|---|---|---|
| `find_clients` | `id` `name` | id > name(部分一致)。顧客ごとに支店(`client_id_eq`)と担当者を付ける | `Client` `Branches` `Contacts` `url`(clients/{id}/edit) |
| `find_vendors` | `id` `name` | 同上。支店・担当者のキーは `payee_id_eq` | `Vendor` `Branches` `Contacts` `url`(payees/{id}/edit) |
| `find_users` | `id` `name` | id > name | `User` |
| `find_projects` | `id` `client_name` `name` `status` `statuses` `contract_status` | AND(id は他を無視)。`client_name` は曖昧ならエラー。status 系は後段の絞り込みで、id / client_name / name のどれかとの併用が必須・互いに排他・statuses は 10 件まで | `Project` `Client` `url`(projects/{id}/edit) |
| `find_estimates` | `id` `project_id` `client_name` `project_name` | id > project_id > client_name(+ project_name) > project_name。名前は fanout(5 件超は拒否) | `Estimate` `ProjectID` `ClientID` `Project` `Client` `url`(documents/{project_id}/edit) |
| `find_orders` | 同上 | 同上 | `Order` ほか同上 |
| `find_deliveries` | 同上 | 同上(案件の `deliveries` 配列の全件) | `Delivery` ほか同上 |
| `find_receipts` | 同上 | 同上(案件の `receipts` 配列の全件) | `Receipt` ほか同上 |
| `find_invoices` | `id` `client_name` `project_name`(未対応) `status` | AND。`client_name` は曖昧ならエラー。`status` は `status_eq` に委譲 | `Invoice` `Project` `Client` `url`(documents/{project_id}/edit) |
| `find_purchase_orders` | `id` `vendor_name` `project_name`(未対応) `status` | 同上(仕入先) | `PurchaseOrder` `Vendor` `Project` `url` |
| `find_payments` | `id` `vendor_name` `purchase_order_id`(未対応) `status` | 同上(仕入先) | `Payment` `Vendor` `Project`(常に null) |

- 結果のキーは Go の `json.Marshal` の出力と同じ: Result 構造体には `url` 以外に JSON タグがなく、Go の MCP は Go のフィールド名(`Client` `Branches` `ProjectID` ...)で返していた(`docs/api-reference.md` の例も同じ)。中のエンティティは BOARD のキー(snake_case)のまま。`url` は空なら出さない(`omitempty`)。
- 書類 4 種に `status` の引数はない(構造的に存在しない)。渡すと `z.strictObject` が `argument_error`(Go の handler reject と同じ結果)。
- 未対応の引数(`project_name` / `purchase_order_id`)はスキーマに残し、description に `(NOT YET SUPPORTED)`。渡すと `argument_error`(Go と同じ文面)。
- `contract_status` の写像(Go と同じ): active = 納品が 未着手/着手中/納品済、ended = 検収済(納品のみ評価)、prospect = 受注が 見積中(高/中/低/除)(受注のみ評価)、all = 全部。
- enrichment(補助情報)の失敗は致命的にしない: その項目は `null`、`warnings` に `{code: "enrichment_failed", resource, id, error_code}`。主体の取得の失敗は伝える(fail-fast)。

## `cache` 配列(読んだリソースの分だけ)

repository の各読み取りの結果の `cache` を、初めて出た順にまとめる(同じリソースは後の値で上書きしつつ `refresh_state` は引き継ぐ)。
**BOARD から直接取ったリソース(支店・担当者・API フォールバック・`response_group` 付きの案件)は載らない**(M04 の `cache` と同じ意味。`cache` は『キャッシュから返した鮮度』)。
Go は種別ごとの固定の一覧(`findResourceMap`)から全部を引いていた。検討して却下した案: 触ったリソース名を集めて `loadCacheInfos` で引く(API 直取りのリソースにも `{resource}` だけの空の項目が付き、LLM が『更新が必要』と誤解しやすい)。

## 逆引き(書類 ID → 案件 ID)の方式: 決定と根拠

### 問題
書類(見積・受注・納品・領収)のエンティティは案件 ID を持たない(Go の PoC で全 8 件の dump が確認済み)。`find_*(id=書類 ID)` で案件・顧客を付けるには逆引きが要る。
Go は miss のたびに projects を `response_group=<種別>` で**全件走査**して表を作った。実測(`plans/board-phase-n-m02-document-poc-report.md`): 2,425 件・25 ページで 16〜32 秒(estimate 19.5 秒 / order 16.5 秒 / delivery 16.9 秒 / receipt 31.9 秒)。10 秒を超えたら `ProjectID=0` に縮退していた。
Workers は 1 回の呼び出しの締め切りが 30 秒で、書類の取得・案件・顧客の取得も同じ枠に入るので、同じ方式では収まらない。

### 検討した案
| 案 | 結論 | 理由 |
|---|---|---|
| (a) projects のキャッシュ(書類を埋め込んだ response_group)から D1 上で引く | **却下** | M03 の refresh は `response_group` なしで取る(`RefreshTarget.buildQuery` は `updatedAtGteq` だけ)。キャッシュの案件に書類 ID が入っていないので引けない。refresh を変えると M03 の範囲外で、全リソースの取得量も増える |
| (b) D1 に逆引きの表を持ち、refresh 時に更新する | 部分的に採用 | 表は持つ。ただし更新を refresh に乗せない(上と同じ理由)。代わりに『find が案件を取るたびに記録する』と『表にない書類だけを再開できる走査で補う』 |
| (c) 締め切りが近ければ縮退する | 採用(安全網) | 走査を締め切りの手前で止め、縮退を応答の警告で伝える |
| Go と同じ全件走査 | 却下 | 上のとおり 30 秒に収まらない |

### 採った方式
1. **対応は D1 の表 `document_project_map(document_kind, document_id, project_id, recorded_at)` に残す**(スキーマ v2。M03 の `ensureSchema` の方式: 冪等な `CREATE TABLE IF NOT EXISTS` を `MIGRATIONS` に足し、版を `cache_meta` に書く)。対応は不変(書類は作られた案件にずっと属する)なので、一度覚えたら二度と引き直さない。Go の表は isolate/プロセスの寿命だったが、Workers では D1 に残るので全 isolate・全利用者で共有される。
2. **追加の通信なしで蓄積する**: find が案件を `response_group=<種別>` 付きで取るたび(`project_id` / `client_name` / `project_name` の各経路)、含まれる書類 ID を表に記録する。
3. **表にない書類(id 指定で miss)だけ、ページ単位の走査で補う**。`response_group=<種別>` で projects を 1 ページ(100 件)ずつ取り、**1 ページ取るごとにその場で表と進捗(`document_scan_state.next_page`)を同じ batch で書く**。見つけたら止める(進捗は残る)。次の呼び出しは保存した次のページから続ける。種別ごとに進捗は独立。
4. **締め切り内に必ず返る**:
   - 走査に使える時間 = `deadline - SCAN_RESERVE_MS(8 秒)`。この後に案件・顧客の取得(補助情報)が待っているため。クライアントにも同じ(予約を残した)締め切りを渡すので、1 ページが長引いても予約を食わない。
   - 1 ページ目に入る前に残り < `MIN_PAGE_BUDGET_MS(1.5 秒)`、または 2 ページ目以降で残り < `max(1.5 秒, 直前のページの所要時間 x 1.5)` なら取らずに止める。
   - 止めたら `unresolved(deadline)`。書類は返し `ProjectID=0`。
5. **縮退が応答から分かる**: `warnings` に `{code: "project_unresolved", resource, document_id, reason, message}`。`reason` は `deadline`(時間切れ。再実行で続きから進む)/ `scan_in_progress`(別の呼び出しが走査中)/ `api_error`(`error_code` つき)/ `not_found`(最後まで見たが無い)。
6. **秘密情報を含まない**: 警告にはコード・ID・固定文だけ。例外の文面と BOARD のエラー本文は載せない(E2E で確認)。
7. **ロック**: `document_scan_state` の条件付き UPDATE(M03 の `tryAcquireLock` と同じ考え方)。stale は 2 分(走査は 1 回の呼び出し = 30 秒以内に終わる)。取れなければ待たず `scan_in_progress`。
8. **古さ**: 最後まで走査して見つからなかった結果(`completed_at`)は 10 分(`COMPLETE_TTL_MS`)信用し、その間の miss は再走査しない。過ぎた miss は先頭から走査し直す(走査のあとに作られた書類を拾う)。

### 時間とレート制限の見積もり
- 1 ページ ≒ 0.7〜1.3 秒(Go の実測: 25 ページで 16〜32 秒)。走査に使える時間は `30 - 8 - (書類の取得 ≒ 1 秒) ≒ 21 秒` なので、1 回の呼び出しで約 16〜25 ページ。2,425 件(25 ページ)なら**最悪でも 2 回の呼び出しで 1 種別を覚え切る**。
- BOARD の 1 日 3,000 回に対して、4 種別の全件走査は最大 100 回(約 3%)で、**一度きり**(表に残る)。Go は 4 種別とも miss のたびに 25 ページを消費し、プロセスが替わるたびにやり直していた。
- 書類を新しく作った案件は、`project_id` / 名前の経路で案件を取れば記録される。id だけ分かる新しい書類は、最悪で TTL 後の再走査で解決する。
- 残るリスク: (1) ページ番号での再開は、走査の合間に前のほうの案件が削除・挿入されるとずれて、取りこぼす案件がありうる(M03 の full と同じ。取りこぼしは TTL 後の再走査か、案件経路の記録で拾う)。(2) 走査の途中に作られた書類は、その回の走査では拾えない(上の TTL)。(3) 最初の 1〜2 回は `ProjectID=0` の警告が出うる。

### 却下した案(補足)
- Durable Objects / Queues で裏の走査: 決定済み事項(DO は使わない)に反する。
- `response_group=all` で 1 回の走査にまとめる: PoC は種別ごとの実測しかなく、`all` のページごとの重さが未計測。必要なら `DOCUMENT_KIND_INFO.group` を変えて試せる(M06 の `test:live` で計測する候補)。

## Go との相違点

| 項目 | Go | Workers 版 | 理由 |
|---|---|---|---|
| 逆引きの表 | プロセス内、miss のたびに全件走査、10 秒で `ProjectID=0` に縮退 | D1 の表 + 再開できる走査 + 蓄積。締め切りの手前で止めて警告 | 上記 |
| 警告 | `slog.Warn`(LLM には見えない) | 応答の `warnings`(コードと ID だけ。最大 20 件 + `warnings_truncated`) | Workers のログは LLM に見えない。縮退を応答から分かるようにする決定 |
| `limit` の説明 | `default: 50, max: 100` | `0 or omitted = unlimited` + `limited` / `total_before_limit` の説明 | Go の実装は 0・未指定を無制限として扱い、上限も強制していない。説明が事実と違っていた |
| 切ったことの通知 | なし | `limited: true` と `total_before_limit`(切ったときだけ) | M04 の指摘。書類の fanout は、候補の書類の数を `total_before_limit` にする(404 で飛ばす分を含む上限) |
| 補助情報の取得 | errgroup で並列 | 逐次 | BOARD クライアントは 1 つで直列化されるので、並列にしても速くならない |
| 補助情報のメモ | なし(同じ顧客を案件ごとに引く) | 1 回の呼び出しの間は (リソース, ID) ごとに 1 度だけ | 通信と締め切りの節約 |
| `refresh` の掛かり方 | 全 repository 呼び出しに同じ指定 | リソースごとに最初の 1 回の読み取りにだけ。名前の解決は掛けない(Go と同じ)。書類は 1 件ごと(full = 取り直し) | 列挙のたびに refresh を走らせない |
| 検証の順序 | 名前の解決 → 検証 → 検索 | 検証(`contract_status` の値も) → 名前の解決 → 検索 | 無駄な通信をしない。結果は同じ |
| 書類 fanout の候補 | 顧客ごとに案件を取りながら、`limit` に達したら残りの顧客の案件検索を省く | 同じ。`limit` に達した時点で残りの顧客の案件検索を止める(このとき `total_before_limit` は見つけた候補数で、確定ではない)。候補の書類が 200 件(`FIND_DOCUMENT_CANDIDATE_CAP`)を超えたら、書類を取らずに `argument_error`(`project_id` か `limit` を促す) | 書類は 1 件ずつ取るので、締め切りとレート制限を守る |
| `contract_status` の検証 | `id` を指定したときは検証しない(ID が優先で、ほかの条件は無視) | `id` を指定したときも検証する(未知の値は `argument_error`) | 通信前に誤りを知らせる。Go との挙動差 |
| 逆引きの表の有効期限 | (走査の結果を 10 秒だけ持つ in-memory) | 対応(書類 ID → 案件 ID)に有効期限はない。書類は作られた案件にずっと属する前提で、一度覚えたら引き直さない(走査の『完了』だけが 10 分の TTL) | D1 に永続化するため。案件の付け替えがあっても反映されない |
| 曖昧・fanout 超過・未対応の引数 | 汎用のエラー | `argument_error`(retryable: false)。0 件は `not_found` | エンベロープの code |
| UI のベース URL | `BOARD_UI_BASE_URL` で上書き可 | `BOARD_BASE_URL` の `://api.` を `://` にして導く(M05 時点では上書きなし。**M06 で任意の `BOARD_UI_BASE_URL` を追加した**) | 設定項目を増やさない |
| `find_projects` の description | `docs/usage/maintenance-contract-search.md` への参照 | 参照を削除(リポジトリにない) | |
| 全 description | Go の文面 | Go の文面 + 応答の形(`cache` / `warnings` / `limited`)の説明、fanout 超過の 5 件の上限 | Workers 版で足した応答 |
| 結果のキー | Go のフィールド名(PascalCase)+ `url` | 同じ | 決定どおり |
| `id` 引数 | 0・未指定は『指定なし』 | 同じ(0 は『指定なし』、負の数・小数は argument_error)。`project_id` も同様 | Go に揃える(verifier の指摘で当初の『0 は argument_error』から変更) |

## テスト

`test/unit/`:
- `find-pure.test.ts`: contract_status(Go の `contract_status_test.go`)・filter(`filter_test.go`)・url・fanout の上限・曖昧な名前の候補(5 件 + `... (N more)`)・クエリの検証(`types_test.go`)
- `find-service.test.ts`: 11 ツールのサービス(偽の repository)。Go の `find_*_test.go` の転記: 優先順位・limit で補助情報を取らない・enrichment 失敗の非致命化と警告・主体の失敗の伝播・status / statuses / contract_status・narrowing・fanout の上限(5 は通り 6 は拒否)・404 の候補を飛ばす・refresh を最初の 1 回だけ・警告の上限
- `document-index.test.ts`: 逆引き(Go の `reverse_map_test.go` を読み替え): 記録して通信なしで引ける・走査して見つけたら止める・進捗を残して次は続きから・最後まで見て not_found・TTL・TTL 後の再走査・種別ごとの独立・時間切れで止まり進捗が残る・クライアントの締め切りのエラー・API エラーは警告のコードだけ・ロック(同時・stale・解放)
- `find-tools-definitions.test.ts`: 名前 11 個・引数が Go と同じ・書類に status がない・description の Go と同じ言葉・引数の検証(13 ケース。BOARD に通信しない)
- `cache-schema.test.ts`(変更): v2 の 2 表・v1 のデータベースに v2 だけを足す
- `tools-list-snapshot.test.ts`: スナップショットを 52 個に更新

`test/e2e/find-tools-flow.test.ts`(Worker 全体・Access JWT・BOARD はモック・D1 は実物):
11 ツールの成功パス / `find_projects` の曖昧エラー(候補 5 件)/ `find_estimates` の fanout(逆引きへの記録も確認)と 5 件超の拒否 / 逆引きの縮退(`Date` だけを偽装して 1 ページごとに 10 秒進める → ProjectID=0 + `deadline` の警告・進捗が残る・2 回目が続きから解決・3 回目は走査しない)/ 最後まで見て無い書類 / `limit` の `limited` / `cache` 配列の中身 / enrichment 失敗の警告 / 秘密情報(正常・縮退・警告・エラーと D1 の全テーブル)。

## 変更した既存ファイル(M05 本体)

`src/cache/schema.ts`(v2)、`src/mcp/args.ts`(`optId`)、`src/mcp/registry.ts`(`ToolContext` に `documentIndex` と `uiBaseUrl`)、`src/mcp/server.ts`(逆引きと UI の URL の組み立て)、`src/mcp/tools/index.ts`、
`test/helpers/d1.ts`(`resetCache` が新しい 2 表も空にする)、`test/helpers/api-tool.ts`、`test/unit/server-info.test.ts` `test/e2e/api-tools-flow.test.ts` `test/e2e/mcp-flow.test.ts` `test/unit/api-tools-definitions.test.ts`(ツール数 41 → 52)。

## やらないこと

cache 系ツール・README・デプロイ設定(M06)、依存の追加、Durable Objects、コミット・push。

## 既知のリスク・未解決

- **逆引きの最初の 1〜2 回**: 表が空のうちは、id 指定の書類が `ProjectID=0` + `deadline` の警告で返りうる(再実行で続きから進む)。`response_group=all` で一度に覚える案は M06 の `test:live` で計測してから判断する。
- **`cache_clear` と逆引きの表**: `projects` を消しても逆引きの表は残る(対応は不変なので害はない)。M06 で『既定では残し、`include_document_map: true` で消せる』と決めた。
- **応答サイズ**: find_clients / find_vendors / find_users は `limit` なしで多数にマッチすると、補助情報の取得が増え、応答も大きくなる(`response_too_large` は `limit` を案内する)。
- **補助情報の通信量**: 顧客 N 件にマッチすると支店・担当者で 2N 回の BOARD 呼び出し(Go と同じ)。締め切りで失敗した分は `enrichment_failed` の警告になる(警告は 20 件で打ち切り)。
- **書類 fanout の途中の失敗**: 書類の取得が 404 以外で失敗すると、それまでの結果は返さずエラーにする(Go と同じ)。取れた書類は D1 に残るので、再試行は続きから進む。
