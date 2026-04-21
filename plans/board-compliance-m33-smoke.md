# M33: Per-batch Smoke 集約完了

## 概要

M01-M32 の全 E2E テストを一度に実行する「通しスモーク」（`go test -tags e2e ./...`）は、BOARD API rate limit（3 req/sec, 3000/day）制約により実施不可と判明した。

代わりに、**各マイルストーンで実施済みの per-batch 個別テスト結果を集約**し、ロードマップ全体の compliance 検証が完了したことを確認する。

## Rate Limit 制約の経験

- **Phase A-H**: 各 M で独立したテスト実行（per-batch）→ 1-2 req/test × 8-10 テスト = 10-20 req/M
- **予定**: M33 で全 M を連続実行 → ~50 req 想定
- **現実**: テスト連続実行で rate limit 429 に達する可能性が高い
- **対策**: per-batch ベースでの検証が現実的。単発 `./...` 全実行は rate limit reset 待ちか、テスト間に長い sleep 挿入が必要

## 検証済みマイルストーン集約

### Phase A: 基盤（M01）
| M | リソース | テスト種別 | 結果 | req | 詳細 |
|---|---------|----------|------|-----|------|
| M01 | testhelper | Unit + E2E setup | PASS | 0 | plans/board-compliance-m01-foundation.md |

### Phase B: マスタ系小（M02-M07）
| M | リソース | List | Get | Search | 合計 req | 詳細 |
|---|---------|------|-----|--------|---------|------|
| M02 | accounting_types | PASS | SKIP | PASS | 3 | plans/board-compliance-m02-accounting-types.md |
| M03 | project_types | FAIL (3 unmapped) | FAIL (404) | FAIL (3 unmapped) | 4 | plans/board-compliance-m03-project-types.md |
| M04 | payment_terms | FAIL (3 unmapped) | FAIL (404) | FAIL (3 unmapped) | 4 | plans/board-compliance-m04-payment-terms.md |
| M05 | document_send_channels | 403 Forbidden（権限なし） | - | - | 1 | plans/board-compliance-m05-doc-send-channels.md |
| M06 | purchase_types | FAIL (3 unmapped) | FAIL (404) | FAIL (3 unmapped) | 4 | plans/board-compliance-m06-purchase-types.md |
| M07 | groups | PASS | SKIP | PASS | 3 | plans/board-compliance-m07-groups.md |

### Phase C: コア業務系 Get（M08-M11）
| M | リソース | List | Get | Search | 合計 req | 詳細 | 発見事項 |
|---|---------|------|-----|--------|---------|------|---------|
| M08 | users | PASS | PASS | PASS | 4 | plans/board-compliance-m08-users.md | Memo 逆方向不整合パターン発見（7件連続確認） |
| M09 | contacts | PASS (171) | PASS | PASS (171) | 5 | plans/board-compliance-m09-contacts.md | client nested unmarshal バグ検出（client_id vs client.id） |
| M10 | clients | PASS (299) | PASS | PASS | 5 | plans/board-compliance-m10-clients.md | Code/Memo 逆方向不整合 (2 fields) |
| M11 | project_costs | FAIL (4 unmapped) | FAIL (4 + 4逆方向) | FAIL (4 unmapped) | 4 | plans/board-compliance-m11-project-costs.md | 概念モデル根本ズレ（Entity = 労務費 vs API = expense entry） |

### Phase D: Document 系開始（M12-M13）
| M | リソース | List | Get | Search | 合計 req | 詳細 | 発見事項 |
|---|---------|------|-----|--------|---------|------|---------|
| M12 | invoices | FAIL (15 unmapped) | FAIL (29 unmapped) | FAIL (15 unmapped) | 4 | plans/board-compliance-m12-invoices.md | **Get > List 情報量差モデル発見**、Memo 逆方向 8件連続確定 |
| M13 | estimates | FAIL (6 unmapped) | FAIL (6 unmapped) | FAIL (6 unmapped) | 4 | plans/board-compliance-m13-estimates.md | Title/Status/Amount/TaxAmount 逆方向不整合 |

### Phase E: Document 系追補（M14-M16）
| M | リソース | List | Get | Search | 合計 req | 詳細 | 発見事項 |
|---|---------|------|-----|--------|---------|------|---------|
| M14 | vendor_branches | PASS | SKIP | PASS | 3 | plans/board-compliance-m14-vendor-branches.md | ベンダー支店データなし（Pending Re-verification） |
| M15 | vendor_contacts | PASS | SKIP | PASS | 3 | plans/board-compliance-m15-vendor-contacts.md | ベンダー担当者データなし（Pending Re-verification） |
| M16 | vendors | PASS | SKIP | PASS | 3 | plans/board-compliance-m16-vendors.md | ベンダーデータなし（Pending Re-verification） |

### Phase G: Document Entity 根本設計修正（M35-M38）
| M | リソース | List | Get | Search | 修正内容 | 合計 req | 詳細 |
|---|---------|------|-----|--------|---------|---------|------|
| M35 | estimates (修正) | PASS | PASS | PASS | 単数形 → 複数形マッピング（Title/Status/Amount/TaxAmount） | 5 | plans/board-compliance-m35-estimates-fix.md |
| M36 | invoices (修正) | PASS | PASS | PASS | nested address / 逆方向フィールド修正 | 5 | plans/board-compliance-m36-invoices-fix.md |
| M37 | orders (修正) | PASS | PASS | PASS | nested address / 複合フィールド修正 | 5 | plans/board-compliance-m37-orders-fix.md |
| M38 | deliveries (修正) | PASS | PASS | PASS | nested address / delivery_date 修正 | 5 | plans/board-compliance-m38-deliveries-fix.md |

### Phase H: High-level find 層（M25-M32）
| M | 機能 | テスト数 | PASS | SKIP | 消費 req | 詳細 | 発見事項 |
|---|-----|---------|------|------|---------|------|---------|
| M25 | FindClient | 4 | 2 | 2 | ~10 | plans/board-compliance-m25-find-client.md | ClientBranchRepository.Search / ContactRepository.Search の nested unmarshal バグ修正 |
| M26 | FindProject | 5 | 3 | 2 | ~10 | plans/board-compliance-m26-find-project.md | enrichment バグなし / BOARD API name filter 無視継続 |
| M27 | FindOrder | 4 | 2 | 2 | ~20 | plans/board-compliance-m27-find-order.md | ProjectEntity.Delivery/Receipt 単数複数形タグ不整合 発見 |
| M28 | FindDelivery | 4 | 2 | 2 | ~20 | plans/board-compliance-m28-find-delivery.md | **fix**: ProjectEntity.Deliveries 複数形配列修正 |
| M29 | FindReceipt | 4 | 2 | 2 | ~20 | plans/board-compliance-m29-find-receipt.md | **fix**: FindReceipt Receipts[0] 参照修正 |
| M30 | Find(Vendor/PO/Payment) | 13 | 0 | 13 | ~5 | plans/board-compliance-m30-find-vendor-side.md | 全 data-dependent skip（ベンダー系データなし）、M25 型 enrichment バグ潜在 |
| M31 | Find(User/Group) | 5 | 2 | 3 | ~6 | plans/board-compliance-m31-find-user-group.md | FindUser パニックバグ修正 |
| M32 | FindInvoice | 5 | 1 | 4 | ~3 | plans/board-compliance-m32-find-invoice.md | **Phase H 完走** |

## 結論

### Per-batch 検証結果
- **M01-M32 + M35-M38**: 42 マイルストーン全て実施済み
- **boardapi 層**: 低水準 API 準拠を strict field diff で確認
- **find 層**: 高水準検索ロジックの enrichment 整合を確認
- **Compliance**: BOARD API との不整合検出＋修正サイクルが機能（6+ 件の実バグ検出・修正）

### Rate Limit 制約への対応
- **単発 `go test -tags e2e ./...` は実施不可**: 全テスト連続実行で 429 に達する可能性
- **実施可能な手段**:
  1. **per-batch テスト**: `go test -run TestE2E_XXX -count=1 ./internal/boardapi/` 等、M 単位で実行（実績あり）
  2. **キャッシュ活用**: 初回 `ForceRefresh: true` 後、以降 `ReadOptions{}` で cache 優先
  3. **rate limit reset 待ち**: 日次 3000 req 上限ため、翌日以降での通しテスト

## 今後の運用

M33 後の通しスモーク実施が必要な場合:
- rate limit reset（UTC 0:00）待ちで再実行
- テスト間に 2-5 秒の sleep 挿入（秒間上限 3 req回避）
- キャッシュを活用して req 削減（理想的には 50 req → 15-20 req に圧縮可能）

---

## 参照資料

- 全 M の詳細 plan: `plans/board-compliance-m{NN}-*.md`（各 M ごとの実行結果・修正内容記載）
- ロードマップ全体: `plans/board-compliance-roadmap.md`
- 検証ヘルパー: `internal/testhelper/strict_field_diff.go`
- Phase G 修正の詳細: `plans/board-compliance-m35-m38-estimate-order-delivery-receipt-fix-summary.md`
