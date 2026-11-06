# board

**BOARD (https://the-board.jp) を AI エージェントから使うための Remote MCP サーバー。**
BOARD API(`https://api.the-board.jp/v1/`)の 22 リソースを、**54 個**のツールで読み取れる。
Cloudflare Workers + D1 + Hono で動き、Cloudflare Access(MCP Server Portal + Managed OAuth)の背後に置く。

> Go 製の CLI + ローカル MCP(v0.11.1)を書き直したもの。Go 版は `archive/go-cli` ブランチと、注釈付きタグ `go-cli-final`(= v0.11.1)に残してある。
> main には含まれない(`git show go-cli-final:<path>` で読める)。CLI 専用の機能(configure / completion / docs / cache path)は移植していない。

## 構成

- Cloudflare Workers + D1 + Hono。Durable Objects は使わない。
- MCP は `agents` の `createMcpHandler` によるステートレス実装(`@modelcontextprotocol/server` v2)。
- **認証は Cloudflare Access に任せる**。Worker は OAuth サーバーを持たない。`/healthz` 以外の全リクエストで `Cf-Access-Jwt-Assertion`(署名・iss・aud・exp)を検証する。
- **BOARD の API キー / トークンは Worker の secret を全員で共有する**(1 デプロイ = 1 BOARD アカウント)。
- BOARD API の rate limit(3 回/秒・3,000 回/日)を避けるため、エンティティを D1 にキャッシュする。鮮度は LLM が `refresh` 引数で決める。

```
MCP クライアント(Claude など)
   │  OAuth(Access の Managed OAuth)
   ▼
┌──────────── Cloudflare ─────────────────────────────────────────────────────┐
│ MCP Server Portal(または MCP server application)                          │
│   │ Cf-Access-Jwt-Assertion                                                  │
│   ▼                                                                          │
│ board(Worker)  /healthz(認証なし)  /mcp(JWT 検証 → ツール)                │
│   │  ツール 54 個                                                             │
│   ├─▶ D1: resource_cache / sync_state / 書類→案件の逆引き                    │
│   └─▶ BOARD API(x-api-key + Bearer。直列・再試行・締め切り 30 秒)           │
└──────────────────────────────────────────────────────────────────────────────┘
```

設計の理由は [docs/adr/](docs/adr/) を参照。

- [ADR 0001: 共有キー + Access Managed OAuth](docs/adr/0001-shared-key-and-access-managed-oauth.md)
- [ADR 0002: D1 キャッシュと再開できる refresh](docs/adr/0002-d1-cache-and-resumable-refresh.md)
- [ADR 0003: 呼び出しの締め切り 30 秒](docs/adr/0003-call-deadline.md)
- [ADR 0004: 書類 ID から案件 ID への逆引き](docs/adr/0004-document-reverse-lookup.md)
- [ADR 0005: upstream / 下流モデルのデプロイ](docs/adr/0005-upstream-downstream-deploy.md)

## ツール一覧(54 個)

ツール名に接頭辞は付けていない。MCP Server Portal 経由だと `board_` が前置される(例 `board_list_clients`)。

| グループ | 個数 | ツール |
|---|---|---|
| `server` | 1 | `server_info` |
| `list` | 18 | `list_accounting_types`, `list_client_branches`, `list_clients`, `list_contacts`, `list_document_send_channels`, `list_groups`, `list_invoices`, `list_payment_terms`, `list_payments`, `list_project_costs`, `list_project_types`, `list_projects`, `list_purchase_orders`, `list_purchase_types`, `list_users`, `list_vendor_branches`, `list_vendor_contacts`, `list_vendors` |
| `get` | 22 | `get_accounting_type`, `get_client`, `get_client_branch`, `get_contact`, `get_delivery`, `get_document_send_channel`, `get_estimate`, `get_group`, `get_invoice`, `get_order`, `get_payment`, `get_payment_term`, `get_project`, `get_project_cost`, `get_project_type`, `get_purchase_order`, `get_purchase_type`, `get_receipt`, `get_user`, `get_vendor`, `get_vendor_branch`, `get_vendor_contact` |
| `find` | 11 | `find_clients`, `find_deliveries`, `find_estimates`, `find_invoices`, `find_orders`, `find_payments`, `find_projects`, `find_purchase_orders`, `find_receipts`, `find_users`, `find_vendors` |
| `cache` | 2 | `cache_clear`, `cache_status` |

- **api 層(`list_*` / `get_*`)**: BOARD API に準拠した低レベルの読み取り。BOARD の生データを snake_case のまま返す。`show_meta` で `_meta`(総件数・ページ情報)も返す。
- **find 層(`find_*`)**: 名前から ID を解決し、関連するエンティティ(顧客・支店・担当者・案件)を付けて返す高レベルの検索。LLM 向け。
- **cache 層**: `cache_status` は D1 の状態の確認(読み取り専用)、`cache_clear` はキャッシュの削除(破壊的)。`cache_expire` は作っていない(読み取りに影響しないため。[計画書](plans/board-workers-m06-finish.md)参照)。
- 読み取りツールには引数 `max_response_tokens`(応答サイズの安全網。既定 50000、0 で無効)が付く。超過は切り詰めず `response_too_large`。
- エラーは `{"schema_version":"1","error":{"code","message","retryable"}}` を `isError: true` で返す。

## 自分の環境にデプロイする

このリポジトリは**公開の upstream** で、実値(Worker 名・D1 の id・ドメイン・Access の設定)は持たない。
**自分の private リポジトリ(下流)に upstream を取り込み**、そこから自分の Cloudflare アカウントにデプロイする。

```bash
# 下流を作る(詳細は docs/runbook.md の 0)
git clone https://github.com/youyo/board.git my-board && cd my-board
git remote rename origin upstream && git remote add origin <自分の private リポジトリの URL>
mise install && mise run install

# 設定を書く(秘密でない値だけ。1 か所)
cp board.config.example.json board.config.json   # <YOUR_...> を埋める

# 初回のデプロイを 1 本で(何度でも再実行できる)
pnpm exec wrangler login
mise run setup
```

- 下流が書く設定は `board.config.json` だけ。`mise run deploy:config` が upstream の `wrangler.jsonc` と合わせて `deploy/wrangler.jsonc` を生成し、**下流でコミットする**(`mise run deploy:config:check` が、生成し直し忘れ・手での書き換え・upstream の変更の取り込み忘れを検出する)。
- `mise run setup` は、D1 の作成 → 生成 → デプロイ → secrets(`BOARD_API_KEY` / `BOARD_API_TOKEN`。隠した入力) → `/healthz` の確認まで進める。Access アプリ・Managed OAuth・Portal の登録は画面の作業で、[docs/runbook.md](docs/runbook.md) が案内する。
- upstream の更新は `git fetch upstream && git merge --ff-only upstream/main`。下流は upstream のファイルを書き換えない。
- 下流の GitHub Actions からデプロイするなら [docs/deploy-ci.md](docs/deploy-ci.md)(雛形は `deploy.example/deploy.workflow.yml`)。
- **セットアップを案内するスキル**: [.claude/skills/board-setup/SKILL.md](.claude/skills/board-setup/SKILL.md)。下流リポジトリで Claude Code / Codex に「board をセットアップしたい」と頼むと、Step ごとに進めてくれる(秘密値は受け取らず、デプロイや secret の登録は運用者の承認の後)。

## 接続方法

1. 上の『自分の環境にデプロイする』と [docs/runbook.md](docs/runbook.md) に従って Worker をデプロイし、Access の MCP server application(Managed OAuth)を作る。
2. MCP Server Portal に Worker の URL `https://<domain>/mcp` を登録する。
3. MCP クライアントから Portal の URL につなぐ(OAuth のログインは Access が行う)。

Worker を直接 MCP クライアントにつなぐ場合も、Access の MCP server application の URL を使う(Worker 自体は OAuth を話さない)。

## 前提

- Cloudflare アカウント(Workers **Paid** プラン推奨。下記)、Cloudflare Access(Zero Trust)、カスタムドメイン(任意。なければ workers.dev で公開する。[docs/runbook.md](docs/runbook.md) の 5)。
- BOARD の API キーとトークン(BOARD の設定画面で発行する)。
- ローカルの作業: [mise](https://mise.jdx.dev/)(node 24 / pnpm 11 は `mise.toml` が入れる)。

**Workers Free プランでは、1 回の Worker 実行あたりの D1 クエリが 50 まで**(Paid は 1,000。`db.batch()` は 1 回と数える。2026-11 時点の Cloudflare D1 Limits)。
full refresh や逆引きの走査は 1 回の呼び出しで多数のクエリを使うので、Paid プランを前提にしている。

## 制約と使い方のコツ

- **rate limit**: BOARD API は 3 回/秒・3,000 回/日。Worker は BOARD への通信を直列にし、429 / 5xx は `Retry-After` を尊重して上限つきで再試行する。キャッシュを優先して読む。複数人が同時にキャッシュミスすると 3 回/秒を超えうる(再試行で吸収する)。
- **締め切り 30 秒**: 1 回のツール呼び出しは 30 秒で打ち切る(MCP クライアントのタイムアウトより前に返すため)。時間内に終わらない処理は途中経過を保存して返し、次の呼び出しで続きから進む。
- **`refresh`**: 一覧・検索系のツールの引数。`none`(既定。キャッシュをそのまま読む)/ `delta`(前回からの差分を取る)/ `full`(全件を取り直し、消えたものを除く)。
  どれが要るかは、応答の `cache`(`cached_at` / `full_refreshed_at` / `refresh_state`)か `cache_status` を見て LLM が決める。
  初めて使うリソースはキャッシュが空なので、`refresh=full` を `refresh_state=complete` になるまで繰り返す。件数の多いリソース(estimates など)は数回かかる。
  `delta` / `full` は共有の D1 キャッシュに書き込み、BOARD API の 1 日あたりの上限(3,000 回)を消費する。BOARD のデータは書き換えないので、読み取りツールの注釈(`readOnlyHint: true`)はそのままにしている。読み取りツールを自動で承認するクライアントは、確認なしに refresh を実行する。必要なときだけ指定する(各ツールの `refresh` 引数の説明にも書いてある)。
  同時に別の refresh が動いていると `refresh_in_progress`(`retry_after_seconds` 秒後に再試行)。
- **入力の上限**: 文字列は 1,000 文字、配列は 100 件まで。超えると `argument_error`。
- **`limit`**: 結果の件数を切る(0・未指定は無制限)。切ったときは `limited: true` と `total_before_limit` が付く。名前が多数にマッチしうる検索では `limit` を付ける。
- **`find_*` の `warnings`**: 補助情報の取得に失敗した、書類の案件 ID を解決できなかった(逆引きの走査が時間切れ)、などの致命的でない問題。再実行で解消することが多い。
- **秘密情報**: BOARD の API キー・トークンと Access の JWT は、ログ・エラー・MCP 応答に出さない。

## 開発

```bash
mise install          # node / pnpm
mise run install      # 依存パッケージ
mise run test         # unit + e2e(workerd、node-e2e)。BOARD API には通信しない
mise run lint         # biome + tsc
mise run deploy:check # board.config.example.json のダミー値でデプロイ設定を生成し wrangler deploy --dry-run
mise run test:live    # デプロイ済み Worker への実環境確認(test には含まれない。runbook 参照)
```

作業規約は [CLAUDE.md](CLAUDE.md)、計画は [plans/](plans/)、実機でしか確かめられない前提は [docs/manual-verification.md](docs/manual-verification.md)。

## ライセンス

MIT(LICENSE)。
