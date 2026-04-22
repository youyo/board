# API コマンドリファレンス

`board api` は BOARD API の 22 リソースへ直接アクセスするための低レベルコマンドです。各リソースに対して `list`、`get`、`search` サブコマンドを提供します（リソースによって利用できる操作は異なります）。

出力は常に JSON 形式です。`--pretty` を付けると整形されます。

---

## グローバルフラグ

すべての `board api` コマンドに共通するフラグです。

| フラグ | 型 | デフォルト | 説明 |
|--------|-----|-----------|------|
| `--pretty` | bool | false | JSON を整形して出力 |
| `--limit` | int | 50 | 返す最大件数（0 = 上限なし） |
| `--profile`, `-p` | string | （現在のプロファイル） | 使用するプロファイル名 |
| `--refresh` | bool | false | 読み込み前にキャッシュを差分更新 |
| `--force-refresh` | bool | false | 読み込み前にキャッシュを全件再取得 |

---

## 共通フラグ（list）

`list` サブコマンドはページネーションをサポートします。

| フラグ | 型 | デフォルト | 説明 |
|--------|-----|-----------|------|
| `--page` | int | — | ページ番号（1 始まり）。指定するとキャッシュをバイパスして API を直接呼び出す |
| `--per-page` | int | 50 | 1 ページあたりの件数（最大 100） |

---

## リソース一覧

### コア

#### clients — 顧客マスタ

BOARD API エンドポイント: `GET /v1/clients`

```sh
# 一覧取得
board api clients list

# ID で取得
board api clients get --id <ID>

# 検索
board api clients search --name "株式会社"
board api clients search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--name` | 顧客名でフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

---

#### client_branches — 顧客支店

BOARD API エンドポイント: `GET /v1/client_branches`

```sh
# 一覧取得
board api client_branches list

# ID で取得
board api client_branches get --id <ID>

# 検索
board api client_branches search --client-id <顧客ID>
board api client_branches search --name "本社"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--client-id` | 顧客 ID でフィルタ |
| `--name` | 支店名でフィルタ |

---

#### contacts — 顧客担当者

BOARD API エンドポイント: `GET /v1/contacts`

```sh
# 一覧取得
board api contacts list

# ID で取得
board api contacts get --id <ID>

# 検索
board api contacts search --client-id <顧客ID>
board api contacts search --name "田中"
board api contacts search --email "tanaka@example.com"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--client-id` | 顧客 ID でフィルタ |
| `--name` | 担当者名でフィルタ |
| `--email` | メールアドレスでフィルタ |

---

#### projects — 案件

BOARD API エンドポイント: `GET /v1/projects`

```sh
# 一覧取得
board api projects list

# ID で取得（response_group 指定でサブドキュメントも取得可能）
board api projects get --id <ID>

# 検索
board api projects search --client-id <顧客ID>
board api projects search --name "プロジェクト名"
board api projects search --status active
board api projects search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--client-id` | 顧客 ID でフィルタ |
| `--name` | 案件名でフィルタ |
| `--status` | ステータスでフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

> **注意**: `projects get` は関連する見積書・受注書・納品書・請求書・領収書のサマリーを `Deliveries` / `Invoices` / `Receipts` フィールドとして返します。

---

#### project_costs — 案件原価

BOARD API エンドポイント: `GET /v1/project_costs`

```sh
# 一覧取得
board api project_costs list

# ID で取得
board api project_costs get --id <ID>

# 検索
board api project_costs search --project-id <案件ID>
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--project-id` | 案件 ID でフィルタ |

---

### ドキュメント

ドキュメント系リソース（estimates、orders、deliveries、receipts）は BOARD API の設計上 `get` のみ対応しています。一覧・検索は `board find` コマンドを使用してください。

invoices のみ `list` と `search` も利用できます。

---

#### estimates — 見積書

BOARD API エンドポイント: `GET /v1/estimates/:id`

```sh
# ドキュメント ID で取得
board api estimates get --id <ドキュメントID>
```

> **注意**: 見積書の一覧・検索には `board find estimate` を使用してください。

---

#### invoices — 請求書

BOARD API エンドポイント: `GET /v1/invoices`

```sh
# 一覧取得
board api invoices list

# ID で取得
board api invoices get --id <ID>

# 検索
board api invoices search --client-id <顧客ID>
board api invoices search --project-id <案件ID>
board api invoices search --status draft
board api invoices search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--client-id` | 顧客 ID でフィルタ |
| `--project-id` | 案件 ID でフィルタ |
| `--status` | ステータスでフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

---

#### orders — 受注書

BOARD API エンドポイント: `GET /v1/orders/:id`

```sh
# ドキュメント ID で取得
board api orders get --id <ドキュメントID>
```

> **注意**: 受注書の一覧・検索には `board find order` を使用してください。

---

#### deliveries — 納品書

BOARD API エンドポイント: `GET /v1/deliveries/:id`

```sh
# ドキュメント ID で取得
board api deliveries get --id <ドキュメントID>
```

> **注意**: 納品書の一覧・検索には `board find delivery` を使用してください。

---

#### receipts — 領収書

BOARD API エンドポイント: `GET /v1/receipts/:id`

```sh
# ドキュメント ID で取得
board api receipts get --id <ドキュメントID>
```

> **注意**: 領収書の一覧・検索には `board find receipt` を使用してください。

---

### ベンダー

#### vendors — 仕入先マスタ

BOARD API エンドポイント: `GET /v1/vendors`

```sh
# 一覧取得
board api vendors list

# ID で取得
board api vendors get --id <ID>

# 検索
board api vendors search --name "仕入先名"
board api vendors search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--name` | 仕入先名でフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

---

#### vendor_branches — 仕入先支店

BOARD API エンドポイント: `GET /v1/vendor_branches`

```sh
# 一覧取得
board api vendor_branches list

# ID で取得
board api vendor_branches get --id <ID>

# 検索
board api vendor_branches search --vendor-id <仕入先ID>
board api vendor_branches search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--vendor-id` | 仕入先 ID でフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

---

#### vendor_contacts — 仕入先担当者

BOARD API エンドポイント: `GET /v1/vendor_contacts`

```sh
# 一覧取得
board api vendor_contacts list

# ID で取得
board api vendor_contacts get --id <ID>

# 検索
board api vendor_contacts search --vendor-id <仕入先ID>
board api vendor_contacts search --name "担当者名"
board api vendor_contacts search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--vendor-id` | 仕入先 ID でフィルタ |
| `--name` | 担当者名でフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

---

#### purchase_orders — 発注書（仕入側）

BOARD API エンドポイント: `GET /v1/purchase_orders`

```sh
# 一覧取得
board api purchase_orders list

# ID で取得
board api purchase_orders get --id <ID>

# 検索
board api purchase_orders search --vendor-id <仕入先ID>
board api purchase_orders search --project-id <案件ID>
board api purchase_orders search --status open
board api purchase_orders search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--vendor-id` | 仕入先 ID でフィルタ |
| `--project-id` | 案件 ID でフィルタ |
| `--status` | ステータスでフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

---

#### payments — 支払い

BOARD API エンドポイント: `GET /v1/payments`

```sh
# 一覧取得
board api payments list

# ID で取得
board api payments get --id <ID>

# 検索
board api payments search --vendor-id <仕入先ID>
board api payments search --purchase-order-id <発注書ID>
board api payments search --status paid
board api payments search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ:

| フラグ | 説明 |
|--------|------|
| `--vendor-id` | 仕入先 ID でフィルタ |
| `--purchase-order-id` | 発注書 ID でフィルタ |
| `--status` | ステータスでフィルタ |
| `--updated-at-from` | 更新日時の下限（ISO 8601 形式） |

---

### マスタ

マスタ系リソースは通常変更頻度が低く、一覧取得が主な用途です。各リソースは `list`、`get`、`search` をサポートします。

#### users — ユーザー

```sh
board api users list
board api users get --id <ID>
board api users search --name "山田"
board api users search --email "yamada@example.com"
board api users search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ: `--name`、`--email`、`--updated-at-from`

---

#### groups — グループ

```sh
board api groups list
board api groups get --id <ID>
board api groups search --name "営業部"
board api groups search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ: `--name`、`--updated-at-from`

---

#### payment_terms — 支払条件

```sh
board api payment_terms list
board api payment_terms get --id <ID>
board api payment_terms search --name "月末締め翌月払い"
board api payment_terms search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ: `--name`、`--updated-at-from`

---

#### project_types — 案件種別

```sh
board api project_types list
board api project_types get --id <ID>
board api project_types search --name "受託開発"
board api project_types search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ: `--name`、`--updated-at-from`

---

#### purchase_types — 購買種別

```sh
board api purchase_types list
board api purchase_types get --id <ID>
board api purchase_types search --name "外注費"
board api purchase_types search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ: `--name`、`--updated-at-from`

---

#### accounting_types — 勘定科目種別

```sh
board api accounting_types list
board api accounting_types get --id <ID>
board api accounting_types search --name "売上"
board api accounting_types search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ: `--name`、`--updated-at-from`

---

#### document_send_channels — 書類送付方法

```sh
board api document_send_channels list
board api document_send_channels get --id <ID>
board api document_send_channels search --name "郵送"
board api document_send_channels search --updated-at-from "2024-01-01T00:00:00+09:00"
```

search フラグ: `--name`、`--updated-at-from`

---

## find コマンドとの使い分け

| 用途 | コマンド |
|------|---------|
| API レスポンスをそのまま取得したい | `board api` |
| 人間が読みやすいフィルターで横断検索したい | `board find` |
| AI アシスタントから使いたい | `board find`（MCP 経由） |
| 見積書・受注書・納品書・領収書の一覧が必要 | `board find estimate/order/delivery/receipt` |

`board find` は複数リソースを組み合わせてより便利な検索を提供します。詳細は [クイックスタート](guides/getting-started.md) を参照してください。

---

## 関連ドキュメント

- [インストール](installation_ja.md)
- [クイックスタート](guides/getting-started.md)
- [MCP サーバー利用ガイド](guides/mcp-server.md)
