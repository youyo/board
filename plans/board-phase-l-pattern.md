# Phase L リソース移行パターン手順書（clients パイロット由来）

本ドキュメントは M50（clients パイロット）で確立した「1 リソースを BOARD API 完全準拠に刷新する」手順書。M51（projects）以降は本手順書を参照して差分実装する。

## 適用対象

Phase L M51-M56 で刷新する 21 リソース:

- **M51**: projects（最大規模、15+ フィルタ）
- **M52**: client_branches, contacts, project_costs
- **M53**: estimates, orders, deliveries, receipts（List/Search 存在可否調査付き）
- **M54**: invoices, purchase_orders, payments
- **M55**: vendors, vendor_branches, vendor_contacts
- **M56**: users, groups, payment_terms, project_types, purchase_types, accounting_types, document_send_channels

## 1. SearchParams → ListOptions 置換

### Before
```go
type ClientSearchParams struct {
    Name          string
    UpdatedAtFrom string
}
```

### After
```go
type ClientListOptions struct {
    Page    int
    PerPage int

    // 全 List 共通
    UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
    UpdatedAtLteq     string
    IncludeArchiveFlg *bool  // nil=送らない, true=1, false=0

    // リソース専用（Ransack 準拠）
    NameCont              string   // <field>_cont = 部分一致
    NameDispCont          string
    InvoiceSystemNumberEq string   // <field>_eq = 完全一致
    CustomNoEq            string
    Tags                  []string // tags[]=A&tags[]=B
    ResponseGroup         string   // "small" / "large"
}
```

**命名ルール**:
- `_cont` → Ransack 部分一致。 `NameCont`, `MemoCont` 等。
- `_eq` → 完全一致。 `ClientIDEq`, `CustomNoEq` 等。
- `_gteq` / `_lteq` → 日時範囲。
- `_in[]` → 複数値。 `OrderStatusIn []int`, `StatusIn []string` 等。
- `*bool` → BOARD API の `flg` 系は 3 値（nil / 0 / 1）。
- `Tags []string` → `tags[]=...` で送る。 `QueryBuilder.Tags` ヘルパ使用。

## 2. ListResult[T] への戻り値刷新

### Before
```go
func (c *Client) ListClients(ctx context.Context) ([]ClientEntity, error)
func (c *Client) GetClient(ctx context.Context, id int) (*ClientEntity, error)
func (c *Client) ListClientsPage(ctx context.Context, page, perPage int) (*PageResult[ClientEntity], error)
func (c *Client) SearchClients(ctx context.Context, params ClientSearchParams) ([]ClientEntity, error)
```

### After
```go
func (c *Client) ListClients(ctx context.Context, opts ClientListOptions) (*ListResult[ClientEntity], error)
func (c *Client) GetClient(ctx context.Context, id int) (*ItemResult[ClientEntity], error)
func (c *Client) ListClientsRaw(ctx context.Context, opts ClientListOptions) ([]byte, http.Header, error)
func (c *Client) GetClientRaw(ctx context.Context, id int) ([]byte, http.Header, error)
```

**ポイント**:
- `Search*` / `*Page` は削除（破壊的変更）。フィルタ指定は `ListOptions` に統合。
- `*Raw` は E2E strict field diff 専用。常にヘッダーも返す（ListMeta の実測用）。
- `*ListResult` / `*ItemResult` の `Meta` から最終ページの response header 情報を JSON に `_meta` として露出可能。

## 3. QueryBuilder でのクエリ組み立て

`internal/boardapi/query.go` の `QueryBuilder` にメソッドチェーンで列挙。

```go
func buildClientsQuery(opts ClientListOptions, page, perPage int) string {
    return NewQueryBuilder().
        Page(page, perPage).
        StrCont("name", opts.NameCont).
        StrCont("name_disp", opts.NameDispCont).
        StrEq("invoice_system_number", opts.InvoiceSystemNumberEq).
        StrEq("custom_no", opts.CustomNoEq).
        DateGteq("updated_at", opts.UpdatedAtGteq).
        DateLteq("updated_at", opts.UpdatedAtLteq).
        Flg01("include_archive_flg", opts.IncludeArchiveFlg).
        Tags(opts.Tags).
        ResponseGroup(opts.ResponseGroup).
        Encode()
}
```

QueryBuilder が「ゼロ値 skip」を保証するので、呼び出し側で `if opts.X != ""` の重複ガードは書かない。

## 4. repository / service / cli 追従チェックリスト

### repository (例: `internal/repository/clients.go`)

- [ ] `List(ctx, readOpts, filter) (*ListResult[T], error)` に刷新（二引数化）
  - **ReadOptions**（cache 制御）と **ListOptions**（API filter）を直交させる
- [ ] ゼロ filter → cache-backed path（従来通り refresh → cache 読み）
- [ ] 非ゼロ filter → **cache bypass**、`api.List*(ctx, filter)` 直呼び
  - フィルタ結果で full-entity cache を汚染しない
- [ ] `GetByID` は従来の cache-miss fallback を維持（シグネチャ変更なし、ただし API 側が `*ItemResult` を返すので `.Item` 展開）
- [ ] `Search(ctx, filter, opts) ([]Entity, error)` は find 層向けの薄いラッパとして残置（`ListEntities` にリネームしても可）

### service/api (例: `internal/service/api/clients.go`)

- [ ] `ListX` を `(ctx, readOpts, filter) (*ListResult[Entity], error)` に変更
- [ ] `SearchX` / `ListXPage` メソッド削除
- [ ] `GetX` は `*Entity` 返却を維持（内部で `*ItemResult.Item` を展開）

### cli (例: `internal/cli/api_clients.go`)

- [ ] `search` サブコマンド削除、`list` に統合
- [ ] 新 flag:
  - `--name-cont`, `--name-disp-cont`（リソース固有の Ransack _cont）
  - `--<field>-eq`（完全一致フィルタ）
  - `--tags` (`StringSlice`, `tags[]=A&tags[]=B`)
  - `--response-group` (small/large)
  - `--include-archive-flg` (`Bool`, nil=未指定は `Changed()` 判定)
  - `--updated-at-gteq`, `--updated-at-lteq`
  - `--show-meta` (default true)
- [ ] `--page` / `--per-page` 削除（ListResult が自動全ページ取得）
- [ ] `--name` / `--updated-at-from` 削除（legacy; Ransack 命名に改名）

### find 層 interface (例: `internal/service/find/service.go`)

- [ ] `ClientRepo` interface の `Search` シグネチャを `(ctx, filter boardapi.XListOptions, opts ReadOptions) ([]Entity, error)` に変更
- [ ] 全件取得は `Search(ctx, XListOptions{}, opts)` で代替（`List` メソッドは削除可）
- [ ] find/find_*.go の `XSearchParams{Name: ...}` → `XListOptions{NameCont: ...}` に全箇所置換
- [ ] find/helpers_test.go の stub は filter ゼロ→`listResult`、非ゼロ→`searchResult` に分岐させる

### fetcher (例: `internal/repository/fetcher.go`)

- [ ] `XFetcher.ListAll` → `api.ListX(ctx, XListOptions{})`
- [ ] `XFetcher.ListUpdatedSince(ctx, since)` → `api.ListX(ctx, XListOptions{UpdatedAtGteq: isoToBoardDateTime(since)})`
- [ ] `isoToBoardDateTime` ヘルパは `SyncState.CursorUpdatedAt` の ISO 8601 を `YYYY-MM-DD HH:MM:SS` に変換（BOARD Ransack `_gteq` 形式の要求）

## 5. E2E 実 API 検証項目（各リソース E1-E10 相当）

各リソース最低でも以下 6-10 項目を run する。予算は 1 run で 10 req（`time.Sleep(400ms)` で 3 req/sec 制約を遵守）。

| # | 検証項目 | 合格判定 |
|---|----------|---------|
| E1 | `name_cont=<実キーワード>` で件数減 | `len < baseline` |
| E2 | ゼロフィルタ再実行で baseline と一致 | `len == baseline` |
| E3 | `response_group=large` で Get 限定フィールドあり | 観測ログ |
| E4 | `response_group=small` で shape 確認 | 観測ログ |
| E5 | `updated_at_gteq=<1年後>` で 0 件 | `len == 0` |
| E6 | `include_archive_flg=1` で件数 ≥ baseline | `len >= baseline` |
| E7 | `tags[]` round-trip エラーなし | `err == nil` |
| E8 | `<field>_eq=<実値>` round-trip エラーなし | `err == nil` |
| E9 | `invoice_system_number_eq` 相当（ある場合）round-trip | `err == nil` |
| E10 | response header 名の dump | `t.Logf("header X = %s")` |

E1-E10 で判明した **正式ヘッダー名** を `plans/board-phase-l-pattern.md` §6 に追記。

## 6. 確定した BOARD API ヘッダー名（M50 実測分）

**実行環境が整い次第、M50 の `TestE2E_Clients_Pilot_M50/E10_HeaderNames` を走らせて実測値を埋める。**

| 名前 | 想定 → 実測 | 備考 |
|------|------------|------|
| X-Total-Count | TBD | 全件数 |
| X-Page | TBD | 現在ページ |
| X-Per-Page | TBD | 1 ページ件数 |
| X-Ratelimit-Remaining / X-RateLimit-Remaining | TBD | どちらの綴りか |
| X-Ratelimit-Limit / X-RateLimit-Limit | TBD | 同上 |
| X-Ratelimit-Reset / X-RateLimit-Reset | TBD | unix timestamp or RFC |
| ETag | TBD | キャッシュバリデーション |
| Last-Modified | TBD | キャッシュバリデーション |
| Retry-After | TBD | 429 / 503 時のみ |

M49 の `parseListMeta` / `parseItemMeta` は両綴りを試す実装なので、どちらでも値は取れる。M51 以降のリソースでも同値が返ると想定（要確認）。

## 7. よくある pitfall

### 7.1 `Meta` の JSON tag
`result.go` の `ListResult.Meta` は **`json:"_meta"`** （先頭アンダースコア）。 `jq '._meta'` 慣習に合わせる。

### 7.2 `time.Time` ゼロ値の omitempty
Go の `encoding/json` は `time.Time{}` を `omitempty` 対象にしない。`ListMeta.MarshalJSON` / `ItemMeta.MarshalJSON` のカスタム実装でゼロ時刻を除外（M49 確立パターン）。

### 7.3 `*bool` フラグ
BOARD API の `include_archive_flg` 等は「未送信 / 0 / 1」の 3 値。Go では `*bool`（nil / &false / &true）で表現し、 `QueryBuilder.Flg01` が nil を skip する。CLI では `Changed()` 判定で `*bool` を作る:

```go
if cmd.Flags().Changed("include-archive-flg") {
    v, _ := cmd.Flags().GetBool("include-archive-flg")
    filter.IncludeArchiveFlg = &v
}
```

### 7.4 `tags[]` のエンコード
Go の `url.Values.Add("tags[]", "A")` で `tags%5B%5D=A` に URL エンコードされる。BOARD 側は `tags[]=A` 形式を要求する可能性があるので、もしサーバが受け付けないなら `QueryBuilder` 側で raw 文字列連結に切り替える（M50 時点では URL エンコード形式で送信、E7 で実動作確認）。

### 7.5 updated_at フォーマット変換
- `cache.SyncState.CursorUpdatedAt` は ISO 8601（`2024-12-26T10:14:11.000+09:00`）。
- BOARD API `_gteq` は `YYYY-MM-DD HH:MM:SS`。
- fetcher で `isoToBoardDateTime` を噛ませて変換（clients パイロット実装参照）。

### 7.6 find 層 interface の最小追従
Phase L では find 層に `*ListResult` を露出させない。 `Search(ctx, filter, opts) ([]Entity, error)` のまま。Phase M で MCP / find 全面刷新。

## 8. 移行時のテスト Green 要件

M51-M56 の各マイルストーンで以下を Green に保つ:

- `go build ./...`
- `go test -count=1 ./...`
- `go vet ./...`
- `go build -tags e2e ./...`（通常 test では実行されない E2E テストのビルド確認）
- `go test -tags e2e -v -count=1 -run TestE2E_<Resource>_Pilot ./internal/boardapi/`（環境が整っている場合のみ）
- `golangci-lint run` は `SA1019 deprecated` 警告のみ許容（旧 PageResult / ListAll の deprecation warning は M57 で一括削除時に解消）

## 9. 参考実装

- clients パイロット: `internal/boardapi/clients.go`, `internal/repository/clients.go`, `internal/service/api/clients.go`, `internal/cli/api_clients.go`
- E2E: `internal/boardapi/e2e_clients_m50_test.go`（E1-E10）, `internal/boardapi/e2e_clients_test.go`（既存テスト刷新版）
- 基盤: `internal/boardapi/result.go`, `internal/boardapi/query.go`, `internal/boardapi/pagination.go#ListAllWithResult`
