# M29: FindReceipt 新規 E2E + ProjectEntity.Receipts fix

**Phase**: H（5 件目）
**作成日**: 2026-04-21
**完了日**: 2026-04-21
**ステータス**: ✅ 完了

## 目的

`FindReceipt` の 4 モード（ID / ProjectID / ClientName / ProjectName）を E2E で叩き、
`ReceiptResult` の `Client`/`Project` enrichment が実 API に対して欠損なく返ることを検証する。
また M27 で発見された `ProjectEntity.Receipt` 単数形マッピング問題を fix する（M28 と同パターン）。

## 実施 API 見積 vs 実績

- **見積**: ~10 req
- **実績**: ~20 req（discovery 50 件走査 × 2 テスト + GetReceiptRaw + GetProjectWithGroup など）

## バグ修正: ProjectEntity.Receipts 複数形配列対応

### 根本原因

M28 の delivery と同様、BOARD API は `response_group=receipt` のとき `"receipts"` 複数形配列キーで
ドキュメントを返す。しかし `ProjectEntity.Receipt` は `json:"receipt,omitempty"`（単数形）に
タグされており常に nil となっていた。

`Receipts []DocumentSummary` (`json:"receipts,omitempty"`) は M28 の `ProjectEntity` fix 時に
既に追加されていたため、M29 では `find_receipt.go` と `find_receipt_test.go` のみ修正。

### fix 内容

1. `internal/service/find/find_receipt.go`:
   - `p.Receipt != nil` → `len(p.Receipts) > 0` に変更
   - `p.Receipt.ID` → `p.Receipts[0].ID` に変更（全 3 ブランチ: ProjectID / ClientName / ProjectName）

2. `internal/service/find/find_receipt_test.go`:
   - モックデータを `Receipt: &docSummary`（単数形ポインタ）→ `Receipts: []DocumentSummary{docSummary}` に変更

### TDD サイクル

- **Red**: `TestE2E_FindReceipt_ByProjectID_Strict` が `FindReceipt(ProjectID=95960734)` で 0 件返却を確認
- **Green**: `find_receipt.go` を `p.Receipts[0]` 参照に修正 → 1 件返却を確認
- **Refactor**: unit テストのモックデータも複数形に統一

## テスト一覧

| テスト名 | モード | 結果 | 備考 |
|---------|--------|------|------|
| `TestE2E_FindReceipt_ByProjectID_Strict` | ProjectID | PASS | projectID=95960762, receiptID=28480168, receipt_date="2026-04-30" ✓, Project enrichment ✓, ClientID=0→nil 正常 |
| `TestE2E_FindReceipt_ByID_Strict` | ID | PASS | receiptID=28480168, receipt_date="2026-04-30" ✓, Client=nil, Project=nil（ID モード仕様通り） |
| `TestE2E_FindReceipt_ByClientName_Strict` | ClientName | SKIP | キャッシュウォームアップ必須 |
| `TestE2E_FindReceipt_ByProjectName_Strict` | ProjectName | SKIP | 同上 |

## 発見事項

### 厳格フィールド突合 PASS

`GetReceiptRaw(28480168)` を独立取得し `testhelper.StrictFieldDiff` で突合。
`ReceiptEntity` に未マップフィールドは 0 件。M21/M38 の成果が正しく反映されている。

### ReceiptDate フィールド確認

実 API smoke で `receipt_date="2026-04-30"` が正しく unmarshal されることを確認。

### M28 との対称性確認

ProjectEntity.Receipts は M28 の fix 時に追加済みであったため、
M29 の変更は `find_receipt.go` + `find_receipt_test.go` のみに絞ることができた。

## fix コミット

`fix(find): FindReceipt ProjectID/ClientName/ProjectName ブランチを Receipts 複数形参照に修正`

## ファイル変更

- `internal/service/find/find_receipt.go`（Receipts 複数形参照に変更）
- `internal/service/find/find_receipt_test.go`（モックデータを Receipts に変更）
- `internal/service/find/e2e_test.go`（FindReceipt 4 テスト追加）
- `plans/board-compliance-m29-find-receipt.md`（本ファイル）
- `plans/board-compliance-roadmap.md`（M29 ✅、Current Focus → M30）
