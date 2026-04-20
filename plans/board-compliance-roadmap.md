# Roadmap: BOARD API 準拠検証 & E2E 網羅

## Meta
| 項目 | 値 |
|------|---|
| ゴール | 全 22 リソース × (List/Get/Search) × (boardapi/find 層) の E2E を網羅し、全 Entity を実 API と**厳格フィールド突合**する |
| 成功基準 | ①未マップフィールド 0 件、②boardapi・service/find の両層で全リソースの成功パスが E2E 通過、③中断リスクに備えた M 単位の独立再開が可能 |
| 制約 | BOARD API rate limit 3000/日・3/秒。手動 1-request 確認フローのため **数日〜2週間**。本番業務未使用のため実運用競合は無視可 |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 親プラン | plans/vivid-strolling-ocean.md |
| 作成日 | 2026-04-20 |
| 最終更新 | 2026-04-20 16:29 |
| ステータス | 未着手（M01 先行詳細化済み） |

## 背景と動機
- 直近 `271cba3` で UserEntity/ContactEntity/VendorContactEntity に計 6 フィールドもの実 API 不整合が発覚。
- 未 E2E の 12 リソース + 既 E2E でも Search/Get 未検証が多数。LLM/MCP 経由でサイレントにデータ欠落するリスク。
- 既存 `board-roadmap.md` は機能完成のマイルストーン構成。**準拠検証は別軸として本ロードマップで走らせる**（既存 M40 までは凍結しない）。

## 運用ルール（全 M 共通）

### Rate Limit 対応
1. M 着手前に **事前 API コール数見積** を `plans/board-compliance-m{NN}-*.md` に記載。
2. 最初の 1 リソースは `ForceRefresh: true`、以降は `ReadOptions{}` で cache 優先。
3. 1 test 1 endpoint の粒度で `go test -run TestE2E_XXX -count=1` を実行し、実際の req 数を確認してから次へ。
4. **403/429 即停止**：skip せず失敗として扱い、本 Roadmap の `Blockers` に記録。
5. 日次上限ガイド: **1500 req/日**（業務未使用のため余裕あり）。

### 厳格フィールド突合
- M01 で提供する `testhelper.StrictFieldDiff(t, raw, entity)` を全 E2E で呼ぶ。
- 生 JSON は `tmp/e2e-artifacts/{resource}_{id}.json`（**.gitignore 済み**、絶対に commit しない）。

### 進捗管理
- M 完了ごとに本ファイル Progress のチェックボックスを更新。
- Changelog には「なぜその順序で進めたか」「どんなずれが見つかったか」を記録。
- 失敗した M は `Blockers` に転記、ユーザー判断待ちに。

## Current Focus
- **マイルストーン**: M01 基盤整備
- **直近の完了**: ロードマップ作成（2026-04-20）
- **次のアクション**: `/devflow:implement` で M01 を実行

## Progress

---

### Phase A: 基盤整備

#### M01: 厳格突合ヘルパー & tmp/ 整備
- [ ] `.gitignore` に `tmp/` を追加
- [ ] `internal/testhelper/strict_field_diff.go` 実装 + unit test
- [ ] `dumpJSON()` を boardapi / find の e2e helper に追加
- [ ] `mise test:e2e:single <pattern>` タスク定義
- [ ] M02 着手用コメントを PR に含める
- 見積: 0 req（unit のみ）
- 詳細: plans/board-compliance-m01-foundation.md

---

### Phase B: マスタ系（小）

#### M02: accounting_types 完走
- [ ] E2E: List / Get / Search（Search 対応なら）
- [ ] 厳格フィールド突合
- [ ] raw JSON を tmp/ にダンプ
- 見積: ~5 req
- 詳細: plans/board-compliance-m02-accounting-types.md（着手時生成）

#### M03: project_types 完走
- [ ] E2E: List / Get / Search
- [ ] 厳格フィールド突合
- 見積: ~5 req
- 詳細: plans/board-compliance-m03-project-types.md（着手時生成）

#### M04: payment_terms 完走
- [ ] E2E: List / Get / Search
- [ ] 厳格フィールド突合
- 見積: ~5 req
- 詳細: plans/board-compliance-m04-payment-terms.md（着手時生成）

#### M05: document_send_channels 完走
- [ ] E2E: List / Get / Search
- [ ] 厳格フィールド突合
- 見積: ~5 req
- 詳細: plans/board-compliance-m05-document-send-channels.md（着手時生成）

#### M06: purchase_types Search/Get 追補
- [ ] E2E: Get（既存 List を活用）
- [ ] E2E: Search
- [ ] 厳格フィールド突合
- 見積: ~3 req
- 詳細: plans/board-compliance-m06-purchase-types.md（着手時生成）

#### M07: groups Get + 厳格突合
- [ ] E2E: Get（既存 List 前提）
- [ ] 厳格フィールド突合（GroupEntity）
- 見積: ~3 req
- 詳細: plans/board-compliance-m07-groups.md（着手時生成）

---

### Phase C: マスタ系（中）

#### M08: users Get/Search 厳格突合
- [ ] E2E: Get（既存）+ 厳格突合
- [ ] E2E: Search（新規）
- [ ] UserEntity の last_sign_in_at / role_* フィールド確認
- 見積: ~5 req
- 詳細: plans/board-compliance-m08-users.md（着手時生成）

---

### Phase D: コア業務（未カバー）

#### M09: client_branches 完走
- [ ] E2E: List / Get / Search
- [ ] 厳格フィールド突合（ClientBranchEntity 11 フィールド）
- 見積: ~8 req
- 詳細: plans/board-compliance-m09-client-branches.md（着手時生成）

#### M10: contacts 完走（19 フィールド）
- [ ] E2E: List / Get / Search
- [ ] 厳格突合（Name/LastName/FirstName/HonorificTitle/Department/Note/ArchiveFlg 等）
- [ ] 既存 271cba3 で追加されたフィールドが漏れなく埋まるか検証
- 見積: ~10 req
- 詳細: plans/board-compliance-m10-contacts.md（着手時生成）

#### M11: project_costs 完走
- [ ] E2E: List / Get / Search
- [ ] 厳格フィールド突合
- 見積: ~8 req
- 詳細: plans/board-compliance-m11-project-costs.md（着手時生成）

---

### Phase E: コア業務（再検証）

#### M12: clients 厳格突合
- [ ] 既存 E2E（List/Get/Search）に厳格突合を追加
- [ ] ClientEntity の全フィールドが埋まるか確認
- 見積: ~5 req
- 詳細: plans/board-compliance-m12-clients.md（着手時生成）

#### M13: projects 厳格突合 + GetWithGroup 全 response_group
- [ ] 既存 List/Get + 厳格突合
- [ ] GetProjectWithGroup を estimate/order/delivery/invoice/receipt/all の各 response_group で検証
- [ ] DocumentSummary の全フィールド確認
- 見積: ~15 req
- 詳細: plans/board-compliance-m13-projects.md（着手時生成）

---

### Phase F: ベンダー系

#### M14: vendor_branches 完走（payee_branches 実パス）
- [ ] E2E: List / Get / Search
- [ ] 厳格フィールド突合
- [ ] `/v1/payee_branches` と boardapi の命名不一致が使用者に混乱を与えていないか確認
- 見積: ~8 req
- 詳細: plans/board-compliance-m14-vendor-branches.md（着手時生成）

#### M15: vendor_contacts 完走（payee_contacts 実パス）
- [ ] E2E: List / Get / Search
- [ ] 厳格フィールド突合（VendorContactEntity）
- 見積: ~8 req
- 詳細: plans/board-compliance-m15-vendor-contacts.md（着手時生成）

#### M16: vendors Get/Search 追補
- [ ] E2E: Get（既存 List 前提）
- [ ] E2E: Search
- [ ] 厳格フィールド突合
- 見積: ~5 req
- 詳細: plans/board-compliance-m16-vendors.md（着手時生成）

---

### Phase G: ドキュメント系

#### M17: documentID discovery helper 確立
- [ ] `findAnyDocumentID(t, api, docType)` を e2e helper に追加
- [ ] projects を response_group 付きで走査し docType が非 nil の最初の 1 件を返す
- [ ] estimates / orders / deliveries / receipts 共通で再利用
- 見積: ~5 req
- 詳細: plans/board-compliance-m17-docid-discovery.md（着手時生成）

#### M18: estimates Get 厳格突合
- [ ] 既存 E2E を M17 helper 経由に切り替え
- [ ] 厳格フィールド突合（EstimateEntity、明細行含む）
- 見積: ~5 req
- 詳細: plans/board-compliance-m18-estimates.md（着手時生成）

#### M19: orders Get + 厳格突合
- [ ] E2E: Get（M17 helper）
- [ ] 厳格フィールド突合（OrderEntity）
- 見積: ~10 req
- 詳細: plans/board-compliance-m19-orders.md（着手時生成）

#### M20: deliveries Get + 厳格突合
- [ ] E2E: Get（M17 helper）
- [ ] 厳格フィールド突合
- 見積: ~10 req
- 詳細: plans/board-compliance-m20-deliveries.md（着手時生成）

#### M21: receipts Get + 厳格突合
- [ ] E2E: Get（M17 helper）
- [ ] 厳格フィールド突合
- 見積: ~10 req
- 詳細: plans/board-compliance-m21-receipts.md（着手時生成）

#### M22: invoices Get/Search + 厳格突合
- [ ] 既存 List + 厳格突合
- [ ] Get / Search 追加
- [ ] 11,000 件アカウントでは per_page=1 など軽量化
- 見積: ~20 req（キャッシュ効かせると激減）
- 詳細: plans/board-compliance-m22-invoices.md（着手時生成）

#### M23: purchase_orders Get/Search 追補
- [ ] E2E: Get / Search
- [ ] 厳格フィールド突合
- 見積: ~10 req
- 詳細: plans/board-compliance-m23-purchase-orders.md（着手時生成）

#### M24: payments Get/Search 追補
- [ ] E2E: Get / Search
- [ ] 厳格フィールド突合
- 見積: ~10 req
- 詳細: plans/board-compliance-m24-payments.md（着手時生成）

---

### Phase H: service/find 層

#### M25: FindClient 厳格化
- [ ] 既存 Name/Text 検索に ClientResult 全フィールド確認を追加
- [ ] Branches/Contacts の enrichment が欠損なく埋まるか検証
- 見積: ~5 req
- 詳細: plans/board-compliance-m25-find-client.md（着手時生成）

#### M26: FindProject 全パス検証
- [ ] ClientID / ClientName / WithEstimate の全パス
- [ ] ProjectResult の Client/Estimate 埋め込み検証
- 見積: ~10 req
- 詳細: plans/board-compliance-m26-find-project.md（着手時生成）

#### M27: FindOrder 新規 E2E
- [ ] ProjectID → FindOrder の結果検証
- [ ] FindEstimate と対比して実装対称性を確認
- 見積: ~10 req
- 詳細: plans/board-compliance-m27-find-order.md（着手時生成）

#### M28: FindDelivery 新規 E2E
- [ ] ProjectID → FindDelivery
- 見積: ~10 req
- 詳細: plans/board-compliance-m28-find-delivery.md（着手時生成）

#### M29: FindReceipt 新規 E2E
- [ ] ProjectID → FindReceipt
- 見積: ~10 req
- 詳細: plans/board-compliance-m29-find-receipt.md（着手時生成）

#### M30: FindVendor / FindPurchaseOrder / FindPayment 新規 E2E
- [ ] 3 Find サービスそれぞれで成功パス検証
- [ ] 厳格フィールド突合
- 見積: ~15 req
- 詳細: plans/board-compliance-m30-find-vendor-side.md（着手時生成）

#### M31: FindUser / FindGroup 厳格突合
- [ ] 既存 E2E + 厳格突合
- [ ] UserEntity の DisplayName フォールバック経路確認
- 見積: ~5 req
- 詳細: plans/board-compliance-m31-find-user-group.md（着手時生成）

#### M32: FindInvoice 軽量 E2E
- [ ] ClientID 検索で per_page=1、キャッシュ利用前提
- [ ] 大量件数アカウントでも数秒で終わる構成
- 見積: ~5 req
- 詳細: plans/board-compliance-m32-find-invoice.md（着手時生成）

---

### Phase I: 仕上げ

#### M33: 全 E2E 通しスモーク（キャッシュ有効）
- [ ] M01-M32 の全 E2E を `go test -tags e2e ./...` で通しパス
- [ ] 実 req 数を記録
- [ ] 失敗時は当該 M へ差戻し
- 見積: ~50 req（キャッシュ効く前提）
- 詳細: plans/board-compliance-m33-smoke.md（着手時生成）

#### M34: ドキュメント反映
- [ ] `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` に発見された不整合と修正を追記
- [ ] `CLAUDE.md` の「テスト戦略」節を更新
- [ ] `memory/` に E2E 運用の learnings を記録
- [ ] 本ロードマップの Changelog に総括を追加
- 見積: 0 req
- 詳細: plans/board-compliance-m34-docs.md（着手時生成）

---

## Blockers
なし

## Architecture Decisions
| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | boardapi + service/find のみを対象（MCP は除外） | MCP は上位 wrapper のため、下層が通れば自動的にカバーされる | 2026-04-20 |
| 2 | マスタ系を先行実行 | item 数が少なく失敗時の損失が小。学習と helper 改善を反復できる | 2026-04-20 |
| 3 | 厳格全フィールド突合を強制 | 271cba3 の再発防止。LLM/MCP 経由の silent data loss 排除 | 2026-04-20 |
| 4 | 403/429 即停止（skip しない） | 環境差異や権限問題を見逃さないため。人間判断で再開 | 2026-04-20 |
| 5 | 生 JSON は tmp/ に dump、commit しない | 顧客名等のシークレット混入リスクを commit 履歴に残さない | 2026-04-20 |
| 6 | documentID は projects response_group から発見 | orders/deliveries/receipts の List 相当を API 仕様範囲内で再現 | 2026-04-20 |

## Changelog
| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-20 16:29 | 作成 | ロードマップ初版作成。親プラン plans/vivid-strolling-ocean.md を参照。34 マイルストーン構成で中断耐性を最大化。 |
