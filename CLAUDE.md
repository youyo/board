# CLAUDE.md

board(BOARD API 向け Remote MCP サーバー。Cloudflare Workers + Hono。キャッシュは D1 を予定)の作業規約。
計画は `plans/`(ロードマップ `plans/board-workers-roadmap.md` と各マイルストーン `plans/board-workers-m*.md`)。
構成と規約は姉妹リポジトリ `../logbutler` に揃えている。決定済み事項はロードマップが正で、勝手に覆さない。
Go 版は `archive/go-cli` ブランチと `go-cli-final` タグにある(このブランチには無い)。

## 絶対に守ること

- **TDD**: 失敗するテストを先に書き、`mise run test:unit` / `mise run test:e2e` で期待どおりの理由で失敗するのを確認してから実装する。テストを弱める・skip・削除しない。バグ修正は再現テストが先。
- **mise**: ランタイムとコマンドは `mise.toml` で管理する。`npm test` や `vitest` を直接叩かず `mise run <task>` を使う。新しいコマンドが要るなら先にタスクへ追加する。
- 完了条件は `mise run test`(unit + e2e)と `mise run lint` が通ること。
- **Durable Objects / McpAgent は使わない**。MCP はステートレス(`createMcpHandler`)。
- **BOARD の API キー・トークン(`BOARD_API_KEY` / `BOARD_API_TOKEN`)と Access の JWT を、ログ・エラー・MCP の応答に出さない**。
- **BOARD API はモックのみ**。unit / e2e テストで実 API に通信しない(`test/helpers/mock-fetch.ts` は未登録の宛先への通信を例外にする)。実 API への確認は `mise run test:live`(M06 で追加。デプロイ済み Worker 経由でユーザーが実行)だけで、`mise run test` と CI には含めない。
- 依存は公開から 2 週間以上たった正確なバージョンに固定する(`^` `~` を使わない)。`wrangler.jsonc` の `compatibility_date` も、固定した workerd が対応する日付まで。
- コミット・push・PR・リモート作成・デプロイ・`wrangler secret put`・D1 の作成は、ユーザーの指示があるときだけ。

## 認証

- OAuth サーバーは持たない。Worker は Cloudflare Access(self-hosted / MCP server application + Managed OAuth)の背後に置き、**`/healthz` 以外の全パス**で `Cf-Access-Jwt-Assertion`(署名・iss・aud・exp)を検証する(`src/auth/access-jwt.ts`、`src/app.ts` のミドルウェア)。JWT なしは 401、それ以外の失敗(JWKS が取れない場合を含む)は 403。理由は応答に出さずログに種別だけ残す。
- BOARD の認証情報は Worker の secret を全員で共有する(1 デプロイ = 1 BOARD アカウント)。
- `workers_dev: false`。カスタムドメインだけで公開し、Access を迂回できる経路を作らない。

## 出力規約

- MCP ツールの出力は JSON を text content で返す(structuredContent は使わない)。BOARD の生データは BOARD のキー(snake_case)のまま。
- エラーは `{"schema_version":"1","error":{"code":"...","message":"...","retryable":false}}` を `isError: true` で返す(`src/errors.ts`)。想定外の例外は `generic_error` にして message を応答に出さない。
- 読み取りツールには引数 `max_response_tokens`(応答サイズの安全網。既定 50000、0 で無効)が `defineTool` で自動的に付く。超過は切り詰めず `response_too_large`(logbutler の ADR 0007 と同じ。`src/mcp/response-limit.ts`)。書き込み系は対象外、巨大な応答を返すテストは `max_response_tokens: 0` を付ける。
- ツール名に接頭辞は付けない(MCP Server Portal が `board_` を前置する)。snake_case。
- **起動時の設定チェック**(`src/config-check.ts`。logbutler の ADR 0010 と同じ): 必須の vars / secrets / D1(`DB` / `ACCESS_TEAM_DOMAIN` / `ACCESS_AUD` / `BOARD_BASE_URL` / `BOARD_API_KEY` / `BOARD_API_TOKEN`)が欠けている・プレースホルダ(`<...>`)のままなら、`/healthz` は 503、それ以外は 500 で項目名だけを返す。ログにも応答にも**値は出さない**。必須の設定を足したら `checkConfig` と unit / E2E を更新する。
- GitHub Actions は**コミット SHA で固定**する(`.github/workflows/ci.yml`)。更新時は `gh api repos/<owner>/<repo>/git/ref/tags/<tag>` で実在を確かめる。

## 構成(M01 時点)

```
src/index.ts              default export { fetch }。checkConfig の後に Hono の app を呼ぶ
src/app.ts                Hono: GET /healthz(認証なし)、全パスの Access JWT 検証、/mcp(createMcpHandler。リクエストごとに McpServer を生成)
src/auth/access-jwt.ts    Cf-Access-Jwt-Assertion の検証(jose。JWKS は https://{team}/cdn-cgi/access/certs から取得し isolate 内で 5 分キャッシュ)
src/config-check.ts       起動時の設定チェック
src/env.ts  src/info.ts   Env の型 / サーバー名とバージョン
src/errors.ts             エラーエンベロープ(BoardError / ArgumentError / ResponseTooLargeError)
src/mcp/registry.ts       defineTool / registerTools(入力は自前で検証してエンベロープにする)
src/mcp/response-limit.ts 応答サイズの上限
src/mcp/server.ts         McpServer のファクトリ
src/mcp/tools/            ツール(index.ts の ALL_TOOLS に足す)。今は疎通確認用の server_info だけ
test/unit/  test/e2e/  test/helpers/  test/snapshots/(tools/list のスナップショット)
```

未実装(BOARD API クライアント、D1 キャッシュ、api 層 / find 層 / cache 系のツール、デプロイ設定の生成、runbook、`test:live`)はロードマップ参照。

## テストの書き方

- 実行環境は workerd(`@cloudflare/vitest-plugin`)。`vitest.config.ts` の `unit` / `e2e` の 2 プロジェクト。
- workerd のテストで `cloudflare:test` を import しない(import すると並列実行時に他ファイルの初回リクエストがタイムアウトする)。Worker 全体を任意の env で呼ぶときは `test/helpers/worker.ts` の `callWorker`(`import worker from "../../src/index"` に `cloudflare:workers` の `env` を上書きして渡す)。D1 のスキーマ適用でも `applyD1Migrations` は使わず、Worker が isolate ごとの初回に冪等に適用する(M03)。
- Access の JWKS は `vi.spyOn(globalThis, "fetch")` で差し替える(`mockOutboundFetch`。同じ isolate で動くので Worker にも効く)。テスト用の鍵と JWT は `test/helpers/access.ts`。JWKS のキャッシュは `clearJwksCache()` で各テストの前にリセットする。
- 時刻に依存する挙動は `vi.useFakeTimers({ toFake: ["Date"] })` で進める。コード側は `Date.now` の参照を保持せず、呼び出し時に読むこと。
- ツールを足す・変えたら `test/snapshots/tools_list.json` を意図して更新する: `mise run test:update-snapshots`。

## API の確認

`agents` / MCP SDK の API 名は記憶でなく `node_modules` の `.d.ts` と docs(`node_modules/agents/docs/mcp-servers.md`)で確認する。
