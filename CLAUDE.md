# CLAUDE.md

> **下流リポジトリ**(upstream の board を取り込んだ自分の private リポジトリ)のセットアップ・デプロイでは、以下の「デプロイしない」等の規約は適用しない(upstream の開発時の規約)。秘密値の扱い(API キー・トークン・JWT を会話・ファイル・ログに出さない)と、コミットや push はユーザーの指示があるときだけ、は下流でも変わらない。`.claude/skills/board-setup/SKILL.md` に従う(Codex も同じパス。`.agents/skills/` からの symlink)。

board(BOARD API 向け Remote MCP サーバー。Cloudflare Workers + Hono + D1。ツール 54 個)の作業規約。
計画は `plans/`(ロードマップ `plans/board-workers-roadmap.md` と各マイルストーン `plans/board-workers-m*.md`)。使い方は `README.md`(英語)と `README_ja.md`(日本語。内容は同じ)、デプロイと運用は `docs/runbook.md`、設計の理由は `docs/adr/`、実機でしか確かめられない前提は `docs/manual-verification.md`。
構成と規約は姉妹リポジトリ `../logbutler` に揃えている。決定済み事項はロードマップが正で、勝手に覆さない。
Go 版は `archive/go-cli` ブランチと `go-cli-final` タグにある(このブランチには無い)。

## 絶対に守ること

- **TDD**: 失敗するテストを先に書き、`mise run test:unit` / `mise run test:e2e` で期待どおりの理由で失敗するのを確認してから実装する。テストを弱める・skip・削除しない。バグ修正は再現テストが先。
- **mise**: ランタイムとコマンドは `mise.toml` で管理する。`npm test` や `vitest` を直接叩かず `mise run <task>` を使う。新しいコマンドが要るなら先にタスクへ追加する。
- 完了条件は `mise run test`(unit + e2e)と `mise run lint` が通ること。デプロイ設定を触ったら `mise run deploy:check` も通す。
- **Durable Objects / McpAgent は使わない**。MCP はステートレス(`createMcpHandler`)。
- **BOARD の API キー・トークン(`BOARD_API_KEY` / `BOARD_API_TOKEN`)と Access の JWT を、ログ・エラー・MCP の応答に出さない**。
- **BOARD API はモックのみ**。unit / e2e テストで実 API に通信しない(`test/helpers/mock-fetch.ts` は未登録の宛先への通信を例外にする)。実 API への確認は `mise run test:live`(デプロイ済み Worker に Access の service token で接続。ユーザーが実行)だけで、`mise run test` と CI には含めない。
- 依存は公開から 2 週間以上たった正確なバージョンに固定する(`^` `~` を使わない)。`wrangler.jsonc` の `compatibility_date` も、固定した workerd が対応する日付まで。
- コミット・push・PR・リモート作成・デプロイ(`mise run deploy`、dry-run 以外の `wrangler deploy`)・`wrangler secret put`・D1 の作成・`mise run test:live` の実行は、ユーザーの指示があるときだけ。
- **ロックで守る書き込みは、必ず所有者条件つきの 1 つの batch で行う**(`WHERE ... lock_owner = ?` / `EXISTS (SELECT 1 ... WHERE lock_owner = ?)` を同じ batch に入れ、`meta.changes` でロックを失っていないか確かめる)。読んで確かめてから別の文で書く方式は、stale ロックを奪われたときに他人の状態を壊す。M03(full の再開)と M05(逆引きの走査)で同じ不具合が 2 回出た。新しいロックを足したら、ロックを奪われた場合のテストを書く。
- **`git diff` は `--no-ext-diff` を付ける**(外部の diff ツールの設定で出力が変わるのを避ける)。

## 認証

- OAuth サーバーは持たない。Worker は Cloudflare Access(self-hosted / MCP server application + Managed OAuth)の背後に置き、**`/healthz` 以外の全パス**で `Cf-Access-Jwt-Assertion`(署名・iss・aud・exp)を検証する(`src/auth/access-jwt.ts`、`src/app.ts` のミドルウェア)。JWT なしは 401、それ以外の失敗(JWKS が取れない場合を含む)は 403。理由は応答に出さずログに種別だけ残す。
- BOARD の認証情報は Worker の secret を全員で共有する(1 デプロイ = 1 BOARD アカウント)。
- `/mcp` の CORS と Origin は既定値に頼らず明示する(`src/app.ts` の `MCP_HANDLER_OPTIONS`: `corsOptions: false`、`allowedOriginHostnames: []`)。`Origin` ヘッダー付きの要求は 403。ブラウザからの呼び出しは想定しない(ADR 0001 の追記)。
- `board.config.json` の `customDomain` は任意。あればカスタムドメインだけで公開(`workers_dev: false`)、空なら workers.dev で公開する(`workers_dev: true`、`routes` なし)。どちらも `preview_urls: false`(Version URL を作らない)を明示する。`src/` は変えず、Worker が `/healthz` 以外で JWT を検証することで、workers.dev に直接来ても 401 / 403 になる(ADR 0001、2026-11-06 の追記)。

## 出力規約

- MCP ツールの出力は JSON を text content で返す(structuredContent は使わない)。BOARD の生データは BOARD のキー(snake_case)のまま。
- エラーは `{"schema_version":"1","error":{"code":"...","message":"...","retryable":false}}` を `isError: true` で返す(`src/errors.ts`)。想定外の例外は `generic_error` にして message を応答に出さない。
- 読み取りツールには引数 `max_response_tokens`(応答サイズの安全網。既定 50000、0 で無効)が `defineTool` で自動的に付く。超過は切り詰めず `response_too_large`(logbutler の ADR 0007 と同じ。`src/mcp/response-limit.ts`)。書き込み系は対象外、巨大な応答を返すテストは `max_response_tokens: 0` を付ける。
- 文字列の引数は 1000 文字、配列は 100 件まで。超えると `argument_error`(`src/mcp/args.ts`)。
- ツール名に接頭辞は付けない(MCP Server Portal が `board_` を前置する)。snake_case。
- **起動時の設定チェック**(`src/config-check.ts`。logbutler の ADR 0010 と同じ): 必須の vars / secrets / D1(`DB` / `ACCESS_TEAM_DOMAIN` / `ACCESS_AUD` / `BOARD_BASE_URL` / `BOARD_API_KEY` / `BOARD_API_TOKEN`)が欠けている・プレースホルダ(`<...>`)のままなら、`/healthz` は 503、それ以外は 500 で項目名だけを返す。ログにも応答にも**値は出さない**。必須の設定を足したら `checkConfig` と unit / E2E を更新する。
- GitHub Actions は**コミット SHA で固定**する(`.github/workflows/ci.yml`)。更新時は `gh api repos/<owner>/<repo>/git/ref/tags/<tag>` で実在を確かめる。

## 構成(M06 時点)

```
src/index.ts              default export { fetch }。checkConfig の後に Hono の app を呼ぶ
src/app.ts                Hono: GET /healthz(認証なし)、全パスの Access JWT 検証、/mcp(createMcpHandler。リクエストごとに McpServer を生成)
src/auth/access-jwt.ts    Cf-Access-Jwt-Assertion の検証(jose。JWKS は https://{team}/cdn-cgi/access/certs から取得し isolate 内で 5 分キャッシュ)
src/config-check.ts       起動時の設定チェック(BOARD_UI_BASE_URL は任意項目。あれば https のオリジン)
src/env.ts  src/info.ts   Env の型 / サーバー名とバージョン
src/errors.ts             エラーエンベロープ(BoardError / ArgumentError / ResponseTooLargeError / RefreshInProgressError)
src/board/                BOARD API クライアント(直列・再試行・Retry-After・ページング・締め切り)と 22 リソースの型(resources/)
src/cache/                D1 キャッシュ: schema(v4。ensureSchema。sync_state と document_scan_state の query_version でアーカイブ・失注が欠けた旧キャッシュ・旧走査を未同期扱いにする。版を見るのは include_archive_flg を受ける 14 リソースだけ。prune できなかった full は版を進めない)・resource-cache・sync-state・document-map(逆引き)・admin(cacheReport / clearCache / clearDocumentMap)・info(応答に同梱する鮮度)
src/refresh/              lock(D1 の条件付き UPDATE)と refresher(delta / full。どちらも再開できる)
src/repository/           cache → refresh → API の統一窓口(22 リソースの表 resources.ts)と書類 → 案件の逆引き(document-index.ts)
src/find/                 find 層(名前の解決・fanout・contract_status・url。resolveUiBaseUrl)
src/mcp/                  registry(defineTool / registerTools)・server・args・response-limit・call-context
src/mcp/tools/            index.ts の ALL_TOOLS に足す: server-info(1)・api(40)・find(11)・cache(2)= 54 個
scripts/deploy-config/    core.ts(純粋なコア。検証・生成・補助)+ main.ts(入口)。board.config.json と wrangler.jsonc から deploy/wrangler.jsonc を生成・検査
scripts/openapi-fields/  OpenAPI(BOARD 公式 v1.9.0。JSON はコミットしない)から test/helpers/openapi-fields.ts(22 リソースの応答キーと一覧のクエリ名)と src/board/openapi-support.ts(一覧が受ける共通フィルタ。delta → full の判定に使う)を生成。`mise run openapi:fields -- <json> [--check]`。仕様を更新したら再生成して型・クエリを合わせる
scripts/setup.sh          mise run setup の本体(前提の確認 → D1 → deploy:config → deploy → secrets → /healthz)。scripts/deploy.sh は mise run deploy の本体
deploy.example/           下流の CI の雛形(deploy.workflow.yml。下流が .github/workflows/deploy.yml にコピーする)
board.config.example.json 下流の設定の雛形(<YOUR_...>)。board.config.json と deploy/ は .gitignore(下流は git add -f で追跡する)
.claude/skills/board-setup/SKILL.md  下流リポジトリのセットアップ案内スキル(`.agents/skills/` から symlink)
test/unit/  test/e2e/  test/node-e2e/  test/live/  test/helpers/  test/snapshots/(tools/list のスナップショット)
docs/  plans/
```

- ツールは `defineTool` で定義し `src/mcp/tools/index.ts` の `ALL_TOOLS` に足す。D1 は `ctx.db()`、repository は `ctx.repositories()`、UI の URL は `ctx.uiBaseUrl`。
- `cache_expire` は作らない(Go では daily auto-refresh の廃止後、読み取りに影響しない印を付けるだけだったため)。`expireCache` は `src/cache/admin.ts` に残るがツールにはつないでいない。
- BOARD の UI の URL は `BOARD_UI_BASE_URL`(任意)があればそれ、なければ `BOARD_BASE_URL` の `://api.` を `://` にして導く。
- **upstream / 下流モデル**(ADR 0005): このリポジトリは公開の upstream で、`wrangler.jsonc` はローカル開発・テスト用のダミー値。**追跡ファイルに実値(Worker 名・D1 の id・ドメイン・Access のチーム / AUD)を書かない**。下流は `board.config.json`(秘密でない値だけ)を書き、`mise run deploy:config` が `deploy/wrangler.jsonc` を生成する(下流でコミット)。`wrangler.jsonc` のトップレベルを変えれば下流に pull で届き、`deploy:config:check` が下流で差分を検出する。`preview_urls: false` は維持する(`workers_dev` は `customDomain` の有無で `scripts/deploy-config/core.ts` が決める。upstream の値は使わない)。`wrangler.jsonc` に相対パスを持つキー(`main` / `$schema` 以外)を足したら、`scripts/deploy-config/core.ts`・unit・E2E・runbook を更新する。生成項目を足したときも同じ。
- `board.config.json` と `deploy/` は upstream の `.gitignore` に入っている。upstream で `git add -f` しない(CI の `no-downstream-files` が止める)。`BOARD_LIVE_*`(`test:live`)は `mise.local.toml` の `[env]` に置く別物。

## テストの書き方

- node-e2e の作業ディレクトリは `mkdtemp` を直接使わず `test/node-e2e/tmp.ts`(`.tmp/node-e2e/`。`BOARD_TEST_TMPDIR` で変えられる)を使い、子プロセスの `TMPDIR` もそこへ向ける。サンドボックスで node-e2e が落ちる場合は、失敗の全文を issue #14 に記録する(サンドボックス内で通るかは未確認)。
- 実行環境は workerd(`@cloudflare/vitest-plugin`)。`vitest.config.ts` のプロジェクトは `unit` / `e2e`(workerd)、`node-e2e`(Node。生成スクリプトと `setup.sh`〔wrangler・mise・curl を偽物に差し替える〕を実プロセスで起動し、`wrangler deploy --dry-run` まで通す。`mise run test:e2e` に含まれる)、`live`(`mise run test:live` だけ。`test` と CI には含めない)。
- `test/live/` と `test/node-e2e/` は `tsconfig.json` の対象外(Node 環境)。`test/unit/readme-tools.test.ts` は `README.md` と `README_ja.md` の両方のツール表と `ALL_TOOLS` を突き合わせる(ツールを足したら両方の表と総数〔`N tools` / `N 個`〕を直す。READMEを変えたら、もう一方の言語も合わせる)。
- D1 を使うテストは `test/helpers/d1.ts` の `testDb()` / `resetCache()`。ツールを呼ぶ unit は `test/helpers/api-tool.ts` の `callApiTool`(実物の D1 + モックした BOARD)。
- workerd のテストで `cloudflare:test` を import しない(import すると並列実行時に他ファイルの初回リクエストがタイムアウトする)。Worker 全体を任意の env で呼ぶときは `test/helpers/worker.ts` の `callWorker`(`import worker from "../../src/index"` に `cloudflare:workers` の `env` を上書きして渡す)。D1 のスキーマ適用でも `applyD1Migrations` は使わず、Worker が isolate ごとの初回に冪等に適用する(M03)。
- Access の JWKS は `vi.spyOn(globalThis, "fetch")` で差し替える(`mockOutboundFetch`。同じ isolate で動くので Worker にも効く)。テスト用の鍵と JWT は `test/helpers/access.ts`。JWKS のキャッシュは `clearJwksCache()` で各テストの前にリセットする。
- 時刻に依存する挙動は `vi.useFakeTimers({ toFake: ["Date"] })` で進める。コード側は `Date.now` の参照を保持せず、呼び出し時に読むこと。
- ツールを足す・変えたら `test/snapshots/tools_list.json` を意図して更新する: `mise run test:update-snapshots`。

## API の確認

`agents` / MCP SDK の API 名は記憶でなく `node_modules` の `.d.ts` と docs(`node_modules/agents/docs/mcp-servers.md`)で確認する。
