# Phase L ロードマップ策定プラン

## Context

board CLI / MCP は v0.4.1 までリリース済み。Phase A〜J（42 M）でエンティティ準拠は完走したが、**リクエスト側のクエリパラメータとレスポンス側のヘッダー処理に大規模な乖離**が残っている。

### 現状の問題（調査で判明）

1. **パラメータ名が BOARD API ドキュメント準拠でない**
   - BOARD API は Ransack 風の命名規則（`name_cont`, `updated_at_gteq`, `client_id_eq`, `order_status_in[]` 等）を使うが、実装は `name`, `updated_at_from`, `client_id`, `status` など **存在しないキー** を送信している
   - 過去 `plans/board-compliance-*.md` のコメントで「`name` フィルタが M03〜M12 の 7 連続で無視されると観測」とあるが、**これはパラメータ名が間違っていたため**。`name_cont` に直せば効くはず
   - 全 List 共通パラメータ `include_archive_flg` / `updated_at_gteq` / `updated_at_lteq` が未対応

2. **response_group など仕様上の重要パラメータが未実装**
   - `clients`: `response_group`(small/large)、`name_disp_cont`、`invoice_system_number_eq`、`tags[]`、`custom_no_eq` 全て未対応
   - `projects`: 15+ 個のフィルタが未対応（Search 用の `projects.go:141-147` ProjectSearchParams は 5 フィールドのみ）

3. **レスポンスヘッダーが上位層に届かない**
   - `boardapi/pagination.go:38-48` で `X-Total-Count` / `X-Page` / `X-Per-Page` をパースする関数は存在
   - しかし `ListPage[T]` のみが利用、通常系の `List* / Search* / Get*` は `DoWithRetry` でボディだけ返し**ヘッダーを捨てている**
   - repository → service/api → cli → output 層への伝達経路が皆無
   - Rate Limit 残量（`X-Ratelimit-*` 推定）や ETag も未取得

4. **ドキュメント系 4 リソース（estimates/orders/deliveries/receipts）は Get のみで List/Search 未実装**
   - BOARD API ドキュメントに List があるかは未検証

### ゴール

BOARD API ドキュメント（https://developers.the-board.jp/doc/）との乖離を徹底的に解消し、**api 層（boardapi + repository + service/api + cli api）が実 API の正確な鏡写し**になる状態を作る。

- **破壊的変更を許容**（全面刷新方針、v0.5.0 で major bump）
- **readonly GET のみ**（POST/PUT/DELETE は対象外）
- **レスポンスヘッダーを `ListResult[T]` 形状で伝達**
- **先行パイロット方式**: `clients` で 1 リソースをフルサイクル検証 → パターン確立後に残り 21 リソースへ横展開
- **find 層は Phase L の対象外**（Phase M 以降で別途仕上げ）

## 成功基準

- [ ] `internal/boardapi/*.go` の全 22 リソースが BOARD API ドキュメント準拠のクエリパラメータ名・response_group を送信
- [ ] `ListResult[T]{Items, Meta, Headers}` が List/Search の戻り値として統一採用され、repository/service/cli まで伝達
- [ ] 実 API で E2E 検証: フィルタが実際に効くこと、ヘッダー値が取得できること（per-batch 実行、rate limit 遵守）
- [ ] `go test ./...` / `go vet ./...` / `golangci-lint run` / `go test -tags e2e` 全 Green
- [ ] v0.5.0 リリース可能状態 + CHANGELOG に破壊的変更案内

## スコープ

### Phase L に含む

- `internal/boardapi/` 全 22 リソースのリクエスト・レスポンス刷新
- `internal/repository/` 各リポジトリの SearchParams 追従
- `internal/service/api/` 各サービスの SearchParams 追従
- `internal/cli/api_*.go` の追従（flag 追加は最小限、新フィルタ代表のみ公開）
- `internal/output/` のヘッダー出力（`--show-meta` or 既定で JSON `_meta` フィールド）
- E2E 実 API 検証（readonly GET のみ）

### Phase L に含まない

- `internal/cli/find_*.go` および `internal/service/find/` の新 API 対応（ビルドが通る最小限追従のみ）
- MCP サーバーの新フィルタ対応（find 層と一緒に Phase M で）
- 新 CLI flag の網羅的追加（find 層仕様に合わせて Phase M で）
- Phase L 完了 = v0.5.0 リリース準備完了

## マイルストーン構成（9 M）

### M49 (L-01): 基盤再設計 — ListResult / QueryBuilder / Header 伝達配管

- `ListResult[T]{Items []T, Meta ListMeta, Headers http.Header}` 型新設
- `ListMeta{TotalCount, Page, PerPage, RateLimitRemaining, RetryAfter, ETag, LastModified}` 型新設
- `QueryBuilder`（リソース共通のパラメータ組み立てヘルパ。`_cont` / `_eq` / `_gteq` / `_lteq` / `_in[]` / bool / response_group を型安全に組み立てる）
- `DoWithRetryFull` は既存（retry.go:98）を活用しつつ、`ListAll` をヘッダー返却型に刷新
- `error.go` の 503 取り扱い再確認（5xx → TEMPORARY で既に網羅済みの確認のみ）
- repository → service/api → cli → output 層に `ListResult[T]` を通す I/F ひな型（代表 1 リソース `clients` だけパス貫通。実装は M50 で）

### M50 (L-02): clients 先行パイロット（フルサイクル検証）

- `ClientSearchParams` を `ClientListOptions` に刷新：`NameCont, NameDispCont, UpdatedAtGteq, UpdatedAtLteq, IncludeArchiveFlg, ResponseGroup(small|large), InvoiceSystemNumberEq, Tags []string, CustomNoEq`
- `ListClients / SearchClients / GetClient` が `ListResult[ClientEntity]` / `*ClientEntity` を返すように変更
- repository/service/api/cli 層の `clients` ルート全通し
- 実 API で E2E 検証: 
  - `name_cont` が実際にフィルタリングに効くこと
  - `response_group=large` で追加フィールドが返ることを確認
  - X-Total-Count 他ヘッダー値の実測（何が返ってくるか判明させる）
- パイロット成果物: 「全 22 リソース移行手順書」として plans/board-phase-l-pattern.md にパターン化

### M51 (L-03): projects 全面移行（最大規模）

- `ProjectSearchParams` を刷新：`NameCont, ClientIDEq, ClientNameCont, OrderStatusIn []int, DeliveryStatusIn []int, ProjectNoEq, ManagementNoEq, DeliveryDateGteq, DeliveryDateLteq, InvoiceDateGteq, InvoiceDateLteq, InvoiceTimingKbnIn []int, Tags []string, CreatedAtGteq, CreatedAtLteq, UpdatedAtGteq, UpdatedAtLteq, IncludeLostFlg, IncludeArchiveFlg, ResponseGroup`
- 過去「name / status フィルタが効かない」観測のリベンジ確認（E2E で実測）
- GetProjectWithGroup の URL エンコード安全化（現在 `path += "?response_group=" + responseGroup` のハードコード）
- downstream（find_project 等）のビルドが通る最小追従

### M52 (L-04): client_branches / contacts / project_costs

- 各 SearchParams を刷新（`client_id_eq`, `vendor_id_eq`, `name_cont`, `email_cont` 等）
- `_eq` 系は BOARD API ドキュメントで明示されているもののみ実装し、未記載のものは省略

### M53 (L-05): ドキュメント系 4 件 List/Search 調査 + 実装

- `estimates / orders / deliveries / receipts` の List/Search が BOARD API に存在するか実 API で検証
- 存在すれば実装、なければ「単一取得のみサポート」を明記してロードマップに反映
- project の response_group で埋め込み取得するのが正道か、独立 List があるかの決定

### M54 (L-06): 取引系 3 件（invoices, purchase_orders, payments）

- `invoices`: `client_id_eq`, `project_id_eq`, `status_in[]` or `invoice_status_in[]`（BOARD 仕様再確認）
- `purchase_orders` (=/v1/expenditures): `vendor_id_eq`, `project_id_eq`, `status_in[]`
- `payments` (=/v1/expenditure_payments): `vendor_id_eq`, `purchase_order_id_eq`

### M55 (L-07): ベンダー系 3 件

- `vendors` (=/v1/payees), `vendor_branches` (=/v1/payee_branches), `vendor_contacts` (=/v1/payee_contacts)
- clients / client_branches / contacts と同一構造を期待（`name_cont`, `vendor_id_eq` 等）

### M56 (L-08): マスタ系 7 件一括

- `users, groups, payment_terms, project_types, purchase_types, accounting_types, document_send_channels`
- 基本的に `name_cont`, `updated_at_gteq/lteq`, `include_archive_flg` の共通 3 点セットで足りる想定
- groups の `SearchGroupsRaw` 欠落（他リソースとパターン不一致）を補完

### M57 (L-09): 仕上げ（追従 + 全体 E2E + リリース準備）

- repository/service/api/cli 層の追従を M50〜M56 と並行で小まめに進めたものを **最終集約**
- CLI `api` 系コマンドの代表 flag 追加（`--name-cont`, `--updated-at-gteq`, `--response-group`, `--include-archive-flg` など共通フラグ）
- `internal/output/` で `ListResult` の Meta/Headers を JSON `_meta` フィールドで露出（`--show-meta` or 既定）
- 全 22 リソースの per-batch E2E 実行（rate limit 3 req/sec 遵守）
- StrictFieldDiff でフィールド差分 0 の再確認
- README / docs/api-reference.md の更新（新フィルタの使い方）
- CHANGELOG に v0.5.0 破壊的変更を明記（`board cache clear` 必須案内）
- find 層ビルドが通る最小追従（Phase M の発射台を整える）

## 依存関係

```
M49 (基盤) ──┬→ M50 (clients パイロット) ──┬→ M51 (projects)
             │                              ├→ M52 (client_branches, contacts, project_costs)
             │                              ├→ M53 (docs: estimates, orders, deliveries, receipts)
             │                              ├→ M54 (invoices, purchase_orders, payments)
             │                              ├→ M55 (vendors 系 3)
             │                              └→ M56 (masters 7)
             │                                      │
             └────────────────────────────────────→ M57 (仕上げ + v0.5.0)
```

- M50 がパターン確立のクリティカルパス。M51〜M56 は並列実施可能だが、ソースの競合を避けるため順次実行を推奨

## リスク

| # | リスク | 影響度 | 対策 |
|---|--------|--------|------|
| 1 | 実 API で新パラメータ名が想定と異なる命名だった場合 | 大 | M50 の clients パイロットで早期発見。必要に応じてドキュメント乖離表を作成 |
| 2 | rate limit 3 req/sec / 3000/day を E2E で超過 | 中 | 既存 per-batch 実行運用を踏襲、M 単位分割 |
| 3 | 下流（find/MCP）の破壊的変更範囲が大きい | 中 | M57 で「ビルドが通る最小追従」にとどめ、本格刷新は Phase M に分離 |
| 4 | レスポンスヘッダーの名前（X-Ratelimit-*, X-Total-Count 等）がドキュメントに未記載 | 中 | M50 の clients パイロットで実 API レスポンスヘッダー dump → 判明した名前で実装 |
| 5 | ドキュメント系 List が存在しない場合の対応 | 小 | M53 で調査結果に基づき方針決定（単一取得のみサポート明記） |
| 6 | SQLite cache schema への影響 | 小 | response_group 追加によりキャッシュキーが増える。cache_meta に version 埋め込み |

## リリース戦略

- M49 完了: v0.5.0-alpha1（基盤のみ、実機能なし）
- M50 完了: v0.5.0-beta1（clients パイロット動作確認済み）
- M56 完了: v0.5.0-rc1（全リソース移行完了）
- M57 完了: **v0.5.0 正式リリース**（破壊的変更 + CHANGELOG 整備）

## 生成するファイル

本スキル（roadmap）の成果物:

1. `plans/board-phase-l-roadmap.md`（Layer 1: ロードマップ本体。M49〜M57 の概要と進捗）
2. `plans/board-phase-l-m49-foundation.md`（Layer 2: M49 詳細計画。即時生成）
3. `plans/board-phase-l-m50-clients-pilot.md`（Layer 2: M50 詳細計画。即時生成。パイロットなので早期に詳細化）
4. M51〜M57 は **着手時に個別に詳細計画化**（遅延生成）

## 次のステップ

Phase L の実行方法:

- **逐次実装**: `/devflow:implement` を M49 から順に呼び出す
- **自律実行**: `/devflow:cycle` で M49〜M57 を連続実行

## 参考ファイル

- `internal/boardapi/client.go` — HTTP クライアント本体（header 取り出し `DoFull` `DoWithRetryFull` は既存）
- `internal/boardapi/pagination.go:38-48` — `parsePaginationHeaders` 既存。`ListAll` がこれを使うよう改修必要
- `internal/boardapi/clients.go:67-71` — `ClientSearchParams` 刷新の出発点
- `internal/boardapi/projects.go:141-147` — `ProjectSearchParams` 刷新の出発点（最大規模）
- `internal/boardapi/error.go:71-90` — `ClassifyStatusCode`（修正不要の確認のみ）
- `plans/board-phase-k-roadmap.md` — 前フェーズ完了ロードマップ（参照元）

## 検証方法（最終）

1. `go build ./...` が通る
2. `go test ./...` 全 Green（ユニットテスト）
3. `go vet ./...` / `golangci-lint run` が 0 警告
4. `go test -tags e2e -v -count=1 ./internal/boardapi/...` が Rate Limit 遵守の per-batch 実行で全 Green
5. 実 CLI で filter 動作を確認:
   - `./board api clients search --name-cont "エス"` → 該当件数が正しく返る
   - `./board api clients list --response-group large` → 追加フィールド付きレスポンス
   - `./board api projects search --order-status-in 1,2` → 指定ステータスでフィルタ
6. `_meta` フィールドに X-Total-Count 等が載る: `./board api clients list | jq '._meta'`
