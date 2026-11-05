# M01: 基盤

ロードマップ: `plans/board-workers-roadmap.md`(決定済み事項はそちらが正。ここでは覆さない)

## 目的

Workers 版 board の土台を作る。Cloudflare Access の JWT で保護された `/mcp` が、疎通確認用のダミーツール
`server_info` を返すところまでを、設定チェック・エラーエンベロープ・応答サイズ上限を含めて通しで動かす。
BOARD API にはまだ通信しない(M02 以降)。構成と書き方は姉妹リポジトリ `../logbutler` に揃える。

## 完了条件

- `mise run lint` と `mise run test`(unit + e2e)が通る
- E2E(Worker 全体への HTTP。Access の JWKS は outbound fetch のモック、`cloudflare:test` は import しない)で次が確認できる
  1. `/healthz` は JWT なしで 200 `{"status":"ok"}`
  2. `/healthz` 以外は JWT なしで 401。不正な JWT(期限切れ・aud 違い・署名不正・別チーム・壊れた文字列)と、JWKS が取れないときは 403。応答とログに JWT・鍵を出さない
  3. 正しい JWT なら `/mcp` の `initialize` → `tools/list` → `tools/call server_info` が通る
  4. 設定が欠けている・プレースホルダのままなら、`/healthz` は 503、それ以外は 500。項目名だけを返し、値は出さない
- `mise run build`(`wrangler deploy --dry-run`)でバンドルできる

## 作業

1. ツールチェーン: `mise.toml`(node 24.21.0 / pnpm 11.28.5。タスク install / test:unit / test:e2e / test / lint / fmt / build / dev / test:update-snapshots)、`package.json`(依存は完全一致で固定。`jose` 6.2.12 を含み、`@cloudflare/workers-oauth-provider` / `@modelcontextprotocol/client` / `yaml` は入れない)、`pnpm-lock.yaml`、`biome.json`、`tsconfig.json`、`vitest.config.ts`(unit / e2e の 2 プロジェクト)、`.gitignore`、`.dev.vars.example`
2. `wrangler.jsonc`: name `board`、`workers_dev: false`、observability 有効、D1 `DB`(ダミーの database_id。M03 で使う)、vars `ACCESS_TEAM_DOMAIN` / `ACCESS_AUD`(プレースホルダ)/ `BOARD_BASE_URL`(既定 `https://api.the-board.jp`)。secrets `BOARD_API_KEY` / `BOARD_API_TOKEN` はコメントで説明
3. `src/config-check.ts`: 必須項目(`DB` / `ACCESS_TEAM_DOMAIN` / `ACCESS_AUD` / `BOARD_BASE_URL` / `BOARD_API_KEY` / `BOARD_API_TOKEN`)の欠け・プレースホルダ・形式違いを検出。`src/index.ts` が Hono の前に呼ぶ
4. `src/auth/access-jwt.ts`: logbutler の `8dac72c^` から復元(jose・JWKS の 5 分キャッシュ・未知の kid での再取得は 60 秒以上あけて・`requiredClaims: ["exp"]`)。`src/app.ts` の Hono ミドルウェアで `/healthz` 以外の全パスに適用。JWT なしは 401、それ以外の失敗は 403
5. MCP: `src/app.ts` の `/mcp` が `createMcpHandler`(リクエストごとに `createMcpServer()`)。`src/mcp/registry.ts`(`defineTool`・`registerTools`。toolset / read_only はまだ持たない)、`src/errors.ts`(エンベロープ)、`src/mcp/response-limit.ts`(`max_response_tokens`、logbutler の ADR 0007 と同じ)、`src/mcp/tools/server-info.ts`(名前とバージョンだけを返す)
6. `.github/workflows/ci.yml`(logbutler と同じアクション・SHA で lint と test。deploy:check は M06)、`CLAUDE.md`

## やらないこと

OAuth サーバー(`@cloudflare/workers-oauth-provider`)、Durable Objects、BOARD API への通信、D1 の作成・スキーマ適用(M03)、
デプロイ設定の生成(M06)、toolsets / read_only のクエリ、コミット・push・デプロイ・`wrangler secret put`。

## TDD の進め方

テストを先に書き、`mise run test:unit` / `mise run test:e2e` が「実装がない」理由で失敗するのを確認してから実装する。

## テスト一覧

### unit(`test/unit/`)
- `config-check.test.ts`: すべてそろえば ok / 各項目の欠け / 空白だけ / D1 でない値 / プレースホルダ / `BOARD_BASE_URL` と `ACCESS_TEAM_DOMAIN` の形式 / 結果に値を載せない
- `access-jwt.test.ts`: 正しい JWT / ヘッダー欠落 / 期限切れ / exp なし / aud 違い / issuer 違い / 署名不正 / JWKS 取得失敗(fail closed) / JWKS キャッシュ / 鍵ローテーション追従と 60 秒の最小間隔
- `errors.test.ts`: エンベロープの形 / コード列挙 / `BoardError`・`ArgumentError`・`ResponseTooLargeError` / 想定外の例外は内部情報を漏らさない
- `response-size-limit.test.ts`: トークン推定 / 既定 50000 / 上限ちょうどと +1 / 0 で無効 / 負の値・小数 / `responseLimit: false` と書き込みは対象外
- `tool-registry.test.ts`: 注釈 / 名前の検証 / 成功の JSON / 入力不正は `argument_error` / `BoardError` のコード / 想定外の例外は `generic_error` でログにも秘密を出さない / 名前の重複
- `server-info.test.ts`: ALL_TOOLS が 1 個 / 名前とバージョンだけを返す / 読み取り専用の注釈
- `tools-list-snapshot.test.ts`: `tools/list` のスナップショット(`test/snapshots/tools_list.json`)

### E2E(`test/e2e/`)
- `healthz.test.ts`: JWT なしで 200、JWKS を取りに行かない
- `access-protection.test.ts`: JWT なしは全パスで 401 / 不正な JWT は 403 で JWT・鍵を出さない / JWKS 取得失敗は 403 / ログに理由の種別だけ / 正しい JWT は `/mcp` に到達 / 存在しないパスは 404
- `mcp-flow.test.ts`: initialize → tools/list → tools/call / 応答に秘密を含めない / 不正な引数と `response_too_large` のエンベロープ / JWKS は 1 回だけ取得し BOARD には通信しない
- `config-check.test.ts`: secret 欠け(500・本文に項目名なし・ログに項目名だけ・設定チェックが認証より先)/ `/healthz` は 503 と項目名 / D1 欠け / プレースホルダ / 正常時
