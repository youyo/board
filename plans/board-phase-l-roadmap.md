# Phase L ロードマップ: api 層 BOARD API 完全準拠化

## Meta
| 項目 | 値 |
|------|---|
| ゴール | `internal/boardapi/` 全 22 リソースが BOARD API ドキュメント準拠のクエリパラメータ・response_group を送信し、レスポンスヘッダー（X-Total-Count / Rate Limit / ETag 等）を上位層に `ListResult[T]{Items, Meta, Headers}` 形状で伝達する状態に到達する |
| 成功基準 | 全 9 M 完了 + `go test -tags e2e` 全 Green + 実 CLI で `--name-cont` / `--response-group` / `_meta` が動作 |
| 制約 | readonly GET のみ対象、Rate Limit 3 req/sec / 3000 req/day 遵守、破壊的変更を許容（v0.5.0 で major bump） |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 作成日 | 2026-04-23 |
| 最終更新 | 2026-04-23 09:55 |
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
- **ステータス**: M49 完了、M50 待機
- **直近の完了**: M49（基盤再設計: ListResult / QueryBuilder / Header 伝達配管、2026-04-23）
- **次のアクション**: M50（clients 先行パイロット）着手

## Progress

### M49 (L-01): 基盤再設計 — ListResult / QueryBuilder / Header 伝達配管 ✅
- [x] `internal/boardapi/result.go` 新設（`ListResult[T]{Items, Meta, Headers}` / `ListMeta{TotalCount, Page, PerPage, RateLimit*, ETag, LastModified}`）
- [x] `internal/boardapi/query.go` 新設（`QueryBuilder` ヘルパ: `_cont` / `_eq` / `_gteq` / `_lteq` / `_in[]` / bool / response_group）
- [x] `ListAllWithResult` 新設（戻り値 `*ListResult[json.RawMessage]`、最終ページヘッダーを保持）
- [x] 旧 `ListAll` / `PageResult[T]` は Deprecated コメント付きで残置（M57 で削除予定）
- [x] `parseListMeta` / `parseItemMeta` 実装（X-Total-Count / X-Page / X-Per-Page / X-Ratelimit-* / ETag / Last-Modified / Retry-After）
- [x] clients パイロット: `ListClients` / `SearchClients` を `*ListResult[ClientEntity]` に変更（repository/fetcher は `.Items` 展開で追従）
- [x] ユニットテスト追加: QueryBuilder 14 種 + parseListMeta/parseItemMeta 5 種 + ListAllWithResult 3 種 + MarshalJSON 3 種
- 📄 詳細: plans/board-phase-l-m49-foundation.md
- **補足**: `ListMeta.MarshalJSON` / `ItemMeta.MarshalJSON` をカスタム実装（`time.Time` ゼロ値が encoding/json の `omitempty` で除外されない問題を回避）。
- **補足**: 環境制約により Agent/Task tool 未提供のため direct execution 経路で実装、advisor() で最終ゲート代替。

### M50 (L-02): clients 先行パイロット（フルサイクル検証）⏳
- [ ] `ClientSearchParams` → `ClientListOptions` に刷新（NameCont, NameDispCont, UpdatedAtGteq, UpdatedAtLteq, IncludeArchiveFlg, ResponseGroup, InvoiceSystemNumberEq, Tags, CustomNoEq）
- [ ] `ListClients / SearchClients / GetClient` が `ListResult[ClientEntity]` / `*ClientEntity` を返すように変更
- [ ] repository/service/api/cli 層の `clients` ルート全通し
- [ ] 実 API E2E 検証（`name_cont` が効くこと、`response_group=large` で Get 限定フィールドが List/Search でも取れること、ヘッダー実測）
- [ ] `internal/output/` の `_meta` フィールド実装（`ListResult.Meta` を JSON 出力）
- [ ] パイロット成果物: `plans/board-phase-l-pattern.md`（全 22 リソース移行手順書）
- 📄 詳細: plans/board-phase-l-m50-clients-pilot.md

### M51 (L-03): projects 全面移行（最大規模）⏳
- [ ] `ProjectSearchParams` 刷新: NameCont, ClientIDEq, ClientNameCont, OrderStatusIn []int, DeliveryStatusIn []int, ProjectNoEq, ManagementNoEq, DeliveryDateGteq/Lteq, InvoiceDateGteq/Lteq, InvoiceTimingKbnIn []int, Tags []string, CreatedAtGteq/Lteq, UpdatedAtGteq/Lteq, IncludeLostFlg, IncludeArchiveFlg, ResponseGroup
- [ ] 「name/status フィルタが効かない」観測のリベンジ E2E
- [ ] `GetProjectWithGroup` の URL エンコード安全化（現状 `path += "?response_group=..."` ハードコード）
- [ ] downstream（find_project 等）のビルドが通る最小追従
- 📄 詳細: 着手時に plans/board-phase-l-m51-projects.md として生成

### M52 (L-04): client_branches / contacts / project_costs ⏳
- [ ] 各 SearchParams 刷新（client_id_eq, name_cont, email_cont 等）
- [ ] ドキュメント記載パラメータに限定（推測追加はしない）
- 📄 詳細: 着手時に plans/board-phase-l-m52-client-side.md として生成

### M53 (L-05): ドキュメント系 4 件 List/Search 調査 + 実装 ⏳
- [ ] estimates/orders/deliveries/receipts の List/Search 存在確認（実 API 探索）
- [ ] 存在すれば実装、なければ「単一取得のみサポート」を明記
- [ ] projects の response_group 経由で埋め込み取得する運用との関係整理
- 📄 詳細: 着手時に plans/board-phase-l-m53-documents.md として生成

### M54 (L-06): 取引系 3 件（invoices, purchase_orders, payments）⏳
- [ ] invoices: client_id_eq, project_id_eq, status_in[] 系
- [ ] purchase_orders (/v1/expenditures): vendor_id_eq, project_id_eq, status_in[]
- [ ] payments (/v1/expenditure_payments): vendor_id_eq, purchase_order_id_eq
- 📄 詳細: 着手時に plans/board-phase-l-m54-transactions.md として生成

### M55 (L-07): ベンダー系 3 件 ⏳
- [ ] vendors (/v1/payees), vendor_branches (/v1/payee_branches), vendor_contacts (/v1/payee_contacts)
- [ ] clients 群と同一構造を期待（name_cont, vendor_id_eq 等）
- 📄 詳細: 着手時に plans/board-phase-l-m55-vendor-side.md として生成

### M56 (L-08): マスタ系 7 件一括 ⏳
- [ ] users, groups, payment_terms, project_types, purchase_types, accounting_types, document_send_channels
- [ ] 共通 3 点セット（name_cont, updated_at_gteq/lteq, include_archive_flg）で統一想定
- [ ] groups.go の SearchGroupsRaw 欠落を補完
- 📄 詳細: 着手時に plans/board-phase-l-m56-masters.md として生成

### M57 (L-09): 仕上げ（追従集約 + 全体 E2E + v0.5.0 リリース準備）⏳
- [ ] repository / service/api / cli 層の追従を最終集約
- [ ] CLI 代表 flag 追加（`--name-cont`, `--updated-at-gteq`, `--response-group`, `--include-archive-flg`）
- [ ] `internal/output/` の `_meta` フィールド仕上げ（`--show-meta` flag も検討）
- [ ] 全 22 リソース per-batch E2E 実行（rate limit 遵守、M 単位分割）
- [ ] StrictFieldDiff で未マップフィールド 0 の再確認
- [ ] README / docs/api-reference.md 更新
- [ ] CHANGELOG v0.5.0 破壊的変更案内（`board cache clear` 必須ガイド含む）
- [ ] find 層ビルドが通る最小追従（Phase M の発射台）
- 📄 詳細: 着手時に plans/board-phase-l-m57-finalize.md として生成

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
