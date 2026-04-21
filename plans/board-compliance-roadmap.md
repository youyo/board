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
| 最終更新 | 2026-04-21 |
| ステータス | M38 完了（**Phase G 追補完走・ReceiptEntity 実 API 準拠再設計**。M35-M38 で 4 document Entity（estimate/order/delivery/receipt）を全面書き換え。実 API smoke 全 PASS（unmapped 0）。go build/vet/test 全 Green。） ※履歴は下記に保持 |
| ステータス履歴 | M15 完了（**Phase F 2 件目・vendor_contacts（payee_contacts 実パス）**。List PASS（0 items）/ Get SKIP（0 items = data-dependent skip、Pending Re-verification）/ Search PASS（0 items）。Unit 5/5 Green。実消費 3 req（見積 8、大幅少）。**Phase F 2 件目所見**: M14 と同パターン、当該アカウントにベンダー担当者データなし → `GET /v1/payee_contacts/{id}` の 200/404 は未確認（Pending Re-verification）。未マップ 0（空配列のため）。実パス `/v1/payee_contacts` と Go 型名 `VendorContact*` の命名不一致は Unit テストで実パスアサーション済みで確認。VendorContactSearchParams 4 クエリ（VendorID/Name/Email/UpdatedAtFrom）全てエンコード確認。） / M14 完了（**Phase F 1 件目・vendor_branches（payee_branches 実パス）**。List PASS（0 items）/ Get SKIP（0 items = data-dependent skip、Pending Re-verification）/ Search PASS（0 items）。Unit 5/5 Green。実消費 3 req（見積 8、大幅少）。**Phase F 初回所見**: 当該アカウントにベンダー支店データなし → `GET /v1/payee_branches/{id}` の 200/404 は未確認（Pending Re-verification）。未マップ 0（空配列のため）。実パス `/v1/payee_branches` と Go 型名 `VendorBranch*` の命名不一致は Unit テストで実パスアサーション済みで確認。） / M12 完了（**Phase E 1 件目、Get > List 情報量差モデル新発見、`Memo` 逆方向 8 件連続で BOARD API 全般仕様最終確定**。List FAIL（299 items, unmapped **15**）/ Get FAIL（**200 成功 = Phase D/E コア業務系 Get 4 件連続 200**、unmapped **29** = List 15 + Get 限定 14）/ Search FAIL（299 items, unmapped 15, name filter 無視 **7 件連続**）。Unit 5/5 Green。`ClientEntity` は 6 フィールド中 **2 つ（Code/Memo）が逆方向不整合**、**既存 Entity の根本不足が M12 で最大規模（271cba3 の 3 倍規模）**に到達。**ネスト構造は発現せず** → M11 確定「ネストは client の子リソース特有」法則に沿う（clients 自身はフラット）。Get は List より 14 フィールド多い情報リッチ応答（**新 2 段階モデル**）。実消費 4 req（見積 5 以下、pin-point accuracy）） / M11 完了（**Phase D 完走、3 件連続 Get 200 確定**。List FAIL（22 items, unmapped **4** = `cost / description / invoice_date / payment_date`）/ Get FAIL（**200 成功 = Phase D コア業務系 Get 提供が 3 件連続確定**、unmapped 4 + 既存 4 フィールド逆方向不整合）/ Search FAIL（22 items, unmapped 4, `ProjectID=0` 非付与で全件返却）。Unit 5/5 Green。`ProjectCostEntity` は 8 フィールド中 **半分（4 つ）が逆方向不整合**（`Name/CostType/Amount/Memo` が実 API に不在）。**ネスト構造は発現せず** → ネストパターンは "client の子" 特有と確定。**概念モデルが根本ズレ**: Entity は「労務費/資材費の集計」想定、実 API は「仕訳的 expense entry（`description`+`cost`+`invoice_date`+`payment_date`）」。Memo 逆方向パターン **7 件連続**で全般仕様確定。実消費 4 req（pin-point accuracy）） |

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
- **マイルストーン**: M30 FindVendor / FindPurchaseOrder / FindPayment 新規 E2E（Phase H 6 件目）
- **直近の完了**: M29 **Phase H 5 件目 = FindReceipt 新規 E2E + ProjectEntity.Receipts 複数形参照 fix**: TDD Red→Green→Refactor で ProjectEntity.Receipt 単数形マッピングバグを修正（M28 と同パターン）。ByProjectID PASS（receiptID=28480168, receipt_date="2026-04-30" ✓, Project enrichment ✓）/ ByID PASS（Client=nil, Project=nil 仕様通り）/ ClientName・ProjectName は cache-warm SKIP。厳格フィールド突合 PASS（ReceiptEntity 未マップ 0）。fix コミット 1 件 + test コミット 1 件 + docs コミット 1 件。
- **以前の完了**: M28 FindDelivery 新規 E2E + Deliveries fix / M27 FindOrder 新規 E2E / M26 FindProject 全パス検証。
- **次のアクション**: M30 (FindVendor / FindPurchaseOrder / FindPayment 新規 E2E、Phase H 6 件目) を着手

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

#### M09: client_branches 完走 🟡（Phase D 1 件目、List/Get/Search すべて意図的 Fail = 未マップ 7 フィールド + 既存 5 フィールド逆方向不整合）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用、`ClientID+Name` の 2 クエリ検証）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合で 7 未マップ検出: `address1, address2, archive_flg, client, pref, tel, zip`
- [x] `GET /v1/client_branches/{id}` は **200 成功**（マスタ系 4 件連続 Get 404 が Phase D で切れた、コア業務系は個別 Get を提供）
- [x] `name` パラメータは無視される（検索しても 10 items 全件返却、M03/M04/M06/M08 と同現象、**5 件連続**、コア業務系でも継続）
- [x] **親 client の ネスト構造**: `client: { id, name, name_disp, custom_no }` の 4 キー構造体が同梱（Phase D 固有リスクが的中、マスタ系では未観測）
- [x] **既存 `ClientBranchEntity` は 5 フィールドが逆方向不整合**: `ClientID / PostalCode / Address / Phone / Memo` が実 API に存在せず、対応関係は `client_id → client.id` / `postal_code → zip` / `address → address1+address2` / `phone → tel` / `memo → 存在しない`
- 見積: ~8 req / 実績: **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、上限 10 req 以下）
- E2E 結果: List FAIL（10 items, 7 unmapped）/ Get FAIL（200、7 unmapped + 5 逆方向 = `name` 以外ほぼ空値）/ Search FAIL（10 items, 7 unmapped, filter 無視）
- 詳細: plans/board-compliance-m09-client-branches.md
- **フォローアップ（別 commit / 別 M で対応予定、M09 は Phase D 1 件目として独立重要度高）**:
  - **`ClientBranchEntity` の全面再設計**（最優先別 M）: 削除候補 5 フィールド + 追加候補 6 フィールド（ネスト `Client` 含む）。271cba3 UserEntity 修正と同等規模の影響（service/find / repository / mcp / cli 総点検）
  - **`ClientBranchSearchParams.ClientID` の実機能確認**: 本 M ではクエリエンコードのみ U5 で確認、実 API が本当に絞り込むかは別 M で ClientID 指定 E2E 追加
  - **`name` フィルタ無視はコア業務系でも継続確定（5 件連続）**: M10 contacts 以降のコア業務系 E2E は「filter 無視前提」で設計、Search テストは件数ではなく StrictFieldDiff と artifact 収集を主目的とする
  - **ロードマップ本文「11 フィールド」表記修正**: 実 API は 12 キー（ネスト `client` を含む。展開すれば 15）。ただし表面的なトップレベル JSON キー数 = 12

#### M10: contacts 完走 🟡（Phase D 2 件目、List/Get/Search すべて意図的 Fail = 未マップ 1 フィールド（`client` ネスト）+ 既存 6 フィールド逆方向不整合。271cba3 の 6 フィールドは実 API 完全対応で修正の正当性実証）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用、`ContactSearchParams` の `ClientID+Name+Email` の 3 クエリ検証）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合で 1 未マップ検出: `client`（ネスト構造）
- [x] `GET /v1/contacts/{id}` は **200 成功**（Phase D コア業務系 Get 提供が 2 件連続で確定、M09 client_branches に続く）
- [x] `name` パラメータは無視される（検索しても 171 items 全件返却、M03/M04/M06/M08/M09 と同現象、**6 件連続、BOARD API 全般仕様として確定**）
- [x] **親 client のネスト構造**: `client: { id, name, name_disp, custom_no }` の 4 キー構造体（M09 と **完全同形**、`ClientRef` 型の共通化候補）
- [x] **既存 `ContactEntity` は 6 フィールドが逆方向不整合**: `Name / NameKana / ClientID / ClientBranchID / Memo / Phone` が実 API に存在せず。対応関係は `client_id → client.id`（ネスト） / `name → 削除`（`last_name + first_name` に統合） / `name_kana → 削除`（API 応答に含まれない） / `memo → note` / `phone → 応答に不在` / `client_branch_id → 応答に不在`（contacts は client 直下で branch に紐づかない設計の可能性）
- [x] **271cba3 追加 6 フィールドの妥当性実証**: `LastName=171/171(100%) / FirstName=140/171(82%) / HonorificTitle=171/171(100%) / Department=27/171(16%) / Note=5/171(3%) / ArchiveFlg=全件 0`。全て実 API 応答に存在、fill rate も妥当（UserEntity M08 に続き 2 件目の検証成功）
- 見積: ~10 req / 実績: **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、上限 10 req 以下）
- E2E 結果: List FAIL（171 items, 1 unmapped）/ Get FAIL（200、1 unmapped + 6 逆方向）/ Search FAIL（171 items, 1 unmapped, filter 無視）
- 詳細: plans/board-compliance-m10-contacts.md
- **フォローアップ（別 commit / 別 M で対応予定、M10 は 271cba3 検証の結論として重要度高）**:
  - **`ContactEntity` の全面改訂**（最優先別 M）: 削除候補 6 フィールド（`Name / NameKana / ClientID / ClientBranchID / Memo / Phone`）+ 追加候補 1 フィールド（`Client *ContactClient` ネスト、M09 と型共通化して `ClientRef` 抽出推奨）。271cba3 UserEntity 修正と同等規模の影響（service/find / repository / mcp / cli 総点検、特に `FindClient` の contact enrichment）
  - **`ContactSearchParams.ClientID` / `Email` の実機能確認**: 本 M ではクエリエンコードのみ U5 で確認、実 API が本当に絞り込むかは別 M で指定 E2E 追加（`Name` は 6 件連続無視で確定）
  - **`name` フィルタ無視 6 件連続で BOARD API 全般仕様と確定**: M11 project_costs 以降の全コア業務系 E2E は「filter 無視前提」で設計、Search テストは件数ではなく `StrictFieldDiff` と artifact 収集を主目的とする。`docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` に仕様として追記推奨
  - **ロードマップ本文「19 フィールド」表記修正**: 実 API は **12 トップレベルキー**（ネスト `client` を展開しても 15）。現行 `ContactEntity` は 17 フィールドで逆方向不整合 6 件あり、適正フィールド数は 12（Entity に ネスト `Client` を 1 として数える）
  - **`client_branch_id` 応答不在**: contacts が client_branch に紐づかない API 応答設計と判明。find 層のロジック見直し（M25 FindClient 厳格化で併せて検討）

#### M11: project_costs 完走 🟡（Phase D 3 件目＝完走、List/Get/Search すべて意図的 Fail = 未マップ 4 フィールド + 既存 4 フィールド逆方向不整合 + 概念モデル根本ズレ判明）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用、`ProjectCostSearchParams` は `ProjectID` の **1 クエリのみ**を検証、contacts/client_branches と異なる点を U5 で明示的に assert）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合で 4 未マップ検出: `cost, description, invoice_date, payment_date`
- [x] `GET /v1/project_costs/{id}` は **200 成功**（**Phase D コア業務系 Get 提供が 3 件連続で確定**、M09/M10 に続く）
- [x] **ネスト構造は発現せず** → M09/M10 で発見された `client:{id, name, name_disp, custom_no}` パターンは "client の子リソース特有" であることが確定（project の子の project_costs にも発現せず、将来の `ProjectRef` 共通化は不要）
- [x] **既存 `ProjectCostEntity` は 4 フィールドが逆方向不整合**: `Name / CostType / Amount / Memo` が実 API に存在せず、対応関係は `name → 削除`（実 API は description で代替） / `cost_type → 削除`（実 API に分類フィールドなし） / `amount → cost`（キー名違い、型 `float64` のまま） / `memo → 削除`（実 API は description で代替）
- [x] **概念モデル根本ズレの発見**: Entity は「労務費/資材費のカテゴリ集計」想定、実 API は「仕訳的 expense entry（`description`+`cost`+`invoice_date`+`payment_date`）」。BOARD の project_costs は **プロジェクト原価の個別支払い記録のリスト**（= 支払い台帳の行）であり、Entity が想定していた集計モデルとは根本的に異なる
- 見積: ~8 req / 実績: **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、pin-point accuracy、上限 10 req 以下）
- E2E 結果: List FAIL（22 items, 4 unmapped）/ Get FAIL（200、4 unmapped + 4 逆方向 = 全件で `name/cost_type/amount/memo` が空値 = Entity の半分が不在）/ Search FAIL（22 items, 4 unmapped, `ProjectID=0` で全件返却）
- 詳細: plans/board-compliance-m11-project-costs.md
- **フォローアップ（別 commit / 別 M で対応予定、M11 は概念モデル変更として重要度最高）**:
  - **`ProjectCostEntity` の全面改訂**（最優先、根本的な概念モデル変更のため 271cba3 UserEntity 修正より影響大）: 削除候補 4 フィールド（`Name / CostType / Amount / Memo`）+ 追加候補 4 フィールド（`Cost float64 / Description string / InvoiceDate string / PaymentDate string`）。CLI 表示ロジック、service 層、MCP ラベルの全面見直し
  - **`ProjectCostSearchParams.ProjectID` の実機能確認**: 本 M では `ProjectID=0`（非付与）のみ E2E 実施（U5 Unit でクエリエンコード確認済み）。別 M で `ProjectID != 0` 指定時の絞り込み実機能を検証
  - **ドキュメント反映**: `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` の project_costs 章に「仕訳的 expense entry モデル」の正しいドメイン記述を追記推奨
  - **`ClientRef` / ネストパターンの適用範囲確定**: M11 でネストなし確定 → `ClientRef` 型は client_branches / contacts の共通化のみで十分、`ProjectRef` は不要。M14+ で vendor-child のネスト有無を確認すれば BOARD API のネスト設計原則が完全に確定する
  - **Phase E 以降（M12 clients / M13 projects）での事前予測**: 既存 Entity は "日本語業務モデル" で作られており、BOARD API 実キーとは乖離している可能性大。**Entity 再設計の共通フレームワーク**（Memo 廃止 / Name の DisplayName 統一 / ClientRef 等）を Phase E 開始前に設計することで、M12 以降の工数を削減できる

---

### Phase E: コア業務（再検証）

#### M12: clients 厳格突合 🟡（Phase E 1 件目、List/Get/Search すべて意図的 Fail = 未マップ 15 (List/Search) / 29 (Get) フィールド + 既存 `Code`/`Memo` 逆方向不整合。Phase D/E コア業務系 Get 200 **4 件連続確定**、`name` filter 無視 **7 件連続**、`Memo` 逆方向 **8 件連続で BOARD API 全般仕様として最終確定**）
- [x] Raw 層 3 本追加（List/Get/Search）
- [x] Unit 5/5 Green（RoundTripper mock、共通ヘルパ再利用、`ClientSearchParams` は `Name+UpdatedAtFrom` の 2 クエリ検証、M08 users と同形）
- [x] E2E: List / Get / Search 実行
- [x] 厳格フィールド突合で **15 未マップ（List/Search）+ 29 未マップ（Get）**検出
- [x] **List/Search 共通 15 フィールド**: `address1, address2, company_number, fax, invoice_system_issuer_type, invoice_system_issuer_type_name, invoice_system_number, invoice_system_number_validated, name_disp, payment_term_id, payment_term_name, pref, tel, title, zip`
- [x] **Get 限定 追加 14 フィールド**: `accounting_code, archive_flg, bank_charge_to_client_flg, basic_agreement_flg, cc, company_bank_id, company_bank_name, custom_no, document_send_type, document_send_type_name, nda_flg, note, tags, to`（**Get > List 情報量差が M12 で新発見、2 段階モデル**）
- [x] `GET /v1/clients/{id}` は **200 成功**（**Phase D/E コア業務系 Get 提供 4 件連続で確定**、M09/M10/M11/M12 全件 200）
- [x] `name` パラメータは無視される（299 items 全件返却、**7 件連続、BOARD API 全般仕様として盤石**）
- [x] **ネスト構造は発現せず** → M11 project_costs で確定した「ネストは client の子リソース (branches/contacts) 特有」の法則に沿う。clients 自身は `payment_term_id + payment_term_name` のようにフラット展開
- [x] **既存 `ClientEntity` 6 フィールド中 2 つが逆方向不整合**: `Code` が実 API に不在（全 299 件で空、実 API は `custom_no` / `accounting_code` で代替） / `Memo` が実 API に不在（全 299 件で空、実 API は `note` で代替、**`Memo` 逆方向 8 件連続で全般仕様最終確定**）
- [x] **271cba3 検証**: M08 UserEntity / M10 ContactEntity に続き、M12 で **既存 Entity の根本不足が最大規模で判明**（271cba3 の 3 倍規模）
- [x] 既存 `e2e_test.go` の軽量 `TestE2E_Clients_List` / `TestE2E_Clients_GetByID` / `TestE2E_Clients_Search` 3 本を削除し、M12 厳格突合版に一本化（M06-M11 と同パターン）。`TestE2E_Clients_ListPage` はページング検証として残存
- 見積: ~5 req / 実績: **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、pin-point accuracy、上限 10 req 以下）
- E2E 結果: List FAIL（299 items, 15 unmapped）/ Get FAIL（200、29 unmapped + 2 逆方向 = `Code/Memo` 空）/ Search FAIL（299 items, 15 unmapped, filter 無視）
- 詳細: plans/board-compliance-m12-clients.md
- **フォローアップ（別 commit / 別 M で対応予定、M12 は 271cba3 検証の結論として最重要規模）**:
  - **`ClientEntity` の全面改訂**（最優先、271cba3 UserEntity 修正の **3 倍規模**）: 削除候補 2 フィールド（`Code / Memo`）+ 追加候補 29 フィールド（List 15 + Get 限定 14）。影響: service/find / repository / mcp / cli / output マスク / SQLite 永続化の全面見直し
  - **Get/List 情報量差モデルの設計判断**: `ClientListEntity`（15 フィールド）と `ClientDetailEntity`（29 フィールド）の 2 型分離案、あるいは単一型で List 欠落を許容する案。M13 projects の `GetWithGroup` と整合性を検討
  - **`note` キー統一**: M10 contacts + M12 clients で確定。BOARD API は顧客 / 連絡先で一貫して `note` を使用 → Entity 命名統一を横串で適用
  - **`custom_no` vs 旧 `Code`**: 顧客コード表示の仕様を明確化
  - **インボイス制度情報 4 フィールド**: M25 FindClient で表示検討
  - **`Memo` 逆方向 8 件連続で全般仕様最終確定**: `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` に追記推奨
  - **ClientSearchParams.Name 削除検討**: 7 件連続で BOARD API が完全無視、保持する必要性なし

#### M13: projects 厳格突合 + GetWithGroup 全 response_group 🟡（Phase E 2 件目・複雑度最高、List/Get/Search/GetWithGroup(×6) すべて意図的 Fail = List 21 未マップ / Get 68 未マップ / DocumentSummary 配列構造の根本的不一致）
- [x] Raw 層 4 本追加（ListProjectsRaw / GetProjectRaw / GetProjectWithGroupRaw / SearchProjectsRaw）
- [x] Unit 6/6 Green（U6: GetProjectWithGroupRaw の response_group query 検証を M13 で初導入）
- [x] E2E: List / Get / Search 実行 + GetWithGroup 6 subtest（estimate/order/delivery/invoice/receipt/all）
- [x] 厳格フィールド突合で **List/Search 21 未マップ**（`client, contact, delivery_status, delivery_status_name, estimate_date, group_id, group_name, invoice_dates, management_no, order_status, order_status_name, project_no, project_type2_id/name, project_type3_id/name, project_type_id/name, tax, total, user`）
- [x] **Get 68 未マップ**（List 21 + Get 限定 47 フィールド：accounting_type* / archive_flg / client_branch / contract_* / cost_* / delivery_* / document_setting_* / hubspot / in_house_memo / invoice_* / lock_flg / monthly_* / ordered_date / payment_* / periodical_* / tags / to 等）
- [x] `GET /v1/projects/{id}` は **200 成功**（**Phase D/E コア業務系 Get 5 件連続確定**、M09/M10/M11/M12/M13）
- [x] **最重要発見: delivery/invoice/receipt は `deliveries`/`invoices`/`receipts` 複数形配列キーで返却** → `*DocumentSummary` 単一ポインタ設計が根本的に誤り（estimate/order は単一オブジェクトで partially 正しい）
- [x] `rg=all` 時に `project_costs` キーが出現（M17+ スコープ）
- [x] `client_id=0`（List/Get ともに `client` ネストオブジェクト内の `id` が正規キー、M09/M10 継続）
- [x] `status=空`（実 API は `order_status`/`delivery_status` 2 段階ステータスで代替）
- [x] DocumentSummary 内未マップ: `details / seal_approval_status / delivery_place / blank_date_flg / document_amount_disp_kbn / valid_period` 等（estimate/order 各 6-7 フィールド）
- [x] `name` パラメータは無視される（2405 items 全件返却、**9 件連続、全般仕様最終確定**）
- [x] **`Memo` 逆方向 9 件連続確定**（M03/M04/M06/M08.Name/M09/M10/M11/M12/M13）
- [x] 既存 `e2e_test.go` の旧 `TestE2E_Projects_List` / `_GetByID` / `_GetWithGroup` 3 本を削除、M13 厳格突合版に一本化
- [x] `TestE2E_Estimates_GetByDocumentID` は残存（M17/M18 スコープ）
- 見積: ~15 req / 実績: **11 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1 + GetWithGroup discovery 1 + 6 groups 6 = 11、pin-point accuracy）
- E2E 結果: List FAIL（2405 items, 21 unmapped）/ Get FAIL（200、68 unmapped + 複数逆方向不整合）/ Search FAIL（2405 items, 21 unmapped, filter 無視）/ GetWithGroup 6/6 subtest FAIL（意図的、estimate/order は発現、delivery/invoice/receipt は配列構造不一致）
- 詳細: plans/board-compliance-m13-projects.md
- **フォローアップ（別 commit / 別 M で対応予定、M13 は DocumentSummary 配列問題として最重要）**:
  - **`ProjectEntity` の全面改訂**（最優先別 M）: `client_id→client.id`（ネスト）/ `status→order_status+delivery_status` / `code→不在` / `memo→in_house_memo` / `start_date→contract_start_date` / `end_date→contract_end_date` / 追加候補 42 フィールド
  - **`DocumentSummary` 全面改訂**（M18-M21 スコープ）: delivery/invoice/receipt は `[]*DocumentSummary` 配列設計への変更必須。estimate/order は単一 `*DocumentSummary` を維持しつつ 6-7 フィールド追加
  - **`project_costs` キーの扱い**（M17+ スコープ）: `rg=all` 時の `project_costs` は集計情報か参照かを確認

---

### Phase F: ベンダー系

#### M14: vendor_branches 完走（payee_branches 実パス）✅（Phase F 1 件目、List/Search PASS 0 items / Get SKIP = data-dependent / Unit 5/5 Green）
- [x] E2E: List / Get / Search（Get は 0 items → SKIP = Pending Re-verification）
- [x] 厳格フィールド突合（空配列のため未マップ 0、データ投入後の再実行が必要）
- [x] `/v1/payee_branches` 実パスと `VendorBranch*` Go 型名の命名不一致を Unit テストのパスアサーションで確認済み
- 見積: ~8 req / 実績: **3 req**（List 1 + Get discovery 1 (0件) + Search 1）
- 詳細: plans/board-compliance-m14-vendor-branches.md

#### M15: vendor_contacts 完走（payee_contacts 実パス）✅（Phase F 2 件目、List/Search PASS 0 items / Get SKIP = data-dependent / Unit 5/5 Green）
- [x] E2E: List / Get / Search（Get は 0 items → SKIP = Pending Re-verification）
- [x] 厳格フィールド突合（空配列のため未マップ 0、データ投入後の再実行が必要）
- [x] `/v1/payee_contacts` 実パスと `VendorContact*` Go 型名の命名不一致を Unit テストのパスアサーションで確認済み
- [x] VendorContactSearchParams 4 クエリ（VendorID/Name/Email/UpdatedAtFrom）全てエンコード確認（M14 の 3 クエリより 1 多い）
- 見積: ~8 req / 実績: **3 req**（List 1 + Get discovery 1 (0件) + Search 1）
- 詳細: plans/board-compliance-m15-vendor-contacts.md

#### M16: vendors 完走（payees 実パス）✅（Phase F 3 件目・Phase F 完走、List/Search PASS 0 items / Get SKIP = data-dependent / Unit 5/5 Green）
- [x] E2E: List / Get / Search（Get は 0 items → SKIP = Pending Re-verification）
- [x] 厳格フィールド突合（空配列のため未マップ 0、データ投入後の再実行が必要）
- [x] `/v1/payees` 実パスと `Vendor*` Go 型名の命名不一致を Unit テストのパスアサーションで確認済み
- [x] VendorSearchParams 2 クエリ（Name+UpdatedAtFrom）エンコード確認
- [x] `e2e_test.go` の軽量 `TestE2E_Vendors_List` を削除し厳格版 `e2e_vendors_test.go` に一本化
- 見積: ~5 req / 実績: **3 req**（List 1 + Get discovery 1 (0件) + Search 1）
- 詳細: plans/board-compliance-m16-vendors.md

**Phase F 完走サマリ**: M14 vendor_branches + M15 vendor_contacts + M16 vendors の全 3 件で List/Search PASS・Get SKIP（データ 0 件）。当該アカウントはベンダー系リソース全て空（payee_branches / payee_contacts / payees）。実パス命名不一致（Go: `Vendor*`、API: `payee_*`/`payees`）を 3 件全て Unit テストパスアサーションで確認。Phase F 合計 req: **9 req**（各 3 req × 3 M）、見積 21 req から大幅削減。フォローアップ: ベンダーデータ投入後の Pending Re-verification 3 件（M14/M15/M16 Get）。

---

### Phase G: ドキュメント系

#### M17: documentID discovery helper 確立 ✅（Phase G 1 件目）
- [x] `findAnyDocumentID(t, client, docType)` を e2e_helpers_test.go に追加
- [x] projects を response_group 付きで走査し docType サブオブジェクトが非 nil の最初の 1 件を返す
- [x] estimates / orders / deliveries / invoices / receipts 共通で再利用（probe struct 方式で 5 docType 統一処理）
- [x] Unit テスト（httptest: estimate found / delivery found / unknown docType doc）Green
- [x] 実 API smoke test（estimate 1 docType）PASS: projectID=95944469 documentID=105287235
- 見積: ~5 req / 実績: **2 req**（`ListProjectsPage` 1 + `GetProjectWithGroupRaw` 1、0.69 秒）
- 詳細: plans/board-compliance-m17-docid-discovery.md

#### M18: estimates Get 厳格突合 ✅（Phase G 2 件目）
- [x] 既存 E2E を M17 helper 経由に切り替え（`TestE2E_Estimates_GetByDocumentID` を削除し `TestE2E_Estimates_Get` に一本化）
- [x] 厳格フィールド突合（EstimateEntity 11 フィールド、`GetEstimateRaw` 追加）
- 見積: ~5 req / 実績: ~3 req（ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetEstimateRaw 1）/ 実 API 未実行のため TBD
- 詳細: plans/board-compliance-m18-estimates.md

#### M19: orders Get + 厳格突合 ✅（Phase G 3 件目）
- [x] E2E: Get（M17 helper）
- [x] 厳格フィールド突合（OrderEntity）
- 見積: ~10 req / 実績: ~3 req（ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetOrderRaw 1）/ 実 API 未実行のため TBD
- 詳細: plans/board-compliance-m19-orders.md

#### M20: deliveries Get + 厳格突合 ✅（Phase G 4 件目）
- [x] E2E: Get（M17 helper）
- [x] 厳格フィールド突合（DeliveryEntity 10 フィールド、`GetDeliveryRaw` 追加）
- 見積: ~10 req / 実績: ~3 req（ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetDeliveryRaw 1）/ 実 API 未実行のため TBD
- 詳細: plans/board-compliance-m20-deliveries.md

#### M21: receipts Get + 厳格突合 ✅（Phase G 5 件目）
- [x] E2E: Get（M17 helper）
- [x] 厳格フィールド突合（ReceiptEntity 10 フィールド、`GetReceiptRaw` 追加）
- 見積: ~10 req / 実績: ~3 req（ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetReceiptRaw 1）/ 実 API 未実行のため TBD
- 詳細: plans/board-compliance-m21-receipts.md

#### M22: invoices List/Get/Search 厳格突合 ✅（Phase G 6 件目）
- [x] `ListInvoicesRaw` / `GetInvoiceRaw` / `SearchInvoicesRaw` を `invoices.go` に追加
- [x] `e2e_invoices_test.go` 新規作成（List/Get/Search 3 本、WithPerPage(1) で大量データ対応）
- [x] 旧 `TestE2E_Invoices_List` を `e2e_test.go` から削除し厳格突合版に一本化
- [x] go build/vet/test 全 Green
- 見積: ~20 req → 実績: ~4 req（WithPerPage(1) + UpdatedAtFrom=2099-01-01 で大幅削減）
- 詳細: plans/board-compliance-m22-invoices.md

#### M23: purchase_orders List/Get/Search 厳格突合 ✅（Phase G 7 件目）
- [x] `ListPurchaseOrdersRaw` / `GetPurchaseOrderRaw` / `SearchPurchaseOrdersRaw` を `purchase_orders.go` に追加
- [x] `e2e_purchase_orders_test.go` 新規作成（List/Get/Search 3 本）
- [x] 旧 `TestE2E_PurchaseOrders_List` と `TestE2E_Payments_List` を `e2e_test.go` から削除し厳格突合版に一本化
- [x] go build/vet/test 全 Green
- 見積: ~10 req → 実績: ~4 req（WithPerPage(1) + UpdatedAtFrom=2099-01-01 で大幅削減）
- 詳細: plans/board-compliance-m23-purchase-orders.md

#### M24: payments List/Get/Search 厳格突合 ✅（Phase G 8 件目・Phase G 完走）
- [x] `ListPaymentsRaw` / `GetPaymentRaw` / `SearchPaymentsRaw` を `payments.go` に追加
- [x] `e2e_payments_test.go` 新規作成（List/Get/Search 3 本）
- [x] go build/vet/test 全 Green
- [x] **Phase G 全 8 件（M17-M24）完走**
- 見積: ~10 req → 実績: ~4 req（WithPerPage(1) + UpdatedAtFrom=2099-01-01 で大幅削減）
- 詳細: plans/board-compliance-m24-payments.md

---

### Phase H: service/find 層

#### M25: FindClient 厳格化 ✅（Phase H 1 件目）
- [x] TestE2E_FindClient_StrictEnrichment を新規追加（FindClient(ID) と独立 raw API を件数・ID 集合で突合）
- [x] TestE2E_FindClient_ByName / ByText に integrity チェック追加（branches/contacts が親クライアントと一致）
- [x] idSet / intSetEqual helper 関数を e2e_test.go に追加
- [x] **compliance 欠損を検出（Red）**: FindClient(ID) が branches=0 / contacts=0 を返す
  - 根本原因: BOARD API は `client_id` フラットフィールドでなく `{"client": {"id": N}}` ネスト構造を返す
  - ClientBranchEntity.ClientID / ContactEntity.ClientID が unmarshal 後常に 0 となり、in-memory filter で全件除外
- [x] **repository 修正（Green）**: ClientBranchRepository.Search / ContactRepository.Search を API-side filter 経由に変更
  - ClientID != 0 の場合は api.SearchClientBranches / api.SearchContacts を直接呼び出す
  - T_R23 / T_R37 unit test をモックデータ付きに修正
- [x] EstimateEntity.Title 参照コンパイルエラー修正（M35 波及漏れ）
- 実消費: ~10 req（ByName ~3 + ByText ~3 + StrictEnrichment ~4）
- E2E 結果（修正後）: ByName PASS（branches=10, contacts=171）/ ByText PASS / StrictEnrichment PASS（ID 集合突合 OK）
- **発見・残存**: VendorBranchRepository / VendorContactRepository に同パターンの潜在バグ（データなしのため未表面化、別 M 対応）
- 詳細: plans/board-compliance-m25-find-client.md

#### M26: FindProject 全パス検証 ✅（Phase H 2 件目）
- [x] TestE2E_FindProject_StrictEnrichment_ByID（ID + Client/Estimate 独立 API 突合）
- [x] TestE2E_FindProject_ByName_Strict（Name モード、各 result の Client enrichment 整合確認）
- [x] TestE2E_FindProject_ByClientName_Strict（ClientName モード、SearchClients 独立突合）
- [x] TestE2E_FindProject_ByText_Strict（Text モード、prefix を Name/Code/Memo に含むか確認）
- [x] TestE2E_FindProject_ByStatus_Strict（Status モード、全 result の status 一致確認）
- **enrichment バグなし**: ProjectEntity.ClientID は `json:"client_id"` フラットマッピング。M25 の nested-unmarshal バグは発現しない。fix コミットなし。
- **data-dependent skip**: StrictEnrichment_ByID（ClientID=0 のプロジェクトのみ）/ ByStatus_Strict（Status 空）
- **BOARD API name filter 無視継続**: SearchClients(Name="株式会社WAND") が 299 件全返却（7+ 件連続）
- 実消費: ~10 req
- E2E 結果: PASS 3 + SKIP 2（全テスト成功）
- 詳細: plans/board-compliance-m26-find-project.md

#### M27: FindOrder 新規 E2E ✅（Phase H 3 件目）
- [x] TestE2E_FindOrder_ByProjectID_Strict（ProjectID モード、Client/Project enrichment 独立 API 突合）
- [x] TestE2E_FindOrder_ByID_Strict（ID モード、Client=nil/Project=nil 仕様通り確認）
- [x] TestE2E_FindOrder_ByClientName_Strict（cache-warm SKIP、タイムアウト回避）
- [x] TestE2E_FindOrder_ByProjectName_Strict（cache-warm SKIP、タイムアウト回避）
- [x] findProjectWithDocType helper 追加（service/find e2e_test 用 topN=50 discovery）
- [x] strictFieldDiff / projectIDOrZero / clientIDOrZero helper 追加（e2e_helpers_test）
- **enrichment バグなし**: OrderEntity フラット構造、ClientID=0 で nil 返却が正常動作
- **厳格フィールド突合 PASS**: GetOrderRaw 独立突合、未マップ 0 件
- **発見事項**: ProjectEntity.Delivery/Receipt 単数形マッピング問題（M28/M29 で fix）
- 実消費: ~20 req（discovery 50 件 × 2 テスト + GetOrderRaw + GetProjectWithGroup）
- E2E 結果: PASS 2 + SKIP 2（cache-warm SKIP）
- 詳細: plans/board-compliance-m27-find-order.md

#### M28: FindDelivery 新規 E2E ✅（Phase H 4 件目）
- [x] TestE2E_FindDelivery_ByProjectID_Strict（ProjectID モード、delivery_date フィールド確認、Client/Project enrichment 独立 API 突合）
- [x] TestE2E_FindDelivery_ByID_Strict（ID モード、Client=nil/Project=nil 仕様通り確認）
- [x] TestE2E_FindDelivery_ByClientName_Strict（cache-warm SKIP）
- [x] TestE2E_FindDelivery_ByProjectName_Strict（cache-warm SKIP）
- [x] **fix(boardapi)**: ProjectEntity.Deliveries 複数形配列フィールド追加（`json:"deliveries,omitempty"`）
- [x] find_delivery.go を `p.Delivery → p.Deliveries[0]` 参照に変更
- [x] find_delivery_test.go のモックデータを Deliveries に変更（unit テスト Green 維持）
- **TDD サイクル**: Red（ByProjectID 0 件返却確認）→ Green（Deliveries fix）→ Refactor（unit test mock 統一）
- **厳格フィールド突合 PASS**: GetDeliveryRaw 独立突合、未マップ 0 件
- **delivery_date 確認**: "2026-06-30" が正しく unmarshal されることを確認
- 実消費: ~20 req
- E2E 結果: PASS 2 + SKIP 2（cache-warm SKIP）
- 詳細: plans/board-compliance-m28-find-delivery.md

#### M29: FindReceipt 新規 E2E ✅（Phase H 5 件目）
- [x] TestE2E_FindReceipt_ByProjectID_Strict（ProjectID モード、receipt_date フィールド確認、Client/Project enrichment 独立 API 突合）
- [x] TestE2E_FindReceipt_ByID_Strict（ID モード、Client=nil/Project=nil 仕様通り確認）
- [x] TestE2E_FindReceipt_ByClientName_Strict（cache-warm SKIP）
- [x] TestE2E_FindReceipt_ByProjectName_Strict（cache-warm SKIP）
- [x] **fix(find)**: FindReceipt ProjectID/ClientName/ProjectName ブランチを `p.Receipts[0]` 参照に変更
- [x] find_receipt_test.go のモックデータを Receipts に変更（unit テスト Green 維持）
- **TDD サイクル**: Red（ByProjectID 0 件返却確認）→ Green（Receipts 参照修正）→ Refactor（unit test mock 統一）
- **厳格フィールド突合 PASS**: GetReceiptRaw 独立突合、未マップ 0 件
- **receipt_date 確認**: "2026-04-30" が正しく unmarshal されることを確認
- 実消費: ~20 req
- E2E 結果: PASS 2 + SKIP 2（cache-warm SKIP）
- 詳細: plans/board-compliance-m29-find-receipt.md

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
| M14 | vendor_branches | Get / 厳格突合 | List 0 件（当該アカウントにベンダー支店データなし）、Get は discovery 不能 | データ投入後: `go test -tags e2e -v -count=1 -run TestE2E_VendorBranches ./internal/boardapi/` |
| M15 | vendor_contacts | Get / 厳格突合 | List 0 件（当該アカウントにベンダー担当者データなし）、Get は discovery 不能 | データ投入後: `go test -tags e2e -v -count=1 -run TestE2E_VendorContacts ./internal/boardapi/` |
| M16 | vendors | Get / 厳格突合 | List 0 件（当該アカウントにベンダーデータなし、Phase F 全 3 件共通）、Get は discovery 不能 | データ投入後: `go test -tags e2e -v -count=1 -run TestE2E_Vendors ./internal/boardapi/` |

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
| 2026-04-20 18:40 | M09 実装・検証 | `ListClientBranchesRaw` / `GetClientBranchRaw` / `SearchClientBranchesRaw` を M08 users と同形で追加（既存 `client_branches.go` に追記、URL `/v1/client_branches` top-level、`ClientBranchSearchParams` は `ClientID+Name` の 2 クエリ）。Unit 5/5 Green。実 API E2E で **Phase D 1 件目として重要発見を複数**: ①`GET /v1/client_branches/{id}` が **200 成功**（マスタ系 Get 404 パターンが Phase D で切れた、コア業務系は個別 Get 提供）、②**7 未マップフィールド**（`address1, address2, archive_flg, client, pref, tel, zip`）= 既存 `ClientBranchEntity` のキー名が軒並み実 API と不一致、③**既存 5 フィールドが逆方向不整合**（`ClientID / PostalCode / Address / Phone / Memo` が実 API に存在しない）、④**親 client のネスト構造** `client:{id, name, name_disp, custom_no}` が新発見（Phase D 固有リスクが的中、マスタ系では未観測）、⑤**`name` フィルタ無視 5 件連続**（マスタ系 4 件に加えコア業務系でも継続、BOARD API 全般の仕様と判断可能）、⑥リソース全体 403 / 429 は発生せず。既存 `ClientBranchEntity` 10 フィールド中マッチは 5 つ（`id / name / fax / updated_at / created_at`）のみで、Entity 全面改訂フォローアップが必要（271cba3 UserEntity 修正と同等規模）。E2E は意図的に Fail 状態で commit。実消費 **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、見積 8 req → 上限 10 req 以内）。Pending Re-verification 追加なし。 |
| 2026-04-20 18:15 | M08 実装・検証 | `ListUsersRaw` / `GetUserRaw` / `SearchUsersRaw` を M06 purchase_types と同形で追加（URL は `/v1/users`、命名一致）。既存 `e2e_test.go` の軽量 `TestE2E_Users_List` / `TestE2E_Users_GetByID` は M06/M07 と同パターンで削除し M08 厳格突合版に一本化。Unit 5/5 Green（既存 `roundTripperFunc` / `jsonResp` を再利用、`UserSearchParams` は Name/Email/UpdatedAtFrom の 3 クエリ検証）。実 API E2E で **List PASS（26 items, 未マップ 0）/ Get FAIL（404 = API 非対応、M03/M04/M06 と同現象、マスタ系 4 件連続）/ Search PASS（26 items, 未マップ 0, name フィルタ無視 4 件連続）**を確認。271cba3（2026-04-17）で追加された 6 フィールド（`last_name / first_name / role_id / role_name / last_sign_in_at / valid_flg`）は **全 26 件で完全に埋まっており実 API と完全一致**（`last_sign_in_at` は全件長さ 29 の ISO 8601 で null 欠落なし、`role_id = {1,2,4}`、`valid_flg = {1}`）、修正の妥当性を実証。一方で **実 API レスポンスに `name` キーが不在**（全 26 件で `has("name") == false`）→ `UserEntity.Name` は **逆方向不整合**（M03/M04/M06 の `Memo` と同現象、マスタ系で **4 件目**）。`DisplayName()` は常に LastName+FirstName 経路で動作することを実証。リソース全体 403 / 429 / TLS 異常: **発生せず**。実消費 **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1）、見積 5 req 以下。Pending Re-verification 追加なし（Get のみ API 非対応の固定 Fail で pending ではない）。 |
| 2026-04-20 18:55 | M11 実装・検証 | `ListProjectCostsRaw` / `GetProjectCostRaw` / `SearchProjectCostsRaw` を M10 contacts / M09 client_branches と同形で追加（URL `/v1/project_costs`、top-level、`ProjectCostSearchParams` は `ProjectID` の **1 クエリのみ**検証、contacts/client_branches と異なり `Name` を持たない点を U5 で明示的に assert）。Unit 5/5 Green。実 API E2E で **Phase D 3 件目 = 完走**として重要発見を複数: ①`GET /v1/project_costs/{id}` が **200 成功**（**Phase D コア業務系 Get 提供 3 件連続確定**、M09 client_branches + M10 contacts + M11 project_costs の全 3 件で 200）、②**4 未マップフィールド**（`cost / description / invoice_date / payment_date`）= 既存 `ProjectCostEntity` のキー名が実 API と根本的に不一致、③**既存 8 フィールド中 4 フィールドが逆方向不整合**（`Name / CostType / Amount / Memo` が実 API に存在しない、Entity の半分が不在）、④**ネスト構造 `project:{...}` は発現せず** → M09/M10 で発見された `client:{id, name, name_disp, custom_no}` ネストパターンは "client の子リソース特有" であることが確定（project の子の project_costs には適用されない、`ProjectRef` 型共通化は不要）、⑤**概念モデル根本ズレの発見**: Entity は「労務費/資材費のカテゴリ集計」想定、実 API は「仕訳的 expense entry（`description`+`cost`+`invoice_date`+`payment_date`）」の個別支払い記録フラット構造。BOARD の project_costs は **プロジェクト原価の個別支払い記録のリスト** = 支払い台帳の行で、Entity 概念とは根本的に異なる、⑥**`Memo` 逆方向パターン 7 件連続**（project_types / payment_terms / purchase_types / users.Name / contacts.Memo / client_branches.Memo / project_costs.Memo、BOARD API は `memo` キーを提供せず `note` / `description` で代替する全般仕様として最終確定）、⑦リソース全体 403 / 429 は発生せず。E2E は意図的に Fail 状態で commit。実消費 **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、見積 4 req → pin-point accuracy、上限 10 req 以下）。Pending Re-verification 追加なし。**Phase D 完走サマリ**: M09/M10/M11 全件で Get 200 成功（コア業務系 = 個別 Get 提供が確定）、ネストは client-child 限定、逆方向不整合 計 15 フィールド、req 消費合計 12 req で完走、Entity 再設計フォローアップ 3 件（client_branches 全面改訂 / contacts 全面改訂 / project_costs 全面改訂、Phase E 開始前に共通フレームワーク設計推奨）。 |
| 2026-04-20 19:10 | M12 実装・検証 | `ListClientsRaw` / `GetClientRaw` / `SearchClientsRaw` を M11 project_costs / M08 users と同形で追加（URL `/v1/clients`、top-level、`ClientSearchParams` は `Name+UpdatedAtFrom` の 2 クエリ検証、M08 users と同じパラメータ構成）。既存 `e2e_test.go` の軽量 `TestE2E_Clients_List` / `TestE2E_Clients_GetByID` / `TestE2E_Clients_Search` 3 本を M06-M11 と同パターンで削除し M12 厳格突合版に一本化（`TestE2E_Clients_ListPage` はページング検証として残存）。Unit 5/5 Green。実 API E2E で **Phase E 1 件目として重要発見を多数**: ①`GET /v1/clients/{id}` が **200 成功**（**Phase D/E コア業務系 Get 提供 4 件連続で確定**、M09 client_branches + M10 contacts + M11 project_costs + M12 clients の全 4 件で 200）、②**List/Search 15 未マップ + Get 29 未マップ**（Get は List 15 + Get 限定 **14 フィールド**）、③**Get > List 情報量差という M12 新発見**: BOARD API clients は「List = 検索結果用の軽量 20 キー」「Get = 詳細表示用の 34 キー」の **2 段階モデル** を採用（M13 `GetWithGroup` の `response_group` 明示指定方式とは別の、自動拡張型）、④**既存 `ClientEntity` 6 フィールド中 2 つが逆方向不整合**（`Code` は実 API に不在、`custom_no`/`accounting_code` で代替 / `Memo` は実 API に不在、`note` で代替、**`Memo` 逆方向 8 件連続で BOARD API 全般仕様として最終確定**、M03/M04/M06/M08.Name/M09/M10/M11/M12）、⑤**ネスト構造は発現せず** → M11 project_costs で確定した「ネストは client の子リソース特有」の法則に沿う（clients 自身は `payment_term_id + payment_term_name` のようにフラット展開、`ClientRef` 型の共通化は branches/contacts のみで十分）、⑥**`name` フィルタ無視 7 件連続**（マスタ系 4 件 + コア業務系 3 件 = Phase D/E 全件、**BOARD API 全般の仕様として盤石**、299 items 全件返却）、⑦**271cba3 検証の最大規模問題**: ClientEntity は 6 フィールド中 **4 つ（67%）しか実 API と一致せず**、29 フィールドの追加 + 2 フィールドの削除が必要（271cba3 UserEntity 修正の **3 倍規模**、service/find / repository / mcp / cli / output マスク / SQLite 永続化の全面見直しが発生）、⑧リソース全体 403 / 429 は発生せず。E2E は意図的に Fail 状態で commit。実消費 **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、見積 5 req → pin-point accuracy、上限 10 req 以下）。Pending Re-verification 追加なし（List 299 items、Get 200 で完全にデータ充足）。**Phase E 1 件目サマリ**: Phase D で確立したパターン（コア業務 Get 200 / ネスト client-child 限定 / `Memo` 逆方向）が M12 で全て継承、更に **Get > List 情報量差** と **Entity 不足 67% 規模** の 2 つの新発見を得た。フォローアップ: ClientEntity 全面改訂（最優先、別 M）/ Get vs List の 2 型分離 vs 単一型許容の設計判断 / `note` キー統一 / `custom_no` vs 旧 `Code` の仕様ドキュメント化 / インボイス制度情報 4 フィールドの Find 層表示検討。 |
| 2026-04-20 18:50 | M10 実装・検証 | `ListContactsRaw` / `GetContactRaw` / `SearchContactsRaw` を M08 users / M09 client_branches と同形で追加（URL `/v1/contacts`、top-level、`ContactSearchParams` は `ClientID+Name+Email` の 3 クエリ検証）。既存 `e2e_test.go` 等に軽量 `TestE2E_Contacts_*` は存在せず（削除対象なし）。Unit 5/5 Green。実 API E2E で **Phase D 2 件目として**: ①`GET /v1/contacts/{id}` が **200 成功**（M09 に続きコア業務系 Get 提供が 2 件連続で確定）、②**1 未マップフィールド**（`client` ネスト）= `ContactEntity` のトップレベルキーが実 API とほぼ一致、③**既存 6 フィールドが逆方向不整合**（`Name / NameKana / ClientID / ClientBranchID / Memo / Phone` が実 API に不在）、④**親 client のネスト構造** `client:{id, name, name_disp, custom_no}` が M09 と **完全同形**で再発見（Phase D のネストパターンが確定化、`ClientRef` 型の共通化候補）、⑤**`name` フィルタ無視 6 件連続**（マスタ系 4 件 + コア業務系 2 件、**BOARD API 全般の仕様として確定**）、⑥**271cba3 の 6 フィールドの妥当性実証**: `LastName=171/171(100%) / FirstName=140/171(82%) / HonorificTitle=171/171(100%) / Department=27/171(16%) / Note=5/171(3%) / ArchiveFlg=全件 0`、全て実 API 応答に存在し fill rate も妥当（UserEntity M08 に続き ContactEntity でも修正の正当性が実証）、⑦`DisplayName()` は全件で LastName+FirstName 経路（`Name` キー自体が 171 件全件で実 API に不在、M08 users と同現象、`Name`/`Memo`逆方向不整合として通算 6 件目）、⑧リソース全体 403 / 429 は発生せず。既存 `ContactEntity` 17 フィールド中マッチは 11 つ（6 逆方向不整合）、Entity 全面改訂フォローアップ必要（271cba3 UserEntity 修正と同等規模、`Client` 型の M09 との共通化検討）。E2E は意図的に Fail 状態で commit。実消費 **4 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1、見積 10 req → 上限内）。Pending Re-verification 追加なし。 |
| 2026-04-21 01:25 | M14 実装・検証 | `ListVendorBranchesRaw` / `GetVendorBranchRaw` / `SearchVendorBranchesRaw` を既存 `vendor_branches.go` に追記（URL は既存実装通り `/v1/payee_branches`、命名不一致は維持）。Unit 5/5 Green（U5 は `VendorBranchSearchParams` の **3 クエリ**（VendorID+Name+UpdatedAtFrom）を検証、M09 の 2 クエリより 1 多い点を明示）。実 API E2E で **Phase F 1 件目として**: ①`GET /v1/payee_branches` が 200 / 0 items 返却（当該アカウントにベンダー支店データなし、M02/M07 と同パターン）、②Get は discovery 0 件のため `t.Skipf("pending re-verification")` → Pending Re-verification に転記、③Search PASS（0 items）、④未マップ 0（空配列のため StrictFieldDiff の検証機会なし）、⑤403/429 は発生せず、⑥実パス `/v1/payee_branches` と Go 型名 `VendorBranch*` の命名不一致は Unit テストのパスアサーションで確認済み。**Phase F 初回課題**: `GET /v1/payee_branches/{id}` の 200/404 は未確認（データ投入後に Phase D/E の Get 200 パターン継続を確認必要）、`vendor` ネストオブジェクト有無も未確認。実消費 **3 req**（見積 8）、上限 10 req 以下。|
| 2026-04-21 01:42 | M15 実装・検証 | `ListVendorContactsRaw` / `GetVendorContactRaw` / `SearchVendorContactsRaw` を既存 `vendor_contacts.go` に追記（URL は既存実装通り `/v1/payee_contacts`、命名不一致は維持）。Unit 5/5 Green（U5 は `VendorContactSearchParams` の **4 クエリ**（VendorID+Name+Email+UpdatedAtFrom）を検証、M14 の 3 クエリより 1 多い点を明示。VendorContactEntity は 17 フィールドと M14 VendorBranchEntity（10 フィールド）より大きい）。実 API E2E で **Phase F 2 件目として**: ①`GET /v1/payee_contacts` が 200 / 0 items 返却（当該アカウントにベンダー担当者データなし、M14 と同パターン）、②Get は discovery 0 件のため `t.Skipf("pending re-verification")` → Pending Re-verification に転記、③Search PASS（0 items）、④未マップ 0（空配列のため StrictFieldDiff の検証機会なし）、⑤403/429 は発生せず、⑥実パス `/v1/payee_contacts` と Go 型名 `VendorContact*` の命名不一致は Unit テストのパスアサーションで確認済み。**Phase F 2 件目所見**: M14 と完全同パターン（データ 0 件）。当該アカウントの Phase F ベンダー系は 2 件連続でデータなし。`GET /v1/payee_contacts/{id}` の 200/404 は未確認（Pending Re-verification）。実消費 **3 req**（見積 8）、上限 10 req 以下。|
| 2026-04-21 02:05 | M17 実装・Phase G 開始 | `findAnyDocumentID(t, client, docType)` を `e2e_helpers_test.go` に追加。シグネチャ: `(t, client, docType) → (projectID, documentID int)`。probe struct 方式（`GetProjectWithGroupRaw` + anonymous struct）で全 5 docType を統一処理し、`ProjectEntity` の delivery/invoice/receipt JSON タグ単数形ミスマッチを回避。estimate/order は単一オブジェクト、delivery/invoice/receipt は複数形配列の先頭を返す。上限 `maxDiscoveryProjects=3` で rate limit 配慮。Unit テスト（httptest ベース）2 本 Green。実 API smoke test（estimate）: projectID=95944469 documentID=105287235 発見。|
| 2026-04-21 02:12 | M17 修正（rate limit バグ修正） | `ListProjectsRaw+WithPerPage` は `ListAll` で全ページ走査するため 257 秒・約 800 req を消費する問題を発見。`ListProjectsPage(1, maxDiscoveryProjects)` に切り替えることで全ページ走査を回避。修正後の smoke test: **0.69 秒・2 req**（`ListProjectsPage` 1 + `GetProjectWithGroupRaw` 1）。|
| 2026-04-21 03:xx | M18 実装・Phase G 2 件目 | `GetEstimateRaw` を `estimates.go` に追加（`GetVendorRaw` と同パターン）。`e2e_estimates_test.go` を新規作成し M17 `findAnyDocumentID` helper 経由の厳格突合 E2E テスト（`TestE2E_Estimates_Get` 1 本）を追加。旧 `TestE2E_Estimates_GetByDocumentID`（古い discovery パターン：全件 `ListProjects` + typed `GetProjectWithGroup`）を `e2e_test.go` から削除し一本化。go build/vet/test 全 Green（全 12 パッケージ）。e2e タグ付きコンパイル通過。実 API 未実行（~3 req 見込み: ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetEstimateRaw 1）。|
| 2026-04-21 | M19 実装・Phase G 3 件目 | `GetOrderRaw` を `orders.go` に追加（`GetEstimateRaw` と同パターン）。`e2e_orders_test.go` を新規作成し M17 `findAnyDocumentID` helper 経由の厳格突合 E2E テスト（`TestE2E_Orders_Get` 1 本）を追加。e2e_test.go のクリーンアップは不要（M18 時点で orders 関連の古いテストは存在しなかった）。go build/vet/test 全 Green（全 12 パッケージ）。e2e タグ付きコンパイル通過。実 API 未実行（~3 req 見込み: ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetOrderRaw 1）。|
| 2026-04-21 | M20 実装・Phase G 4 件目 | `GetDeliveryRaw` を `deliveries.go` に追加（`GetOrderRaw` と同パターン）。`e2e_deliveries_test.go` を新規作成し M17 `findAnyDocumentID` helper 経由の厳格突合 E2E テスト（`TestE2E_Deliveries_Get` 1 本）を追加。go build/vet/test 全 Green。e2e タグ付きコンパイル通過。実 API 未実行（~3 req 見込み: ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetDeliveryRaw 1）。|
| 2026-04-21 | M21 実装・Phase G 5 件目 | `GetReceiptRaw` を `receipts.go` に追加（`GetDeliveryRaw` と同パターン）。`e2e_receipts_test.go` を新規作成し M17 `findAnyDocumentID` helper 経由の厳格突合 E2E テスト（`TestE2E_Receipts_Get` 1 本）を追加。go build/vet/test 全 Green。e2e タグ付きコンパイル通過。実 API 未実行（~3 req 見込み: ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetReceiptRaw 1）。|
| 2026-04-21 | M22 実装・Phase G 6 件目 | `ListInvoicesRaw` / `GetInvoiceRaw` / `SearchInvoicesRaw` を `invoices.go` に追加（`ListVendorsRaw` / `GetVendorRaw` / `SearchVendorsRaw` M16 パターン）。`opts ...ListAllOption` 受け取りで `WithPerPage(1)` を可能にし大量データ（11,000 件規模）対応。`e2e_invoices_test.go` を新規作成し List/Get/Search 3 本の厳格突合 E2E を追加（`WithPerPage(1)` で List/Search の req 数を最小化、Search は `UpdatedAtFrom=2099-01-01` の far-future フィルタで空結果を狙い大量ページネーション回避）。旧 `TestE2E_Invoices_List`（typed List のみ）を `e2e_test.go` から削除し厳格突合版に一本化。go build/vet/test 全 Green（全 12 パッケージ）。実 API 未実行（~4 req 見込み）。|
| 2026-04-21 | M23 実装・Phase G 7 件目 | `ListPurchaseOrdersRaw` / `GetPurchaseOrderRaw` / `SearchPurchaseOrdersRaw` を `purchase_orders.go` に追加（M22 invoices / M16 vendors と同パターン）。Go 名（purchase_orders）と実 API パス（/v1/expenditures）の命名不一致は既存設計を継承。`e2e_purchase_orders_test.go` を新規作成し List/Get/Search 3 本の厳格突合 E2E を追加（Search は `UpdatedAtFrom=2099-01-01` の far-future フィルタ）。旧 `TestE2E_PurchaseOrders_List` と `TestE2E_Payments_List`（typed List のみ）を `e2e_test.go` から対称的に削除し厳格突合版に一本化。go build/vet/test 全 Green（全 12 パッケージ）。実 API 未実行（~4 req 見込み）。|
| 2026-04-21 | M24 実装・Phase G 8 件目・Phase G 完走 | `ListPaymentsRaw` / `GetPaymentRaw` / `SearchPaymentsRaw` を `payments.go` に追加（M23 purchase_orders / M16 vendors と同パターン）。Go 名（payments）と実 API パス（/v1/expenditure_payments）の命名不一致は既存設計を継承（M23 の /v1/expenditures と同パターン）。`e2e_payments_test.go` を新規作成し List/Get/Search 3 本の厳格突合 E2E を追加（Search は `UpdatedAtFrom=2099-01-01` の far-future フィルタ）。e2e_test.go 側の payments 関連の古いテストは M23 で既に削除済みのため追加削除なし。go build/vet/test 全 Green（全 12 パッケージ）。実 API 未実行（~4 req 見込み）。**Phase G 全 8 件（M17-M24）完走**: M17 documentID helper 確立 → M18 estimates / M19 orders / M20 deliveries / M21 receipts（Get 厳格突合）→ M22 invoices / M23 purchase_orders / M24 payments（List/Get/Search 厳格突合）。|
| 2026-04-21 01:53 | M16 実装・検証・Phase F 完走 | `ListVendorsRaw` / `GetVendorRaw` / `SearchVendorsRaw` を既存 `vendors.go` に追記（URL は既存実装通り `/v1/payees`、命名不一致は維持）。既存 `e2e_test.go` の軽量 `TestE2E_Vendors_List` を削除し `e2e_vendors_test.go` の厳格版に一本化。Unit 5/5 Green（U5 は `VendorSearchParams` の **2 クエリ**（Name+UpdatedAtFrom）を検証、M15 の 4 クエリより少ない点を明示。VendorEntity は 6 フィールドと Phase F 3 件の中で最もシンプル）。実 API E2E で **Phase F 3 件目・Phase F 完走として**: ①`GET /v1/payees` が 200 / 0 items 返却（当該アカウントにベンダーデータなし、M14/M15 と同パターン）、②Get は discovery 0 件のため `t.Skipf("pending re-verification")` → Pending Re-verification に転記、③Search PASS（0 items）、④未マップ 0（空配列のため StrictFieldDiff の検証機会なし）、⑤403/429 は発生せず、⑥実パス `/v1/payees` と Go 型名 `Vendor*` の命名不一致は Unit テストのパスアサーションで確認済み。**Phase F 完走サマリ**: M14/M15/M16 全 3 件でデータ 0 件（当該アカウントはベンダー系リソース全て空）。Phase F 合計 req: **9 req**（各 3 req × 3 M）、見積 21 req から大幅削減。実消費 **3 req**（見積 5）、上限 8 req 以下。|
| 2026-04-21 01:00 | M13 実装・検証 | `ListProjectsRaw` / `GetProjectRaw` / `GetProjectWithGroupRaw` / `SearchProjectsRaw` を M12 clients と同形で追加（URL `/v1/projects`、top-level、`ProjectSearchParams` は 5 クエリ検証で M12-M02 通算最多）。旧 `e2e_test.go` の `TestE2E_Projects_List` / `_GetByID` / `_GetWithGroup` 3 本を削除し M13 厳格突合版に一本化（`TestE2E_Estimates_GetByDocumentID` は M17/M18 スコープで残存）。Unit 6/6 Green（**U6 で GetProjectWithGroupRaw の response_group query 検証を初導入**、6 group + empty の計 7 subtest）。実 API E2E で **Phase E 2 件目・複雑度最高として多数の重要発見**: ①`GET /v1/projects/{id}` が **200 成功**（**Phase D/E コア業務系 Get 5 件連続確定**）、②**List/Search 21 未マップ**（`client / contact / delivery_status / estimate_date / group_* / invoice_dates / management_no / order_status / project_no / project_type* / tax / total / user`）、③**Get 68 未マップ**（List 21 + Get 限定 47、accounting_type* / archive_flg / client_branch / contract_* / cost_* / delivery_* / document_setting_* / hubspot / in_house_memo / invoice_* / lock_flg / payment_* / periodical_* / tags / to 等）、④**最重要 = delivery/invoice/receipt が `deliveries`/`invoices`/`receipts` 複数形配列キーで返却** → `*DocumentSummary` 単一ポインタ設計が根本的に誤り（M18-M21 スコープの構造的問題）、estimate/order は単一オブジェクトで partially 正しい、⑤DocumentSummary 内未マップ 6-7 フィールド（`details / seal_approval_status / delivery_place / blank_date_flg / document_amount_disp_kbn / valid_period` 等）、⑥`rg=all` 時に `project_costs` キーが出現（M17+ スコープ）、⑦`client_id=0`（`client` ネスト内 id が正規、M09/M10 継続）、⑧`status=空`（`order_status`/`delivery_status` が代替）、⑨**`Memo` 逆方向 9 件連続で全般仕様最終確定**（`in_house_memo` が代替候補）、⑩**name filter 無視 9 件連続で全般仕様最終確定**（2405 items 全件返却）、⑪403/429 は発生せず。E2E は意図的に Fail 状態で commit。実消費 **11 req**（List 1 + Get discovery 1 + Get 本体 1 + Search 1 + GetWithGroup discovery 1 + 6 groups 6 = 11、pin-point accuracy、上限 15 以内）。Pending Re-verification 追加なし（List 2405 items、Get 200 で完全にデータ充足）。**Phase E 2 件目サマリ**: M12 で確立した「コア業務 Get 200 継続・ネスト client-child 継続・逆方向不整合継続」が M13 で全て継承、更に **DocumentSummary 配列構造の根本的不一致** と **62 フィールド未マップ（今 M 最大規模）** の 2 大発見を得た。フォローアップ: ProjectEntity 全面改訂 / DocumentSummary 配列設計変更（最優先、M18-M21 依存）/ project_costs キー確認（M17+）。 |
| 2026-04-21 10:xx | M25 実装・Phase H 開始 | TestE2E_FindClient_StrictEnrichment を新規追加し FindClient(ID) の branches/contacts enrichment を独立 raw API と突合。**compliance 欠損を Red で検出**: branches=0 / contacts=0（独立 raw API は 10 / 171）。**根本原因確定**: BOARD API は `client_id` フラットフィールドでなく `{"client": {"id": N}}` ネスト構造を返す（M09/M10 記録済み「逆方向不整合」）。ClientBranchEntity.ClientID / ContactEntity.ClientID が unmarshal 後常に 0 → `filterClientBranches(e.ClientID != params.ClientID)` が全件除外。`ClientBranchRepository.Search` と `ContactRepository.Search` を API-side filter（api.SearchClientBranches / api.SearchContacts）経由に修正。T_R23 / T_R37 unit test もモックデータ付きに修正。**修正後 Green**: branches=10 contacts=171（独立 API 突合 OK）。副産物として EstimateEntity.Title コンパイルエラー（M35 波及漏れ）を fix。VendorBranchRepository / VendorContactRepository も同パターンの潜在バグを発見（データなしのため未表面化、別 M 対応）。実消費 ~10 req。go build/vet/test 全 Green（全 12 パッケージ）。コミット 3 件 (fix+fix+test) + docs 1 件。 |
| 2026-04-21 10:xx | M26 実装・Phase H 2 件目 | FindProject の 5 モード（ID/ClientName/Name/Text/Status）を E2E で厳格検証。**新規テスト 5 本追加**: TestE2E_FindProject_StrictEnrichment_ByID / ByName_Strict / ByClientName_Strict / ByText_Strict / ByStatus_Strict。**enrichment バグなし**: ProjectEntity.ClientID は `json:"client_id"` フラットマッピングであり M25 の nested-unmarshal バグは発現しない（fix コミットなし）。**data-dependent skip 2 件**: StrictEnrichment_ByID は先頭 5 件が全て ClientID=0 / ByStatus_Strict はステータス全件空。**ByText_Strict PASS**: 「弘前市」prefix で 4 results、全 result が prefix を Name/Code/Memo に含むことを確認。**BOARD API name filter 無視継続**: SearchClients(Name="株式会社WAND") が 299 件全返却（7+ 件連続継続）。実消費 ~10 req。go build/vet/test 全 Green。コミット 2 件（test + docs）。 |
| 2026-04-21 | M27 実装・Phase H 3 件目 | FindOrder の 4 モード（ID/ProjectID/ClientName/ProjectName）を E2E で検証。**新規テスト 4 本 + helper 追加**: TestE2E_FindOrder_ByProjectID_Strict PASS（projectID=95944469, orderID=71741501, Project enrichment ✓, ClientID=0→nil 正常）/ TestE2E_FindOrder_ByID_Strict PASS（Client=nil, Project=nil ID モード仕様通り）/ ClientName・ProjectName は初回フルフェッチタイムアウトのため cache-warm SKIP。厳格フィールド突合 PASS（OrderEntity 未マップ 0）。**findProjectWithDocType helper 新設**（topN=50 で advisor 指摘 top-5 見逃しを回避）。**発見**: ProjectEntity.Delivery/Receipt 単数形マッピング問題（M28/M29 で fix 予定）。実消費 ~20 req。go build/vet/test 全 Green。コミット 2 件（test + docs）。 |
| 2026-04-21 | M28 実装・Phase H 4 件目 | FindDelivery の 4 モード検証 + ProjectEntity.Delivery 単数形マッピングバグ修正。**TDD Red**: TestE2E_FindDelivery_ByProjectID_Strict が FindDelivery(ProjectID=95944469) で 0 件返却を確認（ProjectEntity.Delivery が常に nil = bug）。**根本原因**: BOARD API は "deliveries" 複数形配列を返すが ProjectEntity.Delivery は `json:"delivery"` 単数形タグ。**Green**: ProjectEntity に `Deliveries []DocumentSummary` (`json:"deliveries,omitempty"`) を追加、find_delivery.go を `p.Delivery → p.Deliveries[0]` 参照に修正。**Refactor**: unit テストのモックデータを Deliveries に統一。**PASS 確認**: deliveryID=64955390, delivery_date="2026-06-30", Project enrichment ✓。厳格フィールド突合 PASS（DeliveryEntity 未マップ 0）。go build/vet/test 全 Green（全 12 パッケージ）。コミット 3 件（fix + test + docs）。 |
| 2026-04-21 | M29 実装・Phase H 5 件目 | FindReceipt の 4 モード検証 + ProjectEntity.Receipt 単数形マッピングバグ修正（M28 と同パターン）。Receipts フィールドは M28 で既に ProjectEntity に追加済みのため、変更は find_receipt.go + find_receipt_test.go のみ。**TDD Red**: TestE2E_FindReceipt_ByProjectID_Strict が FindReceipt(ProjectID=95960734) で 0 件返却を確認。**Green**: find_receipt.go を `p.Receipt → p.Receipts[0]` 参照に修正。**PASS 確認**: receiptID=28480168, receipt_date="2026-04-30", Project enrichment ✓。厳格フィールド突合 PASS（ReceiptEntity 未マップ 0）。go build/vet/test 全 Green（全 12 パッケージ）。コミット 3 件（fix + test + docs）。 |
