---
title: "N01: find 層必要性評価 調査レポート"
project: "board"
phase: "Phase N"
milestone: "N01"
created: "2026-04-24"
status: "Completed — B（ゼロベース再設計）採択、ADR-001 Accepted（2026-04-25）"
related_adr: "docs/adr/ADR-001-find-layer.md"
related_plan: "plans/pure-twirling-coral.md"
---

# N01 find 層必要性評価 調査レポート

> **本ファイルは純調査 + 文書化の成果物であり、コード変更ゼロの調査フェーズ（N01）の一次情報を記録する。**
> 推奨・提言は一切含まない（§10 フラット評価宣言参照）。
> A/B/C/D の選択はユーザーレビュー（Step 7a/7b）で実施する。

---

## §1 サマリ

Phase L（v0.5.0）で api 層 22 リソースが Ransack 準拠となり、BOARD API のほぼ全フィルタを直接表現できる
能力を獲得した。これを受け、Phase H 期に構築された find 高レベル層（12 メソッド / 15 ファイル /
内部実装約 3,945 行 / 47 E2E セット 193 ケース / SKIP 70 件）の存在意義を問い直すのが N01 の目的である。

評価対象の 4 選択肢:

| 選択肢 | 概要 |
|---|---|
| **A: 全廃止** | find 層を全廃止し MCP は api_* 直呼びへ。逆引き・enrichment はクライアント側で実装 |
| **B: ゼロベース再設計** | api 層で実現困難な 5 件に特化した薄い層として再構築 |
| **C: 数本残す** | AND 条件（api 単独代替不可 AND MCP 実利用頻度上位 AND 負債小）で 3-5 本を選別し残りは廃止 |
| **D: 現状維持 + 負債解消** | 構造は維持し TODO(M25-M32) 8 箇所の enrichment/post-filter 復元のみ実施 |

本レポートは §2-§9 に一次情報を転記し、§10 でフラット評価の立場を宣言する。
A/B/C/D の決定は Step 7b のユーザーレビューで行うため、本レポートには保留と明記する。

---

## §2 find 層の現状台帳（12 メソッド）

調査対象: `internal/service/find/` 配下の実装ファイル（非テスト）

**ファイル構成**（2026-04-24 時点）:

```
internal/service/find/
  service.go          — Service 構造体 / 依存インターフェース定義 / resolveXxx ヘルパー
  types.go            — 12 Query 構造体 + 12 Result 構造体
  find_client.go      — FindClient
  find_project.go     — FindProject
  find_estimate.go    — FindEstimate
  find_invoice.go     — FindInvoice
  find_order.go       — FindOrder
  find_delivery.go    — FindDelivery
  find_receipt.go     — FindReceipt
  find_vendor.go      — FindVendor
  find_purchase_order.go — FindPurchaseOrder
  find_payment.go     — FindPayment
  find_user.go        — FindUser
  find_group.go       — FindGroup
  text_match.go       — containsText / derefString ヘルパー
  helpers_test.go     — テスト用ヘルパー
  e2e_helpers_test.go — E2E テスト用ヘルパー
  e2e_test.go         — 全 E2E テスト（193 ケース）
  text_match_test.go  — text_match 単体テスト
  各 find_*_test.go   — 各メソッドのユニットテスト
```

### 12 メソッド台帳

「付加価値型」の分類:
- **逆引き**: 名前 → ID 解決を経由する処理（ClientName → ProjectID 等）
- **enrichment**: response_group を用いた関連ドキュメント解決 + 関連エンティティ付与
- **post-filter**: API では指定できないフィールドでのクライアント側絞り込み
- **text-OR**: 複数フィールドを OR 条件でのフリーテキスト検索（API 単一フィールド _cont の代替）

| メソッド名 | ファイル:行 | 付加価値型 | TODO(M25-M32) | 備考 |
|---|---|---|---|---|
| `FindClient` | `find_client.go:14` | enrichment + text-OR | なし | branches/contacts を resolveClientDetails で付与。text 時はキャッシュ全件 + name/custom_no/note OR 検索 |
| `FindProject` | `find_project.go:14` | 逆引き + enrichment + post-filter | なし | ClientName→ClientID→ProjectID 逆引き。response_group=estimate で EstimateEntity を付与。OrderStatusName/DeliveryStatusName で post-filter |
| `FindEstimate` | `find_estimate.go:20` | enrichment (一部無効) | あり (L19, L29) | response_group 経由で EstimateEntity を取得。ID lookup 時は client/project が nil（enrichment 無効）。Status post-filter は TODO(M25-M32) で無効化 |
| `FindInvoice` | `find_invoice.go:13` | 逆引き + post-filter + text-OR | なし | ClientName/ProjectName → ID 逆引き。Status post-filter 動作。title/memo OR 検索 |
| `FindOrder` | `find_order.go:20` | enrichment (一部無効) | あり (L19, L29) | response_group=order 経由。ID lookup 時は client/project が nil。Status post-filter は TODO(M25-M32) で無効化 |
| `FindDelivery` | `find_delivery.go:23` | enrichment (一部無効) | あり (L19, L32) | response_group=delivery 経由。Deliveries は複数形配列で先頭要素を使用。Status post-filter は TODO(M25-M32) で無効化 |
| `FindReceipt` | `find_receipt.go:23` | enrichment (一部無効) | あり (L19, L32) | response_group=receipt 経由。Receipts は複数形配列で先頭要素を使用。Status post-filter は TODO(M25-M32) で無効化 |
| `FindVendor` | `find_vendor.go:14` | enrichment + text-OR | なし | branches/contacts を resolveVendorDetails で付与。text 時は ListEntities 全件 + name/code/memo OR 検索 |
| `FindPurchaseOrder` | `find_purchase_order.go:14` | 逆引き + post-filter + text-OR | なし | VendorName/ProjectName → ID 逆引き。Status post-filter 動作。title/memo OR 検索 |
| `FindPayment` | `find_payment.go:14` | 逆引き + post-filter + text-OR | なし | VendorName → VendorID 逆引き。PurchaseOrderID 直接検索。Status post-filter 動作。memo OR 検索 |
| `FindUser` | `find_user.go:12` | text-OR | なし | name/email/LastName/FirstName の OR 検索。enrichment なし |
| `FindGroup` | `find_group.go:12` | text-OR | なし | name/memo の OR 検索。enrichment なし |

**TODO(M25-M32) 残存箇所（8 箇所）**:

| ファイル | 行 | 内容 |
|---|---|---|
| `find_estimate.go` | L19 | enrichment を再設計で復元する |
| `find_estimate.go` | L29 | Status post-filter を再設計で復元する |
| `find_order.go` | L19 | enrichment を再設計で復元する |
| `find_order.go` | L29 | Status post-filter を再設計で復元する |
| `find_delivery.go` | L19 | enrichment を再設計で復元する |
| `find_delivery.go` | L32 | Status post-filter を再設計で復元する |
| `find_receipt.go` | L19 | enrichment を再設計で復元する |
| `find_receipt.go` | L32 | Status post-filter を再設計で復元する |

TODO は 4 ドキュメント系（Estimate/Order/Delivery/Receipt）に各 2 件、計 8 箇所。
いずれも「Entity に Status/ClientID/ProjectID フィールドが存在しないため」という共通の根本原因による。

---

## §3 api 層 Ransack 対応マトリクス

調査対象: `internal/boardapi/` 配下の各 `*ListOptions` 構造体

以下は find 層が利用する主要 11 リソースの Ransack フィルタ対応状況。
`✓` = 対応あり、`-` = 対応なし（構造体フィールドに存在しない）。

| リソース | _eq(ID) | _cont(name) | _in[]（複数） | _gteq/_lteq | status_eq | response_group |
|---|---|---|---|---|---|---|
| clients | CustomNoEq | NameCont, NameDispCont | Tags[] | UpdatedAt | - | small/large |
| projects | ClientIDEq, ProjectNoEq, ManagementNoEq | NameCont, ClientNameCont | OrderStatusIn[], DeliveryStatusIn[], InvoiceTimingKbnIn[] | UpdatedAt, CreatedAt, DeliveryDate, InvoiceDate | - | small/large/estimate/order/delivery/invoice/receipt/all |
| invoices | ClientIDEq, ProjectIDEq | - | - | UpdatedAt | StatusEq | small/large |
| orders | - | - | - | - | - | 個別 Get のみ（GetByDocumentID 経由） |
| deliveries | - | - | - | - | - | 個別 Get のみ（GetByDocumentID 経由） |
| receipts | - | - | - | - | - | 個別 Get のみ（GetByDocumentID 経由） |
| vendors | - | NameCont | - | UpdatedAt | - | - |
| purchase_orders | VendorIDEq, ProjectIDEq | - | - | UpdatedAt | StatusEq | small/large |
| payments | VendorIDEq, PurchaseOrderIDEq | - | - | UpdatedAt | StatusEq | small/large |
| users | - | NameCont, EmailCont | - | UpdatedAt | - | - |
| groups | - | NameCont | - | UpdatedAt | - | - |

**注記**:
- estimates は BOARD API において独立した List エンドポイントを持たない。projects の response_group 経由でのみ取得可能。
- orders/deliveries/receipts も同様に独立した List エンドポイントを持たず、projects の response_group + GetByDocumentID の組み合わせでのみ取得可能。
- client_branches は ClientIDEq + NameCont、contacts は ClientIDEq + NameCont + EmailCont、vendor_branches は PayeeIDEq + NameCont、vendor_contacts は PayeeIDEq + NameCont + EmailCont の Ransack フィルタを持つ。

---

## §4 api 層の限界（find でしか困難な処理 5 件）

> **⚠️ 限界注記（必読）**:
> 本節は Explore 段階の一次情報（コードベース調査）+ 仕様ベース分析であり、実 API 検証（PoC）は未実施。
> 記述は現行コードの構造とBOARD API 仕様の読み取りに基づく机上分析である。
> 最終確定は N02 冒頭の「実装前レビュー」で再確認する段階的合意とする。

---

### 4.1 リソース横断逆引き（例: ClientName → ProjectID）

BOARD API は横断検索エンドポイントを提供しない。ClientName から ProjectID を得るには最低 2 往復必要。

**api 層での実現方法（相当コード片）**:
```
// 呼び出し 1: ClientName で clients を検索
GET /v1/clients?q[name_cont]=ABC
// → ClientEntity[] を取得、c.ID を抽出

// 呼び出し 2: ClientID で projects を検索
GET /v1/projects?q[client_id_eq]=123
// → ProjectEntity[] を取得
```
クライアント側でのマージ処理: 0 件・複数件対応のループ処理が呼び出し側コードに発生する（N 客 × 1 回 = 合計 N+1 API 呼び出し）。

**find 層での実現方法**:
```go
// FindProject(ctx, FindProjectQuery{ClientName: "ABC"})
// 内部で clients.Search → projects.Search をシーケンシャルに実行
// 呼び出し側は 1 回の関数呼び出しで完結
```

**差分**: api 層でも実現可能だが、呼び出し側に N+1 ループ + マージ処理が移動する。find 層はこれをカプセル化している。

---

### 4.2 複数 status 統一 post-filter

BOARD API は status_eq（単一値完全一致）のみサポートし、複数 status の OR 検索は提供しない。

**api 層での実現方法**:
```
// status ごとに別々のリクエストが必要
GET /v1/invoices?q[status_eq]=sent
GET /v1/invoices?q[status_eq]=partial_paid
// 呼び出し側でマージ・重複除去
```
呼び出し回数: status 値の数 × 1 回。重複除去処理が呼び出し側に発生する。

**find 層での実現方法**:
```go
// FindInvoice(ctx, FindInvoiceQuery{Status: "sent"})
// 現状は単一 status のみ。複数 status 対応は未実装（TODO ではないが機能ギャップ）
```

**差分**: 現状の find 層も単一 status のみ対応。複数 status 対応は find 層 B/C/D でも追加実装が必要。A の場合は api 層を複数回呼ぶ + マージ処理が呼び出し側に残る。

---

### 4.3 free-text 全文検索（複数フィールド OR）

BOARD API は各フィールドに対して `_cont` フィルタを提供するが、複数フィールドを横断する OR 検索は提供しない。

**api 層での実現方法**:
```
// 複数フィールドを OR 検索する API は存在しない
// 例: clients の name または custom_no に "ABC" を含むものを検索する場合
GET /v1/clients?q[name_cont]=ABC
GET /v1/clients?q[custom_no_eq]=ABC
// 呼び出し側でマージ・重複除去 → または全件取得後にクライアント側フィルタ
```

**find 層での実現方法**:
```go
// text モード: 全件取得後にクライアント側で OR フィルタ
// FindClient: containsText(q.Text, c.Name, derefString(c.CustomNo), derefString(c.Note))
// FindProject: containsText(q.Text, p.Name, derefString(p.ManagementNo), derefString(p.InHouseMemo))
// FindVendor: containsText(q.Text, v.Name, v.Code, v.Memo)
// FindUser: containsText(q.Text, u.DisplayName(), u.LastName, u.FirstName, u.Email)
```

**差分**: api 層の _cont は単一フィールドのみ。複数フィールド OR は全件取得後のクライアント側フィルタが唯一の選択肢であり、find 層はこれを実装している。A の場合、MCP クライアント（LLM）側で全件取得 + OR フィルタのロジックを毎回記述する必要がある。

---

### 4.4 enrichment（join 的参照解決）

FindClient は branches + contacts を 1 回の関数呼び出しで返す。これは 3 リソースの Join に相当する。

**api 層での実現方法（3 API 呼び出し）**:
```
// Step 1: client 取得
GET /v1/clients/123
// → ClientEntity

// Step 2: branches 取得
GET /v1/client_branches?q[client_id_eq]=123
// → ClientBranchEntity[]

// Step 3: contacts 取得
GET /v1/contacts?q[client_id_eq]=123
// → ContactEntity[]

// 呼び出し側でマージ
```
呼び出し回数: 1 client につき 3 回。N 件の client に対しては 3N 回。

**find 層での実現方法**:
```go
// FindClient: resolveClientDetails で branches + contacts を付与
// FindVendor: resolveVendorDetails で branches + contacts を付与
// FindProject: resolveProjectClient で client + estimate を付与
// FindInvoice/PurchaseOrder/Payment: resolveClientAndProject / resolveVendorAndProject で付与
```

**差分**: api 層でも実現可能だが、3N 呼び出し + マージ処理が呼び出し側コードに発生する。find 層はこれをカプセル化している。

---

### 4.5 response_group 経由の document enrichment（Estimate/Order/Delivery/Receipt）

BOARD API は Estimate/Order/Delivery/Receipt に独立した List エンドポイントを持たない。
これらを ProjectName や ClientName から取得するには response_group を経由した複数ステップが必要。

**api 層での実現方法（ClientName から Estimate を取得する場合）**:
```
// Step 1: ClientName → ClientID
GET /v1/clients?q[name_cont]=ABC
// → ClientEntity[]、c.ID を抽出

// Step 2: ClientID + response_group=estimate → Project + Estimate ID
GET /v1/projects?q[client_id_eq]=123&response_group=estimate
// → ProjectEntity[] + p.Estimate.ID を抽出

// Step 3: Estimate ID → EstimateEntity
GET /v1/estimates/{estimate_id}
// → EstimateEntity
```
呼び出し回数: 1 Client に複数 Project がある場合、Step 3 は Project 数分実行される。

**find 層での実現方法**:
```go
// FindEstimate(ctx, FindEstimateQuery{ClientName: "ABC"})
// 内部で clients.Search → projects.Search(ResponseGroup="estimate") → estimates.GetByDocumentID
// (現状: enrichment は一部 TODO で無効化。Status post-filter も TODO)
```

**差分**: response_group を用いた 3 ステップ取得のカプセル化は find 層でしか提供されていない。
api 層は各ステップを個別に提供するが、組み合わせロジックは呼び出し側に委ねられる。
**ただし**: FindEstimate/FindOrder/FindDelivery/FindReceipt の enrichment と Status post-filter は
現状 TODO(M25-M32) で無効化されており、現時点では機能提供が不完全である。

---

## §5 E2E テスト SKIP 実態（70 件内訳）

調査対象: `internal/service/find/` 配下の `t.Skip` 呼び出し

実際の SKIP 件数: **70 件**（grep 実測値。計画書記載の 63 件との差は調査時点の差異）。

> **注記**: 以下の分類は grep で抽出したメッセージ内容をもとに機械的に行ったものである。
> 実際の SKIP 理由は E2E 実行環境（BOARD アカウントのデータ状況）に依存する部分が大きい。

### SKIP カテゴリ分類

| カテゴリ | 件数 | 代表例 |
|---|---|---|
| **cache-warm 依存** | 11 | `"cache-warm required; 11000+ invoices"` (e2e_test.go:922,928,934,940,1151,1162,1298,1304,1440,1446,1633) — 大量データのためキャッシュ暖気が必要 |
| **データ 0 件** | 10 | `"no clients available"` `"no invoices available"` — 該当データが存在しない |
| **テストデータ不足** | 39 | `"no project with order found in top-50"` `"no purchase_orders; pending re-verification"` `"vendors 0 件"` — テスト環境のデータ依存 |
| **その他（条件付き）** | 10 | `"client name too short for text search"` `"E2E: hit rate limit (429)"` `"BOARD_API_KEY and BOARD_API_TOKEN are required"` |

**計**: 70 件（e2e_helpers_test.go の 2 件 + e2e_test.go の 68 件）

### find 層廃止時の SKIP の扱い

- **A（全廃止）採択時**: find 層の E2E テスト（e2e_test.go 全体）が削除対象となるため、70 件のすべての SKIP が消滅する
- **B（ゼロベース再設計）採択時**: 新 E2E テストを新設するため、現在の SKIP は全て破棄されて再設計される
- **C（数本残す）採択時**: 残したメソッドのテストのみ存続、削除メソッドの SKIP は消滅
- **D（現状維持 + 負債解消）採択時**: TODO 解消後も SKIP 自体は環境依存のため残存

### テストデータ不足 39 件の位置づけ

「テストデータ不足」は find 層の設計とは独立した **環境依存負債** である。
vendors/purchase_orders/payments/groups のデータが当該 BOARD アカウントに存在しないことが主な原因。
この負債は選択肢 A/B/C/D に共通して存在し、find 層の廃止によって解消されるわけではなく、
D（現状維持）採択時は別途テストデータ整備が必要となる。

---

## §6 MCP 現状と 3 通り方針マトリクス

調査対象: `internal/mcpserver/tools.go`

### 12 MCP ツール一覧と find 依存

| MCP Tool 名 | 対応する find メソッド | 機能概要 |
|---|---|---|
| `find_clients` | `FindClient` | ID / name（_cont）/ free-text OR 検索。branches + contacts を付与 |
| `find_vendors` | `FindVendor` | ID / name（_cont）/ free-text OR 検索。branches + contacts を付与 |
| `find_users` | `FindUser` | ID / name（_cont）/ free-text OR（name/email）検索 |
| `find_groups` | `FindGroup` | ID / name（_cont）/ free-text OR（name/memo）検索 |
| `find_projects` | `FindProject` | ID / client_name（逆引き）/ name（_cont）/ free-text / status post-filter。client + estimate 付与 |
| `find_estimates` | `FindEstimate` | ID / project_id / client_name（逆引き）/ project_name（逆引き）。response_group 経由 enrichment |
| `find_invoices` | `FindInvoice` | ID / client_name（逆引き）/ project_name（逆引き）/ free-text / status post-filter。client + project 付与 |
| `find_orders` | `FindOrder` | ID / project_id / client_name（逆引き）/ project_name（逆引き）。response_group 経由 enrichment |
| `find_deliveries` | `FindDelivery` | ID / project_id / client_name（逆引き）/ project_name（逆引き）。response_group 経由 enrichment |
| `find_receipts` | `FindReceipt` | ID / project_id / client_name（逆引き）/ project_name（逆引き）。response_group 経由 enrichment |
| `find_purchase_orders` | `FindPurchaseOrder` | ID / vendor_name（逆引き）/ project_name（逆引き）/ free-text / status post-filter。vendor + project 付与 |
| `find_payments` | `FindPayment` | ID / vendor_name（逆引き）/ purchase_order_id / free-text / status post-filter。vendor 付与 |

**全 12 tool が find_* 経由であり、api 層を直接呼び出す tool は現状ゼロ**。

### MCP 3 通り方針（選択肢と直交）

| 方針 | 概要 | MCP tool への影響 |
|---|---|---|
| **(a) 全廃止 + LLM 丸投げ** | MCP は api_* 直呼び化。tools.go を 12 個の api_* ラッパーに書き換える。逆引き・enrichment・free-text OR は LLM 側で実装 | 12 tool すべての Handler を書き換え。tool 引数の意味が変わる（client_name が client_id に変わる等） |
| **(b) 薄い BFF helper 残存** | MCP server 内に薄い逆引きヘルパー関数を残す。find 層は廃止するが MCP server 内に局所的なロジックを移植 | tools.go は現行シグネチャを維持、Handler 内部のみ変更。find 層への依存を MCP 内部実装に置換 |
| **(c) 現状 find_* 維持** | MCP tool のシグネチャは変えず、find 層の内部実装のみ変更 | tools.go は変更不要（または最小変更）。find 層の構造変更は MCP 側に波及しない |

### 選択肢 × MCP 方針の組み合わせマトリクス

| \ | 方針(a) LLM 丸投げ | 方針(b) BFF helper | 方針(c) find_* 維持 |
|---|---|---|---|
| **A: 全廃止** | tools.go 全書換、LLM 側実装が必要 | tools.go シグネチャ維持、MCP 内に移植 | 非整合（find 廃止と矛盾） |
| **B: ゼロベース再設計** | 非整合（新 find 層を作るなら方針(c)が整合） | 非整合（新設層あるなら方針(c)) | tools.go シグネチャ維持または新設、新 find 層を呼ぶ |
| **C: 数本残す** | 残存 tool は方針(c)、廃止 tool は方針(a) のハイブリッド | 廃止 tool を BFF helper で代替、残存は find_* 呼び出し | 削除メソッドに対応する tool のみ書換、残存 tool は変更なし |
| **D: 現状維持 + 負債解消** | 非整合（find 維持と矛盾） | 非整合（find 維持と矛盾） | tools.go 変更不要、find 層内部の TODO 解消のみ |

---

## §7 選択肢 A/B/C/D 対比表（4 列、6 評価軸）

> **記入規則**: 各セルは「事実（動詞 + 数値）」のみ。形容詞は §10 禁止リスト参照。

| 評価軸 | A: 全廃止 | B: ゼロベース再設計 | C: 数本残す | D: 現状維持 + 負債解消 |
|---|---|---|---|---|
| **実装コスト** | find 層関連 3,945 行削除 + MCP 12 tool Handler 書換（方針(a)/(b)に応じて変動）。find 層 E2E テスト 193 ケース削除。削除作業は 3-5 日推定 | 新設計書策定 + 5 件特化の新 find 層実装 + MCP tools.go 更新 + E2E 再構築。工数 2-3 週推定 | 3-5 本の選別作業 + 選別外 7-9 本の削除 + tools.go 部分書換。工数 1-2 週推定 | TODO(M25-M32) 8 箇所の enrichment/post-filter 復元 + E2E SKIP 仕分け。工数 2-3 日推定 |
| **UX（LLM/MCP）** | MCP クライアント側（LLM）で逆引き・enrichment・free-text OR を実装する必要が生じる（方針(a)採択時）。方針(b)採択時は現行 tool シグネチャ維持可能 | 5 件特化の新 MCP tool 体系（未設計）。tool 数・引数が変わる可能性 | 残した X-Y 本の tool は現行シグネチャ維持。廃止した Z 本については LLM 側対応 or BFF 追加が必要 | 現行 12 tool シグネチャ変更なし。TODO 解消で Estimate/Order/Delivery/Receipt の enrichment が回復 |
| **技術的負債** | find 層関連 3,945 行削除 + TODO 8 箇所消滅 + E2E SKIP 70 件消滅 | 現 find 層削除 + 新 find 層の設計負債が発生 + E2E 再構築 | 残した部分の TODO を解消、削除部分の TODO/SKIP は消滅。残存部分の保守は継続 | TODO 8 箇所解消で現行負債を削減。3,945 行の保守対象は継続。E2E SKIP のうち環境依存 39 件は残存 |
| **MCP 影響** | tools.go 全 12 tool Handler 書換（方針(a)/(b)に応じて変動）。tool シグネチャ変更は MCP クライアント（LLM エージェント）の動作変更に直結 | tools.go を新 find 層に対応した全面書換。tool 数・引数が変わる場合、MCP クライアント側の prompt も更新が必要 | 廃止 tool は削除または書換。残存 tool は変更なし。tool 数が減ると MCP クライアントの tool 選択ロジックが変わる | tools.go 変更なし。12 tool のシグネチャおよび動作は現状維持 |
| **工数見積** | 3-5 日（削除 + MCP 書換） | 2-3 週（設計 + 実装 + E2E） | 1-2 週（選別 + 削除 + 部分書換） | 2-3 日（TODO 解消 + SKIP 仕分け） |
| **v0.7.0 リリース時期影響** | v0.7.0 に間に合う（3-5 日） | v0.8.0 送り可能性が高い（2-3 週） | v0.7.0 に間に合う可能性（1-2 週） | v0.7.0 に確実に間に合う（2-3 日） |

---

## §8 12 Find メソッド × 4 選択肢 扱い大表（48 セル）

> **記入規則**:
> 各セルには「削除」「維持」「再設計（新仕様）」「api 直呼びで代替可能」「TODO 解消して維持」「部分機能のみ維持」のいずれかを記入する。
> 形容詞は §10 禁止リスト参照。

| メソッド名 | A: 全廃止 | B: ゼロベース再設計 | C: 数本残す | D: 現状維持 + 負債解消 |
|---|---|---|---|---|
| `FindClient` | 削除（branches/contacts の joint 解決は api 直呼びで代替可能） | 再設計（新仕様）— enrichment + text-OR を 5 件特化層で再定義 | 維持（api 単独で逆引きを代替できないため AND 条件に合致する候補） | 維持（TODO なし、現状のまま存続） |
| `FindProject` | 削除（ClientName→ProjectID 逆引きは api 直呼びで代替可能） | 再設計（新仕様）— ClientName 逆引き + status post-filter を新仕様で再定義 | 維持（ClientName 逆引き + status post-filter が AND 条件に合致する候補） | 維持（TODO なし、現状のまま存続） |
| `FindEstimate` | 削除（response_group 経由の手順は api 直呼びで代替可能） | 再設計（新仕様）— response_group 経由 enrichment を 5 件特化の新仕様で再定義 | 部分機能のみ維持（ClientName/ProjectName 逆引きは維持候補、Status post-filter は TODO 解消条件付き） | TODO 解消して維持（enrichment 復元 + Status post-filter 復元） |
| `FindInvoice` | 削除（ClientName/ProjectName 逆引きは api 直呼びで代替可能） | 再設計（新仕様）— ClientName/ProjectName 逆引き + status post-filter を新仕様で再定義 | 維持（ClientName/ProjectName 逆引きが AND 条件に合致する候補） | 維持（TODO なし、現状のまま存続） |
| `FindOrder` | 削除（response_group 経由の手順は api 直呼びで代替可能） | 再設計（新仕様）— response_group 経由 enrichment を 5 件特化の新仕様で再定義 | 部分機能のみ維持（ClientName/ProjectName 逆引きは維持候補、Status post-filter は TODO 解消条件付き） | TODO 解消して維持（enrichment 復元 + Status post-filter 復元） |
| `FindDelivery` | 削除（response_group 経由の手順は api 直呼びで代替可能） | 再設計（新仕様）— response_group 経由 enrichment + Deliveries 複数形対応を新仕様で再定義 | 部分機能のみ維持（ClientName/ProjectName 逆引きは維持候補、Status post-filter は TODO 解消条件付き） | TODO 解消して維持（enrichment 復元 + Status post-filter 復元） |
| `FindReceipt` | 削除（response_group 経由の手順は api 直呼びで代替可能） | 再設計（新仕様）— response_group 経由 enrichment + Receipts 複数形対応を新仕様で再定義 | 部分機能のみ維持（ClientName/ProjectName 逆引きは維持候補、Status post-filter は TODO 解消条件付き） | TODO 解消して維持（enrichment 復元 + Status post-filter 復元） |
| `FindVendor` | 削除（branches/contacts の joint 解決は api 直呼びで代替可能） | 再設計（新仕様）— enrichment + text-OR を 5 件特化層で再定義 | 維持（api 単独で逆引きを代替できないため AND 条件に合致する候補） | 維持（TODO なし、現状のまま存続） |
| `FindPurchaseOrder` | 削除（VendorName/ProjectName 逆引きは api 直呼びで代替可能） | 再設計（新仕様）— VendorName 逆引き + status post-filter を 5 件特化層で再定義 | 維持（VendorName 逆引きが AND 条件に合致する候補） | 維持（TODO なし、現状のまま存続） |
| `FindPayment` | 削除（VendorName 逆引きは api 直呼びで代替可能） | 再設計（新仕様）— VendorName 逆引き + status post-filter を 5 件特化層で再定義 | 維持（VendorName 逆引きが AND 条件に合致する候補） | 維持（TODO なし、現状のまま存続） |
| `FindUser` | 削除（NameCont は api 直呼びで代替可能、text-OR は全件取得後の処理） | 再設計（新仕様）— UserEntity の text-OR 検索を新仕様で再定義（enrichment なし） | api 直呼びで代替可能（name/_cont のみで十分な場合、text-OR が不要なら削除候補） | 維持（TODO なし、現状のまま存続） |
| `FindGroup` | 削除（NameCont は api 直呼びで代替可能、text-OR は全件取得後の処理） | 再設計（新仕様）— GroupEntity の text-OR 検索を新仕様で再定義（enrichment なし） | api 直呼びで代替可能（name/_cont のみで十分な場合、text-OR が不要なら削除候補） | 維持（TODO なし、現状のまま存続） |

> **注記**: 本大表は各選択肢の処理内容を並列記述したものであり、セル数集計による優劣判断は推奨しない
> （メソッド重要度が均等ではないため §9 重要度スコア併用）。
> C 列の「維持」「api 直呼びで代替可能」の判断は §9 の AND 条件（api 層単独で代替不可能 AND MCP 実利用頻度上位 AND TODO 負債が小さい）に基づくものであり、現時点では仮記述である。
> 最終的な C 選定はユーザーレビュー（Step 7b）で確定する。

---

## §9 意思決定フレームワーク

### 9.1 6 評価軸の定義

| 軸 | 定義 | 測定方法 |
|---|---|---|
| **UX（LLM）** | MCP クライアント（LLM エージェント）が find_* ツールを呼ぶ際の複雑性。シグネチャが変わるほど LLM の tool 選択・引数設定の負荷が増加する | tool シグネチャの変更量（件数）で定量化 |
| **実装コスト** | N01 後の実装フェーズ（N02+）での工数と新規バグ混入リスク | 推定工数（日）で定量化 |
| **技術的負債解消** | TODO 8 箇所 / SKIP 70 件 / 3,945 行の削減量 | 削減行数・TODO 件数・SKIP 件数で定量化 |
| **MCP LLM 親和性** | tool の引数設計が LLM にとって直感的かどうか。「名前で検索できる」「結果に関連情報が付与される」が LLM の効率を上げる | tool 引数の種類（名前 vs ID、enriched 結果か否か）で評価 |
| **v0.7.0 以降の拡張性** | api 層に新リソースが追加された際に find 層の追従コストがどの程度発生するか | 「api 追加 1 件につき find 追従が必要か」で評価 |
| **v0.7.0 リリース時期影響** | 選択した方針が v0.7.0 リリーススケジュールに与える影響 | 工数推定から v0.7.0 への間に合う/遅延リスクを評価 |

### 9.2 C（数本残す）の選定 AND 条件

以下の 3 条件をすべて満たすメソッドを C 選択肢の「残す候補」とする:

1. **api 層単独で代替不可能**: 単純な Ransack フィルタでは実現できない処理（逆引き、複数 step、response_group 組み合わせ）を含む
2. **MCP 実利用頻度上位**: `find_clients` `find_projects` `find_invoices` `find_purchase_orders` 等、LLM エージェントが業務フローで高頻度で呼ぶと想定されるもの
3. **TODO 負債が小さい**: TODO(M25-M32) が残存しておらず、現状で機能が完全に動作するもの

条件 3 より、FindEstimate/FindOrder/FindDelivery/FindReceipt は TODO 解消前は「残す候補」の基準を満たさない。

### 9.3 12 メソッド重要度スコア

各メソッドを「MCP call 頻度の想定 / 代替困難度 / 実利用可能性」の 3 軸で 1-3 点評価。

> **注記**: 以下のスコアは実際の MCP 利用ログが存在しないため、
> コードベース調査と BOARD API の用途から推定した **仮スコア** である。
> 実際の重み付けはユーザー（Step 7b）が確定する。

| メソッド名 | MCP 頻度想定 (1-3) | 代替困難度 (1-3) | 実利用可能性 (1-3) | 合計 | TODO |
|---|---|---|---|---|---|
| `FindClient` | 3（顧客検索は基本操作） | 2（branches/contacts 付与は find でしか提供されない） | 3（TODO なし、完全動作） | 8 | なし |
| `FindProject` | 3（案件検索は中心） | 3（ClientName 逆引き + status post-filter の組み合わせ） | 3（TODO なし、完全動作） | 9 | なし |
| `FindEstimate` | 2（見積検索は中程度） | 3（response_group 経由 3 step） | 1（TODO あり、enrichment/status 無効） | 6 | あり |
| `FindInvoice` | 3（請求書検索は頻繁） | 2（ClientName/ProjectName 逆引き） | 3（TODO なし、完全動作） | 8 | なし |
| `FindOrder` | 2（受注書検索は中程度） | 3（response_group 経由 3 step） | 1（TODO あり、enrichment/status 無効） | 6 | あり |
| `FindDelivery` | 1（納品書検索は低頻度） | 3（response_group 経由 3 step、複数形対応） | 1（TODO あり、enrichment/status 無効） | 5 | あり |
| `FindReceipt` | 1（領収書検索は低頻度） | 3（response_group 経由 3 step、複数形対応） | 1（TODO あり、enrichment/status 無効） | 5 | あり |
| `FindVendor` | 2（仕入先検索は中程度） | 2（branches/contacts 付与は find でしか提供されない） | 3（TODO なし、完全動作） | 7 | なし |
| `FindPurchaseOrder` | 2（発注書検索は中程度） | 2（VendorName/ProjectName 逆引き） | 3（TODO なし、完全動作） | 7 | なし |
| `FindPayment` | 2（支払検索は中程度） | 2（VendorName 逆引き） | 3（TODO なし、完全動作） | 7 | なし |
| `FindUser` | 2（ユーザー検索は中程度） | 1（NameCont が api 層で代替可能） | 3（TODO なし、完全動作） | 6 | なし |
| `FindGroup` | 1（グループ検索は低頻度） | 1（NameCont が api 層で代替可能） | 3（TODO なし、完全動作） | 5 | なし |

---

## §10 フラット評価の立場表明 + 形容詞禁止リスト

### 10.1 フラット評価の立場宣言

**本レポートは A/B/C/D のいずれを推奨しない。**

本レポートの目的は「一次情報の転記」と「評価軸の定義」であり、最終的な A/B/C/D の選択は
ユーザー（プロジェクトオーナー）が Step 7b で行う。

本レポートを読む第三者は、§7/§8 のセル記述が「事実（処理内容）」のみで構成されており、
極性形容詞を含まないことを確認できる（§10.3 禁止語 grep 参照）。

### 10.2「全廃棄+ゼロベース前提」発言との整合

ユーザーは「全廃棄 + ゼロベース前提で問い直したい」と表明しているが、同時にフラット評価も要求している。
この整合を以下のように解決した:

- **A（全廃止）** と **B（ゼロベース再設計）** はユーザー発言の字義通りの解釈を反映する選択肢
- **C（数本残す）** は現実的な落としどころとして中間的な選択肢
- **D（現状維持 + 負債解消）** は弁証法レビューで追加された対抗馬。「ゼロベースで問い直した結果、現状が最善」という結論も評価の選択肢として対等に扱う

4 選択肢を対等に並べることで、「全廃棄を前提にしない」フラット評価の手続的公正性を担保する。

### 10.3 形容詞禁止リスト（機械 grep 用）

以下の語は本レポートの §1-§9 本文で使用を禁止する。
（本節 §10.3 内でのリスト明示は禁止の対象外とする）

```
禁止語リスト（grep 検索用）:
  劣化 / 優秀 / 中途半端 / 理想的 / 改善見込み / 有利 / 不利 / 望ましい / 望ましくない / お勧め / 非推奨
```

**機械 grep コマンド**（§1-§9 本文での出現が 0 件であることを確認）:
```bash
grep -nE "(劣化|優秀|中途半端|理想的|改善見込み|有利|不利|望ましい|望ましくない|お勧め|非推奨)" \
  plans/board-phase-n-m01-find-rationale.md
# 期待: §10.3 の禁止語リスト記述行のみヒット、他は 0 件
```

### 10.4 第三者半盲検チェック手続き

**目的**: 本レポートが特定選択肢に偏った記述をしていないかを検証する。

**手順**:
1. 別セッションの Claude Code または別 LLM（web 版 ChatGPT / Gemini / 別プロファイルの Claude 等）に
   本レポートの §1-§9 を貼り付ける
2. 「このレポートは A/B/C/D のうちどれを推していると読めるか」を質問する
3. 回答が「特定選択肢に偏らない」または「判断できない」であることを確認する

**注記**: 現セッションの advisor はこの会話の全履歴（D 追加決定 + 計画書の全内容）を参照するため、
現セッション内での advisor 呼び出しは真の盲検にならない。
本チェックは「偏り検知のヒント」として扱い、**半盲検** として位置づける。

---

*本レポートは Step 7a（事実確認レビュー）待ちの状態で提出する。*
*§2-§6 の一次情報に事実誤認があればユーザーに指摘を求め、Step 7b（評価軸重み付け + 選択）は*
*別セッションで明示的に開始された後に実施する。*
