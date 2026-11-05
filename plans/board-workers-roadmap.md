# board Workers 版ロードマップ

BOARD API（https://api.the-board.jp/v1/）向けの Remote MCP サーバー。Cloudflare Workers + D1 + Hono。
Cloudflare MCP Server Portal の背後（Access の self-hosted / MCP server application + Managed OAuth）で動かす。
Go 製 CLI / ローカル MCP（v0.11.1）を書き直したもの。Go 版は `archive/go-cli` ブランチと `go-cli-final` タグに残す。

構成と規約は姉妹リポジトリ `../logbutler`（Backlog 用 Remote MCP）に揃える。違う点は「決定済み事項」に書く。

## 決定済み事項

| 項目 | 決定 | 理由・却下した案 |
|---|---|---|
| Go 版の扱い | `archive/go-cli` ブランチ + 注釈付きタグ `go-cli-final`（= v0.11.1）に残し、main からは削除する。Homebrew tap・goreleaser・`/board:docs` スキル・Lambda 配備例も廃止 | 2026-11-05 ユーザー決定。却下: `archive/` ディレクトリへの移動 |
| スコープ | Remote MCP のみ。CLI 専用の機能（configure / completion / docs / cache path）は移植しない | Workers で意味を持たない |
| ツール | **全ツールを移植**: api 層（22 リソースの list / get。Go 版 CLI に存在する組み合わせだけ。`show_meta` 引数あり）+ find 層 11 個（`find_*`、Go 版 MCP と同じ名前・引数）+ cache 系（`cache_status` / `cache_expire` / `cache_clear`）。合計 58 個以下 | 2026-11-05 ユーザー決定（「全ツールの完全移植」）。ツール名の接頭辞はなし（Portal が `board_` を前置する） |
| 認証 | **BOARD の API キー / トークンは Worker の secret（`BOARD_API_KEY` / `BOARD_API_TOKEN`）を全員で共有**する。Worker 自体は OAuth サーバーを持たない。Access アプリで保護し Managed OAuth を有効にする。Worker は `/healthz` 以外の全リクエストで `Cf-Access-Jwt-Assertion`（署名・iss・aud・exp）を検証する | 2026-11-05 ユーザー決定。BOARD API に OAuth がない。Cloudflare 公式: Managed OAuth を使う MCP server application は「`Cf-Access-Jwt-Assertion` の Access JWT を検証しなければならない」。logbutler の ADR 0009（自前 OAuth があるので Managed OAuth を使わない）とは逆の条件。却下: 利用者ごとのトークン入力（OAuth サーバーを Worker 内に持つ） |
| JWT 検証の実装 | logbutler で M06 に削除した `src/auth/access-jwt.ts`（jose・JWKS キャッシュ）と、そのテストを出発点にする（`git -C ../logbutler show 8dac72c^:<path>`） | 実績のあるコードの再利用 |
| 公開範囲 | `workers_dev: false`、カスタムドメインのみ。Access を迂回できる経路を作らない | JWT 検証は多層防御の 2 枚目 |
| デプロイ単位 | 1 デプロイ = 1 BOARD アカウント。profile は持たない | 共有キー方式 |
| キャッシュ | **D1**。Go 版の `resource_cache`（エンティティ単位の JSON）+ `sync_state` + `cache_meta` と、名前の正規化（NFKC + 小文字 + trim）、LLM が決める鮮度（`refresh` 引数の enum）を引き継ぐ | 2026-11-05 ユーザー決定。却下: KV、キャッシュなし |
| D1 のスキーマ適用 | Worker が isolate ごとの初回に冪等なスキーマ適用を行う（Go の `internal/cache/migrate.go` と同じ考え方）。テストで `cloudflare:test` の `applyD1Migrations` を使わない | logbutler の規約（`cloudflare:test` を import すると並列実行でタイムアウトする） |
| 流量制御 | **Durable Objects は使わない**。参照はキャッシュを優先し、1 回のツール呼び出し内では BOARD API を直列に呼ぶ。429 / 5xx は `Retry-After` を尊重して上限付きで再試行する。全件 refresh は D1 の条件付き UPDATE によるロック（Go の `refresh/lock.go` の TryLock + 429 相当、stale は 10 分） | 2026-11-05 ユーザー決定。少人数での利用を想定する。既知のリスク: 複数利用者が同時にキャッシュミスすると 3 回/秒を超えうる（429 の再試行で吸収する）。問題になったら DO による全体制御を再検討 |
| ランタイム・依存 | logbutler と同じ版に固定する（node 24.21.0 / pnpm 11.28.5 / agents 0.28.1 / @modelcontextprotocol/server 2.3.1 / hono 4.13.13 / zod 4.6.7 / vitest 4.1.11 / @cloudflare/vitest-plugin 1.4.1 / wrangler 4.151.0 / biome 2.5.15 / typescript 6.0.3、jose 6.2.12）。`^` `~` は使わない | 公開から 2 週間以上たった版に固定するという規約 |
| MCP | `agents` の `createMcpHandler`（ステートレス）。`McpAgent` は使わない | logbutler と同じ |
| 出力 | JSON を text content で返す。BOARD の生データは BOARD のキー（snake_case）のまま。エラーは `{"schema_version":"1","error":{"code","message","retryable"}}` を `isError: true` で返す。応答サイズの上限は `max_response_tokens`（logbutler の ADR 0007 と同じ） | logbutler と揃える |
| 秘密情報 | API キー・トークン・JWT をログ・エラー・MCP 応答に出さない | Go 版の原則を継承 |
| 設定チェック | 必須の vars / secrets / D1 が欠けていたり `<...>` のプレースホルダのままだったりする場合、`/healthz` は 503、それ以外は 500 を返し、項目名だけを返す（logbutler の ADR 0010） | 同上 |
| デプロイ設定 | logbutler の upstream / 下流モデル（`render-wrangler-config`）は**持ち込まない**。`wrangler.jsonc` はダミー値にし、実値は `mise.local.toml` の `[env]` から `wrangler.deploy.jsonc`（gitignore 対象）を生成する簡易版にする。デプロイはユーザーが行う | board は fork して自組織にデプロイする製品ではない |
| テスト | vitest + workerd。BOARD API はモックのみ（未登録の宛先への通信は例外にする）。実 API への確認は `mise run test:live`（デプロイ済み Worker 経由、ユーザーが実行）で、`mise run test` と CI には含めない | logbutler と同じ |
| エンティティの型 | Go の `internal/boardapi/*.go` の構造体を正とする（実 API と全フィールドを突合済み）。docs/specs や記憶から型を作らない | 準拠検証 38 マイルストーンの成果 |

## 移植元の参照

- Go 版のソース: `git show go-cli-final:<path>`。作業中は読み取り用の worktree `$SCRATCH/go-cli`（detached、編集しない）を使う
- 主要ファイル: `internal/boardapi/`（client・auth・retry・pagination・22 リソース）、`internal/cache/`（schema・fold）、`internal/refresh/`、`internal/repository/`（cache_filter・fetcher）、`internal/service/api/`、`internal/service/find/`（resolver・fanout_cap・reverse_map・contract_status・url）、`internal/mcpserver/tools.go`、`migrations/sqlite/`、`docs/specs/board_cli_mcp_ultra_detailed_design_ja.md`

## マイルストーン

| M | 内容 | 状態 |
|---|---|---|
| M00 | Go 版のアーカイブと削除 | 完了（74dfcde） |
| M01 | 基盤: mise / pnpm / biome / tsconfig / vitest、`/healthz`、設定チェック、Access JWT 検証、`createMcpHandler` とダミーツール 1 個、エラーエンベロープ、応答サイズ上限、CI、CLAUDE.md | 未着手 |
| M02 | BOARD API クライアント（x-api-key + Bearer、直列、再試行と Retry-After、ページング、`_meta` ヘッダ）と 22 リソースの型 | 未着手 |
| M03 | D1 キャッシュ（スキーマ適用、resource_cache / sync_state / cache_meta、名前の正規化）、refresh（enum、差分 / 全件、D1 ロック）、repository 層 | 未着手 |
| M04 | api 層のツール（22 リソースの list / get） | 未着手 |
| M05 | find 層のツール 11 個（名前による ID 解決・曖昧な場合のエラー、fanout の上限、逆引きマップ、contract_status、URL、enrichment の失敗を致命的にしない、cache 配列の同梱） | 未着手 |
| M06 | cache 系ツール、デプロイ設定の生成、runbook（Access アプリ・Managed OAuth・Portal の登録）、README、`test:live` | 未着手 |

各マイルストーンの完了条件: `mise run test`（unit + e2e）と `mise run lint` が通り、verifier が差分とテスト結果を確認していること。完了したらローカルでコミットする（push はしない）。

## Changelog

- 2026-11-05: 作成。M00 完了。
