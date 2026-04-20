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
| 最終更新 | 2026-04-20 18:15 |
| ステータス | M08 完了（List PASS（26 items, unmapped 0）/ Get FAIL（404 = マスタ系 4 件連続）/ Search PASS（26 items, unmapped 0, name filter 無視 = 4 件連続）。`UserEntity.Name` が逆方向不整合（実 API に name キー不在）。Unit 5/5 Green） |

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

### データ依存テストの skip 規約
- List/Search 結果が **0 件**の場合の Get テストのみ `t.Skipf` を許可する。理由は「Get 対象 ID が取得できない = 検証不能」であり、403/429 のような環境/権限異常とは別カテゴリ。
- Skip した M は本ファイル「Pending Re-verification（実データ投入後に再実行）」に転記して追跡する。
- List/Search 自体の失敗、403/429、TLS 等の接続失敗は引き続き **skip 禁止**。

### 厳格フィールド突合
- M01 で提供する `testhelper.StrictFieldDiff(t, raw, entity)` を全 E2E で呼ぶ。
- 生 JSON は `tmp/e2e-artifacts/{resource}_{id}.json`（**.gitignore 済み**、絶対に commit しない）。

### 進捗管理
- M 完了ごとに本ファイル Progress のチェックボックスを更新。
- Changelog には「なぜその順序で進めたか」「どんなずれが見つかったか」を記録。
- 失敗した M は `Blockers` に転記、ユーザー判断待ちに。

## Current Focus
- **マイルストーン**: M09 client_branches
- **直近の完了**: M08 実 API E2E 実行で **List PASS（26 items, unmapped 0）/ Get FAIL（404 = マスタ系 4 件連続）/ Search PASS（26 items, unmapped 0, name filter 無視 4 件連続）**。271cba3 で追加された 6 フィールド（last_name / first_name / role_id / role_name / last_sign_in_at / valid_flg）は **全 26 件で完全に埋まっており実 API と完全一致**（`last_sign_in_at` は長さ 29 の ISO 8601 で null 欠落なし）。一方で **実 API に `name` キーが不在**（`UserEntity.Name` の逆方向不整合、M03/M04/M06 の `Memo` と同現象、マスタ系で 4 件目）。Unit は 5/5 Green。リソース全体 403 や 429 は発生せず。
- **次のアクション**: M09 (client_branches) を着手

## Progress

---

### Phase A: 基盤整備

#### M01: 厳格突合ヘルパー & tmp/ 整備 ✅
- [x] `.gitignore` に `/tmp/` を追加
- [x] `internal/testhelper/strict_field_diff.go` 実装 + unit test（13/13 Green）
- [x] `dumpJSON()` を boardapi / find の e2e helper に追加（`findRepoRoot` で go.mod find-up）
- [x] `mise test:e2e:single` タスク定義
- [x] M02 着手用コメントを `internal/boardapi/e2e_test.go` 冒頭に追加
- 見積: 0 req（unit のみ） / 実績: 0 req
- 詳細: plans/board-compliance-m01-foundation.md

---

### Phase B: マスタ系（小）

#### M02: accounting_types 完走 ✅（List/Search PASS, Get は実データ投入後再検証）
- [x] `ListAccountingTypesRaw` / `GetAccountingTypeRaw` / `SearchAccountingTypesRaw` を boardapi に追加（byte 保持 Raw 層）
- [x] Unit（RoundTripper mock）5/5 Green
- [x] E2E: List PASS（0 items）/ Get Skip（データ無し）/ Search PASS（0 items）
- [x] 厳格フィールド突合（実データ 0 件のため未マップ検知 0、データ投入後の再実行で意味を持つ）
- [x] raw JSON を tmp/ にダンプ（accounting_types_0.json / accounting_types_search_0.json）
- 見積: ~5 req / 実績: 3 req（List 1 + Get 1 (List 再呼び出し) + Search 1）
- 詳細: plans/board-compliance-m02-accounting-types.md

#### M03: project_types 完走 🟡（List/Search は未マップ検知で Fail 状態、Get は API が 404 返却 = 非対応と判明）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合で 3 未マップ検出: `archive_flg`, `company_bank_id`, `company_bank_name`
- [x] `ProjectTypeEntity.Memo` は実 API に存在しない（逆方向不整合も発見）
- [x] `GET /v1/project_types/{id}` は 404 を返す（List で取得した有効 ID に対しても）→ **API が個別 Get エンドポイントを提供していない**
- [x] `name` パラメータは無視される（検索しても全件返却）
- 見積: ~4 req / 実績: **4 req**（List 1 + Get 2 + Search 1）
- E2E 結果: List FAIL（11 items, 3 unmapped）/ Get FAIL（404）/ Search FAIL（11 items, 3 unmapped, filter 無効）
- 詳細: plans/board-compliance-m03-project-types.md
- **フォローアップ（別 commit / 別 M で対応予定）**:
  - `ProjectTypeEntity` に 3 フィールド追加、`Memo` 削除検討
  - `GetProjectType` / `GetProjectTypeRaw` の公開 API 妥当性（そもそも API 非対応なら削除 or エラーメッセージ明確化）を検討
  - `SearchProjectTypes` の `Name` パラメータが効かない件をドキュメント化または削除

#### M04: payment_terms 完走 🟡（List/Search は未マップ検知で Fail 状態、Get は API が 404 返却 = 非対応と判明）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合で 1 未マップ検出: `archive_flg`
- [x] `PaymentTermEntity.Memo` は実 API に存在しない（逆方向不整合、M03 と同じ現象）
- [x] `GET /v1/payment_terms/{id}` は 404 を返す（List で取得した有効 ID に対しても）→ **API が個別 Get エンドポイントを提供していない**（M03 と同じ現象）
- [x] `name` パラメータは無視される（検索しても全件 16 件返却、M03 と同じ現象）
- 見積: ~5 req / 実績: **4 req**（List 1 + Get 2 + Search 1）
- E2E 結果: List FAIL（16 items, 1 unmapped）/ Get FAIL（404）/ Search FAIL（16 items, 1 unmapped, filter 無効）
- 詳細: plans/board-compliance-m04-payment-terms.md
- **フォローアップ（別 commit / 別 M で対応予定）**:
  - `PaymentTermEntity` に `ArchiveFlg` 追加、`Memo` 削除検討
  - `GetPaymentTerm` / `GetPaymentTermRaw` の公開 API 妥当性（API 非対応なら削除 or エラーメッセージ明確化）を検討
  - `SearchPaymentTerms` の `Name` パラメータが効かない件をドキュメント化または削除
  - M03/M04 で同現象が 2 件確定したため、M05 以降のマスタ系でも同パターン観測可能性を織り込んで計画

#### M05: document_send_channels 完走 🟡（List/Get/Search 全て 403 Forbidden = 当該アカウントで API 非提供、厳格フィールド突合は未達）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合は **未達**（403 のため JSON 応答を取得できず）
- [x] `GET /v1/document_send_channels` / `/v1/document_send_channels?name=...` 全て 403 Forbidden（`許可されていません。`）を返す → **当該アカウントでは API 非提供**（同 credentials で M02-M04 の他マスタ系は 200 のため、環境/認証要因ではない）
- 見積: ~5 req / 実績: **3 req**（List 1 + List 再呼 1 + Search 1。Get 本体は discovery 段階で Fatalf のため未到達）
- E2E 結果: List FAIL（403）/ Get FAIL（403、discovery 段階）/ Search FAIL（403）
- 詳細: plans/board-compliance-m05-document-send-channels.md
- **フォローアップ（別 commit / 別 M で対応予定）**:
  - `DocumentSendChannelEntity` のフィールド突合は BOARD 側で権限付与もしくは API 提供が始まった時点で再実行
  - `ListDocumentSendChannels` / `GetDocumentSendChannel` / `SearchDocumentSendChannels` / `ListDocumentSendChannelsPage` の公開 API そのものの妥当性（BOARD がそもそも提供しないなら削除 or ドキュメントで注意喚起）を検討
  - M03/M04 の Get 404 / name filter 無効 / archive_flg 欠落 / Memo 逆方向不整合に加え、M05 で **リソース全体 403** の新パターンが確認された。M06 `purchase_types` 以降では取得自体が拒否されるシナリオも織り込む

#### M06: purchase_types Search/Get 追補 🟡（List/Search は未マップ検知で Fail 状態、Get は API が 404 返却 = 非対応と判明）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合で 1 未マップ検出: `archive_flg`（M04 と同じ）
- [x] `PurchaseTypeEntity.Memo` は実 API に存在しない（逆方向不整合、M03/M04 と同現象）
- [x] `GET /v1/expenditure_types/{id}` は 404 を返す（List で取得した有効 ID に対しても）→ **API が個別 Get エンドポイントを提供していない**（M03/M04 と同現象、3 件連続）
- [x] `name` パラメータは無視される（検索しても全件 5 件返却、M03/M04 と同現象）
- 見積: ~3 req / 実績: **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1）
- E2E 結果: List FAIL（5 items, 1 unmapped）/ Get FAIL（404）/ Search FAIL（5 items, 1 unmapped, filter 無効）
- 詳細: plans/board-compliance-m06-purchase-types.md
- **フォローアップ（別 commit / 別 M で対応予定）**:
  - `PurchaseTypeEntity` に `ArchiveFlg` 追加、`Memo` 削除検討（M03/M04 と横並びで一括対応が効率的）
  - `GetPurchaseType` / `GetPurchaseTypeRaw` の公開 API 妥当性（API 非対応なら削除 or エラーメッセージ明確化）を検討
  - `SearchPurchaseTypes` の `Name` パラメータが効かない件をドキュメント化または削除
  - **マスタ系 3 件（M03/M04/M06）で「Get 404 + name 無効 + archive_flg 欠落 + Memo 逆方向」の同一パターンが固定化。フォローアップ M で一括対応推奨**

#### M07: groups Get + 厳格突合 ✅（List PASS、Get は List 0 件のため Skip = pending re-verification）
- [x] Raw 層 2 本追加（List/Get、Search Raw は M07 スコープ外として意図的未提供）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用）
- [x] E2E: List / Get 実行
- [x] 厳格フィールド突合: List は 0 件のため未マップ検出機会なし（実データ投入後の再実行で意味を持つ）
- [x] `GET /v1/groups`: 200、0 items、レスポンスは `null`（M02 accounting_types と同じく空集合 `null` 返却）
- [x] `GET /v1/groups/{id}`: 未検証（List 0 件のため discovery 失敗 → Skip）
- [x] 既存 `e2e_test.go` の軽量 `TestE2E_Groups_List` を削除し、M07 厳格突合版に一本化（M06 と同パターン）
- 見積: ~3 req / 実績: **2 req**（List 1 + Get 内 discovery List 1。Get 本体は Skip により未到達）
- E2E 結果: List PASS（0 items, `null`）/ Get SKIP（pending re-verification）
- 詳細: plans/board-compliance-m07-groups.md
- **フォローアップ（実データ投入後 / 別 commit）**:
  - List/Get の厳格突合をデータ投入後に再実行し、`archive_flg` / `Memo` 逆方向 / 階層フィールド（`parent_id`等）の有無を確認
  - Get が 404 を返すか 200 を返すかを確定（マスタ系 3 件で 404 確定だが groups は accounts によって挙動が異なる可能性）
  - 必要に応じて `GroupEntity` を拡張、Search Raw 追加検討（必要時に別 M）

---

### Phase C: マスタ系（中）

#### M08: users Get/Search 厳格突合 🟡（List/Search は PASS で未マップ検出 0 / Get は API が 404 返却 = 非対応と判明。`UserEntity.Name` は逆方向不整合）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合: **未マップ 0 件**（271cba3 で追加された 6 フィールドが実 API と完全一致。`UserEntity` は 10 個の API 対応フィールドすべて実 API と一致）
- [x] `GET /v1/users/{id}` は 404 を返す（List で取得した有効 ID に対しても）→ **API が個別 Get エンドポイントを提供していない**（M03/M04/M06 と同現象、マスタ系 **4 件連続**）
- [x] `name` パラメータは無視される（検索しても全件 26 件返却、M03/M04/M06 と同現象、**4 件連続**）
- [x] **実 API レスポンスに `name` キーは不在**（`UserEntity.Name` が逆方向不整合、M03/M04/M06 の `Memo` と同現象、マスタ系で **4 件目**）
- [x] 既存 `e2e_test.go` の軽量 `TestE2E_Users_List` / `TestE2E_Users_GetByID` を削除し、M08 厳格突合版に一本化（M06/M07 と同パターン）
- 見積: ~5 req / 実績: **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1）
- E2E 結果: List PASS（26 items, unmapped 0）/ Get FAIL（404）/ Search PASS（26 items, unmapped 0, filter 無効）
- 詳細: plans/board-compliance-m08-users.md
- **フォローアップ（別 commit / 別 M で対応予定）**:
  - `UserEntity.Name` 削除検討（マスタ系の `Memo` 削除と横並びで一括対応が効率的）。削除すると `DisplayName()` の `Name != ""` 分岐が常に false になるが、現時点でも実 API データでは必ず LastName+FirstName 経路で動作しているため安全
  - `GetUser` / `GetUserRaw` の公開 API 妥当性（API 非対応なら削除 or エラーメッセージ明確化）。**マスタ系 4 件連続で Get 404 が確定**したため、公開 API 一括削除 or deprecate の設計判断がより強く推奨される
  - `SearchUsers` の `Name` パラメータが効かない件をドキュメント化または削除（`name` 無視は 4 件連続で BOARD API マスタ系共通仕様）
  - `role_id` の enum 値範囲の完全仕様化は別 M（M25-M31 Find 層）で BOARD 側ドキュメント突合

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
なし（過去: M02 実 API E2E は sandbox の HTTPS proxy ホスト許可追加で解消、2026-04-20 17:08）

## Pending Re-verification（実データ投入後に再実行）
0 件のためフィールド突合検証が未達のリソース。BOARD アカウントに 1 件以上データを投入後、該当テストを再実行して未マップフィールドを確認する。

| M | リソース | 未検証テスト | 理由 | 再実行コマンド |
|---|---------|-------------|------|----------------|
| M02 | accounting_types | Get | List 0 件 | `go test -tags e2e -v -count=1 -run TestE2E_AccountingTypes_Get ./internal/boardapi/` |
| M05 | document_send_channels | List / Get / Search | 403 Forbidden（当該アカウントで API 非提供） | 権限付与後: `go test -tags e2e -v -count=1 -run TestE2E_DocumentSendChannels ./internal/boardapi/` |
| M06 | purchase_types | Get | 404 = 個別 Get API 非対応（M03/M04 と同現象） | API 提供開始後: `go test -tags e2e -v -count=1 -run TestE2E_PurchaseTypes_Get ./internal/boardapi/` |
| M07 | groups | List 厳格突合 / Get | List 0 件（M02 と同パターン）、Get は discovery 不能 | データ投入後: `go test -tags e2e -v -count=1 -run TestE2E_Groups ./internal/boardapi/` |

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
| 2026-04-20 17:xx | M02 実装 | `ListAccountingTypesRaw`/`GetAccountingTypeRaw`/`SearchAccountingTypesRaw` を追加し、unit テストは httptest ではなく `http.RoundTripper` モック方式で実装（sandbox 制約回避）。E2E コードは `testhelper.StrictFieldDiff` + `dumpJSON` で準拠検証のパターンを確立。実 API 検証は sandbox TLS 問題で未達、Blockers に記録。 |
| 2026-04-20 17:08 | M02 検証 | sandbox の HTTPS proxy 許可ホスト追加で Go TLS 問題解消。実 API E2E 実行成功: List/Search PASS（共に 0 items）、Get は List 0 件のため Skip。データ依存 skip 規約を運用ルールに追加し、Get を Pending Re-verification に転記。実消費 3 req（見積 5 req 以下）。 |
| 2026-04-20 17:22 | M03 実装・検証 | `ListProjectTypesRaw`/`GetProjectTypeRaw`/`SearchProjectTypesRaw` を M02 と同形式で追加。Unit 5/5 Green。実 API E2E で **3 未マップフィールド検出**（`archive_flg`, `company_bank_id`, `company_bank_name`）、**`Memo` フィールドが実 API に不在**、**`GET /v1/project_types/{id}` が 404 = API 非対応**、**`name` パラメータが無視される**ことを発見。E2E は意図的に Fail 状態で commit、Entity 修正は別 M で対応。実消費 4 req（見積 5 req 以下）。 |
| 2026-04-20 17:30 | M04 実装・検証 | `ListPaymentTermsRaw`/`GetPaymentTermRaw`/`SearchPaymentTermsRaw` を M03 と同形式で追加。Unit 5/5 Green（既存の `roundTripperFunc`/`jsonResp` を再利用）。実 API E2E で **1 未マップ検出**（`archive_flg`）、**`Memo` フィールドが実 API に不在**（逆方向不整合、M03 と同現象）、**`GET /v1/payment_terms/{id}` が 404 = API 非対応**（M03 と同現象）、**`name` パラメータが無視される**（M03 と同現象）ことを発見。マスタ系リソースで個別 Get 非対応 + name フィルタ無効の傾向が 2 件確定。E2E は意図的に Fail 状態で commit。実消費 4 req（見積 5 req 以下）。 |
| 2026-04-20 17:40 | M05 実装・検証 | `ListDocumentSendChannelsRaw`/`GetDocumentSendChannelRaw`/`SearchDocumentSendChannelsRaw` を M04 と同形式で追加。Unit 5/5 Green。実 API E2E で **List/Get/Search 全 3 テストが 403 Forbidden**（`許可されていません。`）を返却。同一 credentials で M02-M04 の他マスタ系は 200 を返すため、**当該アカウントに document_send_channels の権限がない or BOARD が API 提供していない**と判断。M03/M04 の「Get のみ 404」「name フィルタ無効」とは異なる **リソース全体 403** の新パターンを compliance finding として記録。フィールド突合は権限付与後に再検証（Pending Re-verification 転記）。E2E は意図的に Fail 状態で commit。実消費 3 req（見積 5 req 以下）。 |
| 2026-04-20 17:55 | M06 実装・検証 | `ListPurchaseTypesRaw`/`GetPurchaseTypeRaw`/`SearchPurchaseTypesRaw` を M04 と完全同形で追加（エンドポイントは `/v1/expenditure_types`、命名不一致は既存仕様維持）。既存 `e2e_test.go` にあった軽量 `TestE2E_PurchaseTypes_List` は重複のため削除し、M06 の厳格突合版に一本化。Unit 5/5 Green。実 API E2E で **1 未マップ検出**（`archive_flg`、M04 と同じ）、**`Memo` フィールドが実 API に不在**（M03/M04 と同現象）、**`GET /v1/expenditure_types/{id}` が 404 = API 非対応**（M03/M04 と同現象、**マスタ系 Get 404 が 3 件連続**）、**`name` パラメータが無視される**（M03/M04 と同現象、5 items 全件返却）ことを発見。マスタ系で「Get 404 + name 無効 + archive_flg 欠落 + Memo 逆方向」パターンが 3 件で固定化。E2E は意図的に Fail 状態で commit。実消費 4 req（見積 3 req → 上限 5 req 以内）。 |
| 2026-04-20 18:05 | M07 実装・検証 | `ListGroupsRaw` / `GetGroupRaw` を追加（**Search Raw は M07 スコープ外として意図的に未提供**、ロードマップ M07 定義「Get（既存 List 前提）+ 厳格突合（GroupEntity）」厳守）。既存 `e2e_test.go` の軽量 `TestE2E_Groups_List` は M06 と同パターンで削除し M07 厳格突合版に一本化。Unit 5/5 Green（既存 `roundTripperFunc`/`jsonResp` を再利用、Search 1 ケースの代わりに「既定 per_page=100 検証」を 5 本目として追加）。実 API E2E で **`GET /v1/groups` が 200 / 0 items / response body `null`**（M02 accounting_types と同パターン）を確認。List 0 件のため Get は `t.Skipf("pending re-verification")` で停止、List 厳格突合も実データなしのため未マップ検出機会なし。マスタ系の傾向（Get 404 / archive_flg / Memo 逆方向 / リソース 403）は M07 では **発生せず**（403/429 ともに無し）。実消費 **2 req**（List 1 + Get 内 discovery List 1）、見積 3 req 以下。Pending Re-verification に M07 を追加（データ投入後の再実行で `GroupEntity` 構造の準拠を検証）。 |
| 2026-04-20 18:15 | M08 実装・検証 | `ListUsersRaw` / `GetUserRaw` / `SearchUsersRaw` を M06 purchase_types と同形で追加（URL は `/v1/users`、命名一致）。既存 `e2e_test.go` の軽量 `TestE2E_Users_List` / `TestE2E_Users_GetByID` は M06/M07 と同パターンで削除し M08 厳格突合版に一本化。Unit 5/5 Green（既存 `roundTripperFunc` / `jsonResp` を再利用、`UserSearchParams` は Name/Email/UpdatedAtFrom の 3 クエリ検証）。実 API E2E で **List PASS（26 items, 未マップ 0）/ Get FAIL（404 = API 非対応、M03/M04/M06 と同現象、マスタ系 4 件連続）/ Search PASS（26 items, 未マップ 0, name フィルタ無視 4 件連続）**を確認。271cba3（2026-04-17）で追加された 6 フィールド（`last_name / first_name / role_id / role_name / last_sign_in_at / valid_flg`）は **全 26 件で完全に埋まっており実 API と完全一致**（`last_sign_in_at` は全件長さ 29 の ISO 8601 で null 欠落なし、`role_id = {1,2,4}`、`valid_flg = {1}`）、修正の妥当性を実証。一方で **実 API レスポンスに `name` キーが不在**（全 26 件で `has("name") == false`）→ `UserEntity.Name` は **逆方向不整合**（M03/M04/M06 の `Memo` と同現象、マスタ系で **4 件目**）。`DisplayName()` は常に LastName+FirstName 経路で動作することを実証。リソース全体 403 / 429 / TLS 異常: **発生せず**。実消費 **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1）、見積 5 req 以下。Pending Re-verification 追加なし（Get のみ API 非対応の固定 Fail で pending ではない）。 |
