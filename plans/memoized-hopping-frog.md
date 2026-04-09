# Plan: BOARD API エンドポイント全面修正

## Context

OpenAPI spec + 実 API 検証により、boardapi クライアントのエンドポイントパスに広範な誤りが判明。
書類系リソースのパス構造、ベンダー系の命名を修正し、ページネーション対応を追加する。

## 正しいパス対応表 (OpenAPI + curl 検証済み)

### パスが間違っているリソース

| 現在のコード | 正しいパス | 備考 |
|---|---|---|
| `GET /estimates` (List) | **存在しない** | 案件API `response_group=estimate` で取得 |
| `GET /estimates/{id}` | `GET /documents/estimates/{id}` | {id}は見積書ID (案件IDではない) |
| `GET /orders` (List) | **存在しない** | `response_group=order` |
| `GET /orders/{id}` | `GET /documents/orders/{id}` | |
| `GET /deliveries` (List) | **存在しない** | `response_group=delivery` |
| `GET /deliveries/{id}` | `GET /documents/deliveries/{id}` | |
| `GET /receipts` (List) | **存在しない** | `response_group=receipt` |
| `GET /receipts/{id}` | `GET /documents/receipts/{id}` | |
| `GET /vendors` | `GET /payees` | |
| `GET /vendors/{id}` | `GET /payees/{id}` | |
| `GET /vendor_branches` | `GET /payee_branches` | |
| `GET /vendor_branches/{id}` | `GET /payee_branches/{id}` | |
| `GET /vendor_contacts` | `GET /payee_contacts` | |
| `GET /vendor_contacts/{id}` | `GET /payee_contacts/{id}` | |
| `GET /purchase_orders` | `GET /expenditures` | |
| `GET /purchase_orders/{id}` | `GET /expenditures/{id}` | |
| `GET /payments` | `GET /expenditure_payments` | |
| `GET /purchase_types` | `GET /expenditure_types` | |

### 書類取得の正しいフロー

```
1. GET /projects?response_group=estimate  →  各 project.estimate.id を取得 (bulk)
2. GET /documents/estimates/{見積書ID}     →  見積書の全フィールドを取得 (hydrate)
```

project.id=95892483 → estimate.id=105235249 のように、書類IDは案件IDとは別。

### API ページネーション仕様
- リクエスト: `page` (1始まり), `per_page` (最大100, デフォルト10)
- レスポンスヘッダー: `X-Total-Count`, `X-Page`, `X-Per-Page`

---

## 設計方針

### 方針1: response_group は repository/search 層を通す (レイヤ維持)

find から boardapi.Client を直接叩かない。
project の Search/List に response_group を渡し、**bulk で** summary を取得する。

```
find → projects.Search(params with ResponseGroup) → repo → boardapi → response_group 付きレスポンス
                                                                         ↓
                                                              project.Estimate.ID を取得 (bulk)
                                                                         ↓
find → estimates.GetByDocumentID(documentID) → repo (on-demand cache) → boardapi → /documents/estimates/{id}
```

`GetProjectWithGroup` は `--project-id` 直指定時の補助に限定。
ClientName/ProjectName 検索では repo.Search に response_group を渡して bulk 取得。

### 方針2: 書類の on-demand cache は残す (List fetcher のみ撤去)

現在の `GetByID` はキャッシュミス時に API から単体取得して upsert する設計。
この on-demand cache は rate limit 対策として有効なので維持する。

変更点:
- List/Search 用の fetcher (ListAll/ListUpdatedSince) → **削除** (一覧エンドポイントが存在しないため)
- List()/Search() メソッド → **削除**
- GetByID → GetByDocumentID に改名 + パス修正 (on-demand cache 維持)
- refresh/sync の依存は残すが、maybeRefresh 呼び出しの fetcher 部分は nil/skip にする

### 方針3: ページネーション — retry 維持 + CLI 契約明確化

**retry**: `ListPage` は既存の `DoWithRetry` 経由。新設の `DoWithRetryFull` で body + headers を返す。

**CLI フラグスコープ**: `--page`, `--per-page` は `board api xxx list` コマンドのローカルフラグ。
root persistent には追加しない (find/cache/mcp に漏れるのを防止)。

**デフォルト動作は変更しない**: 既存の cache-backed `List()` + `--limit` がデフォルト。
`--page` を明示指定した時のみ ListPage (API 直接、キャッシュバイパス)。

**フラグ優先順位**:
```
--page N [--per-page N]  → API 直接 ListPage (キャッシュバイパス)
(未指定)                 → 既存 cache-backed List() + --limit で件数制御
```

`--page` 指定時のみ出力に pagination メタ情報を stderr 表示:
```
# stderr: Total: 2391, Page: 1/48, PerPage: 50
[{...}, {...}, ...]
```

`--all` フラグは廃止。現在のデフォルト動作がそのまま全件 (+ limit) なので不要。

### 方針4: find は --limit のみ (--page なし)

multi-hop aggregation では API ページネーションを透過できない。
find は LLM 向けコマンドとして少数の最適結果を返す設計。
`--limit` で結果数制御 (デフォルト 50)。

---

## 実装ステップ

### Step 0: IsNotFound ヘルパー

`internal/boardapi/error.go`:
```go
func IsNotFound(err error) bool {
    var apiErr *APIError
    if errors.As(err, &apiErr) {
        return apiErr.Code == APIErrorNotFound
    }
    return false
}
```

### Step 1: ページネーション基盤 (boardapi 層)

**`internal/boardapi/client.go`**:
- `DoWithRetryFull(req) ([]byte, http.Header, error)` 追加:
  - 既存 `DoWithRetry` と同じ retry ロジック
  - 成功時に body + response headers を返す

**`internal/boardapi/pagination.go`** (既存ファイル拡張):
- `PageResult[T]` 型追加:
  ```go
  type PageResult[T any] struct {
      Items      []T
      TotalCount int
      Page       int
      PerPage    int
  }
  ```
- `ListPage[T](ctx, makeReq PagedRequest, page, perPage int, unmarshal func) (*PageResult[T], error)` 追加:
  - `DoWithRetryFull` で 1ページ取得
  - `X-Total-Count`, `X-Page`, `X-Per-Page` ヘッダーを解析
  - 既存 `ListAll` は内部的に `DoWithRetry` を使い続ける (変更なし)

**各リソースファイル** (clients.go, projects.go 等):
- `ListXxxPage(ctx, page, perPage int) (*PageResult[XxxEntity], error)` メソッド追加
- 既存 `ListXxx(ctx)` は維持 (キャッシュ refresh 用)

### Step 2: Projects API response_group 対応

**`internal/boardapi/projects.go`**:

`ProjectSearchParams` に追加:
```go
ResponseGroup string // "small", "estimate", "order", "all" 等
```

`ListProjects`, `SearchProjects` のリクエスト組み立て時に `response_group` クエリパラメータを付加。

`GetProject` の拡張: `GetProjectWithGroup(ctx, id int, responseGroup string)` 追加。
→ `--project-id` 直指定時の単体取得用。

`ProjectEntity` にドキュメントサマリフィールド追加:
```go
type ProjectEntity struct {
    // ... existing fields ...
    Estimate *DocumentSummary `json:"estimate,omitempty"`
    Order    *DocumentSummary `json:"order,omitempty"`
    Delivery *DocumentSummary `json:"delivery,omitempty"`
    Invoice  *DocumentSummary `json:"invoice,omitempty"`
    Receipt  *DocumentSummary `json:"receipt,omitempty"`
}
```

**`internal/boardapi/document_summary.go`** (新規):
```go
type DocumentSummary struct {
    ID             int    `json:"id"`
    Total          string `json:"total"`
    Tax            string `json:"tax"`
    TaxWithholding string `json:"tax_withholding"`
    LockFlg        int    `json:"lock_flg"`
    // response_group で返る基本フィールド
}
```

### Step 3: boardapi 書類4リソース修正

各ファイル (`internal/boardapi/{estimates,orders,deliveries,receipts}.go`):
- **削除**: `ListXxx()`, `SearchXxx()`, `XxxSearchParams`
- **修正**: `GetXxx(ctx, documentID int)` のパスを `/v1/documents/{type}/{documentID}` に変更
- 引数名を `id` → `documentID` に (案件ID でないことを明示)

テスト (`internal/boardapi/client_test.go`):
- List/Search テスト削除
- Get テストのパスを `/v1/documents/...` に更新

### Step 4: boardapi ベンダー系命名修正

エンドポイントパスのみ修正 (関数名・型名は維持):
- `vendors.go`: `/v1/vendors` → `/v1/payees`
- `vendor_branches.go`: `/v1/vendor_branches` → `/v1/payee_branches`
- `vendor_contacts.go`: `/v1/vendor_contacts` → `/v1/payee_contacts`
- `purchase_orders.go`: `/v1/purchase_orders` → `/v1/expenditures`
- `payments.go`: `/v1/payments` → `/v1/expenditure_payments`
- `purchase_types.go`: → `/v1/expenditure_types`

テスト: パス文字列の更新

### Step 5: repository 層 — 書類4リソースの fetcher 撤去 + on-demand cache 維持

`internal/repository/{estimates,orders,deliveries,receipts}.go`:
- **削除**: `List()`, `Search()`, `filterXxx()`
- **削除**: `maybeRefresh` 呼び出し (fetcher 経由の bulk refresh は不可)
- **維持**: `GetByDocumentID(ctx, documentID int, opts ReadOptions)`:
  - キャッシュ参照 → ミス時に `api.GetXxx(ctx, documentID)` → upsert → 返却
  - 既存の `GetByID` と同じ on-demand cache パターン
  - opts.ForceRefresh 時はキャッシュスキップ
- **constructor**: refresh/sync 関連の引数は削減 (List refresh 不要のため)
  - ただし cache store と profile は維持 (on-demand cache 用)

`internal/repository/fetcher.go`:
- `estimatesFetcher`, `ordersFetcher`, `deliveriesFetcher`, `receiptsFetcher` 削除

テスト: List/Search テスト削除、GetByDocumentID のキャッシュヒット/ミステストは維持・更新

### Step 6: repository 層 — projects に response_group + ページネーション追加

**response_group**:
`internal/repository/projects.go`:
- `Search(ctx, params, opts)` に `ResponseGroup` を伝播:
  - `ProjectSearchParams.ResponseGroup` が設定されていれば、API 呼び出し時に response_group を付加
  - キャッシュは response_group なし (small) のデータのみ格納
  - response_group 付きリクエストはキャッシュバイパスで API 直接
- `GetByIDWithGroup(ctx, id int, responseGroup string) (*boardapi.ProjectEntity, error)` 追加:
  - キャッシュバイパス。`boardapi.GetProjectWithGroup(ctx, id, responseGroup)` を直接呼ぶ
  - ProjectID 起点の find フローで使用

**ページネーション** (全 List 対応リソース):
`internal/repository/{clients,projects,vendors,...}.go`:
- `ListPage(ctx, page, perPage int) (*boardapi.PageResult[XxxEntity], error)` 追加
  - キャッシュバイパス、API 直接呼び出し
- 既存 `List(ctx, opts)` はキャッシュ経由のまま維持

### Step 7: service/api 層

**書類4リソース**:
- `List`, `Search` 削除
- `GetXxx(ctx, id, opts)` → `GetByDocumentID(ctx, documentID int, opts)` に更新

**リスト系リソース**:
- `ListPage(ctx, page, perPage int)` 追加
- 既存 `List(ctx, opts)` 維持

インターフェース更新 (`internal/service/api/service.go`):
```go
type EstimateRepo interface {
    GetByDocumentID(ctx context.Context, documentID int, opts ReadOptions) (*boardapi.EstimateEntity, error)
}

type ProjectRepo interface {
    List(ctx context.Context, opts ReadOptions) ([]boardapi.ProjectEntity, error)
    ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ProjectEntity], error)
    GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ProjectEntity, error)
    Search(ctx context.Context, params boardapi.ProjectSearchParams, opts ReadOptions) ([]boardapi.ProjectEntity, error)
}
```

### Step 8: service/find 層

#### 8a: インターフェース (find 用)

`internal/service/find/service.go`:
```go
type EstimateRepo interface {
    GetByDocumentID(ctx context.Context, documentID int, opts ReadOptions) (*boardapi.EstimateEntity, error)
}

type ProjectRepo interface {
    // ... 既存メソッド ...
    Search(ctx context.Context, params boardapi.ProjectSearchParams, opts ReadOptions) ([]boardapi.ProjectEntity, error)
    GetByID(ctx context.Context, id int, opts ReadOptions) (*boardapi.ProjectEntity, error)
    GetByIDWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error) // NEW
}
```

`GetByIDWithGroup`: キャッシュバイパスで API 直接。response_group 付きレスポンスを返す。
ProjectRepo.Search に response_group を渡すために、`ProjectSearchParams.ResponseGroup` を使う。
find service は boardapi.Client を直接持たない (レイヤ維持)。

#### 8b: FindProject に estimate 付加

`internal/service/find/types.go`:
```go
type ProjectResult struct {
    Project  boardapi.ProjectEntity    `json:"project"`
    Client   *boardapi.ClientEntity    `json:"client,omitempty"`
    Estimate *boardapi.EstimateEntity  `json:"estimate,omitempty"` // NEW
}
```

`internal/service/find/find_project.go`:
- projects 検索時に `params.ResponseGroup = "estimate"` を設定
- 各 project の `project.Estimate` が非 nil なら、`s.estimates.GetByDocumentID(ctx, project.Estimate.ID, opts)` で hydrate
- hydrate 失敗 (IsNotFound) はスキップ (Estimate=nil)
- project.Estimate が nil のプロジェクトは Estimate=nil のまま返す

#### 8c: FindEstimate 書き換え

`internal/service/find/types.go`:
```go
type FindEstimateQuery struct {
    ID          int    // 書類ID直指定 → estimates.GetByDocumentID で取得 (維持)
    ProjectID   int    // 案件ID → GetByIDWithGroup → hydrate
    ClientName  string // クライアント名 → 案件検索(response_group=estimate) → hydrate
    ProjectName string // 案件名 → 案件検索(response_group=estimate) → hydrate
    Status      string // ポストフィルタ (単独では使用不可、他の検索条件と併用必須)
    Limit       int
    Opts        repository.ReadOptions
}
```

**`--id` は維持** (書類ID直指定)。`--project-id` を追加。互換性を保つ。
**`--status` 単独は禁止**: 一覧エンドポイントがないため、Status 単独検索は不可能。
バリデーション: `ID`, `ProjectID`, `ClientName`, `ProjectName` のいずれか1つ以上必須。

`internal/service/find/find_estimate.go` — 新フロー:
```
1. ID 指定 (書類ID):
   → estimates.GetByDocumentID(documentID, opts) で直接取得
   → project/client enrich (entity の ProjectID/ClientID から)

2. ProjectID 指定:
   → projects.GetByIDWithGroup(projectID, "estimate") で summary 取得
   → project.Estimate が非nil なら estimates.GetByDocumentID(estimate.ID) で hydrate

3. ClientName 指定:
   → clients.Search(name) → client IDs
   → projects.Search(ClientID=xxx, ResponseGroup:"estimate") → bulk で summary 取得
   → 各 project.Estimate が非nil のものを estimates.GetByDocumentID() で hydrate

4. ProjectName 指定:
   → projects.Search(Name=xxx, ResponseGroup:"estimate") → bulk で summary 取得
   → hydrate

5. 共通後処理:
   → IsNotFound はスキップ
   → Status ポストフィルタ (EstimateEntity.Status でフィルタ)
   → Limit で件数制限
   → client/project で enrich → EstimateResult 構築
```

#### 8d: FindOrder/FindDelivery/FindReceipt も同パターン

#### 8e: find のページネーション
find は `--limit` のみ。`--page` は不要。

### Step 9: CLI 層

#### 9a: 書類コマンド
- `board api estimates list` / `search` → 削除
- `board api estimates get`: `--id` 維持 (書類ID)。パス修正のみ
- (orders, deliveries, receipts も同様)

#### 9b: find コマンド
- `board find estimate`:
  - `--id` 維持 (書類ID直指定)
  - `--project-id` 追加
  - `--text` 削除
  - `--status` は他の検索条件と併用必須 (単独使用時はエラー)
  - バリデーション: `--id`, `--project-id`, `--client-name`, `--project-name` のいずれか1つ以上必須
- (orders, deliveries, receipts も同様)

#### 9c: list コマンドにページネーション

`--page`, `--per-page` は `board api xxx list` コマンドの**ローカルフラグ** (root に置かない)。

**デフォルト動作は変更なし**: 既存の cache-backed List() + `--limit`。

**フラグ優先順位**:
```
--page N [--per-page N]  → API 直接 ListPage (キャッシュバイパス)
                           --limit は無視 (per-page が件数制御)
(未指定)                 → 既存 cache-backed List() + --limit (現在と同じ)
```

**出力**:
- `--page` 指定時: JSON 配列 + stderr にページ情報
  ```
  # stderr: Total: 2391, Page: 1/48, PerPage: 50
  [{...}, {...}, ...]
  ```
- デフォルト: JSON 配列のみ (現在と同じ)

**実装**: 各 `list` コマンドにローカルフラグ追加:
```go
cmd.Flags().Int("page", 0, "Page number (1-based, bypasses cache)")
cmd.Flags().Int("per-page", 50, "Items per page (max 100, used with --page)")
```

各 list コマンドの RunE:
- `page > 0` → service.ListPage(page, perPage) → pagination メタ情報を stderr
- `page == 0` → 既存の service.List(opts) (キャッシュ + limit)

### Step 10: MCP ツール

`internal/mcpserver/tools.go`:
- findEstimatesTool: `id` → `project_id`、`text` 削除
- (orders, deliveries, receipts も同パターン)

### Step 11: DI コンテナ

`internal/app/container.go`:
- 書類4リソースの Repository: List refresh 関連引数を削減 (cache/profile は維持)
- find Service への注入は変更なし (ProjectRepo + EstimateRepo で完結)

---

## 実装順序

```
Step 0   → IsNotFound ヘルパー
Step 1   → ページネーション基盤 (DoWithRetryFull, PageResult, ListPage)
Step 2   → Projects response_group + DocumentSummary
Step 3   → boardapi 書類パス修正
Step 4   → boardapi ベンダー命名修正
Step 5   → repository 書類 (fetcher 撤去 + on-demand cache 維持)
Step 6   → repository response_group + ページネーション
Step 7   → service/api 更新
Step 8   → service/find 書き換え
Step 9   → CLI 更新 (ページネーション含む)
Step 10  → MCP 更新
Step 11  → DI コンテナ
```

各ステップ後に `go build ./...` で確認。

## 検証

1. `go build ./...` + `go vet ./...`
2. `mise run test` — ユニットテスト
3. `board api estimates get --document-id 105235249` — 見積書取得
4. `board api clients list --page 1 --per-page 5` — ページネーション
5. `board api clients list --all --limit 10` — 全件 + limit
6. `board find project --name <name>` — estimate 付き結果
7. `board find estimate --project-name <name>` — 案件経由の見積書
8. `board api vendors list` — payees パスで成功
9. E2E テスト

## 主要ファイル

```
internal/boardapi/error.go              — IsNotFound
internal/boardapi/client.go             — DoWithRetryFull
internal/boardapi/pagination.go         — PageResult, ListPage
internal/boardapi/projects.go           — response_group, DocumentSummary フィールド
internal/boardapi/document_summary.go   — DocumentSummary 型 (新規)
internal/boardapi/estimates.go          — パス修正 (/documents/estimates), List/Search 削除
internal/boardapi/orders.go             — 同上
internal/boardapi/deliveries.go         — 同上
internal/boardapi/receipts.go           — 同上
internal/boardapi/vendors.go            — /payees
internal/boardapi/vendor_branches.go    — /payee_branches
internal/boardapi/vendor_contacts.go    — /payee_contacts
internal/boardapi/purchase_orders.go    — /expenditures
internal/boardapi/payments.go           — /expenditure_payments
internal/boardapi/purchase_types.go     — /expenditure_types (要確認)
internal/repository/estimates.go        — List/Search 削除, GetByDocumentID (on-demand cache 維持)
internal/repository/orders.go           — 同上
internal/repository/deliveries.go       — 同上
internal/repository/receipts.go         — 同上
internal/repository/fetcher.go          — 4 fetcher 削除
internal/repository/projects.go         — response_group + ListPage
internal/repository/clients.go          — ListPage
internal/service/api/service.go         — インターフェース更新
internal/service/api/estimates.go       — List/Search 削除
internal/service/find/service.go        — インターフェース更新
internal/service/find/find_estimate.go  — response_group 経由 + hydrate
internal/service/find/find_project.go   — estimate 付加
internal/service/find/types.go          — 型更新
internal/cli/api.go                     — ページネーション用ヘルパー
internal/cli/api_estimates.go           — list/search 削除
internal/cli/find_estimate.go           — フラグ変更
internal/mcpserver/tools.go             — パラメータ更新
internal/app/container.go               — DI 調整
```
