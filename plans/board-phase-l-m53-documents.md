# M53: ドキュメント系 4 件（estimates/orders/deliveries/receipts）移行

## Meta

| 項目 | 値 |
|------|---|
| マイルストーン | M53 (L-05) |
| ステータス | 完了 ✅ |
| 完了日 | 2026-04-24 |
| 対象リソース | estimates, orders, deliveries, receipts |
| Pattern | B（List API 非存在、Get のみ） |

## 調査結果: List API 非存在（OpenAPI 実測）

BOARD API OpenAPI 仕様（`https://developers.the-board.jp/doc/board_openapi.json`）を実測確認した結果、
以下のエンドポイントのみ存在する:

| リソース | エンドポイント | メソッド |
|----------|--------------|---------|
| 見積書 | `/documents/estimates/{id}` | GET, PATCH |
| 発注書 | `/documents/orders/{id}` | GET, PATCH |
| 納品書 | `/documents/deliveries/{id}` | GET, PATCH |
| 領収書 | `/documents/receipts/{id}` | GET, PATCH |

**コレクション取得（List）エンドポイントは存在しない。**

BOARD API の正道は、projects エンドポイントの `response_group=estimate/order/delivery/receipt`
経由の埋め込み取得（`GET /v1/projects?response_group=estimate` 等）。
個別 ID 取得は `GET /v1/documents/{resource}/{id}` のみ。

## 実装方針: Pattern B（単一取得のみ）

List/Search は実装しない。Get のみ。

## 変更内容

### boardapi 層

- `GetEstimate(ctx, id) (*ItemResult[EstimateEntity], error)` に刷新
- `GetOrder(ctx, id) (*ItemResult[OrderEntity], error)` に刷新
- `GetDelivery(ctx, id) (*ItemResult[DeliveryEntity], error)` に刷新
- `GetReceipt(ctx, id) (*ItemResult[ReceiptEntity], error)` に刷新
- `GetXRaw` 系: `([]byte, error)` → `([]byte, http.Header, error)` に刷新（M53 E2E 用）
- 内部実装: `DoWithRetry` → `DoWithRetryFull` に変更し、headers を parseItemMeta でパース

### repository 層

- `EstimateRepository.GetByDocumentID`: `api.GetEstimate(...).Item` で `.Item` 展開
- `OrderRepository.GetByDocumentID`: `api.GetOrder(...).Item` で `.Item` 展開
- `DeliveryRepository.GetByDocumentID`: `api.GetDelivery(...).Item` で `.Item` 展開
- `ReceiptRepository.GetByDocumentID`: `api.GetReceipt(...).Item` で `.Item` 展開

### service/api 層

- `GetEstimate(ctx, id, opts) (*boardapi.ItemResult[EstimateEntity], error)` に刷新
- `GetOrder(ctx, id, opts) (*boardapi.ItemResult[OrderEntity], error)` に刷新
- `GetDelivery(ctx, id, opts) (*boardapi.ItemResult[DeliveryEntity], error)` に刷新
- `GetReceipt(ctx, id, opts) (*boardapi.ItemResult[ReceiptEntity], error)` に刷新
- 内部で repository.GetByDocumentID を呼び、`*ItemResult{Item: entity}` にラップ
- キャッシュヒット時は ItemMeta がゼロ値になる（許容）

### cli 層

- `board api {estimates,orders,deliveries,receipts} get` に `--show-meta` フラグ追加
- `--show-meta` true（デフォルト）で `ItemResult` ごと出力（`_meta` 付き）
- `--no-show-meta` で `.Item` のみ出力

### テスト修正

- `client_test.go` (T68/T74/T77/T80): `got.Field` → `got.Item.Field` に更新
- `e2e_{estimates,orders,deliveries,receipts}_test.go`: `GetXRaw` 呼び出しを 2 変数 → 3 変数に変更
- `service/find/e2e_test.go`: `GetOrderRaw`, `GetDeliveryRaw`, `GetReceiptRaw` の呼び出しを更新
- `estimates_m53_test.go` 新設（U1-U5: GetEstimate, GetEstimateRaw, GetOrder, GetDelivery, GetReceipt の新シグネチャ検証）

## 成功基準（達成確認）

- [x] `go build ./...` が通る
- [x] `go test -count=1 ./...` 全 Green（ユニットテスト 12 パッケージ）
- [x] `go vet ./...` クリーン
- [x] `go vet -tags e2e ./...` クリーン
- [x] `golangci-lint run` で M53 起因の新規警告なし（残存警告は M57 スコープの SA1019）

## 関連ファイル

- `internal/boardapi/estimates.go` — GetEstimate / GetEstimateRaw 刷新
- `internal/boardapi/orders.go` — GetOrder / GetOrderRaw 刷新
- `internal/boardapi/deliveries.go` — GetDelivery / GetDeliveryRaw 刷新
- `internal/boardapi/receipts.go` — GetReceipt / GetReceiptRaw 刷新
- `internal/boardapi/estimates_m53_test.go` — M53 ユニットテスト
- `internal/repository/{estimates,orders,deliveries,receipts}.go` — .Item 展開追従
- `internal/service/api/{estimates,orders,deliveries,receipts}.go` — ItemResult 返却に刷新
- `internal/cli/api_{estimates,orders,deliveries,receipts}.go` — show-meta フラグ追加
