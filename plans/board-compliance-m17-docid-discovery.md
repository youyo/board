# M17: documentID discovery helper 確立（Phase G 開始）

## Meta

| 項目 | 値 |
|------|---|
| マイルストーン | M17 / Phase G 1 件目 |
| ロードマップ | plans/board-compliance-roadmap.md |
| 対象 | `internal/boardapi/e2e_helpers_test.go`（追加のみ） |
| スコープ | `findAnyDocumentID(t, client, docType)` helper の追加。estimates / orders / deliveries / receipts / invoices の E2E Get テストで再利用する基盤。 |
| 見積 | ~5 req（List 1 + GetProjectWithGroupRaw 最大 3〜4 = 4〜5 req） |
| 前提 | M13 GetWithGroup 実装と artifacts（`projects_rg_all_*.json`）が基礎 |

## 背景

### M13 ハンドオフ

M13 で判明した重要な非対称性:

| docType | API JSON キー | 型 | ProjectEntity フィールド |
|---------|-------------|-----|----------------------|
| estimate | `estimate` | 単一オブジェクト | `Estimate *DocumentSummary` （json:"estimate"）|
| order | `order` | 単一オブジェクト | `Order *DocumentSummary` （json:"order"）|
| delivery | `deliveries` | **配列** | `Delivery *DocumentSummary` （json:"delivery"）**ミスマッチ** |
| invoice | `invoices` | **配列** | `Invoice *DocumentSummary` （json:"invoice"）**ミスマッチ** |
| receipt | `receipts` | **配列** | `Receipt *DocumentSummary` （json:"receipt"）**ミスマッチ** |

`delivery`/`invoice`/`receipt` は `ProjectEntity` の JSON タグが単数形（`delivery`/`invoice`/`receipt`）のため、実 API の複数形キー（`deliveries`/`invoices`/`receipts`）とミスマッチ。`ProjectEntity` を通じて ID を取得できない。

### 設計決定

**全 5 docType に対して `GetProjectWithGroupRaw` + probe struct を使う（raw JSON パス統一）**。

`ProjectEntity` の既知ミスマッチを回避し、将来の `ProjectEntity` 変更に依存しない単一コードパスを実現する。

Probe struct:
```go
var probe struct {
    Estimate   *struct{ ID int `json:"id"` } `json:"estimate"`
    Order      *struct{ ID int `json:"id"` } `json:"order"`
    Deliveries []struct{ ID int `json:"id"` } `json:"deliveries"`
    Invoices   []struct{ ID int `json:"id"` } `json:"invoices"`
    Receipts   []struct{ ID int `json:"id"` } `json:"receipts"`
}
```

## シグネチャ

```go
// findAnyDocumentID は projects を response_group={docType} で走査し、
// docType サブオブジェクトが存在する最初の 1 件の
// (projectID, documentID) を返す。
//
// docType: "estimate" / "order" / "delivery" / "invoice" / "receipt"
// データが見つからない場合は t.Skipf で pending 扱い。
// 不明な docType は t.Fatalf（プログラマエラー）。
//
// rate limit 配慮: 上位 maxProjects 件（デフォルト 3）のみ走査。
func findAnyDocumentID(t *testing.T, client *boardapi.Client, docType string) (projectID, documentID int)
```

## 実装詳細

1. `ListProjectsRaw` で project IDs を取得（1 req）
2. 上位 `maxProjects=3` 件に対して `GetProjectWithGroupRaw(id, docType)` を呼ぶ
3. probe struct で JSON を parse し、docType に応じてフィールドを確認:
   - `"estimate"` → `probe.Estimate != nil && probe.Estimate.ID > 0`
   - `"order"` → `probe.Order != nil && probe.Order.ID > 0`
   - `"delivery"` → `len(probe.Deliveries) > 0 && probe.Deliveries[0].ID > 0`
   - `"invoice"` → `len(probe.Invoices) > 0 && probe.Invoices[0].ID > 0`
   - `"receipt"` → `len(probe.Receipts) > 0 && probe.Receipts[0].ID > 0`
   - 不明な docType → `t.Fatalf`
4. 発見できれば `(projectID, documentID)` を return
5. 全件走査後も発見できなければ `t.Skipf("no %s discovered in top %d projects", docType, maxProjects)`

## Rate Limit

- 走査上限: `maxProjects = 3`
- 最悪ケース: List 1 + GetProjectWithGroupRaw × 3 = **4 req**
- 期待ケース: List 1 + GetProjectWithGroupRaw × 1 = **2 req**（最初の project が estimate を持つ場合）
- smoke test（estimate 1件のみ）: **~2 req**
- M17 合計: **~4 req**

## テスト方針

### Unit テスト（httptest ベース）

`e2e_helpers_test.go` に追加（`//go:build e2e` タグ下）:

1. `TestFindAnyDocumentID_Estimate_Found`: estimate を持つ project を返す httptest サーバー → `(projectID, documentID)` を検証
2. `TestFindAnyDocumentID_Delivery_Found`: deliveries 配列を持つ project を返す httptest サーバー → 配列先頭の ID を検証
3. `TestFindAnyDocumentID_Skip_WhenNotFound`: 全 3 件が対象 docType なし → `t.Skip` （`testing.TB` mock で確認）

### 実 API Smoke Test

`TestE2E_FindAnyDocumentID_Estimate` を 1 本:
- `findAnyDocumentID(t, client, "estimate")` を呼ぶ
- `(projectID > 0, documentID > 0)` を検証
- 失敗時は Skip（データなしは pending 扱い）

## リスク

| リスク | 対策 |
|--------|------|
| `delivery`/`invoice`/`receipt` の JSON キーが環境によって単数形になる可能性 | probe struct に両方のフィールドを定義（`json:"delivery"` と `json:"deliveries"` の両立は不可のため raw json で両方チェック）→ M13 artifacts では `deliveries` 複数形を確認済みのため複数形のみで対応 |
| projects 0 件の環境 | `t.Skipf` で graceful skip |
| rate limit（3/秒） | maxProjects=3 + DoWithRetry がすでに retry logic を持つ |

## 既知制約

- `ProjectEntity` の `Delivery`/`Invoice`/`Receipt` フィールドは JSON タグが単数形で API の複数形キーとミスマッチ → M17 では修正しない（別マイルストーン）
- `TestE2E_Estimates_GetByDocumentID`（e2e_test.go）は同等の discovery ロジックを持つが、M18 で helper 経由に切り替える予定 → M17 では残す

## Changelog

| 日付 | 変更 |
|------|------|
| 2026-04-21 | M17 着手・計画生成 |
| 2026-04-21 | `findAnyDocumentID` 実装完了 |
| 2026-04-21 | 実 API smoke test（estimate）PASS: projectID=95944469 documentID=105287235、実消費 **2 req**（List 1 + GetProjectWithGroupRaw 1） |
