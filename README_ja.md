# board

[![CI](https://github.com/youyo/board/actions/workflows/ci.yml/badge.svg)](https://github.com/youyo/board/actions/workflows/ci.yml)
[![Release](https://github.com/youyo/board/actions/workflows/release.yml/badge.svg)](https://github.com/youyo/board/actions/workflows/release.yml)

[English version](README.md)

[BOARD API](https://api.the-board.jp/v1/) 向けの CLI ツール + ローカル MCP サーバー。単一バイナリ `board` として提供。

- **`board api`** — 22 リソース対応の低レベル API コマンド
- **`board find`** — LLM フレンドリーな高レベル横断検索
- **`board mcp serve`** — ローカル HTTP MCP サーバー（AI アシスタント連携用）
- SQLite キャッシュで BOARD API rate limit（3,000 回/日、3 回/秒）に対応

## インストール

### Homebrew（macOS / Linux）

```sh
brew install youyo/tap/board
```

### GitHub Releases

[Releases](https://github.com/youyo/board/releases) からプラットフォーム向けバイナリをダウンロードし、`$PATH` に配置してください。

### ソースからビルド

```sh
git clone https://github.com/youyo/board.git
cd board
mise run build   # mise + Go 1.26 が必要
```

## クイックスタート

```sh
# 1. 認証情報を設定
board configure

# 2. 顧客を検索
board find client --name "株式会社サンプル"

# 3. その顧客の案件を検索
board find project --client-name "株式会社サンプル"

# 4. 整形して出力
board find invoice --client-name "株式会社サンプル" --pretty
```

## CLI コマンド

### `board configure`

認証情報と設定のインタラクティブセットアップウィザード。

```sh
board configure              # ウィザード実行
board configure show         # 現在の設定を表示（シークレットはマスク）
board configure list-profiles
board configure use <profile>
board configure current-profile
board configure path
```

### `board api`

BOARD API への低レベルアクセス。各リソースは `list`、`get`、`search` をサポート。

```sh
board api <resource> list
board api <resource> get --id <ID>
board api <resource> search [flags]
```

**利用可能なリソース**（22 種）:

| カテゴリ | リソース |
|----------|---------|
| コア | `clients`、`client-branches`、`contacts`、`projects`、`project-costs` |
| ドキュメント | `estimates`、`invoices`、`orders`、`deliveries`、`receipts` |
| ベンダー | `vendors`、`vendor-branches`、`vendor-contacts`、`purchase-orders`、`payments` |
| マスタ | `users`、`groups`、`payment-terms`、`project-types`、`purchase-types`、`accounting-types`、`document-send-channels` |

### `board find`

LLM での利用を想定した高レベル横断検索。人間が読めるフィルターを受け付けます。

```sh
board find client --name "サンプル" --text "キーワード"
board find project --client-name "サンプル" --status active
board find invoice --client-name "サンプル" --status draft
board find vendor --name "仕入先"
board find purchase-order --vendor-name "仕入先" --status open
```

**利用可能なリソース**（12 種）: `client`、`project`、`estimate`、`invoice`、`order`、`delivery`、`receipt`、`vendor`、`purchase-order`、`payment`、`user`、`group`

### `board cache`

ローカル SQLite キャッシュの管理。

```sh
board cache status   # キャッシュ状態を表示
board cache expire   # キャッシュを期限切れにする（次回アクセス時にリフレッシュ）
board cache clear    # すべてのキャッシュデータを削除
board cache path     # キャッシュファイルのパスを表示
```

### `board mcp serve`

`board find` を MCP ツールとして公開するローカル HTTP MCP サーバーを起動します。

```sh
board mcp serve                      # デフォルト: 127.0.0.1:3100
board mcp serve --host 0.0.0.0 --port 8080
```

### `board completion`

シェル補完スクリプトを生成します。

```sh
board completion zsh  | sudo tee /usr/local/share/zsh/site-functions/_board
board completion bash > /etc/bash_completion.d/board
```

## グローバルフラグ

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `--profile`、`-p` | （現在のプロファイル） | 使用するプロファイル名 |
| `--pretty` | false | JSON を整形して出力 |
| `--limit` | 50 | 返す最大件数 |
| `--refresh` | false | 読み込み前にキャッシュをリフレッシュ |
| `--force-refresh` | false | キャッシュを強制的に全件リフレッシュ |
| `--version` | — | バージョンを表示 |

## 設定

設定ファイルは TOML 形式で、`~/.config/board/config.toml` に保存されます（XDG: `XDG_CONFIG_HOME` で変更可能）。`board configure path` で解決パスを確認できます。

```toml
current_profile = "default"
timezone = "UTC"

[profiles.default]
base_url = "https://api.the-board.jp"
api_key = ""        # x-api-key ヘッダー
api_token = ""      # Bearer トークン
daily_auto_refresh = true
request_timeout_seconds = 30
retry_max = 5
pretty_default = false
```

複数プロファイルをサポート。`board configure use <profile>` またはコマンドごとに `-p <profile>` で切り替えます。

## MCP サーバー

`board mcp serve` は [Model Context Protocol](https://modelcontextprotocol.io/) を実装したローカル HTTP サーバーを起動します。Claude などの AI アシスタントが接続し、`board find` の各操作を MCP ツールとして呼び出せます。

**Claude Desktop 設定例**（`claude_desktop_config.json`）:

```json
{
  "mcpServers": {
    "board": {
      "command": "board",
      "args": ["mcp", "serve"]
    }
  }
}
```

利用可能な MCP ツールは `board find` に対応: `find_client`、`find_project`、`find_estimate`、`find_invoice`、`find_order`、`find_delivery`、`find_receipt`、`find_vendor`、`find_purchase_order`、`find_payment`、`find_user`、`find_group`

## アーキテクチャ

```
CLI / MCP
  → service/api（低レベル）  /  service/find（高レベル）
  → repository（キャッシュ参照 → リフレッシュ判定 → API フォールバック）
  → refresh（日次 / 差分 / 強制）  +  cache（SQLite）
  → boardapi（HTTP クライアント、認証、リトライ、ページネーション）
```

## ライセンス

[MIT](LICENSE)
