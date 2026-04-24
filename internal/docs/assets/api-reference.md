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

## サンプル JSON

代表的な list / get コマンドのレスポンス例です。実 API dump から必須フィールドのみ抽出しています。

### list（一覧取得）

`--show-meta` を付けると `_meta` フィールドに X-Total-Count / Rate Limit / ETag などのヘッダー情報が含まれます。

```sh
board api clients list --limit 2 --show-meta --pretty
```

```json
{
  "items": [
    {
      "ID": 1001,
      "Name": "株式会社サンプル",
      "NameDisp": "サンプル",
      "CustomNo": "C-001",
      "Tags": ["重要顧客"],
      "UpdatedAt": "2026-04-01T10:00:00+09:00"
    },
    {
      "ID": 1002,
      "Name": "株式会社テスト",
      "NameDisp": "テスト",
      "CustomNo": "C-002",
      "Tags": [],
      "UpdatedAt": "2026-04-02T11:30:00+09:00"
    }
  ],
  "_meta": {
    "total_count": 128,
    "page": 1,
    "per_page": 2,
    "rate_limit_remaining": 2987,
    "rate_limit_reset_at": "2026-04-24T09:00:00Z",
    "etag": "W/\"abc123\""
  }
}
```

### get（ID 指定取得）

`projects get` は `--response-group` を指定するとネストされたサブドキュメント（見積書・受注書・納品書・請求書・領収書）も取得できます。

```sh
board api projects get --id 42 --response-group all --pretty
```

```json
{
  "ID": 42,
  "Name": "新規サイト構築案件",
  "ProjectNo": "P-2026-042",
  "Client": {
    "ID": 1001,
    "Name": "株式会社サンプル"
  },
  "OrderStatus": 4,
  "DeliveryStatus": 2,
  "InvoiceTimingKbn": 1,
  "Estimates": [
    { "ID": 501, "DocumentNo": "E-2026-042-01", "Total": 1000000 }
  ],
  "Orders": [
    { "ID": 601, "DocumentNo": "O-2026-042-01", "Total": 1000000 }
  ],
  "Deliveries": [],
  "Invoices": [],
  "Receipts": [],
  "UpdatedAt": "2026-04-20T14:00:00+09:00"
}
```

### invoices list（請求書一覧）

```sh
board api invoices list --client-id-eq 1001 --limit 1 --pretty
```

```json
{
  "items": [
    {
      "ID": 7001,
      "DocumentNo": "I-2026-001",
      "ClientID": 1001,
      "ProjectID": 42,
      "InvoiceDate": "2026-04-15",
      "Total": 1100000,
      "Status": "sent",
      "UpdatedAt": "2026-04-15T16:00:00+09:00"
    }
  ]
}
```

---

## エラー応答例

### BOARD API から伝播するエラー

BOARD API から 4xx / 5xx が返ると、`board` CLI は exit code 1 で stderr に JSON 形式のエラーを出力します。

#### 401 Unauthorized（認証失敗）

```json
{
  "error": true,
  "message": "BOARD API error: 401 Unauthorized: invalid api_key or api_token",
  "status": 401
}
```

対処: `board configure show` で `api_key` と `api_token` を確認し、正しい値を設定してください。

#### 404 Not Found（存在しない ID）

```sh
board api clients get --id 99999999
```

```json
{
  "error": true,
  "message": "BOARD API error: 404 Not Found: client not found",
  "status": 404
}
```

#### 429 Too Many Requests（rate limit 超過）

```json
{
  "error": true,
  "message": "BOARD API error: 429 Too Many Requests",
  "status": 429,
  "retry_after_seconds": 1
}
```

BOARD API は 3 req/sec、3000 req/day のレート制限があります。`board` 内蔵 SQLite キャッシュが自動で負荷を抑えるため通常は発生しませんが、`--force-refresh` 多用時などに発生しうる。

### CLI 自体のエラー

BOARD API を呼ばずに CLI 側で検知するエラー（不正フラグ、未知リソースなど）も同形式で stderr に出ます。

```sh
board docs foobar
```

```json
{
  "error": true,
  "message": "docs: section not found: foobar"
}
```

```sh
board docs --format xml
```

```json
{
  "error": true,
  "message": "unsupported format: xml (use text or json)"
}
```

---

## Ransack フィルタ完全表（オペレータ俯瞰）

Phase L（v0.5.0）以降、すべての list フラグは Ransack 風命名規則に統一されています。
以下はオペレータの俯瞰表です。リソースごとの利用可能フラグは [リソース一覧](#リソース一覧) の各セクションを参照してください。

### サポートオペレータ

| サフィックス | 意味 | 値の型 | 例 |
|-------------|------|--------|-----|
| `_eq` | 完全一致 | int / string | `--client-id-eq 1001` / `--project-no-eq "P-001"` |
| `_cont` | 部分一致（contains） | string | `--name-cont "株式会社"` |
| `_in` | いずれかに一致（OR） | カンマ区切り int | `--order-status-in 1,2,4` |
| `_gteq` | 以上（>=） | date / datetime | `--updated-at-gteq 2024-01-01` |
| `_lteq` | 以下（<=） | date / datetime | `--delivery-date-lteq 2026-12-31` |

### リソース × オペレータの利用可能性（概要）

全 22 リソースが `--updated-at-gteq` / `--updated-at-lteq` をサポート。それ以外はリソース別詳細セクションを参照。

| リソースカテゴリ | `_eq` | `_cont` | `_in` | `_gteq` / `_lteq` |
|------------------|:-----:|:------:|:-----:|:-----------------:|
| コア（clients / projects など 5 種） | ○ | ○ | ○ (projects のみ) | ○ |
| ドキュメント（invoices など） | ○ | — | — | ○ |
| ベンダー（vendors など 5 種） | ○ | ○ | — | ○ |
| マスタ（users など 7 種） | — | ○ (name のみ) | — | ○ |

### 日付フォーマット

`_gteq` / `_lteq` の値は以下を受け付けます:

- 日付のみ: `2026-04-24` → 00:00:00+09:00 として扱われる
- 日時: `2026-04-24T10:00:00+09:00`（ISO 8601）

タイムゾーンは `config.toml` の `timezone` 設定に従います（デフォルト UTC）。

### AND / OR

同一フラグを複数指定した場合は AND で結合されます。OR が必要な場合は `_in` オペレータ（カンマ区切り）を使用してください。

```sh
# AND: 顧客 ID = 1001 かつ 更新日 >= 2024-01-01
board api projects list --client-id-eq 1001 --updated-at-gteq 2024-01-01

# OR: 受注ステータスが 1, 2, 4 のいずれか
board api projects list --order-status-in 1,2,4
```

---

## CLI 補完値一覧

`board completion zsh`（または `bash` / `fish`）で生成される補完スクリプトは、以下の固定列挙フラグで値補完を提供します。TAB 押下で候補と日本語説明が表示されます（zsh のみ、bash は候補値のみ）。

| フラグ | 対象コマンド | 補完候補 |
|--------|------------|---------|
| `--response-group` | `api clients list` | `small`, `large` |
| `--response-group` | `api invoices list` | `small`, `large` |
| `--response-group` | `api payments list` | `small`, `large` |
| `--response-group` | `api purchase_orders list` | `small`, `large` |
| `--response-group` | `api projects list` | `small`, `large`, `estimate`, `order`, `delivery`, `invoice`, `receipt`, `all`（8 値） |
| `--response-group` | `api projects get` | `estimate`, `order`, `delivery`, `invoice`, `receipt`, `all`（6 値） |
| `--order-status-in` | `api projects list` | `1`=見積中(高) / `2`=見積中(中) / `3`=見積中(低) / `4`=受注確定 / `5`=受注済 / `8`=見積中(除) |
| `--delivery-status-in` | `api projects list` | `1`=未着手 / `2`=着手中 / `3`=納品済 / `4`=検収済 |
| `--invoice-timing-kbn-in` | `api projects list` | `1`=一括請求 / `2`=定期請求 |
| `--format` | `docs` | `text`, `json` |

> **注意**: `--status-eq`（invoices / payments / purchase_orders）の補完値は BOARD API 仕様書に明示された列挙値がないため、現時点では補完対象外です。

### 補完スクリプトの導入

```sh
# zsh
board completion zsh | sudo tee /usr/local/share/zsh/site-functions/_board

# bash
board completion bash | sudo tee /etc/bash_completion.d/board

# fish
board completion fish > ~/.config/fish/completions/board.fish
```

---

## board docs サブコマンド

`board docs` はバイナリに埋め込まれたドキュメント（README / api-reference / installation / guides）を CLI から参照できるサブコマンドです。

```sh
board docs                          # README を表示
board docs --list                   # 埋め込みドキュメント一覧
board docs clients                  # api-reference.md から clients セクションを抽出
board docs clients --format json    # JSON 形式で取得（LLM / MCP 向け）
board docs --search "Ransack"       # 全文検索（大文字小文字無視、±2 行コンテキスト）
```

### `--format json` 出力スキーマ

トップレベルは `{mode, query, results}` の 3 フィールド。`mode` に応じて `results[]` の形が変わります。

| mode | query | results[].file | results[].section | results[].content | results[].line | results[].size |
|------|-------|----------------|-------------------|-------------------|:--------------:|:--------------:|
| `readme` | — | `"README.md"` | — | 全文 | — | — |
| `list` | — | 各ファイルの相対パス | — | — | — | バイトサイズ |
| `search` | キーワード | マッチしたファイル | 直近 `####` 見出し | ±2 行 context | マッチ行番号（1-based） | — |
| `resource` | リソース名 | `"api-reference.md"` | リソース名 | 抽出本文 | — | — |

### 例（search モード）

```sh
board docs --search "Ransack" --format json | jq '.results[0]'
```

```json
{
  "file": "api-reference.md",
  "line": 27,
  "section": "共通フラグ（list）",
  "content": "... Ransack 風フィルタの共通フラグ ..."
}
```

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
