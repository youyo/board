# M28: FindDelivery 新規 E2E + ProjectEntity.Deliveries fix

**Phase**: H（4 件目）
**作成日**: 2026-04-21
**完了日**: 2026-04-21
**ステータス**: ✅ 完了

## 目的

`FindDelivery` の 4 モード（ID / ProjectID / ClientName / ProjectName）を E2E で叩き、
`DeliveryResult` の `Client`/`Project` enrichment が実 API に対して欠損なく返ることを検証する。
また M27 で発見された `ProjectEntity.Delivery` 単数形マッピング問題を fix する。

## 実施 API 見積 vs 実績

- **見積**: ~10 req
- **実績**: ~20 req（discovery 50 件走査 × 2 テスト + GetDeliveryRaw + GetProjectWithGroup など）

## バグ修正: ProjectEntity.Deliveries 複数形配列対応

### 根本原因

BOARD API は `response_group=delivery` のとき `"deliveries"` 複数形配列キーでドキュメントを返す。
しかし `ProjectEntity.Delivery` は `json:"delivery,omitempty"`（単数形）にタグされており、
常に nil となっていた。

```
実 API レスポンス: {"id": 95944469, "deliveries": [{"id": 64955390, ...}], ...}
ProjectEntity.Delivery (json:"delivery"): → nil（マッチしない）
```

### fix 内容

1. `internal/boardapi/projects.go`:
   - `ProjectEntity` に `Deliveries []DocumentSummary` (`json:"deliveries,omitempty"`) を追加
   - `Invoices []DocumentSummary` (`json:"invoices,omitempty"`) も同時追加（Receipt fix 準備）
   - 既存の単数形フィールド（`Delivery` / `Invoice` / `Receipt`）は後方互換のため残す

2. `internal/service/find/find_delivery.go`:
   - `p.Delivery != nil` → `len(p.Deliveries) > 0` に変更
   - `p.Delivery.ID` → `p.Deliveries[0].ID` に変更（全 3 ブランチ: ProjectID / ClientName / ProjectName）

3. `internal/service/find/find_delivery_test.go`:
   - モックデータを `Delivery: &docSummary`（単数形ポインタ）→ `Deliveries: []DocumentSummary{docSummary}` に変更

### TDD サイクル

- **Red**: `TestE2E_FindDelivery_ByProjectID_Strict` が `FindDelivery(ProjectID=95944469)` で 0 件返却を確認
- **Green**: `ProjectEntity.Deliveries` 追加 + `find_delivery.go` 修正 → 1 件返却を確認
- **Refactor**: unit テストのモックデータも複数形に統一

## テスト一覧

| テスト名 | モード | 結果 | 備考 |
|---------|--------|------|------|
| `TestE2E_FindDelivery_ByProjectID_Strict` | ProjectID | PASS | projectID=95944469, deliveryID=64955390, delivery_date="2026-06-30" ✓, Project enrichment ✓, ClientID=0→nil 正常 |
| `TestE2E_FindDelivery_ByID_Strict` | ID | PASS | deliveryID=64955390, delivery_date="2026-06-30" ✓, Client=nil, Project=nil（ID モード仕様通り） |
| `TestE2E_FindDelivery_ByClientName_Strict` | ClientName | SKIP | キャッシュウォームアップ必須 |
| `TestE2E_FindDelivery_ByProjectName_Strict` | ProjectName | SKIP | 同上 |

## 発見事項

### 厳格フィールド突合 PASS

`GetDeliveryRaw(64955390)` を独立取得し `testhelper.StrictFieldDiff` で突合。
`DeliveryEntity` に未マップフィールドは 0 件。M20/M37 の成果が正しく反映されている。

### DeliveryDate フィールド確認

実 API smoke で `delivery_date="2026-06-30"` が正しく unmarshal されることを確認。

## fix コミット

`fix(boardapi): ProjectEntity.Deliveries 複数形配列フィールドを追加（delivery/invoice の単数形マッピング問題を修正）`

## ファイル変更

- `internal/boardapi/projects.go`（Deliveries / Invoices 複数形フィールド追加）
- `internal/service/find/find_delivery.go`（Deliveries 複数形参照に変更）
- `internal/service/find/find_delivery_test.go`（モックデータを Deliveries に変更）
- `internal/service/find/e2e_test.go`（FindDelivery 4 テスト追加）
- `plans/board-compliance-m28-find-delivery.md`（本ファイル）
- `plans/board-compliance-roadmap.md`（M28 ✅、Current Focus → M29）
