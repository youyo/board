# MCP サーバー利用ガイド

`board mcp serve` は [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) に準拠したローカル HTTP サーバーを起動します。Claude Desktop や Claude Code などの AI アシスタントから `board find` の検索機能をツールとして呼び出せます。

## 前提条件

- `board` がインストール済みで、`board configure` で認証情報が設定済みであること
- AI クライアント（Claude Desktop、Claude Code 等）がインストール済みであること

---

## MCP サーバーの起動

```sh
board mcp serve
```

デフォルトでは `127.0.0.1:3100` で待ち受けます。

```sh
# ポートを変更する
board mcp serve --port 8080

# ホストを変更する（外部から接続を受け付ける場合）
board mcp serve --host 0.0.0.0 --port 3100
```

> セキュリティ上の理由から、デフォルトではローカルホストのみに接続を制限することを推奨します（`--host 0.0.0.0` は信頼できるネットワーク環境でのみ使用してください）。

サーバーが起動すると、接続待機状態になります（Ctrl+C で停止）。

---

## Claude Desktop の設定

Claude Desktop に `board` MCP サーバーを登録します。

### 設定ファイルの場所

| OS    | パス |
|-------|------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |

### 設定例

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

`command` に `board` バイナリのフルパスを指定することもできます（PATH が通っていない場合）:

```json
{
  "mcpServers": {
    "board": {
      "command": "/usr/local/bin/board",
      "args": ["mcp", "serve"]
    }
  }
}
```

設定後、Claude Desktop を再起動すると `board` ツールが利用可能になります。

---

## Claude Code の設定

Claude Code でプロジェクトローカルに設定するには `.mcp.json` を作成します。

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

または CLI フラグで一時的に追加することもできます（Claude Code のバージョンによって異なります。公式ドキュメントを参照してください）。

---

## 利用可能な MCP ツール一覧

`board mcp serve` は以下の 12 ツールを公開します。各ツールは `board find` コマンドに対応しています。

| MCP ツール名 | 対応コマンド | 説明 |
|---|---|---|
| `find_client` | `board find client` | 顧客（支店・担当者を含む）を検索 |
| `find_project` | `board find project` | 案件（顧客情報を含む）を検索 |
| `find_estimate` | `board find estimate` | 見積書を検索 |
| `find_invoice` | `board find invoice` | 請求書を検索 |
| `find_order` | `board find order` | 発注書を検索 |
| `find_delivery` | `board find delivery` | 納品書を検索 |
| `find_receipt` | `board find receipt` | 領収書を検索 |
| `find_vendor` | `board find vendor` | 仕入先（支店・担当者を含む）を検索 |
| `find_purchase_order` | `board find purchase-order` | 発注書（仕入先版）を検索 |
| `find_payment` | `board find payment` | 支払いを検索 |
| `find_user` | `board find user` | ユーザーを検索 |
| `find_group` | `board find group` | グループを検索 |

---

## ツールのパラメータ詳細

### find_client

顧客とその支店・担当者を検索します。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | 顧客 ID（最優先） |
| `name` | string | 顧客名の部分一致検索 |
| `text` | string | 名前・コード・メモの自由テキスト検索 |
| `limit` | integer | 最大件数（デフォルト: 50） |

### find_project

案件とその顧客情報を検索します。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | 案件 ID（最優先） |
| `name` | string | 案件名の部分一致検索 |
| `client_name` | string | 顧客名で案件を絞り込む |
| `status` | string | ステータスでフィルタ |
| `text` | string | 名前・コード・メモの自由テキスト検索 |
| `limit` | integer | 最大件数（デフォルト: 50） |

### find_estimate / find_invoice / find_order / find_delivery / find_receipt

見積書・請求書・受注書・納品書・領収書を検索します（パラメータは共通）。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | ドキュメント ID（最優先） |
| `project_id` | integer | 案件 ID で絞り込む |
| `client_name` | string | 顧客名で絞り込む |
| `project_name` | string | 案件名で絞り込む |
| `status` | string | ステータスでフィルタ |
| `limit` | integer | 最大件数（デフォルト: 50） |

### find_vendor

仕入先とその支店・担当者を検索します。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | 仕入先 ID（最優先） |
| `name` | string | 仕入先名の部分一致検索 |
| `text` | string | 名前・コード・メモの自由テキスト検索 |
| `limit` | integer | 最大件数（デフォルト: 50） |

### find_purchase_order

仕入発注書を検索します。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | 発注書 ID（最優先） |
| `vendor_name` | string | 仕入先名で絞り込む |
| `project_name` | string | 案件名で絞り込む |
| `status` | string | ステータスでフィルタ |
| `text` | string | 自由テキスト検索 |
| `limit` | integer | 最大件数（デフォルト: 50） |

### find_payment

支払いを検索します。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | 支払い ID（最優先） |
| `vendor_name` | string | 仕入先名で絞り込む |
| `purchase_order_id` | integer | 発注書 ID で絞り込む |
| `status` | string | ステータスでフィルタ |
| `text` | string | メモの自由テキスト検索 |
| `limit` | integer | 最大件数（デフォルト: 50） |

### find_user

ユーザーを検索します。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | ユーザー ID（最優先） |
| `name` | string | ユーザー名の部分一致検索 |
| `text` | string | 名前・メールアドレスの自由テキスト検索 |
| `limit` | integer | 最大件数（デフォルト: 50） |

### find_group

グループを検索します。

| パラメータ | 型 | 説明 |
|---|---|---|
| `id` | integer | グループ ID（最優先） |
| `name` | string | グループ名の部分一致検索 |
| `text` | string | 名前・メモの自由テキスト検索 |
| `limit` | integer | 最大件数（デフォルト: 50） |

---

## Claude での使用例

MCP サーバー接続後、Claude に自然言語で依頼できます:

```
「株式会社サンプル」の未払い請求書を調べて
```

Claude は内部で `find_invoice` ツールを呼び出し、顧客名から請求書を検索します。

---

## トラブルシューティング

### MCP サーバーが起動しない

`board configure` で認証情報が設定されているか確認します:

```sh
board configure show
```

ポートが使用中の場合は別のポートを指定します:

```sh
board mcp serve --port 8081
```

### Claude Desktop にツールが表示されない

1. `claude_desktop_config.json` の JSON 形式が正しいか確認します（`command` のパスが正しいかも確認）
2. Claude Desktop を完全に終了して再起動します
3. `board` コマンドが PATH で解決できるか確認します:

   ```sh
   which board
   ```

### ツールを呼び出すと rate limit エラーになる

BOARD API の rate limit（3,000 回/日、3 回/秒）に達した可能性があります。

- キャッシュが有効な場合、2 回目以降の同じリクエストはキャッシュから返却されます
- `--refresh-full` を多用すると rate limit を消費します
- 翌日まで待つか、しばらく時間をおいてから再試行してください

---

## 関連ドキュメント

- [クイックスタート](getting-started.md)
- [API コマンドリファレンス](../api-reference.md)
