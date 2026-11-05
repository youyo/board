# M06: cache 系ツール・デプロイ設定・runbook・README・test:live

ロードマップ: `plans/board-workers-roadmap.md`(決定済み事項はそちらが正。ここでは覆さない)
前提: M01〜M05(ツール 52 個。`server_info` + api 40 + find 11)
手本: 姉妹リポジトリ `../logbutler`(README・docs/runbook.md・docs/manual-verification.md・scripts/render-wrangler-config/・test/node-e2e/・test/live/・test/unit/readme-tools.test.ts)

## 目的

Workers 版の最後のマイルストーン。cache 系ツールを足して Go 版の全ツールを移し(54 個)、デプロイと運用に必要なもの(デプロイ設定の生成、runbook、実機確認の `test:live`、README、ADR)をそろえる。

## 完了条件

- `mise run lint`・`mise run test`(unit + e2e + node-e2e)・`mise run deploy:check` が通る
- `tools/list` が 54 個(スナップショットを `mise run test:update-snapshots` で意図して更新)
- 実 BOARD API・実 Cloudflare には通信しない。デプロイ・`wrangler secret put`・D1 の作成・`test:live` の実行はしない(ユーザーが行う)

## 作ったもの

| 領域 | パス | 内容 |
|---|---|---|
| cache ツール | `src/mcp/tools/cache.ts` | `cache_status`(ReadOnly)/ `cache_clear`(Destructive、`destructiveHint: true`) |
| cache の土台 | `src/cache/admin.ts` | `cacheReport`(状態の組み立て)・`clearDocumentMap`(逆引きの削除)を追加。`CacheStatusRow.full_prune_skipped` を追加 |
| ToolContext | `src/mcp/registry.ts` `src/mcp/server.ts` | `ctx.db()` を追加(D1 を cache ツールに渡す) |
| UI の URL | `src/find/url.ts` `src/config-check.ts` `src/env.ts` `src/mcp/server.ts` | 任意の `BOARD_UI_BASE_URL`。`resolveUiBaseUrl` |
| デプロイ設定 | `scripts/render-wrangler-config/{core,main}.ts` | wrangler.jsonc + `BOARD_*` → `wrangler.deploy.jsonc`(gitignore 済み) |
| mise | `mise.toml` | `deploy:render` / `deploy:check` / `deploy` / `test:live`。`test:e2e` に node-e2e を含めた |
| CI | `.github/workflows/ci.yml` | `mise run deploy:check` を追加 |
| live | `test/live/live.test.ts` `test/live/helpers/live.ts` `vitest.config.ts` | service token で接続して計測。環境変数がなければ全体を skip |
| 文書 | `README.md` `docs/runbook.md` `docs/manual-verification.md` `docs/adr/0001〜0004` `CLAUDE.md` | |

## cache ツール

### `cache_status`(引数なし)
BOARD を呼ばず D1 だけを読む。応答:

```
{
  resources: [{ resource, cached_at?, full_refreshed_at?, refresh_state, row_count, full_prune_skipped, locked, last_error_code? }]  // 22 件
  document_scans: [{ document_kind, scan_state, mapped_rows, next_page?, started_at?, completed_at?, locked }]                      // 4 種別
}
```

- `refresh_state`: `partial`(`full_next_page` あり)> `failed`(直近の `last_sync_status = error`。`last_error_code` つき)> `complete`(同期済み)> `not_synced`(一度も同期していない。この値は `cache` 配列の `refresh_state`(complete / partial / failed)にはない新しい値で、『未取得』を明示するため)。
- `locked`: stale(10 分。逆引きは 2 分)でないロックがある。
- `document_scans.scan_state`: `not_started` / `in_progress`(`next_page` から再開)/ `complete`。

### `cache_clear`(`resources?: string[]`、`include_document_map?: boolean`)
- M03 の `clearCache` を使う(ロックを先に取り、所有者条件つきの batch で消す)。refresh 中は何も消さず `refresh_in_progress`(`retry_after_seconds` 30)。未知のリソースは `argument_error`。
- 逆引きの対応表は消さない(対応は不変)。`include_document_map: true` で `clearDocumentMap` も実行する: 種別ごとの走査ロックを先に取り、所有者条件つきの batch で `document_project_map` と `document_scan_state` を消す。
- `include_document_map` の逆引きが走査中で消せなかった場合は、リソースの削除は済んでいるので**エラーにせず**、`{ cleared, document_map_cleared: false, document_map_error: { code: "refresh_in_progress", ..., retry_after_seconds } }` を返す(再実行で消える)。
- 応答: `{ cleared: [...], document_map_cleared: boolean }`。
- annotations: `readOnlyHint: false`、`destructiveHint: true`、`idempotentHint: true`(`defineTool` の Destructive)。`max_response_tokens` は付かない。

### Go との相違点

| 項目 | Go | Workers 版 | 理由 |
|---|---|---|---|
| `cache_expire` | あり(sync_state に `expired_at` を付ける) | **作らない**(`expireCache` は `admin.ts` に残るがツールにつなげない) | daily auto-refresh の廃止後、expire の印は読み取りの挙動を変えなくなった(情報用の列を埋めるだけ)。LLM に渡しても何も起きない |
| `cache_status` の項目 | cached_rows / last_synced_at / ... | `row_count` / `cached_at` / `full_refreshed_at` / `refresh_state` / `full_prune_skipped` / `locked` + 逆引きの走査状態 | 応答の `cache` 配列と同じ語彙にそろえた。full の再開と逆引きは Workers 版で足したもの |
| `cache_clear` と逆引き | (プロセス内の表で、キャッシュとは別) | 既定では消さない。`include_document_map` で消せる | 対応は不変。D1 に永続化したため、消す手段を別に用意した |
| UI のベース URL | `BOARD_UI_BASE_URL` で上書き可 | 同じ(M06 で追加。任意、https のオリジン) | M05 で『上書きなし』にしていたものを Go に戻した |

## 任意の `BOARD_UI_BASE_URL`

`resolveUiBaseUrl(env)`: 設定(空白だけは未設定)があれば末尾の `/` を除いて使い、なければ `uiBaseUrlFromApiBase(BOARD_BASE_URL)`。
設定チェックでは任意項目: 未設定・空は問題にしない。設定されていて https のオリジンでない(http は loopback でも不可・パス付き・プレースホルダ)なら `invalid`(`/healthz` は 503、他は 500)。

## デプロイ設定の生成(logbutler の簡易版)

logbutler の upstream / 下流モデルは持ち込まない(ロードマップの決定済み事項)。`core.ts` は純粋(文字列とオブジェクトだけ)、`main.ts` は入出力・終了コード(0 成功 / 1 検証エラー / 2 引数エラー)。

| 環境変数 | 必須 | 差し替え先 | 検証 |
|---|---|---|---|
| `BOARD_WORKER_NAME` | はい | `name` | 小文字・数字・ハイフン(63 文字まで) |
| `BOARD_D1_DATABASE_ID` | はい | `d1_databases[binding=DB].database_id` | UUID |
| `BOARD_D1_DATABASE_NAME` | はい | 同 `database_name` | 英数字・`_`・`-` |
| `BOARD_ACCESS_TEAM_DOMAIN` | はい | `vars.ACCESS_TEAM_DOMAIN` | ホスト名 |
| `BOARD_ACCESS_AUD` | はい | `vars.ACCESS_AUD` | 64 桁の 16 進 |
| `BOARD_CUSTOM_DOMAIN` | はい | `routes: [{ pattern, custom_domain: true }]` | ホスト名 |
| `BOARD_BASE_URL` | いいえ(省略時は wrangler.jsonc の値) | `vars.BOARD_BASE_URL` | https のオリジン |
| `BOARD_UI_BASE_URL` | いいえ(省略時は vars から除く) | `vars.BOARD_UI_BASE_URL` | https のオリジン |

- `workers_dev` は常に `false`。カスタムドメインが必須(workers.dev を作らないため)。`env` ブロックは持ち越さない。secrets(API キー・トークン)は扱わず、設定にも載せない。
- 失敗: 未設定・プレースホルダ(`<...>`)・形式違いを**項目名と理由だけ**で報告する(値は出さない)。ファイルは作らない。
- `deploy:check`: `--example` のダミー値(`EXAMPLE_ENV`)で `wrangler.deploy.example.jsonc` を生成して `wrangler deploy --dry-run`。環境変数もアカウントも要らない。CI に追加した。
- `deploy`: 生成 → `wrangler deploy -c wrangler.deploy.jsonc`。**実行はユーザー**。

## test:live

`BOARD_LIVE_URL`(`https://<domain>/mcp`)・`BOARD_LIVE_ACCESS_CLIENT_ID`・`BOARD_LIVE_ACCESS_CLIENT_SECRET`。ヘッダー `CF-Access-Client-Id` / `CF-Access-Client-Secret` で Worker に直接接続する(Access が JWT に変換する)。
状態を持たない MCP なので `initialize` なしで `tools/call` を送る。service token の値は出力・エラーに出さない。

呼ぶ: `server_info`、`cache_status`、`list_users`(`limit: 5`)、`list_clients`(`limit: 5`)、`find_projects`(`limit: 5`)。さらに `list_users` / `list_clients` / `find_projects` を `refresh: "full"`(`limit: 1`)で `complete` になるまで(最大 40 回)繰り返す。
出力する計測: 各呼び出しの時間と応答サイズ(KiB)、full refresh が complete になるまでの呼び出し回数と、1 回あたり(最小・中央・最大・合計)の時間、1 ページあたりの時間(= 合計 / 見積もりページ数)。
`mise run test` と CI には含めない(`vitest.config.ts` の `live` プロジェクトは `mise run test:live` だけ)。環境変数がなければ、足りない変数名を表示して全体を skip する。
**full refresh は D1 のキャッシュを実際に育てる**(BOARD への書き込みはしない)。

## テスト(Red を確認してから実装)

- `test/unit/cache-admin.test.ts`(追加): `cacheReport`(項目・partial / failed / not_synced・locked と stale・逆引きの 4 種・読み取り専用)、`clearDocumentMap`(全消去・走査中は何も消さない・ロックを奪われたら何も消さない)
- `test/unit/cache-tools.test.ts`: annotations・`cache_expire` がない・status の形・clear の各経路(指定 / 省略 / include_document_map / refresh_in_progress / 走査中 / 未知のリソース / 未知の引数)
- `test/e2e/cache-tools-flow.test.ts`: Worker 全体(JWT → ツール → D1)で `list_clients refresh=full` → `cache_status` → `cache_clear`、refresh 中の `refresh_in_progress`
- `test/unit/config-check.test.ts` `test/e2e/config-check.test.ts` `test/unit/find-pure.test.ts`: `BOARD_UI_BASE_URL`
- `test/unit/render-wrangler-config.test.ts`(core)、`test/node-e2e/render-wrangler-config.test.ts`(実プロセスで生成 → `wrangler deploy --dry-run -c`)
- `test/node-e2e/live-config.test.ts`: `loadLiveConfig`・`LiveClient`(ヘッダーと secret の伏せ)・環境変数なしで `live` プロジェクトが全体 skip し理由を表示
- `test/unit/readme-tools.test.ts`: README のツール表と `ALL_TOOLS` の突き合わせ
- 既存のツール数の期待(52 → 54)とスナップショットを更新

## やらないこと

デプロイ・`wrangler secret put`・D1 の作成・`test:live` の実行(実環境への接続)、依存の追加、コミット・push、Go の worktree と logbutler の編集。

## 既知のリスク・未解決

- 実機でしか確かめられない前提は `docs/manual-verification.md`(V1〜V9)。特に Portal 経由の `Cf-Access-Jwt-Assertion`、MCP の abort、D1 の `meta.changes`、1 ページあたりの時間。
- `test:live` の『1 ページあたり』は、cache_status の `row_count` の増分から見積もったページ数で割った値(Worker は何ページ取ったかを返さない)。粗い見積もり。
- Access の画面の名前は 2026-11 時点の公式ドキュメントに基づく。変わっていたら公式を優先する。
- `(eval):1: == not found` の警告は、リポジトリ側に原因はない(下記。2026-11-05 に原因を訂正)。

## 調査: `(eval):1: == not found`

**訂正(2026-11-05)**: 当初は「エージェント実行環境のシェルラッパー由来」と書いたが誤り。原因は、司令塔のセッションが実行していた確認用コマンドに含まれる、引用符なしの `echo ===`(`echo ======`)だった。
zsh の EQUALS オプションは `=` で始まる単語を `=cmd`(コマンドのパスへの展開)と解釈し、`==` という名前のコマンドが見つからずに `(eval):1: == not found` でその行全体を中断する。そのため、その後に続く `mise run lint` などの出力が消えていた。
`mise.toml` やリポジトリのコードには関係しない。zsh で区切り線を出すときは `echo "==="` のように引用符で囲む。
