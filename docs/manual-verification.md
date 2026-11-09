# 実機でしか確かめられない前提の一覧

テスト(`mise run test`)は workerd と、BOARD API・Access の JWKS のモックで動く。次の項目は実環境でしか確かめられない。
デプロイ後に確認し、記入欄に結果を書く(日付・確認者・結果。値に秘密情報を含めない)。
`mise run test:live` で測れるもの(V4・V5・V6)は、出力をそのまま転記する。手順は docs/runbook.md。

| ID | 前提 | 確認の方法 | 外れたときの影響 |
|---|---|---|---|
| V1 | Portal と Managed OAuth が `Cf-Access-Jwt-Assertion` を付けて Worker に届く(署名・iss・aud・exp が検証を通る) | Portal 経由と、Managed OAuth の直接接続の両方で `server_info` を呼ぶ。401 / 403 なら `wrangler tail` で `access jwt rejected: <理由>` を見る | 全ツールが使えない(`ACCESS_AUD` / `ACCESS_TEAM_DOMAIN` の誤り、または届かないヘッダー) |
| V2 | MCP の abort(クライアントの切断)で `signal` が発火し、BOARD への通信が止まる | `refresh=full` で時間のかかる呼び出しを送り、クライアントを途中で切断する。`wrangler tail` で、以降の BOARD 呼び出しが止まるか見る | 切断後も締め切り(30 秒)まで BOARD を叩く(rate limit の無駄。締め切りで必ず止まる) |
| V3 | D1 の `meta.changes` が条件付き UPDATE(ロックの取得・所有者条件つきの batch)で期待どおりの値を返す | `refresh=full` を 2 つ同時に送る。片方が `refresh_in_progress` になること。`cache_clear` を refresh 中に送ると何も消えず `refresh_in_progress` | ロックが効かず refresh が二重に走る、または所有者条件の書き込みが空振りしない |
| V4 | 実 API の 1 ページ(100 件)あたりの時間が見積もり(0.7〜1.3 秒)の範囲 | `test:live` の `refresh=full` の出力(1 ページあたり) | 30 秒の締め切りに収まるページ数が変わる。full の呼び出し回数と逆引きの走査の回数に影響 |
| V5 | 応答サイズ(`find_projects` の `limit` 付き、`list_*`)が `max_response_tokens` の既定(50000)に収まる | `test:live` の出力(応答 KiB)。`response_too_large` が出ないこと | 既定の上限を超えて `limit` が必須になる |
| V6 | `X-Total-Count` が返る(full refresh の件数照合に使う) | `list_*` を `show_meta: true` で呼び、`_meta` に総件数があること。`cache_status` の `full_prune_skipped` が false のままであること | 件数の照合ができず旧世代の行を消さない(`full_prune_skipped` が立ち続ける) |
| V7 | Workers Paid プランで full refresh と逆引きの走査が 1 回の呼び出しの D1 クエリ上限に収まる | `test:live` が最後まで通ること。D1 のエラー(`generic_error`)が出ないこと | Free プランでは収まらない(runbook 9) |
| V8 | 書類(見積・受注・納品・領収)の逆引きが 1〜2 回の呼び出しで覚え切る | 書類の ID を `find_estimates` などで引き、`cache_status` の `document_scans` が `complete` に進むのを見る | 初回のみ `project_unresolved` の警告が続く |
| V9 | Portal 経由のツール名が `board_` 付きで見え、`tools/list` の 54 個がすべて出る | Portal 経由でツール一覧を確認 | 名前の前提(README)が違う |
| V12 | workers.dev ホストで、`/healthz` だけを対象にした Bypass の Access アプリが、ホスト全体のアプリより優先される(未確認) | runbook の 5 の手順で作り、JWT なしで `/healthz` が 200、`/mcp` が Access に止められることを見る | `/healthz` が Access の内側のまま。ブラウザでログインして開く、または `wrangler tail` で確認する |
| V13 | 下流の `deploy.yml` が、main への push で `ci.yml` が成功したあとに `workflow_run` で起動し、Environment `production` の secrets が渡ってデプロイが通る(未確認) | 下流で main に push し、Actions の実行を確認する。PR の CI では起動せず、`ci.yml` が失敗したときは skipped になる | 自動デプロイが動かない。手動の `mise run deploy` に戻す(docs/deploy-ci.md) |
| V14 | 仕様(OpenAPI v1.9.0)に無い `GET /v1/{invoices,expenditure_payments,users,groups,payment_terms,project_types,expenditure_types,accounting_types,document_send_channels}/{id}` の 9 本が、実 API に存在するか(未確認。仕様には一覧しか無い) | `cache_clear` のあとに、対応する `get_*`(`get_invoice` / `get_payment` / `get_user` / `get_group` / `get_payment_term` / `get_project_type` / `get_purchase_type` / `get_accounting_type` / `get_document_send_channel`)を一覧で見えた ID で呼ぶ。キャッシュに当たると確認にならないので、必ず先に `cache_clear` する | 存在しなければ `not_found` になる(見えるエラー)。その場合は `get_*` を一覧から引く方式に変える |
| V15 | 請求書を案件で絞る(issue #31。未確認): `list_invoices` の `project_project_no_eq` / `project_name_cont` / `project_management_no_eq` と、`find_invoices` の `project_name` / `project_id` が、その案件の請求書だけを返す。実 API が `project_project_no_eq` を効かせるか | `find_invoices project_name=<案件名>` と `list_invoices project_project_no_eq=<案件番号>` を、請求書を持つ案件で呼ぶ。キャッシュに当たると API の確認にならないので、`cache_clear` のあとに呼ぶ | 他の案件の請求書が混ざる(`find_invoices` は後段で `project_id` を突き合わせるので混ざらないが、`list_invoices` は混ざる)。その場合は API が無視している |
| V16 | アーカイブ・失注を含めて取る(issue #35。未確認): (a) `list_projects refresh=full` を complete まで実行したあと、`cache_status` の projects の `row_count` が、BOARD の `include_archive_flg=1` かつ `include_lost_flg=1` の `total_count` と一致する。(b) `include_archive_flg=1` だけ、`include_lost_flg=1` だけの組み合わせの意味(失注・アーカイブのどちらが増えるか、両方が要るか)。(c) 案件をアーカイブ(または失注)にする操作で `updated_at` が更新され、`refresh=delta` で拾える | (a) `list_projects`(API 直接。`show_meta: true`)の `total_count` を、(指定なし / `include_archive_flg=1` / `include_lost_flg=1` / 両方)で比べる。次に `refresh=full` を complete まで回して `cache_status` の `row_count` を見る。(c) テスト用の案件をアーカイブして `refresh=delta` し、その案件の行が `archive_flg=1`(`response_group=large` の `get_project`)になるか見る | (a) 一致しなければ、どちらかのフラグの意味が想定と違う。refresh の既定のクエリを見直す。(c) アーカイブで `updated_at` が動かなければ、delta ではアーカイブへの変更を拾えない(次の `refresh=full` で入れ替わる。それまでは古い行のまま) |

## 記入欄

### V1 Portal / Managed OAuth と `Cf-Access-Jwt-Assertion`
- 確認日 / 確認者:
- Portal 経由: [ ] 届いた  [ ] 届かない  / メモ:
- Managed OAuth の直接接続: [ ] 届いた  [ ] 届かない  / メモ:
- service token(`test:live`)の JWT が `email` なし・`sub` 空・`common_name` = Client ID の形で届き、Worker が受け入れた(`kind: "service"`): [ ] はい  [ ] いいえ  / メモ:
  (形は Cloudflare の Application token の仕様に基づく。違っていたら `src/auth/access-jwt.ts` を直す)

### V2 abort が切断で発火するか
- 確認日 / 確認者:
- 結果: [ ] 発火した(切断後に BOARD 呼び出しが止まった)  [ ] 発火しない(締め切りまで続いた)
- メモ:

### V3 D1 の `meta.changes`
- 確認日 / 確認者:
- 二重 refresh の弾き: [ ] 期待どおり  [ ] 異常 / メモ:
- refresh 中の `cache_clear`: [ ] 何も消えず refresh_in_progress  [ ] 異常 / メモ:

### V4 実 API の 1 ページあたりの時間
- 確認日 / 確認者:
- users: ____ 件 / ____ 回で complete / 1 ページあたり ____ s
- clients: ____ 件 / ____ 回で complete / 1 ページあたり ____ s
- projects: ____ 件 / ____ 回で complete / 1 ページあたり ____ s
- 見積もり(0.7〜1.3 秒)との差:

### V5 応答サイズ
- 確認日 / 確認者:
- `list_users {limit:5}`: ____ KiB / `list_clients {limit:5}`: ____ KiB / `find_projects {limit:5}`: ____ KiB
- `response_too_large`: [ ] 出なかった  [ ] 出た(ツール:      )

### V6 `X-Total-Count`
- 確認日 / 確認者:
- `_meta` の総件数: [ ] あり  [ ] なし
- `full_prune_skipped`: [ ] false  [ ] true(リソース:      )

### V7 Paid プランでの D1 クエリ数
- 確認日 / 確認者 / プラン:
- 結果: [ ] 収まった  [ ] D1 のエラー / メモ:

### V8 逆引き
- 確認日 / 確認者:
- estimate: ____ 回で complete / order: ____ / delivery: ____ / receipt: ____
- `response_group=all` で一度に覚える案を試す必要: [ ] なし  [ ] あり

### V9 Portal のツール名
- 確認日 / 確認者:
- 結果: [ ] `board_` 付きで 54 個  / メモ:

### V10 workers.dev での Access と Managed OAuth(`customDomain` が空のとき。2026-11-06 追加)
- 確認日 / 確認者:
- Worker の Settings > Domains & Routes の workers.dev で Enable Cloudflare Access できた: [ ] はい  [ ] いいえ(メモ:      )
- MCP server application 型を workers.dev ホストで作れた: [ ] 作れた  [ ] self-hosted のみ  [ ] どちらも不可
- workers.dev で Managed OAuth が動き、`Cf-Access-Jwt-Assertion` が Worker に届く(JWT なしは 401、ログイン後は 200): [ ] はい  [ ] いいえ
- Version URL(`<version>-<worker>.<subdomain>.workers.dev`)が無効になっている(`preview_urls: false`。アクセスすると 404 などで Worker に届かない): [ ] 無効  [ ] 有効のまま(メモ:      )
- Portal に `https://<worker>.<subdomain>.workers.dev/mcp` を登録して Ready になる: [ ] はい  [ ] いいえ

### V11 Portal と Managed OAuth の経路が `Origin` を付けない(2026-11-06 追加。確認済み)
- `/mcp` は `Origin` ヘッダー付きのリクエストを 403 にする(`allowedOriginHostnames: []`、CORS 無効。ADR 0001 の追記)。
- 確認: 2026-11-06、運用者が下流の環境で確認した。
- 結果: [x] 確認済み。MCP Portal 経由と Managed OAuth 経由のどちらも `Origin` を付けず、ツール呼び出しが通る / メモ: ブラウザから直接呼ぶクライアント(MCP Inspector など)は 403 になる

### V12 workers.dev ホストで `/healthz` の Bypass アプリが優先される(2026-11-06 追加。**未確認**)
- 前提: Worker の workers.dev に Enable Cloudflare Access して、ホスト全体を保護するアプリがある状態で、同じホストの `/healthz` パスだけを対象にした self-hosted アプリ(Bypass ポリシー)を別に作る(docs/runbook.md の 5)。
- 期待: 「パスつきのアプリがホスト全体のアプリより優先される」ので、`/healthz` だけ認証なしで 200 になり、`/mcp` などそれ以外のパスは Access の内側のまま(JWT なしは Access のログインへの 302 か 401)。
- 確認日 / 確認者:
- `curl -i https://<worker>.<subdomain>.workers.dev/healthz` が 200 になる: [ ] はい  [ ] いいえ(Access に止められる)
- `/mcp` は引き続き Access に止められる(Bypass が広がっていない): [ ] はい  [ ] いいえ
- 外れたときの影響: `/healthz` は Access の内側のまま。runbook の確認手順(ブラウザでログインして開く / `wrangler tail`)を使う。`mise run setup` は Access の手前で止まったと案内して失敗にしない。

### V13 下流の `deploy.yml` が `workflow_run` で起動し、Environment の secrets が渡る(2026-11-06 追加。未確認)
- 確認日 / 確認者:
- `ci.yml` が main への push で成功したあとに `deploy.yml` が起動する: [ ] はい  [ ] いいえ(メモ:      )
- PR の CI(`event` が `pull_request`)ではデプロイが起動しない(`workflow_run` の対象は `branches: [main]` だけ): [ ] はい  [ ] いいえ
- `ci.yml` が失敗したとき、`deploy.yml` はデプロイしない(job が skipped): [ ] はい  [ ] いいえ
- `ref: head_sha` で CI が検査したコミットが checkout される(push が重なっても取り違えない): [ ] はい  [ ] いいえ
- Environment `production` の `CLOUDFLARE_API_TOKEN` が `deploy` のステップに渡り、デプロイが通る: [ ] はい  [ ] いいえ
- 下流で `deploy/wrangler.jsonc` をコミットするときの pre-commit の gitleaks が、upstream の `.gitleaks.toml` で `ACCESS_AUD` を止めない: [ ] はい  [ ] いいえ

### V14 仕様に無い `GET /v1/<リソース>/{id}` の 9 本(2026-11-09 追加。issue #28。未確認)
- 前提: OpenAPI v1.9.0 に載っているのは一覧(`GET /v1/<リソース>`)だけで、単体取得の `/{id}` は無い(請求書の詳細は `/documents/invoices/{id}`)。コードは Go 版のまま `/{id}` を呼ぶ。存在しなければ `not_found` になる。
- 手順: 各リソースの一覧で ID を 1 つ取り、`cache_clear` を実行してから、`get_*` をその ID で呼ぶ(`cache_clear` を挟まないと、キャッシュから返って確認にならない)。
- 確認日 / 確認者:
- invoices(`get_invoice`): [ ] 取れた  [ ] `not_found`
- expenditure_payments(`get_payment`): [ ] 取れた  [ ] `not_found`
- users(`get_user`): [ ] 取れた  [ ] `not_found`
- groups(`get_group`): [ ] 取れた  [ ] `not_found`
- payment_terms(`get_payment_term`): [ ] 取れた  [ ] `not_found`
- project_types(`get_project_type`): [ ] 取れた  [ ] `not_found`
- expenditure_types(`get_purchase_type`): [ ] 取れた  [ ] `not_found`
- accounting_types(`get_accounting_type`): [ ] 取れた  [ ] `not_found`
- document_send_channels(`get_document_send_channel`): [ ] 取れた  [ ] `not_found`
- 外れたときの影響: その `get_*` が `not_found` になる。`list_*` は影響を受けない。取れないものは一覧から ID で引く方式に変える。

### V15 請求書を案件で絞る(2026-11-09 追加。issue #31。未確認)
- 前提: `GET /v1/invoices` が受ける `project_project_no_eq` / `project_management_no_eq` / `project_name_cont` を送る(OpenAPI v1.9.0 に載っている)。`find_invoices` は案件番号で絞った結果を `project_id` でも突き合わせる。
- 手順: 請求書を持つ案件を 1 つ選び、`cache_clear` のあとに `find_invoices project_name=<案件名>` と `list_invoices project_project_no_eq=<案件番号>` を呼ぶ。
- 確認日 / 確認者:
- `find_invoices project_name=<案件名>` がその案件の請求書だけを返す: [ ] はい  [ ] いいえ
- `list_invoices project_project_no_eq=<案件番号>` がその案件の請求書だけを返す(キャッシュを空にして API 経由で): [ ] はい  [ ] いいえ
- `list_invoices project_name_cont=<案件名の一部>` / `project_management_no_eq=<管理番号>` が同様に効く: [ ] はい  [ ] いいえ
- 外れたときの影響: `list_invoices` が他の案件の請求書を返す。`find_invoices` は後段の `project_id` で除くので正しいが、取得件数が増える。

### V16 アーカイブ・失注を含めて取る(2026-11-10 追加。issue #35。未確認)
- 前提: `GET /v1/projects` は `include_archive_flg` / `include_lost_flg` を送らないとアーカイブ済み・失注を除外する(実機で、指定なし約 2,900 件、`include_archive_flg=true` 約 4,700 件、両方約 5,900 件。issue #35)。v1.8.0 の refresh と API 経路は、未指定なら両方 `=1` を送る。版(`query_version`)を見るのは `include_archive_flg` を受ける 14 リソースだけ。書類の逆引き走査の状態(`document_scan_state`)にも版があり、旧版は page 1 から走査し直す。
- 手順: `list_projects`(API 直接、`show_meta: true`)で件数を 4 通り(指定なし / `include_archive_flg=true` / `include_lost_flg=true` / 両方)取る。`list_projects refresh=full` を complete まで回し、`cache_status` を見る。
- 確認日 / 確認者:
- `cache_status` の projects の `row_count` が、両方 `true` の `total_count` と一致する: [ ] はい  [ ] いいえ(`row_count` = ____ / `total_count` = ____)
- `include_archive_flg=true` だけ・`include_lost_flg=true` だけの件数(それぞれが何を増やすか): ____ / ____
- 上げた直後、`cache_status` の `document_scans` が `not_started`、invoices / users などが `complete` のまま: [ ] はい  [ ] いいえ
- 案件をアーカイブすると `updated_at` が更新される(`refresh=delta` で拾える): [ ] はい  [ ] いいえ
- 同名の案件の片方がアーカイブ済みのとき、`find_invoices project_name=<案件名>` が曖昧エラー(候補 2 件)になる: [ ] はい  [ ] いいえ
- 外れたときの影響: `row_count` が合わなければ、キャッシュに欠け(または余り)がある。アーカイブで `updated_at` が動かなければ、`delta` は変更を拾えない(`full` で入れ替える)。
