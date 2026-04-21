# M32: FindInvoice 軽量 E2E

## 概要

Phase H 8 件目（Phase H 完走）。FindInvoice の ID モードのみ E2E 化する。
当該アカウントは 11,000+ invoices のため、全件 pagination が必要なモード（ClientName/ProjectName/Text/Status）は cache-warm 必須で明示 SKIP。

## 対象コード

- `internal/service/find/find_invoice.go` — FindInvoice: ID/ClientName/ProjectName/Text/Status モード
- `internal/service/find/e2e_test.go` — ID モード追加 + 他モード明示 SKIP + 既存コメント更新

## 追加テスト

| テスト名 | 結果 | 備考 |
|---|---|---|
| `TestE2E_FindInvoice_ByID_Strict` | PASS | clientID=0→nil, projectID=82448572→Project enrichment ✓ |
| `TestE2E_FindInvoice_ByClientName` | SKIP | cache-warm required; 11000+ invoices |
| `TestE2E_FindInvoice_ByProjectName` | SKIP | cache-warm required; 11000+ invoices |
| `TestE2E_FindInvoice_ByText` | SKIP | cache-warm required; 11000+ invoices |
| `TestE2E_FindInvoice_ByStatus` | SKIP | cache-warm required; 11000+ invoices |

## 発見事項

### FindInvoice ID モード enrichment

```
FindInvoice(ID=59164813): clientID=0 client_resolved=false projectID=82448572 project_resolved=true
```

- `ClientID=0` → `r.Client=nil` 正常
- `ProjectID=82448572` → `r.Project != nil && r.Project.ID=82448572` 正常
- FindOrder/Delivery/Receipt と異なり、FindInvoice の ID モードは enrichment を実施する（`resolveClientAndProject` を呼ぶ設計）

### 実行時間

`TestE2E_FindInvoice_ByID_Strict` は 115 秒かかった。これは cache miss で GetInvoice + GetProject（enrichment）のネットワーク fetch が発生したため。キャッシュ warm 後は大幅に短縮される見込み。

## API コール数

- ListInvoicesPage(1, 1) × 1
- GetInvoice(ID) × 1 (cache miss → API)
- GetProject(ProjectID) × 1 (enrichment, cache miss → API)
- 合計: ~3 req

## コミット

1. `test(e2e): M32 FindInvoice の軽量 E2E（ID モード）を追加`
2. `docs(plans): M32 完了 + Phase H 完走をロードマップに反映`
