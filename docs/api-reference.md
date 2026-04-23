# API コマンドリファレンス

`board api` は BOARD API の 22 リソースへ直接アクセスするための低レベルコマンドです。各リソースに対して `list` と `get` サブコマンドを提供します（v0.5.0 で `search` を廃止し `list` に統合）。

出力は常に JSON 形式です。`--pretty` を付けると整形されます。

---

## グローバルフラグ

すべての `board api` コマンドに共通するフラグです。

| フラグ | 型 | デフォルト | 説明 |
|--------|-----|-----------|------|
| `--pretty` | bool | false | JSON を整形して出力 |
| `--limit` | int | 0（無制限） | 返す最大件数（0 = 上限なし） |
| `--profile`, `-p` | string | （現在のプロファイル） | 使用するプロファイル名 |
| `--refresh` | bool | false | 読み込み前にキャッシュを差分更新 |
| `--force-refresh` | bool | false | 読み込み前にキャッシュを全件再取得 |

---

## 共通フラグ（list）

`list` サブコマンドは以下のフラグをサポートします。

| フラグ | 型 | デフォルト | 説明 |
|--------|-----|-----------|------|
| `--name-cont` | string | — | 名称の部分一致フィルタ（Ransack 風） |
| `--updated-at-gteq` | string | — | 更新日時の下限（`YYYY-MM-DD` 形式） |
| `--updated-at-lteq` | string | — | 更新日時の上限（`YYYY-MM-DD` 形式） |
| `--include-archive-flg` | bool | false | アーカイブ済みを含める |
| `--show-meta` | bool | false | `_meta`（X-Total-Count / Rate Limit / ETag 等）を JSON に含める |

> **注意**: `_meta` フィールドに含まれる `rate_limit_remaining` で API レート上限（3,000 回/日、3 回/秒）の残量を確認できます。

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

# フィルタ付き一覧（Ransack 風）
board api clients list --name-cont "株式会社"
board api clients list --updated-at-gteq 2024-01-01
board api clients list --response-group large --show-meta
```

list フラグ（clients 固有）:

| フラグ | 説明 |
|--------|------|
| `--name-cont` | 顧客名の部分一致 |
| `--name-disp-cont` | 表示名の部分一致 |
| `--invoice-system-number-eq` | 請求システム番号の完全一致 |
| `--custom-no-eq` | カスタム番号の完全一致 |
| `--tags` | タグでフィルタ（カンマ区切り） |
| `--response-group` | `small` または `large`（追加フィールドを取得） |
| `--updated-at-gteq` | 更新日時の下限（`YYYY-MM-DD`） |
| `--updated-at-lteq` | 更新日時の上限（`YYYY-MM-DD`） |
| `--include-archive-flg` | アーカイブ済みを含める |

---

#### client_branches — 顧客支店

BOARD API エンドポイント: `GET /v1/client_branches`

```sh
# 一覧取得
board api client_branches list

# ID で取得
board api client_branches get --id <ID>

# フィルタ付き一覧
board api client_branches list --client-id-eq <顧客ID>
board api client_branches list --name-cont "本社"
```

list フラグ（client_branches 固有）:

| フラグ | 説明 |
|--------|------|
| `--client-id-eq` | 顧客 ID の完全一致 |
| `--name-cont` | 支店名の部分一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

---

#### contacts — 顧客担当者

BOARD API エンドポイント: `GET /v1/contacts`

```sh
# 一覧取得
board api contacts list

# ID で取得
board api contacts get --id <ID>

# フィルタ付き一覧
board api contacts list --client-id-eq <顧客ID>
board api contacts list --name-cont "田中"
board api contacts list --email-cont "tanaka@example.com"
```

list フラグ（contacts 固有）:

| フラグ | 説明 |
|--------|------|
| `--client-id-eq` | 顧客 ID の完全一致 |
| `--name-cont` | 担当者名の部分一致 |
| `--email-cont` | メールアドレスの部分一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

---

#### projects — 案件

BOARD API エンドポイント: `GET /v1/projects`

```sh
# 一覧取得
board api projects list

# ID で取得（response_group 指定でサブドキュメントも取得可能）
board api projects get --id <ID>
board api projects get --id <ID> --response-group invoice

# フィルタ付き一覧
board api projects list --client-id-eq <顧客ID>
board api projects list --name-cont "プロジェクト名"
board api projects list --order-status-in 1,2
board api projects list --updated-at-gteq 2024-01-01 --show-meta
```

list フラグ（projects 固有）:

| フラグ | 説明 |
|--------|------|
| `--name-cont` | 案件名の部分一致 |
| `--client-id-eq` | 顧客 ID の完全一致 |
| `--client-name-cont` | 顧客名の部分一致 |
| `--order-status-in` | 受注ステータス（カンマ区切り整数） |
| `--delivery-status-in` | 納品ステータス（カンマ区切り整数） |
| `--project-no-eq` | 案件番号の完全一致 |
| `--management-no-eq` | 管理番号の完全一致 |
| `--delivery-date-gteq` | 納品日の下限（`YYYY-MM-DD`） |
| `--delivery-date-lteq` | 納品日の上限（`YYYY-MM-DD`） |
| `--invoice-date-gteq` | 請求日の下限（`YYYY-MM-DD`） |
| `--invoice-date-lteq` | 請求日の上限（`YYYY-MM-DD`） |
| `--invoice-timing-kbn-in` | 請求タイミング区分（カンマ区切り整数） |
| `--tags` | タグでフィルタ（カンマ区切り） |
| `--include-lost-flg` | 失注を含める |
| `--include-archive-flg` | アーカイブ済みを含める |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |
| `--response-group` | 追加フィールドを取得（`invoice`、`estimate` 等） |

> **注意**: `projects get` は関連する見積書・受注書・納品書・請求書・領収書のサマリーを `Deliveries` / `Invoices` / `Receipts` フィールドとして返します。

---

#### project_costs — 案件原価

BOARD API エンドポイント: `GET /v1/project_costs`

```sh
# 一覧取得
board api project_costs list

# ID で取得
board api project_costs get --id <ID>

# フィルタ付き一覧
board api project_costs list --project-id-eq <案件ID>
```

list フラグ（project_costs 固有）:

| フラグ | 説明 |
|--------|------|
| `--project-id-eq` | 案件 ID の完全一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

---

### ドキュメント

ドキュメント系リソース（estimates、orders、deliveries、receipts）は BOARD API の設計上 `get` のみ対応しています。一覧・検索は `board find` コマンドを使用してください。

invoices のみ `list` も利用できます。

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

# フィルタ付き一覧
board api invoices list --client-id-eq <顧客ID>
board api invoices list --project-id-eq <案件ID>
board api invoices list --updated-at-gteq 2024-01-01
```

list フラグ（invoices 固有）:

| フラグ | 説明 |
|--------|------|
| `--client-id-eq` | 顧客 ID の完全一致 |
| `--project-id-eq` | 案件 ID の完全一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

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

BOARD API エンドポイント: `GET /v1/payees`

```sh
# 一覧取得
board api vendors list

# ID で取得
board api vendors get --id <ID>

# フィルタ付き一覧
board api vendors list --name-cont "仕入先名"
board api vendors list --updated-at-gteq 2024-01-01
```

list フラグ（vendors 固有）:

| フラグ | 説明 |
|--------|------|
| `--name-cont` | 仕入先名の部分一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |
| `--include-archive-flg` | アーカイブ済みを含める |

---

#### vendor_branches — 仕入先支店

BOARD API エンドポイント: `GET /v1/payee_branches`

```sh
# 一覧取得
board api vendor_branches list

# ID で取得
board api vendor_branches get --id <ID>

# フィルタ付き一覧
board api vendor_branches list --payee-id-eq <仕入先ID>
board api vendor_branches list --updated-at-gteq 2024-01-01
```

list フラグ（vendor_branches 固有）:

| フラグ | 説明 |
|--------|------|
| `--payee-id-eq` | 仕入先 ID の完全一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

---

#### vendor_contacts — 仕入先担当者

BOARD API エンドポイント: `GET /v1/payee_contacts`

```sh
# 一覧取得
board api vendor_contacts list

# ID で取得
board api vendor_contacts get --id <ID>

# フィルタ付き一覧
board api vendor_contacts list --payee-id-eq <仕入先ID>
board api vendor_contacts list --name-cont "担当者名"
```

list フラグ（vendor_contacts 固有）:

| フラグ | 説明 |
|--------|------|
| `--payee-id-eq` | 仕入先 ID の完全一致 |
| `--name-cont` | 担当者名の部分一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

---

#### purchase_orders — 発注書（仕入側）

BOARD API エンドポイント: `GET /v1/purchase_orders`

```sh
# 一覧取得
board api purchase_orders list

# ID で取得
board api purchase_orders get --id <ID>

# フィルタ付き一覧
board api purchase_orders list --vendor-id-eq <仕入先ID>
board api purchase_orders list --project-id-eq <案件ID>
board api purchase_orders list --updated-at-gteq 2024-01-01
```

list フラグ（purchase_orders 固有）:

| フラグ | 説明 |
|--------|------|
| `--vendor-id-eq` | 仕入先 ID の完全一致 |
| `--project-id-eq` | 案件 ID の完全一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

---

#### payments — 支払い

BOARD API エンドポイント: `GET /v1/payments`

```sh
# 一覧取得
board api payments list

# ID で取得
board api payments get --id <ID>

# フィルタ付き一覧
board api payments list --vendor-id-eq <仕入先ID>
board api payments list --purchase-order-id-eq <発注書ID>
board api payments list --updated-at-gteq 2024-01-01
```

list フラグ（payments 固有）:

| フラグ | 説明 |
|--------|------|
| `--vendor-id-eq` | 仕入先 ID の完全一致 |
| `--purchase-order-id-eq` | 発注書 ID の完全一致 |
| `--updated-at-gteq` | 更新日時の下限 |
| `--updated-at-lteq` | 更新日時の上限 |

---

### マスタ

マスタ系リソースは通常変更頻度が低く、一覧取得が主な用途です。各リソースは `list` と `get` をサポートします。

#### users — ユーザー

```sh
board api users list
board api users list --name-cont "山田"
board api users list --updated-at-gteq 2024-01-01
board api users get --id <ID>
```

list フラグ: `--name-cont`、`--updated-at-gteq`、`--updated-at-lteq`、`--include-archive-flg`

---

#### groups — グループ

```sh
board api groups list
board api groups list --name-cont "営業部"
board api groups list --updated-at-gteq 2024-01-01
board api groups get --id <ID>
```

list フラグ: `--name-cont`、`--updated-at-gteq`、`--updated-at-lteq`

---

#### payment_terms — 支払条件

```sh
board api payment_terms list
board api payment_terms list --name-cont "月末締め翌月払い"
board api payment_terms get --id <ID>
```

list フラグ: `--name-cont`、`--updated-at-gteq`、`--updated-at-lteq`

---

#### project_types — 案件種別

```sh
board api project_types list
board api project_types list --name-cont "受託開発"
board api project_types get --id <ID>
```

list フラグ: `--name-cont`、`--updated-at-gteq`、`--updated-at-lteq`

---

#### purchase_types — 購買種別

```sh
board api purchase_types list
board api purchase_types list --name-cont "外注費"
board api purchase_types get --id <ID>
```

list フラグ: `--name-cont`、`--updated-at-gteq`、`--updated-at-lteq`

---

#### accounting_types — 勘定科目種別

```sh
board api accounting_types list
board api accounting_types list --name-cont "売上"
board api accounting_types get --id <ID>
```

list フラグ: `--name-cont`、`--updated-at-gteq`、`--updated-at-lteq`

---

#### document_send_channels — 書類送付方法

```sh
board api document_send_channels list
board api document_send_channels list --name-cont "郵送"
board api document_send_channels get --id <ID>
```

list フラグ: `--name-cont`、`--updated-at-gteq`、`--updated-at-lteq`

---

## v0.5.0 破壊的変更

v0.5.0 で以下の変更が行われました。キャッシュの再構築が必要です。

```sh
board cache clear
```

- **`search` サブコマンドを廃止**: `list` サブコマンドに統合。フィルタは Ransack 風フラグ（`--name-cont`、`--client-id-eq` 等）で指定します
- **フラグ命名規則の変更**: `--name` → `--name-cont`、`--updated-at-from` → `--updated-at-gteq`、`--client-id` → `--client-id-eq` 等
- **`_meta` フィールドの導入**: `--show-meta` で `X-Total-Count`、`Rate Limit` 等のヘッダー情報を JSON で取得可能

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
