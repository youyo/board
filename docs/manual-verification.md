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
