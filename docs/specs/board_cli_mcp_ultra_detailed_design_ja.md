# BOARD CLI / MCP 統合ツール 超完全版・超詳細設計書

## 0. 文書情報

- 文書名: BOARD CLI / MCP 統合ツール 超完全版設計書
- 言語: 日本語
- 対象読者:
  - コーディングエージェント
  - 実装担当エンジニア
  - レビュー担当者
- 文書目的:
  - 実装方針の確定
  - ディレクトリ構成、責務分離、データ構造、コマンド体系、キャッシュ戦略、MCP 連携方針の明文化
  - 追加の要件確認なしで実装に着手できるレベルの仕様提供
- 実装対象:
  - 単一 Go バイナリ
  - CLI
  - ローカル HTTP MCP サーバー
  - BOARD API クライアント
  - SQLite キャッシュ
  - `config.toml` による設定管理

---

## 1. 背景と目的

本プロジェクトは、BOARD API を利用する Go 製ツールを実装するためのものである。  
ツールは以下の 2 つの利用形態を同時に満たす。

1. `board` CLI としての利用
2. `board mcp serve` によるローカル HTTP MCP サーバーとしての利用

本ツールの主眼は「人間向け CLI」ではなく、「LLM が使いやすい CLI / MCP」を提供する点にある。  
そのため、CLI の出力はデフォルトで JSON とし、`--pretty` 指定時のみ人間向け整形表示を行う。

また、BOARD API は静的認証（API Key + API Token）であり、OAuth は BOARD API 認証方式としては不要である。  
一方で MCP 側の将来的な多人数利用・認証拡張余地は残すが、MVP ではローカル HTTP のみを対象とし、MCP サーバー自身の OAuth 実装は行わない。

本設計の最重要要件は以下である。

- API 呼び出し回数を抑えること
- rate limit に耐えること
- CLI と MCP で同一のキャッシュ基盤を共有すること
- API 準拠の low-level コマンドと、LLM 向けの high-level コマンドを両立すること
- 実装責務を明確に分離すること

---

## 2. ゴールと非ゴール

### 2.1 ゴール

- 単一 Go バイナリ `board` を提供する
- `board configure` による対話型 profile 設定を提供する
- `board api ...` 配下に BOARD API 準拠の low-level コマンド群を提供する
- `board find ...` 配下に LLM 向け high-level コマンド群を提供する
- `board mcp serve` によりローカル HTTP MCP サーバーを起動できる
- CLI と MCP の両方で SQLite キャッシュを共有する
- キャッシュは JSON blob 方式で entity 単位保存する
- daily auto refresh を実装する
- `--refresh`, `--force-refresh`, `cache expire`, `cache clear` を実装する
- Zsh completion を提供する
- 実装しやすく、将来 write 系 API を追加しやすいレイヤ構造にする

### 2.2 非ゴール

- BOARD API 用 OAuth 実装
- リモート公開用の MCP 認証基盤
- 分散キャッシュ
- 複数ノード間共有ロック
- BOARD UI と完全に同等な検索機能
- 初期段階でのすべての write 操作対応
- Bash / Fish completion
- RDB 的な正規化ストレージ
- 全件事前同期専用コマンドの常設公開

---

## 3. 設計原則

### 3.1 API 準拠 low-level と LLM 向け high-level を分離する

コマンドを 2 層に分ける。

- low-level:
  - `board api ...`
  - BOARD API の resource/action にできるだけ準拠
  - キャッシュ管理責務を持つ
- high-level:
  - `board find ...`
  - LLM にとって使いやすい抽象度
  - low-level を組み合わせる
  - 独自永続キャッシュは持たない

### 3.2 キャッシュ責務は low-level 側に集約する

キャッシュは API 呼び出し量削減と freshness 制御のための仕組みであり、本質的に API 境界に近い責務である。  
そのため high-level に別キャッシュを持たせない。

### 3.3 デフォルト出力は JSON

CLI も MCP も LLM 利用が主目的であるため、標準出力はデフォルトで JSON とする。  
`--pretty` は補助的な整形表示であり、JSON を本筋とする。

### 3.4 SQLite は単一ファイル・単一 DB とする

CLI と MCP が同一 SQLite ファイルを共有する。  
profile ごとの論理分離はテーブルキーで行う。

### 3.5 キャッシュは entity 単位の JSON blob とする

レスポンス全体の丸ごとキャッシュではなく、entity 単位で保存する。  
正規化は行わない。

### 3.6 refresh は内部処理であり、独立 sync コマンドは公開しない

同期はユーザーに明示操作させる主機能ではなく、`get/list/search` 実行時の内部挙動として扱う。  
ユーザーが制御するのは `--refresh`, `--force-refresh`, `cache expire`, `cache clear` とする。

### 3.7 daily auto refresh はデフォルト ON

ただし config により無効化可能とする。  
timezone はデフォルト UTC とし、config で変更可能とする。

---

## 4. 想定ユースケース

### 4.1 CLI

- BOARD profile を設定する
- 案件、顧客、プロジェクト、チケットを検索する
- 特定 ID の entity を取得する
- キャッシュ状態を確認する
- キャッシュを明示的に expire / clear する
- `--pretty` でざっと目視確認する
- LLM から CLI を直接叩いて JSON を処理する

### 4.2 MCP

- ローカル HTTP MCP サーバーとして起動する
- LLM から `find_*` ツールを呼び出す
- 必要な entity をキャッシュ優先で取得する
- 複数ユーザーが同一サーバーを叩く可能性に備え、サーバー側で daily 判定・refresh 制御を行う

---

## 5. アーキテクチャ全体像

以下の依存方向で構成する。

```text
CLI / MCP
  → service
  → repository
  → refresh + cache
  → boardapi
```

より詳細には以下。

```text
cli
  ├─ configure
  ├─ api
  ├─ find
  ├─ cache
  └─ mcp
        ↓
service/api        （low-level）
service/find       （high-level）
        ↓
repository         （cache を読む / 必要時に API を呼ぶ）
        ↓
refresh            （daily 判定、force refresh、resource refresh）
cache              （SQLite）
        ↓
boardapi           （生 HTTP client）
```

---

## 6. リポジトリ構成

最終的な推奨構成は以下とする。

```text
board/
├─ cmd/
│  └─ board/
│     └─ main.go
│
├─ internal/
│  ├─ cli/
│  │  ├─ root.go
│  │  ├─ completion.go
│  │  ├─ configure.go
│  │  ├─ configure_set.go
│  │  ├─ configure_get.go
│  │  ├─ configure_show.go
│  │  ├─ configure_use.go
│  │  ├─ configure_list_profiles.go
│  │  ├─ configure_current_profile.go
│  │  ├─ configure_path.go
│  │  ├─ cache.go
│  │  ├─ cache_status.go
│  │  ├─ cache_expire.go
│  │  ├─ cache_clear.go
│  │  ├─ cache_path.go
│  │  ├─ api.go
│  │  ├─ api_clients.go
│  │  ├─ api_customers.go
│  │  ├─ api_deals.go
│  │  ├─ api_projects.go
│  │  ├─ api_tickets.go
│  │  ├─ find.go
│  │  ├─ find_customer.go
│  │  ├─ find_deal.go
│  │  ├─ find_project.go
│  │  ├─ find_ticket.go
│  │  ├─ mcp.go
│  │  └─ mcp_serve.go
│  │
│  ├─ app/
│  │  ├─ app.go
│  │  ├─ container.go
│  │  └─ runtime.go
│  │
│  ├─ config/
│  │  ├─ config.go
│  │  ├─ load.go
│  │  ├─ save.go
│  │  ├─ profile.go
│  │  ├─ defaults.go
│  │  ├─ path.go
│  │  └─ prompt.go
│  │
│  ├─ boardapi/
│  │  ├─ client.go
│  │  ├─ transport.go
│  │  ├─ auth.go
│  │  ├─ errors.go
│  │  ├─ pagination.go
│  │  ├─ retry.go
│  │  ├─ types_common.go
│  │  ├─ clients.go
│  │  ├─ customers.go
│  │  ├─ deals.go
│  │  ├─ projects.go
│  │  └─ tickets.go
│  │
│  ├─ cache/
│  │  ├─ db.go
│  │  ├─ migrate.go
│  │  ├─ schema.go
│  │  ├─ resource_cache.go
│  │  ├─ sync_state.go
│  │  ├─ keys.go
│  │  └─ jsonblob.go
│  │
│  ├─ refresh/
│  │  ├─ policy.go
│  │  ├─ daily.go
│  │  ├─ resource_refresh.go
│  │  ├─ force_refresh.go
│  │  └─ updater.go
│  │
│  ├─ repository/
│  │  ├─ clients_repository.go
│  │  ├─ customers_repository.go
│  │  ├─ deals_repository.go
│  │  ├─ projects_repository.go
│  │  └─ tickets_repository.go
│  │
│  ├─ service/
│  │  ├─ api/
│  │  │  ├─ clients_service.go
│  │  │  ├─ customers_service.go
│  │  │  ├─ deals_service.go
│  │  │  ├─ projects_service.go
│  │  │  └─ tickets_service.go
│  │  │
│  │  └─ find/
│  │     ├─ customers_service.go
│  │     ├─ deals_service.go
│  │     ├─ projects_service.go
│  │     └─ tickets_service.go
│  │
│  ├─ mcpserver/
│  │  ├─ server.go
│  │  ├─ tools.go
│  │  ├─ tool_find_customers.go
│  │  ├─ tool_find_deals.go
│  │  ├─ tool_find_projects.go
│  │  ├─ tool_find_tickets.go
│  │  ├─ schema.go
│  │  └─ transport_http.go
│  │
│  ├─ output/
│  │  ├─ json.go
│  │  ├─ pretty.go
│  │  └─ mask.go
│  │
│  └─ util/
│     ├─ clock.go
│     ├─ json.go
│     ├─ ptr.go
│     ├─ strings.go
│     └─ validate.go
│
├─ migrations/
│  └─ sqlite/
│     ├─ 0001_init.sql
│     └─ 0002_indexes.sql
│
├─ docs/
│  ├─ config.md
│  ├─ cache.md
│  ├─ cli.md
│  └─ mcp.md
│
├─ go.mod
├─ go.sum
└─ README.md
```

---

## 7. パッケージ責務

### 7.1 `internal/cli`

- Cobra command 定義のみを置く
- フラグ受け取り
- 引数検証
- 出力フォーマット呼び分け
- 業務ロジックは書かない

### 7.2 `internal/app`

- 依存関係組み立て
- config 読み込み
- DB 初期化
- boardapi client 初期化
- repository/service の組み立て
- CLI と MCP の共通 runtime 提供

### 7.3 `internal/config`

- `config.toml` の型定義
- デフォルト値埋め込み
- ファイルパス解決
- 読み込み / 保存
- 対話型 configure の質問処理

### 7.4 `internal/boardapi`

- BOARD API 生クライアント
- 認証ヘッダ付与
- retry / backoff
- pagination
- resource ごとの HTTP 呼び出し
- API エラー正規化

### 7.5 `internal/cache`

- SQLite 接続
- migration 実行
- `resource_cache` 管理
- `sync_state` 管理
- JSON blob 保存 / 読み出し

### 7.6 `internal/refresh`

- daily auto refresh 判定
- `--refresh` / `--force-refresh`
- resource 単位差分取得
- sync cursor 更新
- refresh 状態管理

### 7.7 `internal/repository`

- キャッシュ参照
- 必要なら refresh 実行
- API fallback
- entity upsert
- list/get/search の共通参照窓口

### 7.8 `internal/service/api`

- `board api ...` コマンドのユースケース
- API 準拠 low-level 操作
- list/get/search を提供

### 7.9 `internal/service/find`

- `board find ...` コマンドのユースケース
- 複数 resource 横断
- LLM 向けの便利検索
- low-level service / repository を組み合わせる

### 7.10 `internal/mcpserver`

- ローカル HTTP MCP サーバー
- 公開ツール定義
- JSON Schema
- リクエスト/レスポンス整形
- high-level service 呼び出し

### 7.11 `internal/output`

- デフォルト JSON 出力
- `--pretty` 表示
- secrets のマスク処理

---

## 8. コマンド設計

### 8.1 ルートコマンド

```bash
board
```

### 8.2 `configure`

```bash
board configure
board configure --profile readonly

board configure set profiles.readonly.api_token xxx
board configure get profiles.readonly.api_token
board configure list-profiles
board configure use readonly
board configure current-profile
board configure show
board configure path
```

### 8.3 `cache`

```bash
board cache status
board cache expire deals
board cache expire all
board cache clear deals
board cache clear all
board cache path
```

### 8.4 `api` low-level

```bash
board api clients list
board api clients get --id 123
board api clients search --name "ABC"

board api customers list
board api customers get --id 123
board api customers search --name "ABC"

board api deals list
board api deals get --id 123
board api deals search --customer "ABC"
board api deals search --status open

board api projects list
board api projects get --id 123
board api projects search --client "ABC"

board api tickets list
board api tickets get --id 123
board api tickets search --status open
board api tickets search --assignee "youyo"
```

### 8.5 `find` high-level

```bash
board find deal --id 123
board find deal --customer "ABC"
board find deal --status open
board find deal --text "保守"

board find project --id 123
board find project --client "ABC"
board find project --text "開発"

board find ticket --id 123
board find ticket --status open
board find ticket --assignee "youyo"
board find ticket --text "障害"

board find customer --id 123
board find customer --name "ABC"
board find customer --text "株式会社"
```

### 8.6 `mcp`

```bash
board mcp serve
board mcp serve --profile readonly
board mcp serve --host 127.0.0.1
board mcp serve --port 8080
```

### 8.7 `completion`

```bash
board completion zsh
```

---

## 9. 共通フラグ

read 系コマンドで有効な共通フラグは以下。

- `--profile`
- `--refresh`
- `--force-refresh`
- `--pretty`
- `--limit`

### 9.1 `--profile`

使用する profile を明示する。  
省略時は `current_profile` を使う。

### 9.2 `--refresh`

daily 状態を無視して差分 refresh を実行する。  
通常の `resource` 単位差分取得を行う。

### 9.3 `--force-refresh`

差分ではなく全件再取得を行う。  
実質 `must_full_resync=true` 相当。

### 9.4 `--pretty`

JSON の代わりに見やすい整形表示を行う。  
MCP では無効。

### 9.5 `--limit`

検索結果件数上限。  
デフォルト 50、最大 100 を推奨。

---

## 10. 設定ファイル設計

### 10.1 保存場所

推奨:

- config: `~/.config/board/config.toml`
- cache DB: `~/.cache/board/cache.db`

OS ごとの標準ディレクトリは XDG を優先し、未設定時は妥当な fallback を使う。

### 10.2 `config.toml` スキーマ

```toml
current_profile = "default"
timezone = "UTC"

[profiles.default]
base_url = "https://api.the-board.jp"
api_key = "YOUR_API_KEY"
api_token = "YOUR_API_TOKEN"
daily_auto_refresh = true
request_timeout_seconds = 30
retry_max = 5
pretty_default = false
```

### 10.3 Go 型イメージ

```go
type Config struct {
    CurrentProfile string                 `toml:"current_profile"`
    Timezone       string                 `toml:"timezone"`
    Profiles       map[string]ProfileConfig `toml:"profiles"`
}

type ProfileConfig struct {
    BaseURL              string `toml:"base_url"`
    APIKey               string `toml:"api_key"`
    APIToken             string `toml:"api_token"`
    DailyAutoRefresh     bool   `toml:"daily_auto_refresh"`
    RequestTimeoutSeconds int   `toml:"request_timeout_seconds"`
    RetryMax             int    `toml:"retry_max"`
    PrettyDefault        bool   `toml:"pretty_default"`
}
```

### 10.4 `board configure` 対話式仕様

質問順は以下。

1. profile 名
2. base_url
3. api_key
4. api_token
5. daily_auto_refresh
6. timezone（global）
7. current profile にするか

既存値があれば初期値として表示する。  
未入力なら既存値維持。  
新規 profile なら default 値を使う。

### 10.5 `configure show`

- デフォルトでは secrets をマスクする
- 例:
  - `api_key = "ab********yz"`
  - `api_token = "to********en"`

---

## 11. BOARD API クライアント設計

### 11.1 責務

- BASE URL 管理
- API Key / Token ヘッダ付与
- HTTP 実行
- retry
- pagination
- API レスポンスの struct 化
- API エラー正規化

### 11.2 client 構造

```go
type Client struct {
    baseURL    string
    apiKey     string
    apiToken   string
    httpClient *http.Client
    retryMax   int
}
```

### 11.3 認証

BOARD API 用に profile に設定された API Key / API Token をヘッダ付与する。  
認証方式の実体は本設計では抽象化し、`auth.go` に閉じ込める。  
CLI / MCP / repository からは認証ヘッダ詳細を見せない。

### 11.4 retry

- 対象:
  - 429
  - 5xx
  - 一時的ネットワークエラー
- 動作:
  - 指数バックオフ + ジッター
  - `Retry-After` があれば尊重
- 非対象:
  - 4xx の恒久エラー
  - validation error
  - unauthorized / forbidden

### 11.5 pagination

- `per_page` の上限を API 仕様に合わせる
- 内部でページングを吸収して `[]Entity` を返す helper を用意する
- list/search の実装で共通利用する

### 11.6 エラー型

```go
type APIErrorCode string

const (
    APIErrorUnauthorized APIErrorCode = "UNAUTHORIZED"
    APIErrorForbidden    APIErrorCode = "FORBIDDEN"
    APIErrorNotFound     APIErrorCode = "NOT_FOUND"
    APIErrorRateLimit    APIErrorCode = "RATE_LIMIT"
    APIErrorValidation   APIErrorCode = "VALIDATION"
    APIErrorTemporary    APIErrorCode = "TEMPORARY"
    APIErrorUnknown      APIErrorCode = "UNKNOWN"
)

type APIError struct {
    Code       APIErrorCode
    StatusCode int
    Message    string
    Body       string
}
```

---

## 12. Resource と Entity 設計

### 12.1 対象 resource（MVP）

- `clients`
- `customers`
- `deals`
- `projects`
- `tickets`

### 12.2 用語整理

- `customer`
  - 顧客企業のマスタ
- `client`
  - 案件文脈の相手先

ただし low-level では API 準拠のため `clients` を独立 resource として持つ。  
high-level では `client` は補助条件または entity の一部として扱う。

### 12.3 Entity 保存単位

1 row = 1 entity

例:
- project 1件
- deal 1件
- ticket 1件
- customer 1件
- client 1件

レスポンス全体丸ごと保存はしない。

---

## 13. SQLite キャッシュ設計

### 13.1 方針

- 1ファイル SQLite
- CLI と MCP で共有
- profile ごとに論理分離
- resource ごとに論理分離
- payload は JSON blob
- 差分比較はしない
- `updated_at` は cursor と index 用に持つ

### 13.2 テーブル一覧

- `resource_cache`
- `sync_state`
- `cache_meta`

---

## 14. `resource_cache` テーブル

### 14.1 役割

entity 本体の保存

### 14.2 カラム

- `profile_name` TEXT NOT NULL
- `resource_name` TEXT NOT NULL
- `entity_id` TEXT NOT NULL
- `payload_json` TEXT NOT NULL
- `updated_at` TEXT NULL
- `fetched_at` TEXT NOT NULL

### 14.3 主キー

- `(profile_name, resource_name, entity_id)`

### 14.4 index

- `(profile_name, resource_name)`
- `(profile_name, resource_name, updated_at)`

### 14.5 DDL

```sql
CREATE TABLE IF NOT EXISTS resource_cache (
  profile_name TEXT NOT NULL,
  resource_name TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  updated_at TEXT,
  fetched_at TEXT NOT NULL,
  PRIMARY KEY (profile_name, resource_name, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_resource_cache_resource
  ON resource_cache(profile_name, resource_name);

CREATE INDEX IF NOT EXISTS idx_resource_cache_updated
  ON resource_cache(profile_name, resource_name, updated_at);
```

---

## 15. `sync_state` テーブル

### 15.1 役割

- resource 単位の同期状態
- daily auto refresh 判定
- force refresh / expire 制御
- エラー追跡
- 観測用 refresh 状態

### 15.2 単位

- profile × resource = 1 row

### 15.3 カラム

- `profile_name` TEXT NOT NULL
- `resource_name` TEXT NOT NULL
- `last_synced_at` TEXT NULL
- `cursor_updated_at` TEXT NULL
- `last_full_synced_at` TEXT NULL
- `last_sync_mode` TEXT NULL
- `last_sync_status` TEXT NULL
- `last_daily_refresh_date` TEXT NULL
- `must_full_resync` INTEGER NOT NULL DEFAULT 0
- `expired_at` TEXT NULL
- `invalidate_reason` TEXT NULL
- `last_error_at` TEXT NULL
- `last_error_code` TEXT NULL
- `last_error_message` TEXT NULL
- `consecutive_failures` INTEGER NOT NULL DEFAULT 0
- `refresh_started_at` TEXT NULL
- `refresh_owner` TEXT NULL
- `cache_version` INTEGER NOT NULL DEFAULT 1
- `schema_version` INTEGER NOT NULL DEFAULT 1

### 15.4 主キー

- `(profile_name, resource_name)`

### 15.5 DDL

```sql
CREATE TABLE IF NOT EXISTS sync_state (
  profile_name TEXT NOT NULL,
  resource_name TEXT NOT NULL,
  last_synced_at TEXT,
  cursor_updated_at TEXT,
  last_full_synced_at TEXT,
  last_sync_mode TEXT,
  last_sync_status TEXT,
  last_daily_refresh_date TEXT,
  must_full_resync INTEGER NOT NULL DEFAULT 0,
  expired_at TEXT,
  invalidate_reason TEXT,
  last_error_at TEXT,
  last_error_code TEXT,
  last_error_message TEXT,
  consecutive_failures INTEGER NOT NULL DEFAULT 0,
  refresh_started_at TEXT,
  refresh_owner TEXT,
  cache_version INTEGER NOT NULL DEFAULT 1,
  schema_version INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (profile_name, resource_name)
);
```

---

## 16. `cache_meta` テーブル

### 16.1 役割

DB 全体メタ情報

### 16.2 カラム

- `key` TEXT PRIMARY KEY
- `value` TEXT NOT NULL
- `updated_at` TEXT NOT NULL

### 16.3 用途例

- `db_schema_version`
- `created_at`
- `last_migrated_at`

---

## 17. daily auto refresh 設計

### 17.1 基本方針

- デフォルト ON
- config で OFF にできる
- timezone は config で指定
- デフォルト timezone は UTC
- 判定単位は `profile × resource`

### 17.2 判定式

```text
today_in_config_timezone != last_daily_refresh_date
```

### 17.3 対象コマンド

- `board api <resource> list`
- `board api <resource> get`
- `board api <resource> search`
- `board find ...`
- MCP の `find_*`

### 17.4 `get` の挙動

`get` も daily auto refresh 対象とする。  
ただし refresh は resource 単位で 1 日 1 回のみ。

フロー:

```text
1. daily 判定
2. 未実行なら resource 単位差分 refresh
3. cache lookup
4. あれば返す
5. なければ API get
6. upsert
7. 返す
```

---

## 18. refresh 設計

### 18.1 refresh は内部処理

独立 `sync` コマンドは提供しない。  
refresh は各 read 操作の内部で動く。

### 18.2 種別

- 通常:
  - キャッシュ優先 + daily 判定
- `--refresh`:
  - daily 状態無視で差分 refresh
- `--force-refresh`:
  - 全件再取得
- `cache expire`:
  - 次回を実質 full refresh 扱いにする

### 18.3 refresh 粒度

- refresh 粒度 = resource 単位
- cache 粒度 = entity 単位

### 18.4 差分 refresh アルゴリズム

1. `sync_state.cursor_updated_at` を取得
2. API に `updated_at >= cursor` 相当の条件で問い合わせ
3. 返却 entity を entity 単位 upsert
4. 取得結果中の最大 `updated_at` を cursor として保存
5. `last_synced_at`, `last_daily_refresh_date`, `last_sync_status` を更新

### 18.5 重複対策

`updated_at >= cursor` による境界重複があり得るため、  
重複は upsert で吸収する。

### 18.6 force refresh

1. 対象 resource の全件 API 取得
2. entity 単位 upsert
3. 必要なら既存データのクリーンアップ
4. `must_full_resync=false`
5. `last_full_synced_at` 更新

### 18.7 削除データ

MVP では削除検知を厳密には扱わない。  
将来 full refresh 時に「取得されなくなった entity を消す」処理を追加可能なようにコードを分離する。

---

## 19. ロックと多重実行制御

### 19.1 方針

- 主制御は process 内 mutex
- DB には観測用・復旧用の状態を残す

### 19.2 CLI

単発実行が基本。  
DB ロックと軽い mutex で十分。

### 19.3 MCP

常駐 server のため、同一 resource について同時 refresh を避ける。  
`profile + resource` ごとの in-process lock を持つ。

### 19.4 stale lock

- `refresh_started_at` が古すぎる場合は stale とみなして上書き可能
- 例: 10 分以上経過

---

## 20. repository 設計

### 20.1 repository の責務

- cache lookup
- daily 判定
- 必要なら refresh 呼び出し
- API fallback
- upsert
- list/get/search の統一窓口

### 20.2 public interface 例

```go
type DealsRepository interface {
    List(ctx context.Context, profile string, opts ListOptions) ([]DealEntity, error)
    GetByID(ctx context.Context, profile string, id string, opts ReadOptions) (*DealEntity, error)
    Search(ctx context.Context, profile string, query DealSearchQuery, opts ReadOptions) ([]DealEntity, error)
}
```

### 20.3 `ReadOptions`

```go
type ReadOptions struct {
    Refresh      bool
    ForceRefresh bool
    Limit        int
}
```

### 20.4 repository の read フロー

#### list/search

1. refresh policy 判定
2. 必要なら resource refresh
3. cache を読む
4. 条件不足なら API search/list を呼ぶ
5. 結果を upsert
6. 返す

#### get

1. refresh policy 判定
2. 必要なら resource refresh
3. cache を読む
4. 見つかれば返す
5. 見つからなければ API get
6. upsert
7. 返す

---

## 21. `board api` low-level 設計

### 21.1 方針

- API 準拠
- resource / action をそのまま表現
- キャッシュを持つ
- write 系は将来追加しやすいインターフェースにする

### 21.2 共通 action

- `list`
- `get`
- `search`
- 将来:
  - `create`
  - `update`
  - `delete`

### 21.3 resource 一覧

- `clients`
- `customers`
- `deals`
- `projects`
- `tickets`

### 21.4 例

```bash
board api deals list --profile default
board api deals get --id 123
board api deals search --customer "ABC" --status open --limit 20
board api tickets search --assignee "youyo" --pretty
```

### 21.5 search 条件

resource ごとに変える。  
完全共通化しない。

#### deals
- `--id`
- `--customer`
- `--status`
- `--text`
- `--project-id`
- `--limit`

#### projects
- `--id`
- `--client`
- `--text`
- `--deal-id`
- `--limit`

#### tickets
- `--id`
- `--status`
- `--assignee`
- `--text`
- `--deal-id`
- `--limit`

#### customers
- `--id`
- `--name`
- `--text`
- `--limit`

#### clients
- `--id`
- `--name`
- `--text`
- `--limit`

---

## 22. `board find` high-level 設計

### 22.1 方針

- LLM 向けの便利層
- low-level を組み合わせる
- 独自キャッシュは持たない
- フラグ命名をできるだけ統一する

### 22.2 コマンド構成

- `board find deal`
- `board find project`
- `board find ticket`
- `board find customer`

### 22.3 共通フラグ候補

- `--id`
- `--name`
- `--text`
- `--customer`
- `--client`
- `--status`
- `--assignee`
- `--project-id`
- `--deal-id`
- `--limit`
- `--refresh`
- `--pretty`

### 22.4 high-level の原則

- `find deal --id 123` は実質 low-level get のラッパ
- `find deal --customer "ABC"` のような複数解決は high-level で吸収
- `find ... --text` は万能検索枠として扱う
- キャッシュは low-level / repository に完全委譲

### 22.5 high-level 実行例

#### `board find deal --customer "ABC"`

1. customer 解決
2. 必要なら client 解決
3. deal 検索
4. 結果整形
5. 返却

#### `board find ticket --text "障害"`

1. tickets resource の daily 判定
2. 必要なら refresh
3. cache 検索
4. 足りなければ API search
5. upsert
6. 返却

---

## 23. MCP 設計

### 23.1 基本方針

- 公開ツールは high-level のみ
- low-level を MCP に直接露出しない
- cache / refresh は完全隠蔽
- ローカル HTTP transport のみ

### 23.2 ツール一覧

- `find_deals`
- `find_projects`
- `find_tickets`
- `find_customers`

### 23.3 ツール名命名原則

- 動詞 + 複数名詞
- 検索系は複数返却前提なので plural を使う

### 23.4 共通入力 schema

```json
{
  "id": "string",
  "name": "string",
  "text": "string",
  "customer": "string",
  "client": "string",
  "status": "string",
  "assignee": "string",
  "project_id": "string",
  "deal_id": "string",
  "limit": 50,
  "refresh": false
}
```

### 23.5 入力方針

- すべて optional
- ID も string
- status も string
- バリデーションはサーバー側で吸収
- `limit` はデフォルト 50、最大 100

### 23.6 出力方針

JSON を返す。  
要約フィールドと raw payload を両方含む。

例:

```json
[
  {
    "id": "123",
    "name": "案件A",
    "status": "open",
    "customer": "株式会社ABC",
    "updated_at": "2026-04-01T00:00:00Z",
    "raw": {
      "...": "..."
    }
  }
]
```

### 23.7 not found

空配列 `[]`

### 23.8 エラー

```json
{
  "error": {
    "code": "RATE_LIMIT",
    "message": "..."
  }
}
```

### 23.9 CLI との対応

- `board find deal --customer ABC`
  - MCP では `find_deals({"customer":"ABC"})`

完全 1:1 で対応づけられるようにする。

### 23.10 MCP server 起動

```bash
board mcp serve --profile default --host 127.0.0.1 --port 8080
```

---

## 24. `cache` コマンド設計

### 24.1 `cache status`

表示内容:

- profile
- resource
- row_count
- last_synced_at
- cursor_updated_at
- last_daily_refresh_date
- last_sync_status
- must_full_resync
- consecutive_failures
- last_error_code
- last_error_message

JSON がデフォルト、`--pretty` で表表示。

### 24.2 `cache expire <resource>`

動作:

- `expired_at = now`
- `invalidate_reason = manual`
- `must_full_resync = true`

### 24.3 `cache expire all`

全 resource に対して上記を実行

### 24.4 `cache clear <resource>`

- `resource_cache` の対象 resource を削除
- `sync_state` を初期状態に戻す

### 24.5 `cache clear all`

全 resource / 全 profile は危険なため、profile 単位を必須にするか、確認フラグを求める。  
MVP では少なくとも `--profile` を必須にすること。

### 24.6 `cache path`

SQLite ファイルパスを表示

---

## 25. 出力仕様

### 25.1 デフォルト JSON

- CLI の成功出力は JSON
- 機械処理しやすい形にする
- map より struct ベースで安定 shape を優先する

### 25.2 `--pretty`

- CLI だけで有効
- 色付けは後回し
- 表形式またはインデント JSON
- LLM 主目的なので prettify は最小限でよい

### 25.3 エラー出力

- stderr に JSON または整形メッセージ
- exit code 非 0

エラー JSON 例:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "deal not found",
    "details": {
      "resource": "deals",
      "id": "123"
    }
  }
}
```

---

## 26. `configure` 実装詳細

### 26.1 `configure`

対話式。  
既存 profile があれば上書き編集モード。

### 26.2 `configure set`

- key path を受け取る
- 例:
  - `profiles.default.api_key`
  - `profiles.readonly.daily_auto_refresh`
  - `timezone`

### 26.3 `configure get`

- key path を受け取る
- 値を JSON 互換形式で返す

### 26.4 `configure list-profiles`

- profile 名の配列を返す

### 26.5 `configure use`

- `current_profile` を更新

### 26.6 `configure current-profile`

- 現在の profile 名を返す

### 26.7 `configure show`

- 対象 profile または全体設定を返す
- secrets はデフォルトでマスク

### 26.8 `configure path`

- `config.toml` の実パスを返す

---

## 27. 検索戦略

### 27.1 モデルB採用

- キャッシュ検索優先
- entity ベース保存
- daily auto refresh は resource 単位
- 必要時に API fallback

### 27.2 list/search/get の考え方

- `get`:
  - キャッシュ優先
  - daily refresh 対象
  - 不在時 API get
- `search`:
  - daily refresh
  - キャッシュ検索
  - 必要なら API search
- `list`:
  - daily refresh
  - 基本は cache から返す
  - 足りない場合のみ API list/search

### 27.3 high-level の検索

high-level では
- customer を手掛かりに deal を探す
- client を手掛かりに project を探す
- deal に紐づく ticket を探す

などの横断検索を行う。

---

## 28. パフォーマンス方針

### 28.1 rate limit 回避

本丸は retry ではなくキャッシュである。  
retry は補助。

### 28.2 一覧 API 呼び出し最小化

- daily refresh は 1 日 1 回 / resource
- `updated_at` 差分取得を活用
- `--force-refresh` は明示時のみ

### 28.3 limit 強制

MCP / CLI ともデフォルト limit を持ち、上限を設ける。

### 28.4 JSON blob での保存

join を避ける。  
高速な一覧展開より実装単純性と安定性を優先する。

---

## 29. セキュリティ方針

### 29.1 secrets

- `config.toml` に保存される API Key / Token はマスク表示のみ
- ログに出さない
- エラーにも出さない
- pretty 出力にも出さない

### 29.2 file permission

可能なら `config.toml` を 0600 相当で作成する。

### 29.3 MCP

MVP はローカル HTTP のみ。  
listen はデフォルト `127.0.0.1` とし、外部公開しない。

---

## 30. テスト戦略

### 30.1 単体テスト

対象:

- config load/save
- refresh policy
- sync_state 更新
- cache upsert
- key path set/get
- search query builder
- output formatting

### 30.2 boardapi テスト

- HTTP モックサーバーを使う
- 認証ヘッダ
- retry
- pagination
- エラー分類

### 30.3 repository テスト

- SQLite 実 DB または temp DB
- cache hit
- cache miss
- daily refresh
- force refresh
- expire 後の挙動

### 30.4 service テスト

- low-level service の list/get/search
- high-level service の組み合わせ検索

### 30.5 MCP テスト

- tool schema
- tool execution
- JSON response
- error response

---

## 31. エッジケース

### 31.1 cache にあるが API では消えている

MVP では stale が残る可能性を許容。  
将来 full refresh 時に prune を導入できるようにする。

### 31.2 refresh 失敗

- 既存 cache があればそれを返す
- `sync_state.last_error_*` を更新
- `consecutive_failures` を増加

### 31.3 get 対象不在

- cache miss
- API 404
- エラー返却
- negative cache は MVP では不要

### 31.4 同時 refresh

- 片方のみ実行
- 他方は古い cache を返すか待機
- MVP は古い cache を返す方針を優先

---

## 32. 実装順序

以下の順で実装する。

### Phase 1: 基盤

1. `config`
2. `boardapi` 共通 client
3. SQLite 初期化と migration
4. `resource_cache`, `sync_state` 実装

### Phase 2: refresh / repository

5. refresh policy
6. resource refresh
7. repository 実装

### Phase 3: low-level CLI

8. `board api deals`
9. `board api projects`
10. `board api tickets`
11. `board api customers`
12. `board api clients`

### Phase 4: configure / cache CLI

13. `configure` 系
14. `cache` 系
15. `completion zsh`

### Phase 5: high-level CLI

16. `board find deal`
17. `board find project`
18. `board find ticket`
19. `board find customer`

### Phase 6: MCP

20. `board mcp serve`
21. `find_*` tool 実装
22. schema / response 安定化

---

## 33. MVP 実装チェックリスト

- [ ] `board configure` が対話式で動く
- [ ] `config.toml` を保存/再読込できる
- [ ] profile 切り替えができる
- [ ] SQLite DB が自動作成される
- [ ] migration が走る
- [ ] `resource_cache` に JSON blob が保存される
- [ ] `sync_state` が更新される
- [ ] daily auto refresh が動く
- [ ] `--refresh` が動く
- [ ] `--force-refresh` が動く
- [ ] `cache status` が動く
- [ ] `cache expire` が動く
- [ ] `cache clear` が動く
- [ ] `board api deals get` が動く
- [ ] `board api deals search` が動く
- [ ] `board find deal` が動く
- [ ] `board mcp serve` が起動する
- [ ] `find_deals` tool が動く
- [ ] `board completion zsh` が動く

---

## 34. 実装上の細かいルール

### 34.1 エンティティ ID

- 文字列で扱う
- 整数でも内部で string に正規化してよい

### 34.2 時刻

- DB 保存時刻は RFC3339 UTC 推奨
- daily 判定時のみ config timezone を使う

### 34.3 JSON

- DB 保存 payload は API response をできるだけそのまま使う
- high-level 返却では軽量フィールド + raw を返す

### 34.4 migration

- SQL ファイル管理でよい
- schema version は `cache_meta` と `sync_state.schema_version` で管理可能

### 34.5 pretty 出力

- 実装負荷を抑えるため、まずは indented JSON でもよい
- 表形式は後から追加してよい

---

## 35. 擬似コード

### 35.1 daily 判定

```text
function needsDailyRefresh(profile, resource, timezone):
    state = sync_state(profile, resource)
    today = nowInTimezone(timezone).format("YYYY-MM-DD")
    if state.last_daily_refresh_date != today:
        return true
    return false
```

### 35.2 get

```text
function getEntity(profile, resource, id, opts):
    if opts.forceRefresh:
        refreshResource(profile, resource, mode=full)
    else if opts.refresh:
        refreshResource(profile, resource, mode=delta)
    else if dailyAutoRefreshEnabled(profile) and needsDailyRefresh(profile, resource):
        refreshResource(profile, resource, mode=delta)

    entity = cache.get(profile, resource, id)
    if entity exists:
        return entity

    entity = api.get(resource, id)
    cache.upsert(profile, resource, entity)
    return entity
```

### 35.3 search

```text
function searchEntities(profile, resource, query, opts):
    if opts.forceRefresh:
        refreshResource(profile, resource, mode=full)
    else if opts.refresh:
        refreshResource(profile, resource, mode=delta)
    else if dailyAutoRefreshEnabled(profile) and needsDailyRefresh(profile, resource):
        refreshResource(profile, resource, mode=delta)

    result = cache.search(profile, resource, query, opts.limit)
    if result is sufficient:
        return result

    apiResult = api.search(resource, query)
    cache.upsertMany(profile, resource, apiResult)
    return merge(result, apiResult)
```

### 35.4 refresh

```text
function refreshResource(profile, resource, mode):
    acquireLock(profile, resource)
    markRunning(profile, resource)

    if mode == full or sync_state.must_full_resync:
        entities = api.listAll(resource)
        cache.upsertMany(profile, resource, entities)
        updateFullSyncState(profile, resource, entities)
    else:
        cursor = sync_state.cursor_updated_at
        entities = api.listUpdatedSince(resource, cursor)
        cache.upsertMany(profile, resource, entities)
        updateDeltaSyncState(profile, resource, entities)

    markSuccess(profile, resource)
    releaseLock(profile, resource)
```

---

## 36. 今後の拡張余地

- write 系 API (`create`, `update`, `delete`)
- full refresh 時の prune
- MCP tool の追加
  - `get_deal`
  - `get_ticket`
  - `create_ticket`
- low-level の API coverage 拡大
- high-level の検索 alias 増加
- pretty 出力改善
- remote MCP + 認証
- config secrets の OS キーチェーン連携

---

## 37. 実装禁止事項

- CLI command に業務ロジックを書かない
- high-level 側で独自永続キャッシュを持たない
- BOARD API 直叩きを service 層の外にばら撒かない
- refresh ロジックを複数箇所に重複実装しない
- config / cache パスを各所で直書きしない
- secrets をログやエラーメッセージに出さない

---

## 38. 最終実装方針まとめ

本プロジェクトは、次の設計思想で実装する。

- BOARD API を叩く low-level CLI と、LLM 向け high-level CLI を両立する
- キャッシュは low-level 側に一元化する
- キャッシュは 1ファイル SQLite + JSON blob entity 保存
- refresh は内部処理とし、daily auto refresh をデフォルト ON とする
- `get/list/search` すべてを daily auto refresh 対象にする
- CLI と MCP は同一 repository / cache / refresh 基盤を共有する
- MCP には high-level のみ公開する
- デフォルト出力は JSON とし、`--pretty` を補助機能とする
- 実装責務を `cli / service / repository / refresh / cache / boardapi` に明確分離する

以上をもって、本設計書を実装の正とする。
