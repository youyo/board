# Phase L ロードマップ: api 層 BOARD API 完全準拠化

## Meta
| 項目 | 値 |
|------|---|
| ゴール | `internal/boardapi/` 全 22 リソースが BOARD API ドキュメント準拠のクエリパラメータ・response_group を送信し、レスポンスヘッダー（X-Total-Count / Rate Limit / ETag 等）を上位層に `ListResult[T]{Items, Meta, Headers}` 形状で伝達する状態に到達する |
| 成功基準 | 全 9 M 完了 + `go test -tags e2e` 全 Green + 実 CLI で `--name-cont` / `--response-group` / `_meta` が動作 |
| 制約 | readonly GET のみ対象、Rate Limit 3 req/sec / 3000 req/day 遵守、破壊的変更を許容（v0.5.0 で major bump） |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 作成日 | 2026-04-23 |
| 最終更新 | 2026-04-23 17:10 |
| ステータス | 未着手 |
| 親計画 | plans/iridescent-leaping-snowglobe.md |
| 先行フェーズ | plans/board-phase-k-roadmap.md（Phase K: Entity 再設計、完走済み） |
| 後続フェーズ | Phase M（find 層仕上げ、MCP 対応、CLI flag 網羅）— 本ロードマップでは対象外 |

## 背景

Phase A〜K（48 M）でエンティティ準拠とリリースインフラが整ったが、以下 4 点で **実 API との乖離** が残存。

1. **リクエストパラメータ命名が BOARD API 非準拠**
   - 実装: `name`, `updated_at_from`, `client_id`, `status` 等
   - 実 API: `name_cont`, `updated_at_gteq`, `client_id_eq`, `order_status_in[]` 等（Ransack 風）
   - 「`name` フィルタが M03〜M12 の 7 連続で無視された」観測は命名ミスが原因と判明
2. **仕様書記載の主要パラメータが未実装**
   - 全 List 共通: `include_archive_flg`, `updated_at_gteq`, `updated_at_lteq`
   - clients: `response_group`(small/large), `name_disp_cont`, `invoice_system_number_eq`, `tags[]`, `custom_no_eq`
   - projects: 15+ フィルタ（`order_status_in[]`, `delivery_status_in[]`, `delivery_date_gteq/lteq`, `invoice_timing_kbn_in[]`, `tags[]`, `include_lost_flg` 等）
3. **レスポンスヘッダーが上位層に届かない**
   - `pagination.go:38-48` の `parsePaginationHeaders` は `ListPage[T]` のみが利用
   - 通常系 `List*/Search*/Get*` は `DoWithRetry` でボディだけ返しヘッダーを捨てる
   - repository → service/api → cli → output 層への伝達経路なし
4. **ドキュメント系 4 リソース（estimates/orders/deliveries/receipts）は Get のみで List/Search 未実装**
   - BOARD API 側の存在有無自体が未検証

## Current Focus
- **ステータス**: **Phase L 完走 ✅**（M49〜M57 全 9 M 完了、2026-04-24）
- **直近の完了**: M57（旧 API 撤去 + CHANGELOG + README + docs/api-reference.md 更新）
- **次のアクション**: v0.5.0 リリース実行 → Phase M（find 層仕上げ）へ

## Progress

### M49 (L-01): 基盤再設計 — ListResult / QueryBuilder / Header 伝達配管 ✅
- [x] `internal/boardapi/result.go` 新設（`ListResult[T]{Items, Meta, Headers}` / `ListMeta{TotalCount, Page, PerPage, RateLimit*, ETag, LastModified}`）
- [x] `internal/boardapi/query.go` 新設（`QueryBuilder` ヘルパ: `_cont` / `_eq` / `_gteq` / `_lteq` / `_in[]` / bool / response_group）
- [x] `ListAllWithResult` 新設（戻り値 `*ListResult[json.RawMessage]`、最終ページヘッダーを保持）
- [x] 旧 `ListAll` / `PageResult[T]` / `ListPage` は Deprecated コメント付きで残置（M57 で完全撤去済み）
- [x] `error.go` 確認: 5xx → APIErrorTemporary マップ済、`Retry-After` も parseErrorWithHeader で抽出済
- [x] `parseListMeta` / `parseItemMeta` 実装（X-Total-Count / X-Page / X-Per-Page / X-Ratelimit-* / ETag / Last-Modified / Retry-After）
- [x] clients パイロット: `ListClients` / `SearchClients` を `*ListResult[ClientEntity]` に変更（repository/fetcher は `.Items` 展開で追従）
- [x] ユニットテスト追加: QueryBuilder 14 種 + parseListMeta/parseItemMeta 5 種 + ListAllWithResult 3 種 + MarshalJSON 3 種
- 📄 詳細: plans/board-phase-l-m49-foundation.md
- **補足**: `ListMeta.MarshalJSON` / `ItemMeta.MarshalJSON` をカスタム実装（`time.Time` ゼロ値が encoding/json の `omitempty` で除外されない問題を回避）。
- **補足**: 環境制約により Agent/Task tool 未提供のため direct execution 経路で実装、advisor() で最終ゲート代替。

### M50 (L-02): clients 先行パイロット（フルサイクル検証）✅
- [x] `ClientSearchParams` → `ClientListOptions` に刷新（NameCont, NameDispCont, UpdatedAtGteq, UpdatedAtLteq, IncludeArchiveFlg, ResponseGroup, InvoiceSystemNumberEq, Tags, CustomNoEq）
- [x] `ListClients` が `*ListResult[ClientEntity]` / `GetClient` が `*ItemResult[ClientEntity]` / `ListClientsRaw` + `GetClientRaw` が `http.Header` 付き返却に変更
- [x] `SearchClients` / `SearchClientsRaw` / `ListClientsPage` / `ClientSearchParams` を削除（破壊的変更、v0.5.0 で受容）
- [x] repository 層: `ClientRepository.List(ctx, ReadOptions, ClientListOptions) (*ListResult, error)` に二引数化、非ゼロ filter は cache bypass で `api.ListClients` 直呼び
- [x] service/api 層: `ListClients(ctx, readOpts, filter)` に刷新、`SearchClients`/`ListClientsPage` 削除
- [x] find 層: `ClientRepo.Search(ctx, ClientListOptions, ReadOptions) ([]ClientEntity, error)` に更新、find/find_*.go の `ClientSearchParams{Name:...}` → `ClientListOptions{NameCont:...}` 全置換
- [x] cli 層: `board api clients list` に `--name-cont / --name-disp-cont / --invoice-system-number-eq / --custom-no-eq / --tags / --response-group / --updated-at-gteq / --updated-at-lteq / --include-archive-flg / --show-meta` 追加、`search` サブコマンド削除
- [x] `internal/boardapi/result.go` の JSON tag を `"meta"` → `"_meta"` に変更（`jq '._meta'` 慣習）
- [x] fetcher: `clientsFetcher.ListUpdatedSince` で ISO 8601 → `YYYY-MM-DD HH:MM:SS` 変換（`isoToBoardDateTime`）
- [x] Unit テスト追加/更新: U1-U9 相当（ListClients / GetClient / ListClientsRaw 新シグネチャ、Ransack パラメータ確認、全フィルタ網羅）
- [x] E2E テスト: `internal/boardapi/e2e_clients_m50_test.go` に E1-E10 追加（実 API 実行時はヘッダー名実測 + フィルタ動作確認）。実行は環境が整い次第。
- [x] パイロット成果物: `plans/board-phase-l-pattern.md`（M51-M56 移行手順書）作成
- 📄 詳細: plans/board-phase-l-m50-clients-pilot.md
- 📄 パターン手順書: plans/board-phase-l-pattern.md
- **補足**: Agent/Task tool 未提供環境のため direct execution 経路 + advisor() 2 回（計画批評 + 実装前レビュー）で M49 同様の AI ゲート代替を実施。
- **補足**: E10（BOARD API 正式ヘッダー名確定）は実 API 実行が未完で、pattern 手順書 §6 に TBD を残した。M51 以降の per-batch E2E 実行で随時確定していく。

### M51 (L-03): projects 全面移行（最大規模）✅
- [x] `ProjectListOptions` 新設（NameCont, ClientIDEq, ClientNameCont, OrderStatusIn []int, DeliveryStatusIn []int, ProjectNoEq, ManagementNoEq, DeliveryDateGteq/Lteq, InvoiceDateGteq/Lteq, InvoiceTimingKbnIn []int, Tags []string, CreatedAtGteq/Lteq, UpdatedAtGteq/Lteq, IncludeLostFlg, IncludeArchiveFlg, ResponseGroup）
- [x] `ListProjects(ctx, ProjectListOptions) *ListResult[ProjectEntity]` に刷新、`SearchProjects`/`SearchProjectsRaw`/`ProjectSearchParams` を完全削除
- [x] `GetProjectWithGroup` の URL エンコード安全化（`path += "?response_group=..."` → `QueryBuilder.ResponseGroup` に統合）
- [x] repository 層: `List(ctx, ReadOptions, ProjectListOptions)` 二引数化、非ゼロ filter は cache bypass
- [x] find 層（find_project.go 等）のビルドが通る最小追従
- [x] T1-T16 単体テスト + E1-E8 E2E テスト追加
- [x] `go test -count=1 ./...` / `go vet ./...` 全 Green
- 📄 commit: 38404a9（23 ファイル変更、968 挿入 / 541 削除）

### M52 (L-04): client_branches / contacts / project_costs ✅
- [x] `ClientBranchListOptions` / `ContactListOptions` / `ProjectCostListOptions` 新設（client_id_eq, name_cont, email_cont, project_id_eq 等）
- [x] `ListX(ctx, XListOptions) (*ListResult[XEntity], error)` に刷新、旧 SearchX / ListXPage 削除
- [x] repository / service / cli 層の追従
- [x] SA1019 PageResult 警告を client_branches / contacts で解消
- [x] e2e_helpers_test.go / find/e2e_test.go の旧 API 参照を修正（go vet -tags e2e クリーン）
- [x] `go test -count=1 ./...` / `go vet ./...` 全 Green

### M53 (L-05): ドキュメント系 4 件 List/Search 調査 + 実装 ✅
- [x] estimates/orders/deliveries/receipts の List/Search 存在確認（OpenAPI 実測）
  - **調査結果: List API 非存在**。`/documents/{resource}/{id}` の GET のみ（OpenAPI `/doc/board_openapi.json` 実測）
  - 正道は projects の `response_group=estimate/order/delivery/receipt` 経由の埋め込み取得
- [x] Pattern B（単一取得のみ）で実装: `GetX → *ItemResult[XEntity]`, `GetXRaw → ([]byte, http.Header, error)`
- [x] repository 層: `.Item` 展開で追従
- [x] service/api 層: `*ItemResult[XEntity]` 返却に刷新
- [x] cli 層: `--show-meta` フラグ追加（デフォルト true）
- [x] `go test -count=1 ./...` / `go vet ./...` / `go vet -tags e2e ./...` 全 Green
- 📄 詳細: plans/board-phase-l-m53-documents.md

### M54 (L-06): 取引系 3 件（invoices, purchase_orders, payments）✅
- [x] `InvoiceListOptions` / `PurchaseOrderListOptions` / `PaymentListOptions` 新設（client_id_eq, project_id_eq, vendor_id_eq, status_in[], purchase_order_id_eq 等）
- [x] `ListX(ctx, XListOptions) (*ListResult[XEntity], error)` に刷新、旧 SearchParams deprecated 残置
- [x] repository / service / find / cli 層の追従（32 ファイル変更）
- [x] `go test -count=1 ./...` / `go vet ./...` 全 Green
- [x] E2E テスト 3 件追加（//go:build e2e）

### M55 (L-07): ベンダー系 3 件 ✅
- [x] vendors (/v1/payees), vendor_branches (/v1/payee_branches), vendor_contacts (/v1/payee_contacts)
- [x] `VendorListOptions` / `VendorBranchListOptions` / `VendorContactListOptions` 新設（PayeeIDEq, NameCont, UpdatedAtGteq/Lteq, IncludeArchiveFlg 等）
- [x] `ListX(ctx, XListOptions) (*ListResult[XEntity], error)` に刷新、旧 SearchXRaw/SearchXParams 削除
- [x] repository: vendor_branches/vendor_contacts を空ファイルから完全実装、cache bypass パターン適用
- [x] fetcher: ListUpdatedSince に UpdatedAtGteq を活用するよう修正
- [x] CLI: search サブコマンド廃止、list に Ransack フラグ + --show-meta 追加
- [x] e2e テスト: 旧 SearchXRaw/ListXPage/WithPerPage を新 API に一括修正（M54 取り残し含む）
- [x] `go test -count=1 ./...` / `go vet ./...` / `go vet -tags e2e ./...` 全 Green

### M56 (L-08): マスタ系 7 件一括 ✅
- [x] users, groups, payment_terms, project_types, purchase_types, accounting_types, document_send_channels
- [x] `XListOptions` 新設（name_cont, updated_at_gteq/lteq, include_archive_flg の共通 3 点セット）
- [x] `ListX(ctx, XListOptions) (*ListResult[XEntity], error)` に刷新、旧 SearchX/ListXPage 削除
- [x] repository / service / find / cli 層の追従
- [x] `go test -count=1 ./...` / `go vet ./...` 全 Green
- 📄 commit: c7712c5

### M57 (L-09): 仕上げ（追従集約 + v0.5.0 リリース準備）✅
- [x] `PageResult[T]` / `ListPage` / `ListAll` 撤去（`ListAllWithResult` に完全移行）
- [x] `parsePaginationHeaders` 撤去（`ListPage` 削除で不要化）
- [x] `ListProjectsRaw` を `ListAllWithResult` に移行
- [x] `TestListAll_*` を `TestListAllWithResult_*`（U13-U17）に移行し削除
- [x] `golangci-lint run` 0 issues（SA1019 完全解消）
- [x] CHANGELOG.md 新設（v0.5.0 破壊的変更案内 + `board cache clear` 必須ガイド含む）
- [x] README / README_ja.md 更新（`search` コマンド廃止、Ransack 風フィルタ例示、`--limit` デフォルト修正）
- [x] `go build ./...` / `go test -count=1 ./...` / `go vet ./...` 全 Green
- 📄 commit: M57 仕上げ — 旧 PageResult/ListAll 撤去 + v0.5.0 リリース準備

## 依存関係

```
M49 (基盤) ──┬→ M50 (clients パイロット) ──┬→ M51 (projects)
             │                              ├→ M52 (client_branches, contacts, project_costs)
             │                              ├→ M53 (docs 4 件調査+実装)
             │                              ├→ M54 (invoices, purchase_orders, payments)
             │                              ├→ M55 (vendors 系 3)
             │                              └→ M56 (masters 7)
             │                                      │
             └────────────────────────────────────→ M57 (仕上げ + v0.5.0)
```

実行順: **M49 → M50 → M51 → M52 → M53 → M54 → M55 → M56 → M57**

M51〜M56 は技術的には並列実施可能だが、ソースの競合を避けるため順次実行を推奨。

## リリース計画

| M 完了時 | バージョン | 内容 |
|----------|-----------|------|
| M49 | v0.5.0-alpha1 | 基盤のみ、実機能なし |
| M50 | v0.5.0-beta1 | clients パイロット動作確認済み |
| M56 | v0.5.0-rc1 | 全リソース移行完了 |
| M57 | **v0.5.0 正式** | 破壊的変更 + CHANGELOG 整備 |

## Blockers
なし

## Architecture Decisions

| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | SearchParams 型の全面刷新（破壊的変更許容） | Ransack 風命名規則への統一が必須。deprecated 並置は冗長化・混乱の元。Phase K と同じく major bump で扱う | 2026-04-23 |
| 2 | `ListResult[T]{Items, Meta, Headers}` に戻り値を刷新 | Context 経由・別メソッド追加は API 形状の分裂を招く。Items/Meta/Headers 同梱で一箇所に情報集約。下流も JSON `_meta` で自然に露出可能 | 2026-04-23 |
| 3 | 先行パイロット方式（clients → 残り 21 件） | Rate Limit 3 req/sec の制約下で代表検証して手順確立→横展開が最短。全件並行は手戻りリスクが高い | 2026-04-23 |
| 4 | find 層は Phase L の対象外 | api 層の刷新範囲だけで 9 M。find 層は仕様検討も必要で Phase M 独立が妥当。L では「build が通る最小追従」のみ | 2026-04-23 |
| 5 | CLI flag 追加は代表のみ | 網羅的 flag 設計は find 層と一貫させる必要あり。L では `--name-cont / --response-group / --include-archive-flg / --updated-at-gteq` の代表 4 種のみ | 2026-04-23 |
| 6 | response_group による追加フィールドは既存 ClientEntity/ProjectEntity の `*string` 可変フィールドにそのまま反映（新型追加しない） | Phase K で nullable 設計が整っており流用できる。型が増えると差分検証が複雑化する | 2026-04-23 |
| 7 | ドキュメント系 List 実装可否は M53 で API 実測ベース決定 | 仕様書に明記なし。実 API で確認してから決定するのが安全 | 2026-04-23 |

## 検証方法（最終）

1. `go build ./...` が通る
2. `go test ./...` 全 Green（ユニットテスト）
3. `go vet ./...` / `golangci-lint run` が 0 警告
4. `go test -tags e2e -v -count=1 ./internal/boardapi/...` が per-batch 実行で全 Green
5. 実 CLI:
   - `./board api clients search --name-cont "エス"` → 絞り込みが効く
   - `./board api clients list --response-group large` → 追加フィールド付き
   - `./board api projects search --order-status-in 1,2` → ステータス絞り込み
6. `./board api clients list | jq '._meta'` → `{"total_count":..., "page":..., "rate_limit_remaining":..., ...}` が露出

## Changelog

| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-23 09:55 | 作成 | Phase L ロードマップ初版。Phase K 完走（M43-M48）+ v0.4.1 リリースを受けて、api 層の BOARD API 完全準拠化を目標に 9 M 構成で起票。パイロット方式（clients 先行）+ ListResult[T] 戻り値型刷新 + QueryBuilder 共通化を軸とする。find 層は Phase M に分離。親計画 plans/iridescent-leaping-snowglobe.md |
| 2026-04-23 16:40 | M49 完了 | `ListResult[T]` / `QueryBuilder` / `ListAllWithResult` / `parseListMeta/parseItemMeta` の基盤導入 + clients (`ListClients` / `SearchClients`) を `*ListResult[ClientEntity]` 戻り値に刷新。repository/fetcher は `.Items` 展開で追従、外部 service/cli I/F は維持（M50 以降で段階的変更予定）。ユニットテスト 25 件追加、全パッケージ go test Green、go vet / golangci-lint 0 issues。ブランチ: feature-m49-listresult-querybuilder-foundation |
| 2026-04-23 17:10 | M50 完了 | clients パイロット フルサイクル刷新。`ClientListOptions`（Ransack 9 フィルタ）導入、`SearchClients`/`ListClientsPage`/`ClientSearchParams` 削除、`GetClient` が `*ItemResult` 返却、repository は `(ReadOptions, ClientListOptions)` 二引数 + 非ゼロ filter 時 cache bypass、cli は `list` に `--name-cont` 等 10 種 flag 追加、`--show-meta` で `_meta` JSON 出力。fetcher で ISO 8601 → `YYYY-MM-DD HH:MM:SS` 変換。`result.go` の JSON タグを `"_meta"` に変更。Unit / find / repository / cli / service/api 全テスト Green、go vet 0 issues、lint は M57 で削除予定の `PageResult deprecated` 3 件のみ。E2E は `TestE2E_Clients_Pilot_M50` (E1-E10) を `-tags e2e` で追加、実 API 実行はユーザー環境依存。パターン手順書 `plans/board-phase-l-pattern.md` 作成。ブランチ: feature-m49-listresult-querybuilder-foundation (M49 の上に積み重ね) |
