# クイックスタート（5 分ガイド）

このガイドでは、`board` CLI のインストール後に BOARD API へ接続し、データを取得するまでの最小限の手順を説明します。

## 前提条件

- `board` がインストール済みであること（[インストールガイド](../installation_ja.md)参照）
- BOARD の管理者権限または API アクセス権限を持つアカウント

---

## Step 1: API キーとアクセストークンの取得

BOARD の Web 管理画面にログインし、API キーとアクセストークンを取得します。

1. BOARD 管理画面にログイン
2. 右上のアカウントメニュー → 「API 設定」または「開発者設定」へ移動
3. 「API キー」と「アクセストークン」を確認またはを新規発行
4. 両方の値をメモしておきます

> API キーとアクセストークンは秘密情報です。他者に漏らさないよう注意してください。

---

## Step 2: 認証情報を設定する（`board configure`）

`board configure` を実行すると、対話式ウィザードが起動します。

```sh
board configure
```

プロンプトに従って入力します:

```
Enter API Key: <API キーを入力>
Enter API Token: <アクセストークンを入力>
Profile name [default]: default
```

設定は `~/.config/board/config.toml` に保存されます。設定内容を確認するには:

```sh
board configure show
```

出力例（シークレットはマスクされます）:

```json
{
  "base_url": "https://api.the-board.jp",
  "api_key": "***",
  "api_token": "***",
  "daily_auto_refresh": true,
  "request_timeout_seconds": 30,
  "retry_max": 5,
  "pretty_default": false
}
```

設定ファイルのパスを確認するには:

```sh
board configure path
```

---

## Step 3: 動作確認（低レベル API コマンド）

`board api` コマンドで BOARD API を直接呼び出し、接続が正常かどうか確認します。

```sh
board api clients list --limit 5 --pretty
```

初回実行時はキャッシュが空のため、API からデータを取得します（数秒かかる場合があります）。

出力例:

```json
[
  {
    "id": 1001,
    "name": "株式会社サンプル",
    "custom_no": "C-001",
    "created_at": "2023-01-15T09:00:00+09:00",
    "updated_at": "2024-06-01T12:00:00+09:00"
  },
  ...
]
```

エラーが表示された場合は「[トラブルシューティング](#トラブルシューティング)」を参照してください。

---

## Step 4: 高レベル検索（`board find` コマンド）

`board find` は複数リソースを横断して人間が読みやすいフィルターで検索できます。LLM との連携や日常的な調査に最適です。

### 顧客を名前で検索

```sh
board find client --name "株式会社" --pretty
```

### 特定顧客の案件を検索

```sh
board find project --client-name "株式会社サンプル" --pretty
```

### ステータスでフィルタ

```sh
board find invoice --client-name "株式会社サンプル" --status draft --pretty
```

### 自由テキスト検索

```sh
board find client --text "東京" --pretty
```

2 回目以降はキャッシュから返却されるため、高速に動作します。デフォルトでは 1 日 1 回キャッシュが自動更新されます。

---

## Step 5: MCP サーバーを起動する

Claude などの AI アシスタントから `board` を使うには、MCP サーバーとして起動します。

```sh
board mcp serve
```

デフォルトでは `127.0.0.1:3100` で待ち受けます。Claude Desktop から接続すると、`board find` の各機能を AI ツールとして利用できます。

詳細な設定方法は [MCP サーバー利用ガイド](mcp-server.md) を参照してください。

---

## Step 6: キャッシュの管理

`board` はローカルの SQLite キャッシュを使って BOARD API の rate limit（3,000 回/日）に対応しています。

```sh
# キャッシュの状態を確認
board cache status

# キャッシュを期限切れにする（次回アクセス時に更新）
board cache expire

# キャッシュをすべて削除して最新データを取得する
board cache clear

# キャッシュファイルのパスを確認
board cache path
```

---

## トラブルシューティング

### 認証エラー（401 Unauthorized）

API キーまたはアクセストークンが正しくありません。

```sh
board configure
```

で再設定してください。現在の設定を確認するには:

```sh
board configure show
```

### 429 Too Many Requests

BOARD API の rate limit（3,000 回/日 または 3 回/秒）に達しました。

- しばらく待つか、翌日まで待ちます
- キャッシュを活用することで API 呼び出し回数を削減できます
- `--refresh` や `--force-refresh` を頻繁に使うと消費量が増えます

### データが古い（キャッシュが更新されない）

デフォルトでは 1 日 1 回自動更新されます。強制的に更新するには:

```sh
board api clients list --refresh      # 差分更新
board api clients list --force-refresh  # 全件再取得
```

または:

```sh
board cache expire   # 次回アクセス時に更新
board cache clear    # キャッシュを完全削除
```

### コマンドが見つからない（command not found）

`board` が `$PATH` に含まれていません。

- Homebrew の場合: `brew doctor` を実行してパス設定を確認
- go install の場合: `export PATH="$PATH:$(go env GOPATH)/bin"` を `~/.zshrc` または `~/.bashrc` に追加

---

## 次のステップ

- [MCP サーバー利用ガイド](mcp-server.md) — Claude Desktop / Claude Code との連携
- [API コマンドリファレンス](../api-reference.md) — 全 22 リソースのコマンド一覧
- 複数プロファイルを使う場合: `board configure list-profiles` / `board configure use <profile>`
